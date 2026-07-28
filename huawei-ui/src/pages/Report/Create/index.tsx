import React, { useState, useEffect } from 'react';
import { Table, Button, Input, InputNumber, Form, Row, Col, Card, Descriptions, Tag, Select, App, Divider, Space, message, Modal, Drawer, List, Spin, DatePicker, Radio, Alert, Checkbox } from 'antd';
import { ArrowDownOutlined, ArrowUpOutlined, FileTextOutlined, CalculatorOutlined, EditOutlined, PlusOutlined, DownloadOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from '@umijs/max';
import { getSamples, generateReport, getPatientResultsCompare, listModels, updateSampleGeneData, calculateModelFormula, getModelGeneThresholds, updateResultSignalValue, getTemplates, createTemplate } from '@/services/api';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer } from 'recharts';
import { formatReportProject } from '@/utils/reportProject';

const Create: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const [sample, setSample] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [compareData, setCompareData] = useState<any>(null);
  const [compareLoading, setCompareLoading] = useState(false);
  const [models, setModels] = useState<any[]>([]);
  const [selectedModel, setSelectedModel] = useState<any>(null);
  const [calculationResult, setCalculationResult] = useState<number | null>(null);
  const [calculationLoading, setCalculationLoading] = useState(false);
  const [modelsLoading, setModelsLoading] = useState(false);
  const [resultExplanation, setResultExplanation] = useState<string>('');
  const [signalValueExplanation, setSignalValueExplanation] = useState<string>('');
  const [organization, setOrganization] = useState<string>('个人送检');
  const [currentRemarks, setCurrentRemarks] = useState<string>('');
  const [reportType, setReportType] = useState<'normal' | 'high' | 'screening'>('normal');
  const [thresholds, setThresholds] = useState<Record<string, number>>({});
  const [thresholdsLoading, setThresholdsLoading] = useState(false);
  const [resultId, setResultId] = useState<number | null>(null);
  const [isEditingCalculation, setIsEditingCalculation] = useState(false);
  const [editCalculationValue, setEditCalculationValue] = useState<number | null>(null);
  const [historicalReports, setHistoricalReports] = useState<any[]>([]);
  const [selectedHistoricalReports, setSelectedHistoricalReports] = useState<React.Key[]>([]);
  const [historySignalOverrides, setHistorySignalOverrides] = useState<Record<string, number>>({});
  const [historicalReportsLoading, setHistoricalReportsLoading] = useState<boolean>(false);
  const [manualHistoryModalVisible, setManualHistoryModalVisible] = useState<boolean>(false);
  const [manualHistoryForm] = Form.useForm();
  const [templateDrawerVisible, setTemplateDrawerVisible] = useState<boolean>(false);
  const [currentTemplateType, setCurrentTemplateType] = useState<string>('');
  const [templates, setTemplates] = useState<any[]>([]);
  const [createTemplateModalVisible, setCreateTemplateModalVisible] = useState<boolean>(false);
  const [newTemplate, setNewTemplate] = useState<{ title: string; content: string }>({ title: '', content: '' });
  const [primarySampleId, setPrimarySampleId] = useState<number | null>(null);
  const [mergePromptVisible, setMergePromptVisible] = useState(false);
  const [mergeSamePatientSamples, setMergeSamePatientSamples] = useState(true);
  const [existingReportId, setExistingReportId] = useState<number | null>(null);
  const [testPdfLoading, setTestPdfLoading] = useState(false);
  // 历史样本数据
  const [historicalSamples, setHistoricalSamples] = useState<any[]>([]);
  const [selectedHistoricalSamples, setSelectedHistoricalSamples] = useState<number[]>([]);
  const [historicalSamplesLoading, setHistoricalSamplesLoading] = useState<boolean>(false);
  const { message: appMessage } = App.useApp();
  const navigate = useNavigate();

  const formatDateToYYYYMMDD = (dateString: string): string => {
    if (!dateString) return '';
    try {
      const date = new Date(dateString);
      if (!isNaN(date.getTime())) {
        return date.toISOString().split('T')[0];
      }
      if (dateString.includes('T')) {
        return dateString.split('T')[0];
      }
      return dateString;
    } catch (error) {
      if (dateString.includes('T')) {
        return dateString.split('T')[0];
      }
      return dateString;
    }
  };

  const parseMaybeJson = (value: any) => {
    if (!value) return null;
    if (typeof value === 'object') return value;
    try {
      return JSON.parse(value);
    } catch (error) {
      return null;
    }
  };

  const getHistoryKey = (record: any): React.Key => {
    const source = record?.source || 'report';
    return `${source}-${record?.id ?? record?.sampleId ?? record?.sampleCode ?? record?.sample_code}`;
  };

  const getHistoryDateValue = (record: any): string => {
    return record?.receive_date || record?.receiveDate || record?.sampleReceivedAt || record?.createdAt || record?.created_at || record?.generatedTime || record?.generated_time || record?.testDate || record?.test_date || record?.testTime || record?.test_time || record?.collection_date || record?.collectionDate || record?.time || '';
  };

  const getHistorySignalValue = (record: any): number => {
    const directValue = record?.signalValue ?? record?.signal_value ?? record?.signal ?? record?.calculationResult ?? record?.calculation_result;
    const parsedDirect = Number(directValue);
    if (!isNaN(parsedDirect) && directValue !== undefined && directValue !== null && directValue !== '') {
      return parsedDirect;
    }

    const resultData = parseMaybeJson(record?.result_data || record?.resultData || record?.report_data || record?.reportData);
    const resultSignal = resultData?.signalValue ?? resultData?.signal_value ?? resultData?.signal ?? resultData?.calculationResult ?? resultData?.CalculationResult ?? resultData?.score;
    const parsedResultSignal = Number(resultSignal);
    return !isNaN(parsedResultSignal) ? parsedResultSignal : 0;
  };

  const getHistoryTimestamp = (record: any): number => {
    const value = getHistoryDateValue(record);
    const timestamp = value ? new Date(value).getTime() : 0;
    return Number.isFinite(timestamp) ? timestamp : 0;
  };

  const calculateAdjacentTrend = (previousSignal: number, currentSignal: number): string => {
    if (!Number.isFinite(previousSignal) || !Number.isFinite(currentSignal)) return '-';
    if (currentSignal > previousSignal) return '↑';
    if (currentSignal < previousSignal) return '↓';
    return '-';
  };

  const normalizeReportHistoryRecord = (record: any) => {
    const historyKey = getHistoryKey(record);
    const overrideValue = historySignalOverrides[String(historyKey)];
    return {
      ...record,
      historyKey,
      sampleCode: record?.sampleCode || record?.sample_code || record?.sampleId || record?.sample_id || '未知样本',
      createdAt: getHistoryDateValue(record),
      signalValue: Number.isFinite(Number(overrideValue)) ? Number(overrideValue) : getHistorySignalValue(record),
      treatmentStageName: record?.treatmentStageName || record?.treatment_stage_name || record?.type || '',
      trend: record?.trend || '',
      remarks: record?.remarks || record?.note || '',
    };
  };

  const getSampleTreatmentStageName = (record: any): string => {
    return String(record?.treatmentStageName || record?.treatment_stage_name || record?.type || '').trim();
  };

  const getSampleTypeName = (record: any): string => {
    return String(record?.sampleType || record?.sampleTypeName || record?.sample_type_name || '').trim();
  };

  const normalizeSampleRecord = (record: any) => {
    if (!record || typeof record !== 'object') return record;
    const treatmentStageName = getSampleTreatmentStageName(record);
    const sampleTypeName = getSampleTypeName(record);
    const samePatientSamples = Array.isArray(record.samePatientSamples)
      ? record.samePatientSamples
      : (Array.isArray(record.same_patient_batch_samples) ? record.same_patient_batch_samples : []);
    const normalizedSamePatientSamples = samePatientSamples.map((item: any) => {
      const itemTreatmentStageName = getSampleTreatmentStageName(item);
      const itemSampleTypeName = getSampleTypeName(item);
      return {
        ...item,
        sampleCode: item.sampleCode || item.sample_code,
        sample_code: item.sample_code || item.sampleCode,
        treatmentStageName: itemTreatmentStageName,
        treatment_stage_name: itemTreatmentStageName,
        sampleType: item.sampleType || item.sampleTypeName || itemSampleTypeName,
        sample_type_name: item.sample_type_name || itemSampleTypeName,
      };
    });
    return {
      ...record,
      sampleCode: record.sampleCode || record.sample_code,
      sample_code: record.sample_code || record.sampleCode,
      treatmentStageName,
      treatment_stage_name: treatmentStageName,
      sampleType: record.sampleType || record.sampleTypeName || sampleTypeName,
      sample_type_name: record.sample_type_name || sampleTypeName,
      samePatientSamples: normalizedSamePatientSamples,
      same_patient_batch_samples: normalizedSamePatientSamples,
    };
  };

  // 获取患者历史检测样本
  const fetchHistoricalSamples = async (patientId: number, currentSample?: any) => {
    setHistoricalSamplesLoading(true);
    try {
      const response = await fetch(`/api/results/patient/${patientId}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const result = await response.json();
      let samples: any[] = [];
      if (result.code === 200 && result.data && result.data.results) {
        samples = Array.isArray(result.data.results) ? result.data.results : [];
        // 过滤掉当前样本，避免与本次表达值重复
        // 使用传递的 currentSample 或状态中的 sample
        const sampleToUse = currentSample || sample;
        if (sampleToUse) {
          const currentSampleCode = sampleToUse.sample_code || sampleToUse.sampleCode;
          if (currentSampleCode) {
            samples = samples.filter((item: any) => {
              // 使用 sampleCode 进行比较，确保过滤掉所有与当前样本编号相同的历史样本
              const itemSampleCode = item.sampleCode || item.sample_code;
              return itemSampleCode !== currentSampleCode;
            });
          }
        }
      }
      
      // 不添加当前样本到历史样本列表，避免与本次表达值重复
      
      // 按时间顺序排序样本
      const sortedSamples = Array.isArray(samples) ? samples.sort((a: any, b: any) => {
        try {
          const dateA = new Date(getHistoryDateValue(a));
          const dateB = new Date(getHistoryDateValue(b));
          return dateB.getTime() - dateA.getTime(); // 降序排序，最新的在前
        } catch (error) {
          return 0;
        }
      }) : [];
      
      setHistoricalSamples(sortedSamples.slice().sort((a: any, b: any) => getHistoryTimestamp(a) - getHistoryTimestamp(b)));
    } catch (error) {
      console.error('获取历史样本失败:', error);
      setHistoricalSamples([]);
    } finally {
      setHistoricalSamplesLoading(false);
    }
  };

  // 工具函数：根据身份证号计算年龄
  const calculateAge = (idCard?: string): string => {
    if (!idCard || idCard.length < 18) {
      return '-';
    }
    
    // 从身份证号中提取出生年月日
    const year = parseInt(idCard.substring(6, 10));
    const month = parseInt(idCard.substring(10, 12));
    const day = parseInt(idCard.substring(12, 14));
    
    // 获取当前日期
    const now = new Date();
    const currentYear = now.getFullYear();
    const currentMonth = now.getMonth() + 1;
    const currentDay = now.getDate();
    
    // 计算年龄
    let age = currentYear - year;
    
    // 调整年龄：如果当前月份小于出生月份，或者月份相同但日期小于出生日期，则年龄减1
    if (currentMonth < month || (currentMonth === month && currentDay < day)) {
      age--;
    }
    
    return age.toString();
  };

  const normalizeText = (value: any) => String(value || '').trim().toLowerCase();

  const normalizeTreatmentStageName = (stageName?: string) => {
    const stage = String(stageName || '').trim();
    if (stage === '健康筛查') return '健康体检';
    if (stage === '术后' || stage === '手术后') return '术后检测';
    if (stage === '残留检测（术前中后）') return '残留检测';
    return stage;
  };

  const isPreoperativeStage = (stageName?: string) => normalizeTreatmentStageName(stageName).includes('术前');

  const treatmentStageRank = (stageName?: string) => {
    const order = ['健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'];
    const index = order.indexOf(normalizeTreatmentStageName(stageName));
    return index >= 0 ? index : order.length;
  };

  const pickReportIssueSample = (samples: any[] = []) => {
    return [...samples].sort((a: any, b: any) => {
      const rankDiff = treatmentStageRank(b.treatmentStageName || b.treatment_stage_name) - treatmentStageRank(a.treatmentStageName || a.treatment_stage_name);
      if (rankDiff !== 0) return rankDiff;
      return String(b.sampleCode || b.sample_code || '').localeCompare(String(a.sampleCode || a.sample_code || ''));
    })[0];
  };

  const getModelGenes = (model: any): string[] => {
    if (Array.isArray(model?.geneSymbols)) return model.geneSymbols.filter(Boolean);
    if (typeof model?.genes === 'string' && model.genes.trim()) {
      return model.genes.split(',').map((gene: string) => gene.trim()).filter(Boolean);
    }
    return [];
  };

  const getBestModelForSample = (sampleData: any, modelList: any[]) => {
    const sampleGenes = Object.keys(sampleData?.gene_data || {}).filter((gene) => gene && gene !== 'sqrt');
    const sampleGeneSet = new Set(sampleGenes.map(normalizeText));
    const sampleCancerTypeName = sampleData?.cancer_type_name || sampleData?.cancerTypeName;
    const sampleCancerTypeId = Number(sampleData?.cancer_type_id || sampleData?.cancerTypeId || 0);

    const savedModelId = Number(sampleData?.model_id || sampleData?.modelId || 0);
    const candidates = modelList
      .filter((model) => Number(model?.isActive ?? model?.is_active) === 1 && !Number(model?.isDeprecated ?? model?.is_deprecated))
      .map((model) => {
        const modelGenes = getModelGenes(model);
        const missingModelGenes = modelGenes.filter((gene) => !sampleGeneSet.has(normalizeText(gene)));
        const selectable = modelGenes.length === 0 || missingModelGenes.length === 0;
        const exactCancerTypeMatched = sampleCancerTypeId > 0
          ? Number(model?.cancerTypeId || model?.cancer_type_id || 0) === sampleCancerTypeId
          : Boolean(sampleCancerTypeName) && normalizeText(model?.cancerTypeName) === normalizeText(sampleCancerTypeName);
        const looseCancerTypeMatched = !sampleCancerTypeName
          || Number(model?.cancerTypeId || model?.cancer_type_id || 0) === sampleCancerTypeId
          || normalizeText(model?.cancerTypeName) === normalizeText(sampleCancerTypeName)
          || (Array.isArray(model?.applicableItems) && model.applicableItems.some((item: string) => normalizeText(item) === normalizeText(sampleCancerTypeName)));
        const extraGeneCount = modelGenes.filter((gene) => !sampleGeneSet.has(normalizeText(gene))).length;
        return { model, selectable, exactCancerTypeMatched, looseCancerTypeMatched, extraGeneCount };
      })
      .filter((item) => item.selectable);

    const savedModel = candidates.find((item) => (
      Number(item.model?.id) === savedModelId
      && (!sampleCancerTypeName && sampleCancerTypeId <= 0 || item.exactCancerTypeMatched)
    ));
    if (savedModel) return savedModel.model;

    return candidates
      .sort((a, b) => {
        const exactCancerDiff = Number(b.exactCancerTypeMatched) - Number(a.exactCancerTypeMatched);
        if (exactCancerDiff) return exactCancerDiff;
        const looseCancerDiff = Number(b.looseCancerTypeMatched) - Number(a.looseCancerTypeMatched);
        if (looseCancerDiff) return looseCancerDiff;
        if (a.extraGeneCount !== b.extraGeneCount) return a.extraGeneCount - b.extraGeneCount;
        return getModelGenes(b.model).length - getModelGenes(a.model).length;
      })[0]?.model;
  };

  const fetchSample = async () => {
    if (id) {
      setLoading(true);
      try {
        const response = await getSamples({ id });
        if (response.data && response.data.list && response.data.list.length > 0) {
          const sampleData = normalizeSampleRecord(response.data.list[0]);
          setSample(sampleData);
          setPrimarySampleId(Number(sampleData.id));
          const samePatientSamples = sampleData.samePatientSamples || sampleData.same_patient_batch_samples || [];
          if (Array.isArray(samePatientSamples) && samePatientSamples.length > 1) {
            const issueSample = pickReportIssueSample(samePatientSamples);
            if (issueSample && Number(issueSample.id) && Number(issueSample.id) !== Number(sampleData.id)) {
              navigate(`/report/create/${issueSample.id}`);
              return;
            }
          }
          if (Array.isArray(samePatientSamples) && samePatientSamples.length > 1) {
            setMergePromptVisible(true);
          }
          if (sampleData.patient_id) {
            fetchCompareData(sampleData.patient_id);
            fetchHistoricalReports(sampleData.patient_id, sampleData.id);
            // 传递 sampleData 给 fetchHistoricalSamples，确保过滤逻辑能够立即执行
            fetchHistoricalSamples(Number(sampleData.patient_id), sampleData);
          }
          fetchModels();
          // 获取样本对应的结果ID
          try {
            const resultsResponse = await fetch(`/api/results?sample_id=${id}`);
            if (resultsResponse.ok) {
              const resultsData = await resultsResponse.json();
              if (resultsData.data && resultsData.data.list && resultsData.data.list.length > 0) {
                setResultId(resultsData.data.list[0].id);
              }
            }
          } catch (error) {
            console.error('获取结果ID失败:', error);
          }
          // 从样本数据中获取送检单位
          if (sampleData.organization) {
            setOrganization(sampleData.organization);
          }
          try {
            const reportResponse = await fetch(`/api/reports/${encodeURIComponent(sampleData.sample_code || sampleData.sampleCode || id)}`);
            if (reportResponse.ok) {
              const reportResult = await reportResponse.json();
              const reportId = Number(reportResult?.data?.id || 0);
              setExistingReportId(reportId || null);
            } else {
              setExistingReportId(null);
            }
          } catch (_error) {
            setExistingReportId(null);
          }
        } else {
          appMessage.error('获取样本信息失败');
        }
      } catch (_error) {
        appMessage.error('获取样本信息失败');
      } finally {
        setLoading(false);
      }
    }
  };

  useEffect(() => {
    fetchSample();
  }, [id]);

  useEffect(() => {
    if (!sample || !models.length || selectedModel) return;
    const defaultModel = getBestModelForSample(sample, models);
    if (defaultModel) {
      setSelectedModel(defaultModel);
    }
  }, [sample, models, selectedModel]);

  const fetchCompareData = async (patientId: string) => {
    setCompareLoading(true);
    try {
      const response = await getPatientResultsCompare(patientId);
      if (response.data) {
        setCompareData(response.data);
      } else {
        appMessage.error('获取患者历史结果失败');
        setCompareData(null);
      }
    } catch (_error) {
      appMessage.error('获取患者历史结果失败');
      setCompareData(null);
    } finally {
      setCompareLoading(false);
    }
  };

  const fetchModels = async () => {
    setModelsLoading(true);
    try {
      const response = await listModels();
      if (response.data) {
        setModels(response.data);
      } else {
        appMessage.error('获取模型列表失败');
      }
    } catch (_error) {
      appMessage.error('获取模型列表失败');
    } finally {
      setModelsLoading(false);
    }
  };

  const fetchThresholds = async (modelId?: number) => {
    if (!modelId) {
      setThresholds({});
      return;
    }
    setThresholdsLoading(true);
    try {
      const response = await getModelGeneThresholds(modelId);
      if (response.data) {
        const thresholdMap: Record<string, number> = {};
        response.data.forEach((gene: any) => {
          const threshold = Number(gene.threshold || 0);
          if (gene.geneSymbol) thresholdMap[gene.geneSymbol] = threshold;
          if (gene.geneName) thresholdMap[gene.geneName] = threshold;
          if (gene.name) thresholdMap[gene.name] = threshold;
        });
        setThresholds(thresholdMap);
      } else {
        appMessage.error('获取模型阈值失败');
      }
    } catch (_error) {
      appMessage.error('获取模型阈值失败');
    } finally {
      setThresholdsLoading(false);
    }
  };

  useEffect(() => {
    fetchThresholds(selectedModel?.id);
  }, [selectedModel?.id]);

  const fetchHistoricalReports = async (patientId: number, currentSampleId?: number) => {
    setHistoricalReportsLoading(true);
    try {
      const params = new URLSearchParams({ limit: '50' });
      if (currentSampleId) {
        params.set('exclude_sample_id', String(currentSampleId));
      }
      const response = await fetch(`/api/reports/patient/${patientId}?${params.toString()}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        // 检查返回数据格式，后端可能直接返回数组或包装在data字段中
        let reportsData: any[] = [];
        if (Array.isArray(result.data)) {
          reportsData = result.data;
        } else if (Array.isArray(result)) {
          // 后端直接返回数组的情况
          reportsData = result;
        }
        
        if (Array.isArray(reportsData) && reportsData.length > 0) {
          // 解析历史报告数据，优先使用API直接返回的字段，其次从report_data中提取
          const parsedReports = reportsData.map((report: any) => {
            let reportData = {};
            try {
              if (report.report_data) {
                reportData = JSON.parse(report.report_data);
              }
            } catch (error) {
              console.error('解析历史报告数据失败:', error);
            }
            return {
              id: report.id,
              sampleCode: report.sampleCode,
              createdAt: report.created_at || report.generated_time || report.createdAt || report.generatedTime || '',
              signalValue: report.signalValue || (reportData as any).CalculationResult || (reportData as any).calculationResult || 0,
              trend: (reportData as any).trend || '',
              treatmentStageName: report.treatmentStageName || (reportData as any).treatmentStageName || '',
              remarks: report.remarks || (reportData as any).remarks || '',
              source: 'report',
            };
          });
          setHistoricalReports(parsedReports.slice().sort((a: any, b: any) => getHistoryTimestamp(a) - getHistoryTimestamp(b)));
        } else {
          // 无历史报告时设置为空数组
          setHistoricalReports([]);
        }
        setSelectedHistoricalReports([]);
      } else {
        appMessage.error('获取历史报告失败');
        setHistoricalReports([]);
        setSelectedHistoricalReports([]);
      }
    } catch (_error) {
      appMessage.error('获取历史报告失败');
      setHistoricalReports([]);
      setSelectedHistoricalReports([]);
    } finally {
      setHistoricalReportsLoading(false);
    }
  };

  const handleAddManualHistory = async () => {
    try {
      const values = await manualHistoryForm.validateFields();
      const createdAt = values.createdAt ? values.createdAt.format('YYYY-MM-DD') : '';
      const record = {
        id: -Date.now(),
        sampleCode: values.sampleCode || '手动添加',
        createdAt,
        signalValue: Number(values.signalValue || 0),
        trend: values.trend || '',
        treatmentStageName: values.treatmentStageName || '',
        remarks: values.remarks || '',
        source: 'manual',
      };
      setHistoricalReports((prev) => [...prev, record].sort((a: any, b: any) => getHistoryTimestamp(a) - getHistoryTimestamp(b)));
      setSelectedHistoricalReports((prev) => [...prev, getHistoryKey(record)].slice(0, 3));
      manualHistoryForm.resetFields();
      setManualHistoryModalVisible(false);
      appMessage.success('已添加历史记录');
    } catch (_error) {
      // 表单校验失败时保持弹窗打开
    }
  };

  const fetchTemplates = async (templateType: string) => {
    try {
      const response = await getTemplates({ type: templateType });
      if (response.data) {
        setTemplates(response.data.list || []);
      } else {
        appMessage.error('获取模板列表失败');
      }
    } catch (_error) {
      appMessage.error('获取模板列表失败');
    }
  };

  const handleOpenTemplateDrawer = (templateType: string) => {
    setCurrentTemplateType(templateType);
    setTemplateDrawerVisible(true);
    fetchTemplates(templateType);
  };

  const handleCreateTemplate = async () => {
    try {
      const response = await createTemplate({
        title: newTemplate.title,
        content: newTemplate.content,
        type: currentTemplateType
      });
      if (response.data) {
        appMessage.success('创建模板成功');
        setCreateTemplateModalVisible(false);
        setNewTemplate({ title: '', content: '' });
        fetchTemplates(currentTemplateType);
      } else {
        appMessage.error('创建模板失败');
      }
    } catch (_error) {
      appMessage.error('创建模板失败');
    }
  };

  const handleUseTemplate = (template: any) => {
    if (currentTemplateType === 'result_explanation') {
      setResultExplanation(template.content);
    } else if (currentTemplateType === 'signal_explanation') {
      setSignalValueExplanation(template.content);
    }
    setTemplateDrawerVisible(false);
    appMessage.success('使用模板成功');
  };

  const calculateByModel = async () => {
    if (!sample || !selectedModel) {
      appMessage.warning('请选择模型');
      return;
    }
    
    if (!sample.gene_data) {
      appMessage.warning('样本无基因表达数据');
      return;
    }
    
    setCalculationLoading(true);
    try {
      // 从样本中获取基因值
      const geneData = sample.gene_data;
      
      // 转换基因数据为数字类型
      const numericGeneData: any = {};
      for (const [gene, value] of Object.entries(geneData)) {
        numericGeneData[gene] = parseFloat(value as string) || 0;
      }
      
      // 使用模型公式接口计算结果，阈值由后端按当前模型读取 setting_model_gene_threshold。
      const response = await calculateModelFormula({
        modelId: selectedModel.id,
        geneData: numericGeneData,
      });
      const result = response.data?.results?.[0];
      if (result && result.score !== undefined) {
        // 四舍五入计算结果
        const roundedResult = Math.round(Number(result.score || 0) * 10) / 10;
        setCalculationResult(roundedResult);
        
        // 更新数据库中的signalvalue字段
        if (resultId) {
          try {
            await updateResultSignalValue(resultId, roundedResult);
            appMessage.success('计算完成');
          } catch (error) {
            console.error('更新信号值失败:', error);
            appMessage.success('更新数据库失败');
          }
        } else {
          appMessage.success('计算完成');
        }
      } else {
        appMessage.error('计算失败');
      }
    } catch (error: any) {
      appMessage.error(`计算失败：${error.message || '未知错误'}`);
    } finally {
      setCalculationLoading(false);
    }
  };

  const handleEditCalculation = () => {
    if (calculationResult !== null) {
      setEditCalculationValue(calculationResult);
      setIsEditingCalculation(true);
    }
  };

  const handleSaveCalculation = async () => {
    if (editCalculationValue !== null && resultId) {
      try {
        // 四舍五入编辑后的值
        const roundedValue = Math.round(editCalculationValue * 10) / 10;
        // 更新状态
        setCalculationResult(roundedValue);
        // 更新数据库
        await updateResultSignalValue(resultId, roundedValue);
        appMessage.success('计算结果已更新到数据库');
        setIsEditingCalculation(false);
      } catch (error) {
        console.error('更新计算结果失败:', error);
        appMessage.error('更新计算结果失败');
      }
    } else {
      setIsEditingCalculation(false);
    }
  };

  const handleCancelEdit = () => {
    setIsEditingCalculation(false);
    setEditCalculationValue(null);
  };

  const handleSubmit = async () => {
    if (existingReportId) {
      appMessage.info('该样本已生成报告，正在打开报告详情');
      navigate(`/report/view/${encodeURIComponent(sample?.sample_code || sample?.sampleCode || id || '')}`);
      return;
    }
    if (!canGenerateReport) {
      appMessage.warning('只有批次提交后才能生成报告');
      return;
    }
    // 验证必填字段
    if (!resultExplanation.trim()) {
      appMessage.warning('请填写结果说明');
      return;
    }
    
    if (!signalValueExplanation.trim()) {
      appMessage.warning('请填写信号值说明');
      return;
    }

    try {
      // 准备历史报告数据（TIME, SIGNAL, TREND, TYPE, NOTE）
      let time1 = formatDateToYYYYMMDD(sample.receive_date || sample.collection_date || '');
      let signal1 = calculationResult || 0;
      let trend1 = reportTimeline[0]?.trend || '-';
      let type1 = getSampleTreatmentStageName(primarySample || sample);
      let note1 = currentRemarks.trim();
      
      let time2 = '';
      let signal2 = 0;
      let trend2 = '';
      let type2 = '';
      let note2 = '';
      
      let time3 = '';
      let signal3 = 0;
      let trend3 = '';
      let type3 = '';
      let note3 = '';
      
      let time4 = '';
      let signal4 = 0;
      let trend4 = '';
      let type4 = '';
      let note4 = '';
      
      const selectedHistoryData = orderedSelectedHistoricalRecordsWithTrend.map((record) => ({
        time: formatDateToYYYYMMDD(record.createdAt || ''),
        signal: Number(record.signalValue || 0),
        type: record.treatmentStageName || '',
        trend: record.trend || '-',
        note: record.remarks || '',
      }));

      if (selectedHistoryData[0]) {
        time2 = selectedHistoryData[0].time;
        signal2 = selectedHistoryData[0].signal;
        trend2 = selectedHistoryData[0].trend;
        type2 = selectedHistoryData[0].type;
        note2 = selectedHistoryData[0].note;
      }

      if (selectedHistoryData[1]) {
        time3 = selectedHistoryData[1].time;
        signal3 = selectedHistoryData[1].signal;
        trend3 = selectedHistoryData[1].trend;
        type3 = selectedHistoryData[1].type;
        note3 = selectedHistoryData[1].note;
      }

      if (selectedHistoryData[2]) {
        time4 = selectedHistoryData[2].time;
        signal4 = selectedHistoryData[2].signal;
        trend4 = selectedHistoryData[2].trend;
        type4 = selectedHistoryData[2].type;
        note4 = selectedHistoryData[2].note;
      }
      
      // 准备历史报告数据，只包含选中的历史检测，不包含本次
      const historicalReportsData = selectedHistoryData.length > 0 ? selectedHistoryData : [
        {
          time: time2,
          signal: signal2,
          type: type2,
          trend: trend2,
          note: note2
        },
        {
          time: time3,
          signal: signal3,
          type: type3,
          trend: trend3,
          note: note3
        },
        {
          time: time4,
          signal: signal4,
          type: type4,
          trend: trend4,
          note: note4
        },
        {
          time: '',
          signal: 0,
          type: '',
          trend: '',
          note: ''
        }
      ];
      
      // 准备报告数据，包含计算结果
      const reportData = {
        sampleId: effectivePrimarySampleId,
        primarySampleId: effectivePrimarySampleId,
        secondarySampleIds,
        reportType,
        calculationResult: calculationResult,
        selectedModelId: selectedModel?.id,
        geneData: sample.gene_data,
        resultExplanation: resultExplanation,
        signalValueExplanation: signalValueExplanation,
        selectedHistoricalReports: historicalReportsData,
        time1: time1,
        signal1: signal1,
        trend1: trend1,
        type1: type1,
        note1: note1,
        time2: time2,
        signal2: signal2,
        trend2: trend2,
        type2: type2,
        note2: note2,
        time3: time3,
        signal3: signal3,
        trend3: trend3,
        type3: type3,
        note3: note3,
        time4: time4,
        signal4: signal4,
        trend4: trend4,
        type4: type4,
        note4: note4,
        treatmentStageName: getSampleTreatmentStageName(primarySample || sample),
        sampleType: getSampleTypeName(primarySample || sample),
        SampleTimedata: sample.receive_date || sample.collection_date || '',
        remarks: currentRemarks.trim(),
        trend: trend1,
        organization: organization
      };
      
      console.log('提交的报告数据:', reportData);
      const response = await generateReport(reportData);
      if (response.data) {
        // 导航到新的报告页面
        navigate(`/report/view/${encodeURIComponent(response.data.sampleCode || primarySample?.sampleCode || primarySample?.sample_code || sample?.sample_code || sample?.sampleCode || '')}`);
      } else {
        appMessage.error('报告生成失败');
      }
    } catch (_error) {
      appMessage.error('报告生成失败');
    }
  };

  const handleDownloadConciseTestPdf = async () => {
    try {
      setTestPdfLoading(true);
      const sampleTypeId = Number(sample?.sample_type_id || sample?.sampleTypeId || primarySample?.sample_type_id || primarySample?.sampleTypeId || 1);
      const params = new URLSearchParams({
        reportType: 'normal',
        sampleTypeId: String(sampleTypeId || 1),
      });
      const response = await fetch(`/api/reports/concise-test-pdf?${params.toString()}`, {
        method: 'GET',
      });
      if (!response.ok) {
        throw new Error('下载测试PDF失败');
      }
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = '简洁版报告测试数据.pdf';
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);
      appMessage.success('测试PDF已导出');
    } catch (error) {
      console.error('导出测试PDF失败:', error);
      appMessage.error('导出测试PDF失败');
    } finally {
      setTestPdfLoading(false);
    }
  };

  const samePatientSamples = sample && Array.isArray(sample.samePatientSamples)
    ? sample.samePatientSamples
    : (sample && Array.isArray(sample.same_patient_batch_samples) ? sample.same_patient_batch_samples : []);
  const batchStatus = String(sample?.batch_status || sample?.batchStatus || sample?.result?.batch_status || '').toLowerCase();
  const canGenerateReport = batchStatus === 'submitted';
  const effectivePrimarySampleId = primarySampleId || Number(sample?.id || 0);
  const secondarySampleIds = samePatientSamples
    .map((item: any) => Number(item.id))
    .filter((sampleId: number) => mergeSamePatientSamples && sampleId && sampleId !== effectivePrimarySampleId);
  const primarySample = samePatientSamples.find((item: any) => Number(item.id) === effectivePrimarySampleId) || sample || {};
  const samePatientHistoryRecords = mergeSamePatientSamples
    ? samePatientSamples
      .filter((item: any) => Number(item.id) !== effectivePrimarySampleId)
      .map((item: any) => normalizeReportHistoryRecord({ ...item, source: 'same-patient' }))
    : [];
  const reportHistoryOptions = Array.from(new Map([
    ...samePatientHistoryRecords,
    ...(Array.isArray(historicalSamples) ? historicalSamples.map((item) => normalizeReportHistoryRecord({ ...item, source: 'sample' })) : []),
    ...(Array.isArray(historicalReports) ? historicalReports.map((item) => normalizeReportHistoryRecord(item)) : []),
  ].map((item) => [item.historyKey, item])).values())
    .sort((a: any, b: any) => getHistoryTimestamp(a) - getHistoryTimestamp(b));

  useEffect(() => {
    if (!sample || !mergeSamePatientSamples || selectedHistoricalReports.length > 0 || reportHistoryOptions.length === 0) return;
    const samePatientKeys = reportHistoryOptions
      .filter((record: any) => record.source === 'same-patient' || isPreoperativeStage(record.treatmentStageName || record.treatment_stage_name))
      .map((record: any) => record.historyKey)
      .slice(0, 3);
    if (samePatientKeys.length > 0) {
      setSelectedHistoricalReports(samePatientKeys);
    }
  }, [sample?.id, mergeSamePatientSamples, reportHistoryOptions.length, selectedHistoricalReports.length]);

  const selectedHistoryRecordMap = new Map(reportHistoryOptions.map((record: any) => [record.historyKey, record]));
  const orderedSelectedHistoricalRecords = selectedHistoricalReports
    .map((key) => selectedHistoryRecordMap.get(key))
    .filter(Boolean)
    .slice(0, 3);
  const reportTimeline = [
    {
      historyKey: 'current-sample',
      sampleCode: primarySample.sampleCode || primarySample.sample_code || sample?.sample_code,
      createdAt: sample?.receive_date || sample?.collection_date || '',
      signalValue: Number(calculationResult || 0),
      treatmentStageName: getSampleTreatmentStageName(primarySample) || getSampleTreatmentStageName(sample),
      source: 'current',
      trend: '-',
    },
    ...orderedSelectedHistoricalRecords,
  ].map((record: any, index: number, rows: any[]) => ({
    ...record,
    trend: index === 0 ? '-' : calculateAdjacentTrend(Number(rows[index - 1]?.signalValue || 0), Number(record.signalValue || 0)),
  }));
  const orderedSelectedHistoricalRecordsWithTrend = reportTimeline
    .slice(1)
    .map((record: any) => ({
      ...record,
      trend: record.trend || '-',
    }));

  const moveSelectedHistory = (historyKey: React.Key, direction: -1 | 1) => {
    setSelectedHistoricalReports((current) => {
      const next = [...current];
      const index = next.indexOf(historyKey);
      const target = index + direction;
      if (index < 0 || target < 0 || target >= next.length) return current;
      [next[index], next[target]] = [next[target], next[index]];
      return next;
    });
  };

  if (loading) {
    return <div style={{ textAlign: 'center', marginTop: '50px' }}>加载中...</div>;
  }

  if (!sample) {
    return <div style={{ textAlign: 'center', marginTop: '50px' }}>样本不存在</div>;
  }

  return (
    <div style={{ padding: '20px' }}>
      <h2>报告生成</h2>

      {!canGenerateReport && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message="当前样本所属批次未提交或已退回，不能生成报告"
        />
      )}

      {existingReportId && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="该样本已生成报告"
          description="当前页面仍会读取样本信息和治疗阶段；如需查看或编辑已生成报告，请进入报告详情。"
          action={(
            <Button size="small" type="primary" onClick={() => navigate(`/report/view/${encodeURIComponent(sample?.sample_code || sample?.sampleCode || id || '')}`)}>
              查看报告
            </Button>
          )}
        />
      )}

      {samePatientSamples.length > 1 && (
        <Card title="同患者样本合并生成" style={{ marginBottom: 16 }}>
          <Alert
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
            message="识别到同一患者样本，默认合并显示并只生成一份报告"
          />
          <Checkbox
            checked={mergeSamePatientSamples}
            onChange={(event) => {
              setMergeSamePatientSamples(event.target.checked);
              if (!event.target.checked) {
                setSelectedHistoricalReports([]);
              }
            }}
            style={{ marginBottom: 16 }}
          >
            同患者样本合并显示
          </Checkbox>
          <Radio.Group
            value={effectivePrimarySampleId}
            onChange={(event) => {
              const nextPrimaryId = Number(event.target.value);
              setPrimarySampleId(nextPrimaryId);
              if (nextPrimaryId !== Number(sample.id)) {
                navigate(`/report/create/${nextPrimaryId}`);
              }
            }}
            style={{ width: '100%' }}
          >
            <Space direction="vertical" style={{ width: '100%' }}>
              <div style={{ padding: 12, border: '1px solid #d9d9d9', borderRadius: 4 }}>
                <div style={{ fontWeight: 600, marginBottom: 8 }}>报告出具样本</div>
                <Radio value={Number(primarySample.id)}>
                  {primarySample.sampleCode || primarySample.sample_code || sample.sample_code}
                  {getSampleTreatmentStageName(primarySample) ? ` ${getSampleTreatmentStageName(primarySample)}` : ''}
                </Radio>
              </div>
              <div style={{ padding: 12, border: '1px solid #d9d9d9', borderRadius: 4 }}>
                <div style={{ fontWeight: 600, marginBottom: 8 }}>子报告样本</div>
                <Space direction="vertical">
                  {samePatientSamples
                    .filter((item: any) => Number(item.id) !== effectivePrimarySampleId)
                    .map((item: any) => (
                      <Radio key={item.id} value={Number(item.id)}>
                        {item.sampleCode || item.sample_code}
                        {getSampleTreatmentStageName(item) ? ` ${getSampleTreatmentStageName(item)}` : ''}
                      </Radio>
                    ))}
                </Space>
              </div>
            </Space>
          </Radio.Group>
        </Card>
      )}


      {/* 报告基本信息 */}
      <Card title="报告基本信息">
        <Descriptions column={2} bordered>
          <Descriptions.Item label="姓名">{sample.patient_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="性别">{sample.gender || '-'}</Descriptions.Item>
          <Descriptions.Item label="年龄">{calculateAge(sample.idCard || sample.id_card || sample.patient_id_card)}</Descriptions.Item>
          <Descriptions.Item label="样本类型">{getSampleTypeName(sample) || '-'}</Descriptions.Item>
          <Descriptions.Item label="采样时间">
            {sample.receive_date || sample.collection_date ? (
              function() {
                const d = new Date(sample.receive_date || sample.collection_date);
                const year = d.getFullYear();
                const month = String(d.getMonth() + 1).padStart(2, '0');
                const day = String(d.getDate()).padStart(2, '0');
                const hours = String(d.getHours()).padStart(2, '0');
                const minutes = String(d.getMinutes()).padStart(2, '0');
                const seconds = String(d.getSeconds()).padStart(2, '0');
                return `${year}年${month}月${day}日 ${hours}:${minutes}:${seconds}`;
              }()
            ) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="检测项目">{formatReportProject(sample.cancer_type_name || sample.cancerTypeName, reportType) || '-'}</Descriptions.Item>
          <Descriptions.Item label="编号">{sample.sample_code || '-'}</Descriptions.Item>
          <Descriptions.Item label="送检单位">{organization}</Descriptions.Item>
          <Descriptions.Item label="报告类型">
            <Select value={reportType} onChange={setReportType} style={{ width: 120 }}>
              <Select.Option value="normal">高敏</Select.Option>
              <Select.Option value="high">超敏</Select.Option>
              <Select.Option value="screening">早筛</Select.Option>
            </Select>
          </Descriptions.Item>
          {calculationResult !== null && (
            <Descriptions.Item label="计算结果">{calculationResult.toFixed(1)}</Descriptions.Item>
          )}
          {selectedModel && (
            <Descriptions.Item label="模型名称">{selectedModel.name || '未知模型'} {selectedModel.version ? `[${selectedModel.version.startsWith('V') ? '' : 'V'}${selectedModel.version}]` : ''}</Descriptions.Item>
          )}
          <Descriptions.Item label="对应癌种">{sample.cancer_type_name || '-'}</Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 基因表达值 */}
      <Card title="基因表达值" style={{ marginTop: 16 }}>
        {sample.gene_data ? (
          <div>

            <div style={{ marginTop: 16 }}>
              <h4>基因值列表</h4>
              <div style={{ marginTop: 8, marginBottom: 16 }}>
                <Select
                  mode="multiple"
                  placeholder="选择历史检测样本进行对比"
                  loading={historicalSamplesLoading}
                  onChange={(values) => {
                    // 处理选中样本的逻辑，values现在是id数组
                    setSelectedHistoricalSamples(Array.isArray(values) ? values : []);
                  }}
                  value={Array.isArray(selectedHistoricalSamples) ? selectedHistoricalSamples : []}
                  style={{ width: '100%' }}
                >
                  {Array.isArray(historicalSamples) && historicalSamples.map((item) => {
                    // 格式化时间
                    let formattedDate = '';
                    try {
                      // 尝试多种日期格式
                      let dateValue = item.created_at || item.createdAt || item.generated_time || item.generatedTime || '';
                      if (dateValue) {
                        const date = new Date(dateValue);
                        if (!isNaN(date.getTime())) {
                          formattedDate = date.toISOString().split('T')[0];
                        } else if (typeof dateValue === 'string') {
                          // 如果日期对象无效，尝试直接提取日期部分
                          if (dateValue.includes('T')) {
                            formattedDate = dateValue.split('T')[0];
                          } else {
                            formattedDate = dateValue;
                          }
                        }
                      }
                    } catch (error) {
                      console.error('日期格式化失败:', error);
                    }
                    return (
                      <Select.Option key={item.id} value={item.id}>
                        {item.sampleCode || item.sample_code || '未知样本'} ({formattedDate || '未知日期'})
                      </Select.Option>
                    );
                  })}
                </Select>
              </div>
              <Table
                dataSource={sample.gene_data ? Object.entries(sample.gene_data).map(([gene, value], index) => {
                  const row: any = {
                    key: index,
                    gene,
                    value: parseFloat(value as string) || 0,
                    threshold: thresholds[gene] || 0
                  };
                  
                  // 为每个选中的历史样本添加对应的检测值
                  if (Array.isArray(selectedHistoricalSamples)) {
                    selectedHistoricalSamples.forEach((sampleId: number) => {
                      // 根据id找到对应的样本对象
                      const sampleItem = historicalSamples.find(item => item.id === sampleId);
                      if (sampleItem) {
                        // 解析历史样本的基因表达值
                        let historicalValue: string | number = '-';
                        try {
                          // 尝试多种可能的基因数据字段
                          let geneData: any = null;
                          
                          // 尝试从 result_data 中提取
                          if (sampleItem.result_data) {
                            try {
                              const resultData = JSON.parse(sampleItem.result_data);
                              geneData = resultData.gene_data || resultData.geneData;
                            } catch (error) {
                              console.error('解析 result_data 失败:', error);
                            }
                          }
                          
                          // 尝试从 resultData 中提取（后端返回的字段名）
                          if (!geneData && sampleItem.resultData) {
                            try {
                              const resultData = JSON.parse(sampleItem.resultData);
                              geneData = resultData.gene_data || resultData.geneData;
                            } catch (error) {
                              console.error('解析 resultData 失败:', error);
                            }
                          }
                          
                          // 尝试从其他可能的字段中提取
                          if (!geneData) {
                            geneData = sampleItem.gene_data || sampleItem.geneData;
                          }
                          
                          // 如果找到基因数据，尝试获取对应基因的值
                          if (geneData) {
                            const geneValue = geneData[gene];
                            if (geneValue !== undefined && geneValue !== null) {
                              const parsedValue = parseFloat(geneValue.toString());
                              if (!isNaN(parsedValue)) {
                                historicalValue = parsedValue;
                              } else {
                                historicalValue = geneValue;
                              }
                            }
                          }
                        } catch (error) {
                          console.error('解析历史样本基因数据失败:', error);
                        }
                        row[`sample_${sampleId}`] = historicalValue;
                      }
                    });
                  }
                  
                  return row;
                }) : []}
                columns={[
                  { title: '基因', dataIndex: 'gene' },
                  // 动态插入历史样本列
                  ...(Array.isArray(selectedHistoricalSamples) ? selectedHistoricalSamples
                    // 按时间从旧到新排序历史样本
                    .map(sampleId => {
                      const sampleItem = historicalSamples.find(item => item.id === sampleId);
                      return {
                        id: sampleId,
                        sampleItem
                      };
                    })
                    .sort((a, b) => {
                      // 获取日期值
                      const dateValueA = a.sampleItem?.created_at || a.sampleItem?.createdAt || a.sampleItem?.generated_time || a.sampleItem?.generatedTime || '';
                      const dateValueB = b.sampleItem?.created_at || b.sampleItem?.createdAt || b.sampleItem?.generated_time || b.sampleItem?.generatedTime || '';
                      
                      // 解析日期
                      const dateA = dateValueA ? new Date(dateValueA) : new Date(0);
                      const dateB = dateValueB ? new Date(dateValueB) : new Date(0);
                      
                      // 从旧到新排序
                      return dateA.getTime() - dateB.getTime();
                    })
                    .map(({ id }) => {
                      const sampleItem = historicalSamples.find(item => item.id === id);
                      // 格式化日期
                      let formattedDate = '';
                      try {
                        let dateValue = sampleItem?.created_at || sampleItem?.createdAt || sampleItem?.generated_time || sampleItem?.generatedTime || '';
                        if (dateValue) {
                          const date = new Date(dateValue);
                          if (!isNaN(date.getTime())) {
                            formattedDate = date.toISOString().split('T')[0];
                          } else if (typeof dateValue === 'string') {
                            if (dateValue.includes('T')) {
                              formattedDate = dateValue.split('T')[0];
                            } else {
                              formattedDate = dateValue;
                            }
                          }
                        }
                      } catch (error) {
                        console.error('日期格式化失败:', error);
                      }
                      return {
                        title: `${sampleItem?.sampleCode || `样本${id}`} (${formattedDate || '未知日期'})`,
                        dataIndex: `sample_${id}`,
                        key: `sample_${id}`
                      };
                    }) : []),
                  { title: '本次表达值', dataIndex: 'value' },
                  {
                    title: '阈值',
                    dataIndex: 'threshold',
                    render: (text: number, record: any) => (
                      <InputNumber
                        min={0}
                        step={0.1}
                        value={thresholds[record.gene]}
                        onChange={(value) => {
                          setThresholds(prev => ({
                            ...prev,
                            [record.gene]: value || 0
                          }));
                        }}
                        style={{ width: 120 }}
                      />
                    ),
                  }
                ]}
                pagination={{ pageSize: 10 }}
              />
            </div>
          </div>
        ) : (
          <p>无基因表达数据</p>
        )}
      </Card>

      {/* 按模型生成结果 */}
      <Card title="按模型生成结果" style={{ marginTop: 16 }}>
        <Form layout="vertical">
          <Form.Item
            name="model_id"
            label="选择模型"
            rules={[{ required: true, message: '请选择模型' }]}
          >
            <Select 
              placeholder="请选择模型" 
              loading={modelsLoading}
              value={selectedModel?.id}
              onChange={(value) => {
                const model = models.find(m => m.id === value);
                setSelectedModel(model);
              }}
            >
              {models.map((model) => (
                <Select.Option key={model.id} value={model.id}>
                  {model.name || '未知模型'} {model.version ? `[${model.version.startsWith('V') ? '' : 'V'}${model.version}]` : ''}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>
          
          <Button 
            type="primary" 
            icon={<CalculatorOutlined />}
            onClick={calculateByModel}
            loading={calculationLoading}
            disabled={!selectedModel}
          >
            按模型生成结果
          </Button>
          
          {calculationResult !== null && (
            <div style={{ marginTop: 16, padding: 16, backgroundColor: '#f5f5f5', borderRadius: 8 }}>
              <h4>计算结果</h4>
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <p style={{ fontSize: 18, fontWeight: 'bold' }}>信号值: {calculationResult.toFixed(1)}</p>
                <Button 
                  type="link" 
                  icon={<EditOutlined />} 
                  onClick={handleEditCalculation}
                >
                  手动修改
                </Button>
              </div>
            </div>
          )}
          
          {/* 手动修改计算结果模态框 */}
          <Modal
            title="手动修改计算结果"
            open={isEditingCalculation}
            onOk={handleSaveCalculation}
            onCancel={handleCancelEdit}
            okText="保存"
            cancelText="取消"
          >
            <Form layout="vertical">
              <Form.Item
                label="计算结果"
                rules={[{ required: true, message: '请输入计算结果' }]}
              >
                <InputNumber
                  min={0}
                  step={0.1}
                  precision={1}
                  value={editCalculationValue}
                  onChange={(value) => setEditCalculationValue(value)}
                  style={{ width: '100%' }}
                />
              </Form.Item>
            </Form>
          </Modal>
        </Form>
      </Card>

      {/* 历史检测结果 */}
      {compareData && compareData.comparison && compareData.comparison.length > 0 && (
        <Card title="历史检测结果" style={{ marginTop: 16 }}>
          <div style={{ height: 300, marginBottom: 20 }}>
            <ResponsiveContainer width="100%" height="100%">
              <LineChart
                data={compareData.comparison}
                margin={{ top: 5, right: 30, left: 20, bottom: 5 }}
              >
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="testDate" />
                <YAxis />
                <Tooltip />
                <Legend />
                <Line 
                  type="monotone" 
                  dataKey="signalValue" 
                  name="信号值" 
                  stroke="#8884d8" 
                  activeDot={{ r: 8 }} 
                />
              </LineChart>
            </ResponsiveContainer>
          </div>

          <Table
            dataSource={compareData.comparison}
            columns={[
              {
                title: '检测日期',
                dataIndex: 'testDate',
                key: 'testDate',
              },
              {
                title: '信号值',
                dataIndex: 'signalValue',
                key: 'signalValue',
              },
              {
                title: '趋势',
                dataIndex: 'trend',
                key: 'trend',
                render: (trend: string) => (
                  <Tag color={trend === '上升' ? 'red' : trend === '下降' ? 'green' : 'blue'}>
                    {trend || '无'}
                  </Tag>
                ),
              },
            ]}
            rowKey="testDate"
          />
        </Card>
      )}

      {/* 报告历史检测选择 */}
      <Card
        title="报告中显示的历史检测"
        style={{ marginTop: 16 }}
        extra={
          <Button icon={<PlusOutlined />} onClick={() => setManualHistoryModalVisible(true)}>
            手动添加
          </Button>
        }
      >
        <p style={{ marginBottom: 16 }}>请选择要显示在生成报告中的历史检测（最多选择3个），也可以手动添加：</p>
        {historicalReportsLoading || historicalSamplesLoading ? (
          <Spin size="small" />
        ) : reportHistoryOptions.length > 0 ? (
          <Table
            dataSource={reportHistoryOptions}
            rowSelection={{
              selectedRowKeys: selectedHistoricalReports,
              onChange: (keys) => {
                const validKeys = new Set(reportHistoryOptions.map(report => report.historyKey));
                const nextKeys = keys
                  .filter(key => validKeys.has(key))
                  .sort((a, b) => {
                    const recordA = selectedHistoryRecordMap.get(a);
                    const recordB = selectedHistoryRecordMap.get(b);
                    return getHistoryTimestamp(recordA) - getHistoryTimestamp(recordB);
                  });
                if (nextKeys.length > 3) {
                  appMessage.warning('最多只能选择3个历史检测');
                }
                setSelectedHistoricalReports(nextKeys.slice(0, 3));
              },
            }}
            columns={[
              {
                title: '来源',
                dataIndex: 'source',
                key: 'source',
                width: 110,
                render: (source: string) => {
                  const sourceMap: Record<string, { text: string; color: string }> = {
                    'same-patient': { text: '同患者样本', color: 'purple' },
                    sample: { text: '历史检测', color: 'blue' },
                    report: { text: '历史报告', color: 'green' },
                    manual: { text: '手动', color: 'orange' },
                  };
                  const meta = sourceMap[source] || { text: '历史记录', color: 'default' };
                  return <Tag color={meta.color}>{meta.text}</Tag>;
                },
              },
              {
                title: '样本编号',
                dataIndex: 'sampleCode',
                key: 'sampleCode',
                render: (text: any, record: any) => {
                  return (
                    <Space>
                      <span>{record.sampleCode || record.sample_code || '-'}</span>
                    </Space>
                  );
                }
              },
              {
                title: '检测日期',
                dataIndex: 'createdAt',
                key: 'createdAt',
                render: (text: any, record: any) => {
                  try {
                    return formatDateToYYYYMMDD(record.createdAt || getHistoryDateValue(record)) || '-';
                  } catch (error) {
                    return '-';
                  }
                }
              },
              {
                title: '治疗阶段',
                dataIndex: 'treatmentStageName',
                key: 'treatmentStageName',
                render: (text: any, record: any) => {
                  return record.treatmentStageName || record.treatment_stage_name || '-';
                }
              },
              {
                title: '信号值',
                dataIndex: 'signalValue',
                key: 'signalValue',
                render: (text: any, record: any) => {
                  return (
                    <InputNumber
                      min={0}
                      precision={1}
                      step={0.1}
                      style={{ width: 120 }}
                      value={Number(record.signalValue || 0)}
                      onChange={(value) => {
                        const nextValue = Number(value ?? 0);
                        setHistorySignalOverrides((current) => ({
                          ...current,
                          [String(record.historyKey)]: Number.isFinite(nextValue) ? nextValue : 0,
                        }));
                      }}
                    />
                  );
                }
              },
            ]}
            rowKey="historyKey"
            size="small"
          />
        ) : (
          <div style={{ height: 100, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: '#f5f5f5', borderRadius: 4 }}>
            暂无可选历史检测，可点击手动添加
          </div>
        )}
        {reportTimeline.length > 1 && (
          <div style={{ marginTop: 16 }}>
            <Divider orientation="left">报告显示顺序</Divider>
            <Table
              dataSource={reportTimeline}
              pagination={false}
              size="small"
              rowKey="historyKey"
              columns={[
                {
                  title: '顺序',
                  width: 80,
                  render: (_text: any, _record: any, index: number) => index + 1,
                },
                {
                  title: '样本编号',
                  dataIndex: 'sampleCode',
                  render: (text: any, record: any) => (
                    <Space>
                      <span>{text || record.sample_code || '-'}</span>
                      {record.source === 'current' && <Tag color="red">本样本</Tag>}
                    </Space>
                  ),
                },
                {
                  title: '检测日期',
                  dataIndex: 'createdAt',
                  render: (text: any, record: any) => formatDateToYYYYMMDD(text || getHistoryDateValue(record)) || '-',
                },
                {
                  title: '信号值',
                  dataIndex: 'signalValue',
                  render: (value: any, record: any) => (
                    record.source === 'current' ? (
                      Number(value || 0).toFixed(1)
                    ) : (
                      <InputNumber
                        min={0}
                        precision={1}
                        step={0.1}
                        style={{ width: 120 }}
                        value={Number(value || 0)}
                        onChange={(next) => {
                          const nextValue = Number(next ?? 0);
                          setHistorySignalOverrides((current) => ({
                            ...current,
                            [String(record.historyKey)]: Number.isFinite(nextValue) ? nextValue : 0,
                          }));
                        }}
                      />
                    )
                  ),
                },
                {
                  title: '相邻趋势',
                  dataIndex: 'trend',
                  render: (trendText: string, record: any, index: number) => (
                    <Tag color={trendText === '↑' ? 'red' : trendText === '↓' ? 'green' : 'blue'}>
                      {index === 0 ? '基准' : trendText}
                    </Tag>
                  ),
                },
                {
                  title: '调整',
                  width: 120,
                  render: (_text: any, record: any, index: number) => (
                    record.source === 'current' ? (
                      <span style={{ color: '#999' }}>固定</span>
                    ) : (
                      <Space>
                        <Button
                          size="small"
                          icon={<ArrowUpOutlined />}
                          disabled={index <= 1}
                          onClick={() => moveSelectedHistory(record.historyKey, -1)}
                        />
                        <Button
                          size="small"
                          icon={<ArrowDownOutlined />}
                          disabled={index >= reportTimeline.length - 1}
                          onClick={() => moveSelectedHistory(record.historyKey, 1)}
                        />
                      </Space>
                    )
                  ),
                },
              ]}
            />
          </div>
        )}
      </Card>

      <Modal
        title="合并生成报告"
        open={mergePromptVisible}
        onOk={() => setMergePromptVisible(false)}
        onCancel={() => setMergePromptVisible(false)}
        okText="确认"
        cancelText="暂不处理"
      >
        <p>
          识别到同一患者样本，默认合并显示并只生成一份报告。请选择报告出具样本，另一个作为子报告样本。
        </p>
        <Checkbox
          checked={mergeSamePatientSamples}
          onChange={(event) => setMergeSamePatientSamples(event.target.checked)}
          style={{ marginBottom: 12 }}
        >
          同患者样本合并显示
        </Checkbox>
        <Radio.Group
          value={effectivePrimarySampleId}
          onChange={(event) => {
            const nextPrimaryId = Number(event.target.value);
            setPrimarySampleId(nextPrimaryId);
            if (nextPrimaryId !== Number(sample.id)) {
              setMergePromptVisible(false);
              navigate(`/report/create/${nextPrimaryId}`);
            }
          }}
        >
          <Space direction="vertical">
            {samePatientSamples.map((item: any) => (
              <Radio key={item.id} value={Number(item.id)}>
                {item.sampleCode || item.sample_code} 作为报告出具样本
              </Radio>
            ))}
          </Space>
        </Radio.Group>
      </Modal>

      <Modal
        title="手动添加历史记录"
        open={manualHistoryModalVisible}
        onOk={handleAddManualHistory}
        onCancel={() => setManualHistoryModalVisible(false)}
        okText="添加"
        cancelText="取消"
        destroyOnHidden
      >
        <Form form={manualHistoryForm} layout="vertical">
          <Form.Item name="sampleCode" label="样本编号">
            <Input placeholder="可选，未填写时显示手动添加" />
          </Form.Item>
          <Form.Item name="createdAt" label="检测日期" rules={[{ required: true, message: '请选择检测日期' }]}>
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="signalValue" label="信号值" rules={[{ required: true, message: '请输入信号值' }]}>
            <InputNumber min={0} step={0.1} precision={1} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="treatmentStageName" label="治疗阶段">
            <Input placeholder="如 术后/随访/治疗中" />
          </Form.Item>
          <Form.Item name="trend" label="趋势">
            <Select allowClear placeholder="可选，不填则后端自动计算">
              <Select.Option value="↑">↑</Select.Option>
              <Select.Option value="↓">↓</Select.Option>
              <Select.Option value="-">-</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="remarks" label="备注">
            <Input.TextArea rows={3} placeholder="可选" />
          </Form.Item>
        </Form>
      </Modal>

      {/* 结果说明 */}
      <Card title="结果说明" style={{ marginTop: 16 }}>
        <Form.Item label="本次备注" extra="填写后显示在报告趋势表的本次备注列">
          <Input.TextArea
            value={currentRemarks}
            onChange={(event) => setCurrentRemarks(event.target.value)}
            rows={2}
            maxLength={100}
            showCount
            placeholder="可选"
          />
        </Form.Item>
        <Divider />
        <div>
          <Row align="middle" justify="space-between" style={{ marginBottom: 8 }}>
            <Col>
              <h4>信号值说明 <span style={{ color: 'red' }}>*</span></h4>
            </Col>
            <Col>
              <a href="#" onClick={() => handleOpenTemplateDrawer('signal_explanation')} style={{ fontSize: '12px', color: '#1890ff' }}>使用模板</a>
            </Col>
          </Row>
          <Form.Item label="信号值说明">
            <Input.TextArea 
              value={signalValueExplanation} 
              onChange={(e) => setSignalValueExplanation(e.target.value)} 
              rows={4} 
              placeholder="请填写信号值说明"
            />
          </Form.Item>
          
          <Divider />
          
          <Row align="middle" justify="space-between" style={{ marginBottom: 8 }}>
            <Col>
              <h4>结果说明 <span style={{ color: 'red' }}>*</span></h4>
            </Col>
            <Col>
              <a href="#" onClick={() => handleOpenTemplateDrawer('result_explanation')} style={{ fontSize: '12px', color: '#1890ff' }}>使用模板</a>
            </Col>
          </Row>
          <Form.Item label="结果说明">
            <Input.TextArea 
              value={resultExplanation} 
              onChange={(e) => setResultExplanation(e.target.value)} 
              rows={4} 
              placeholder="请填写结果说明"
            />
          </Form.Item>
        </div>
      </Card>

      {/* 生成报告按钮 */}
      {canGenerateReport && (
        <div style={{ marginTop: 24, marginBottom: 48, textAlign: 'center' }}>
          <Space>
            <Button
              icon={<DownloadOutlined />}
              loading={testPdfLoading}
              onClick={handleDownloadConciseTestPdf}
            >
              导出测试简洁版PDF
            </Button>
            <Button
              type="primary"
              size="large"
              icon={<FileTextOutlined />}
              onClick={handleSubmit}
            >
              生成报告
            </Button>
          </Space>
        </div>
      )}

      {/* 模板抽屉 */}
      <Drawer
        title={currentTemplateType === 'result_explanation' ? '报告详情模板' : '信号值详情模板'}
        placement="right"
        onClose={() => setTemplateDrawerVisible(false)}
        open={templateDrawerVisible}
        width={400}
      >
        <div style={{ marginBottom: 16 }}>
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={() => setCreateTemplateModalVisible(true)}
          >
            创建模板
          </Button>
        </div>
        <List
          itemLayout="horizontal"
          dataSource={templates}
          renderItem={(template) => (
            <List.Item
              actions={[
                <a key="use" onClick={() => handleUseTemplate(template)}>使用</a>
              ]}
            >
              <List.Item.Meta
                title={template.title}
                description={
                  <div>
                    <div style={{ fontSize: '12px', color: '#999', marginBottom: 8 }}>
                      创建时间: {new Date(template.createdAt).toLocaleString()}
                    </div>
                    <div style={{ fontSize: '12px', color: '#666' }}>
                      {template.content.length > 100 ? template.content.substring(0, 100) + '...' : template.content}
                    </div>
                  </div>
                }
              />
            </List.Item>
          )}
        />
      </Drawer>

      {/* 创建模板模态框 */}
      <Modal
        title="创建模板"
        open={createTemplateModalVisible}
        onOk={handleCreateTemplate}
        onCancel={() => setCreateTemplateModalVisible(false)}
      >
        <Form layout="vertical">
          <Form.Item
            label="模板标题"
            rules={[{ required: true, message: '请输入模板标题' }]}
          >
            <Input
              value={newTemplate.title}
              onChange={(e) => setNewTemplate({ ...newTemplate, title: e.target.value })}
              placeholder="请输入模板标题"
            />
          </Form.Item>
          <Form.Item
            label="模板内容"
            rules={[{ required: true, message: '请输入模板内容' }]}
          >
            <Input.TextArea
              value={newTemplate.content}
              onChange={(e) => setNewTemplate({ ...newTemplate, content: e.target.value })}
              placeholder="请输入模板内容"
              rows={4}
            />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Create;
