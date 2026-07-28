import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Descriptions, Table, Card, Tabs, Tag, Alert, Button, message, Modal, Space, Input, Form, Collapse, Select, Checkbox } from 'antd';
import { useNavigate, useParams } from '@umijs/max';
import { ArrowLeftOutlined, DownloadOutlined, EditOutlined, DeleteOutlined, EyeOutlined, SaveOutlined, FileTextOutlined } from '@ant-design/icons';
import { getBatchMultiDetail, mergeBatchData, exportBatchResult, matchGenes, deleteBatch, submitBatch, deleteSampleFromBatch, getBatchDetail, listCancerTypes, updateBatchCancerType, updateSampleCancerType, autoMatchCancerType, getSelectableCancerTypes, listGenes, listModels, applyBatchGeneMatches, resetSubmittedBatch, partialResetSubmittedBatch, getBatchDuplicateSamples, createBatchRetestSamples } from '@/services/api';

const { Panel } = Collapse;
const { Option } = Select;
const CLOSE_TAB_EVENT = 'hw-close-tab';
const RESETTABLE_BATCH_STATUSES = ['submitted', 'completed', 'forced_completed'];
const BATCH_DETAIL_BUILD_REVISION = '2026-06-23-cache-bust-1';
type DuplicateSampleAction = 'deleteSample' | 'retest' | 'overwrite';

const normalizeBatchStatus = (status: any) => String(status || '').trim().toLowerCase();
const formatBatchSampleCount = (actualCount: number, declaredCount?: number) => {
  const declared = Number(declaredCount || 0);
  if (actualCount > 0 && declared > 0 && actualCount !== declared) {
    return `实际 ${actualCount} / 登记 ${declared}`;
  }
  return String(actualCount || declared || 0);
};

const Detail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [batch, setBatch] = useState<any>(null);
  const [platforms, setPlatforms] = useState<string[]>([]);
  const [files, setFiles] = useState<any[]>([]);
  const [samples, setSamples] = useState<any[]>([]);
  const [mergedData, setMergedData] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [updatingCancerType, setUpdatingCancerType] = useState(false);
  const [editingSample, setEditingSample] = useState<any>(null);
  const [editForm] = Form.useForm();
  const [isMultiPlatform, setIsMultiPlatform] = useState(false);
  const [originalBatchData, setOriginalBatchData] = useState<any>(null);
  const [originalResults, setOriginalResults] = useState<any[]>([]);
  const [originalMissingSamples, setOriginalMissingSamples] = useState<string[]>([]);
  const [originalGeneColumns, setOriginalGeneColumns] = useState<string[]>([]);
  const [originalBeadCountWarnings, setOriginalBeadCountWarnings] = useState<string[]>([]);
  const [originalValidationStatus, setOriginalValidationStatus] = useState<'success' | 'error' | 'warning' | null>(null);
  const [originalValidationMessage, setOriginalValidationMessage] = useState<string>('');
  const [unmatchedGenes, setUnmatchedGenes] = useState<string[]>([]);
  const [geneOptions, setGeneOptions] = useState<any[]>([]);
  const [geneMatchValues, setGeneMatchValues] = useState<Record<string, string>>({});
  const [savingGeneMatches, setSavingGeneMatches] = useState(false);
  // Panel匹配相关状态
  const [panel, setPanel] = useState<any>(null);
  const [panelMatch, setPanelMatch] = useState<any>(null);
  // 样本Panel匹配结果
  const [samplePanelMatches, setSamplePanelMatches] = useState<any[]>([]);
  // 检测类型相关状态
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [models, setModels] = useState<any[]>([]);
  const [matchedCancerType, setMatchedCancerType] = useState<any>(null);
  // 可选择的检测类型（用于检测类型选择限制）
  const [selectableCancerTypes, setSelectableCancerTypes] = useState<any[]>([]);
  const [autoMatching, setAutoMatching] = useState(false);
  // 记录是否已尝试过自动匹配（防止无限循环）
  const [hasTriedAutoMatch, setHasTriedAutoMatch] = useState(false);
  // 对照水数据
  const [controlWaterData, setControlWaterData] = useState<any>(null);
  const [selectedSampleCodes, setSelectedSampleCodes] = useState<string[]>([]);
  const [duplicateSamples, setDuplicateSamples] = useState<any[]>([]);
  const [duplicateModalVisible, setDuplicateModalVisible] = useState(false);
  const [duplicateActions, setDuplicateActions] = useState<Record<string, DuplicateSampleAction>>({});
  const [resolvingDuplicateAction, setResolvingDuplicateAction] = useState('');
  const resolvingDuplicateActionRef = useRef('');
  const detailRequestRef = useRef<{ key: string; promise: Promise<void> } | null>(null);
  const duplicateRequestRef = useRef<{ key: string; promise: Promise<void> } | null>(null);
  const duplicateLoadedKeyRef = useRef('');
  const duplicateModalShownKeyRef = useRef('');
  const cancerTypesLoadedRef = useRef(false);
  const geneOptionsLoadedRef = useRef(false);
  const modelsLoadedRef = useRef(false);

  const getSampleCode = (sample: any) => String(sample?.sampleCode || sample?.sample_code || sample?.displaySampleName || '').trim();
  const getCurrentBatchSampleCodes = () => Array.from(new Set(
    (isMultiPlatform ? samples : originalResults)
      .map((item: any) => getSampleCode(item))
      .filter((sampleCode: string) => sampleCode && sampleCode !== 'H'),
  ));
  const isSampleMatched = (sample: any) => Number(sample?.matchStatus ?? sample?.match_status ?? 0) === 1 || Boolean(sample?.patientName || sample?.patient_name);

  const toggleSelectedSample = (sampleCode: string, checked: boolean) => {
    if (!sampleCode || sampleCode === 'H') return;
    setSelectedSampleCodes((current) => {
      if (checked) {
        return current.includes(sampleCode) ? current : [...current, sampleCode];
      }
      return current.filter((code) => code !== sampleCode);
    });
  };

  useEffect(() => {
    if (id) fetchDetail();
    if (!cancerTypesLoadedRef.current) {
      cancerTypesLoadedRef.current = true;
      fetchCancerTypes();
    }
    if (!geneOptionsLoadedRef.current) {
      geneOptionsLoadedRef.current = true;
      fetchGeneOptions();
    }
    if (!modelsLoadedRef.current) {
      modelsLoadedRef.current = true;
      fetchModels();
    }
  }, [id]);

  // 数据加载完成后检查检测类型（不再自动匹配，改为手动触发）
  useEffect(() => {
    if (
      !loading &&
      samples.length > 0 &&
      cancerTypes.length > 0
    ) {
      // 检查是否已有检测类型
      const hasCancerType = samples.some(
        (s: any) => s.cancerTypeName && s.cancerTypeName !== ''
      );
      // 不再自动匹配，仅记录状态
      if (!hasCancerType && !hasTriedAutoMatch) {
        setHasTriedAutoMatch(true);
      }
    }
  }, [loading, samples, cancerTypes]);

  // 获取检测类型列表
  const fetchCancerTypes = async () => {
    try {
      const response = await listCancerTypes();
      setCancerTypes(response.data);
    } catch (error) {
      cancerTypesLoadedRef.current = false;
      console.error('获取检测类型列表失败:', error);
    }
  };

  const fetchGeneOptions = async () => {
    try {
      const response = await listGenes({ activeOnly: 1 });
      setGeneOptions(Array.isArray(response.data) ? response.data : []);
    } catch (error) {
      geneOptionsLoadedRef.current = false;
      console.error('获取基因列表失败:', error);
    }
  };

  const fetchModels = async () => {
    try {
      const response = await listModels({ activeOnly: 1, includeDeprecated: 0 });
      setModels(Array.isArray(response.data) ? response.data : []);
    } catch (error) {
      modelsLoadedRef.current = false;
      console.error('获取模型列表失败:', error);
    }
  };

  // 获取可选择的检测类型列表
  const fetchSelectableCancerTypes = async (currentCancerTypeId: string | number = 0) => {
    try {
      const response = await getSelectableCancerTypes(currentCancerTypeId);
      setSelectableCancerTypes(response.data);
    } catch (error) {
      console.error('获取可选择检测类型列表失败:', error);
      setSelectableCancerTypes(cancerTypes);
    }
  };

  const resetDuplicateCache = () => {
    duplicateLoadedKeyRef.current = '';
    duplicateRequestRef.current = null;
  };

  const fetchDuplicateSamples = async (force = false) => {
    if (!id) return;
    const requestKey = String(id);
    if (!force && duplicateLoadedKeyRef.current === requestKey) return;
    if (!force && duplicateRequestRef.current?.key === requestKey) {
      await duplicateRequestRef.current.promise;
      return;
    }

    const promise = (async () => {
      const response = await getBatchDuplicateSamples(id, { skipErrorHandler: true });
      const duplicates = response.data?.duplicateSamples || [];
      setDuplicateSamples(duplicates);
      setDuplicateActions((current) => {
        const next: Record<string, DuplicateSampleAction> = {};
        duplicates.forEach((item: any) => {
          const sampleCode = String(item?.sampleCode || '').trim();
          if (sampleCode) next[sampleCode] = current[sampleCode] || 'overwrite';
        });
        return next;
      });
      duplicateLoadedKeyRef.current = requestKey;
      const shownKey = `${id}:${duplicates.map((item: any) => item.sampleCode).join(',')}`;
      if (duplicates.length > 0 && shownKey !== duplicateModalShownKeyRef.current) {
        duplicateModalShownKeyRef.current = shownKey;
        setDuplicateModalVisible(true);
      }
    })();

    duplicateRequestRef.current = { key: requestKey, promise };
    try {
      await promise;
    } catch (error) {
      console.warn('获取重复样本失败:', error);
    } finally {
      if (duplicateRequestRef.current?.promise === promise) {
        duplicateRequestRef.current = null;
      }
    }
  };

  // 自动匹配并应用检测类型（静默版，页面加载时自动执行）
  const autoMatchAndApply = async () => {
    setAutoMatching(true);
    try {
      const response = await autoMatchCancerType(id!);
      if (response.data.recommendedCancerType) {
        await updateBatchCancerType(id!, response.data.recommendedCancerType.id.toString());
        message.success(`检测类型已自动匹配为: ${response.data.recommendedCancerType.name}`);
        // 使用 fetchDetail 刷新而非整页重载
        await fetchDetail();
      }
      // 未找到匹配时静默处理，不弹错误
    } catch (error: any) {
      console.warn('自动匹配检测类型:', error.message || error);
    } finally {
      setAutoMatching(false);
    }
  };

  // 手动触发自动匹配（带提示）
  const handleAutoMatchCancerType = async () => {
    setAutoMatching(true);
    try {
      const response = await autoMatchCancerType(id!);
      if (response.data.recommendedCancerType) {
        await updateBatchCancerType(id!, response.data.recommendedCancerType.id.toString());
        message.success(`检测类型已自动匹配为: ${response.data.recommendedCancerType.name}`);
        await fetchDetail();
      } else {
        message.warning('未找到匹配的检测类型，请手动选择');
      }
    } catch (error: any) {
      message.error(error.message || '自动匹配检测类型失败');
    } finally {
      setAutoMatching(false);
    }
  };

  // 修改批次检测类型
  const handleUpdateCancerType = async (cancerTypeId: string) => {
    setUpdatingCancerType(true);
    try {
      await updateBatchCancerType(id!, cancerTypeId);
      message.success('检测类型更新成功');
      await fetchDetail({ forceDuplicateRefresh: true });
    } catch (error: any) {
      message.error(error.message || '检测类型更新失败');
    } finally {
      setUpdatingCancerType(false);
    }
  };

  // 修改单个样本的检测类型
  const [selectedSampleForCancerType, setSelectedSampleForCancerType] = useState<string>('');
  const [cancerTypeModalVisible, setCancerTypeModalVisible] = useState(false);
  const [selectedCancerTypeId, setSelectedCancerTypeId] = useState<string>('');
  const [selectedModelId, setSelectedModelId] = useState<string>('');
  const [selectedSamplePanelMatches, setSelectedSamplePanelMatches] = useState<any[]>([]);

  const isGeneMetaColumn = (key: string) => ['', 'Sample', 'sample_code', 'location', 'Location', 'Total Events', 'totalEvents'].includes(String(key || '').trim());

  const normalizeGeneName = (gene: any) => String(gene || '').trim().toLowerCase();

  const uniqueGenes = (genes: any[]) => {
    const byKey = new Map<string, string>();
    genes.forEach((gene) => {
      const text = String(gene || '').trim();
      if (!text) return;
      const key = normalizeGeneName(text);
      if (!byKey.has(key)) byKey.set(key, text);
    });
    return Array.from(byKey.values()).sort();
  };

  const parsePanelIds = (cancerType: any): number[] => {
    if (Array.isArray(cancerType?.panels) && cancerType.panels.length > 0) {
      return cancerType.panels.map((panel: any) => Number(panel.id)).filter(Boolean);
    }
    return String(cancerType?.panel_ids || cancerType?.panelIds || '')
      .split(',')
      .map((id) => Number(String(id).trim()))
      .filter(Boolean);
  };

  const getCancerTypeRequiredGenes = (cancerType: any): string[] => {
    if (Array.isArray(cancerType?.requiredGenes) && cancerType.requiredGenes.length > 0) {
      return uniqueGenes(cancerType.requiredGenes);
    }
    const genes: string[] = [];
    if (Array.isArray(cancerType?.panels)) {
      cancerType.panels.forEach((panelItem: any) => {
        if (Array.isArray(panelItem?.genes)) {
          panelItem.genes.forEach((gene: any) => genes.push(gene.geneSymbol || gene.gene_symbol || gene.name || gene.geneName || gene));
        }
        if (Array.isArray(panelItem?.geneSymbols)) {
          genes.push(...panelItem.geneSymbols);
        }
        if (Array.isArray(panelItem?.panelGenes)) {
          genes.push(...panelItem.panelGenes);
        }
      });
    }
    if (genes.length > 0) return uniqueGenes(genes);

    const panelIds = new Set(parsePanelIds(cancerType));
    const genesFromMatches: string[] = [];
    samplePanelMatches.forEach((sampleMatch: any) => {
      (sampleMatch.panelMatches || []).forEach((panelMatchItem: any) => {
        if (panelIds.has(Number(panelMatchItem.panelId))) {
          genesFromMatches.push(...(panelMatchItem.panelGenes || []));
        }
      });
    });
    return uniqueGenes(genesFromMatches);
  };

  const getSampleGenes = (sampleCode: string): string[] => {
    const sample = samples.find((item: any) => item.sampleCode === sampleCode);
    const genes: string[] = [];
    const addKeys = (data: any) => {
      if (!data || typeof data !== 'object') return;
      Object.keys(data).forEach((key) => {
        if (!isGeneMetaColumn(key)) genes.push(key);
      });
    };
    addKeys(sample?.median);
    addKeys(sample?.count);

    const sampleMatch = samplePanelMatches.find((item: any) => item.sampleCode === sampleCode);
    (sampleMatch?.panelMatches || []).forEach((panelMatchItem: any) => {
      genes.push(...(panelMatchItem.sampleGenes || []));
    });
    return uniqueGenes(genes);
  };

  const getModelGenes = (model: any): string[] => {
    if (Array.isArray(model?.geneSymbols)) return uniqueGenes(model.geneSymbols);
    if (typeof model?.genes === 'string') return uniqueGenes(model.genes.split(','));
    return [];
  };

  const getSelectableModelsForSample = (sampleCode: string, cancerTypeId: string | number) => {
    const sampleGeneSet = new Set(getSampleGenes(sampleCode).map(normalizeGeneName));
    return models
      .filter((model: any) => (
        Number(model?.isActive ?? model?.is_active) === 1
        && !Number(model?.isDeprecated ?? model?.is_deprecated)
        && Number(model?.cancerTypeId || model?.cancer_type_id || 0) === Number(cancerTypeId)
      ))
      .map((model: any) => {
        const modelGenes = getModelGenes(model);
        const missingGenes = modelGenes.filter((gene) => !sampleGeneSet.has(normalizeGeneName(gene)));
        return { ...model, modelGenes, missingGenes, canMatch: modelGenes.length > 0 && missingGenes.length === 0 };
      })
      .sort((a: any, b: any) => Number(b.canMatch) - Number(a.canMatch) || b.modelGenes.length - a.modelGenes.length);
  };

  const chooseDefaultModelId = (sampleCode: string, cancerTypeId: string | number, currentModelId?: string | number) => {
    const selectableModels = getSelectableModelsForSample(sampleCode, cancerTypeId);
    const savedModel = selectableModels.find((model: any) => String(model.id) === String(currentModelId || '') && model.canMatch);
    return String(savedModel?.id || selectableModels.find((model: any) => model.canMatch)?.id || '');
  };

  useEffect(() => {
    if (!cancerTypeModalVisible || !selectedCancerTypeId || selectedModelId || !models.length) return;
    const sample = samples.find((item: any) => item.sampleCode === selectedSampleForCancerType);
    setSelectedModelId(chooseDefaultModelId(
      selectedSampleForCancerType,
      selectedCancerTypeId,
      sample?.modelId || sample?.model_id,
    ));
  }, [cancerTypeModalVisible, models.length, selectedCancerTypeId, selectedModelId, selectedSampleForCancerType]);

  const getSamplePanelMatches = (sampleCode: string) => {
    const sampleMatch = samplePanelMatches.find((item: any) => item.sampleCode === sampleCode);
    if (sampleMatch?.panelMatches) return sampleMatch.panelMatches;
    const sample = samples.find((item: any) => item.sampleCode === sampleCode);
    return sample?.panelMatches || [];
  };

  const buildSelectableCancerTypesForSample = (sampleCode: string) => {
    const sampleGenes = getSampleGenes(sampleCode);
    const sampleGeneSet = new Set(sampleGenes.map(normalizeGeneName));

    return cancerTypes
      .map((ct: any) => {
        const panelIds = parsePanelIds(ct);
        const requiredGenes = getCancerTypeRequiredGenes(ct);
        const missingGenes = requiredGenes.filter((gene) => !sampleGeneSet.has(normalizeGeneName(gene)));
        return {
          ...ct,
          panelIds,
          panelCount: panelIds.length || ct.panelCount || 0,
          requiredGenes,
          geneCount: requiredGenes.length,
          canMatch: requiredGenes.length > 0 && missingGenes.length === 0,
          missingGenes,
        };
      })
      .sort((a: any, b: any) => Number(b.canMatch) - Number(a.canMatch) || b.geneCount - a.geneCount);
  };

  const handleUpdateCancerTypeForSample = (sampleCode: string) => {
    setSelectedSampleForCancerType(sampleCode);
    // 获取当前样本的检测类型ID
    const sample = samples.find(s => s.sampleCode === sampleCode);
    const currentCancerTypeId = sample?.cancerTypeId || sample?.cancer_type_id || 0;
    const currentModelId = sample?.modelId || sample?.model_id || 0;
    const panelMatches = getSamplePanelMatches(sampleCode);
    const selectableTypes = buildSelectableCancerTypesForSample(sampleCode);
    setSelectedSamplePanelMatches(panelMatches);
    setSelectableCancerTypes(selectableTypes);
    setSelectedCancerTypeId(currentCancerTypeId ? String(currentCancerTypeId) : '');
    setSelectedModelId(chooseDefaultModelId(sampleCode, currentCancerTypeId, currentModelId));
    setCancerTypeModalVisible(true);
  };

  const handleConfirmCancerTypeChange = async () => {
    if (!selectedCancerTypeId) {
      message.error('请选择检测类型');
      return;
    }
    if (!selectedModelId) {
      message.error('请选择检测模型');
      return;
    }

    setUpdatingCancerType(true);
    try {
      if (selectedSampleForCancerType) {
        const selectedType = selectableCancerTypes.find((ct: any) => String(ct.id) === selectedCancerTypeId);
        if (selectedType && selectedType.canMatch === false) {
          message.error('该检测类型所需基因未被样本基因完全覆盖，无法匹配');
          return;
        }
        const selectedModel = getSelectableModelsForSample(selectedSampleForCancerType, selectedCancerTypeId)
          .find((model: any) => String(model.id) === selectedModelId);
        if (!selectedModel || !selectedModel.canMatch) {
          message.error('样本基因不能完整覆盖所选模型');
          return;
        }
        await updateSampleCancerType(id!, selectedSampleForCancerType, selectedCancerTypeId, selectedModelId);
        message.success(`样本 ${selectedSampleForCancerType} 的检测类型和模型更新成功`);
      } else {
        await updateBatchCancerType(id!, selectedCancerTypeId);
        message.success('检测类型更新成功（已更新批次中所有样本）');
      }
      setCancerTypeModalVisible(false);
      await fetchDetail();
    } catch (error: any) {
      message.error(error.message || '检测类型更新失败');
    } finally {
      setUpdatingCancerType(false);
    }
  };

  // 根据样本的检测类型匹配检测类型配置（优先从samplePanelMatches获取）
  const matchedCancerTypes = useMemo(() => {
    if (!cancerTypes.length) return [];

    const cancerTypeIds = new Set<string>();

    if (samplePanelMatches && samplePanelMatches.length > 0) {
      samplePanelMatches.forEach(sampleMatch => {
        if (sampleMatch.cancerTypeId) {
          cancerTypeIds.add(String(sampleMatch.cancerTypeId));
        }
      });
    }
    samples.forEach(sample => {
      if (sample.cancerTypeId || sample.cancer_type_id) {
        cancerTypeIds.add(String(sample.cancerTypeId || sample.cancer_type_id));
      }
    });
    
    // 查找匹配的检测类型
    const matched = [];
    for (const cancerTypeId of cancerTypeIds) {
      const cancerType = cancerTypes.find(ct => String(ct.id) === cancerTypeId);
      if (cancerType) {
        matched.push(cancerType);
      }
    }
    
    return matched;
  }, [samples, cancerTypes, samplePanelMatches]);

  useEffect(() => {
    // 如果只有一个匹配的检测类型，设置为matchedCancerType
    if (matchedCancerTypes.length === 1) {
      setMatchedCancerType(matchedCancerTypes[0]);
    } else {
      setMatchedCancerType(null);
    }
  }, [matchedCancerTypes]);

  // 时间格式化函数
  const formatDateTime = (dateStr: string): string => {
    if (!dateStr || dateStr === '-') return '-';
    try {
      // 处理多种时间格式
      let date;
      if (dateStr.includes('/')) {
        // 格式如: 5/6/2026 12:43:45 PM
        date = new Date(dateStr);
      } else {
        // 标准格式
        date = new Date(dateStr);
      }
      
      if (isNaN(date.getTime())) {
        return dateStr;
      }
      
      const year = date.getFullYear();
      const month = String(date.getMonth() + 1).padStart(2, '0');
      const day = String(date.getDate()).padStart(2, '0');
      const hours = String(date.getHours()).padStart(2, '0');
      const minutes = String(date.getMinutes()).padStart(2, '0');
      const seconds = String(date.getSeconds()).padStart(2, '0');
      
      return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
    } catch (e) {
      return dateStr;
    }
  };

  const fetchDetail = async (options: { forceDuplicateRefresh?: boolean } = {}) => {
    const requestKey = String(id || '');
    if (requestKey && detailRequestRef.current?.key === requestKey) {
      await detailRequestRef.current.promise;
      return;
    }

    const promise = (async () => {
      setLoading(true);
      try {
      // 先尝试获取多平台数据
      const response = await getBatchMultiDetail(id!, { skipErrorHandler: true });
      if (response.data && response.data.batch) {
        const batchData = response.data.batch;
        const platformsData = response.data.platforms || {};
        const filesData = response.data.files || [];
        const samplesData = response.data.samples || [];
        const mergedDataData = response.data.mergedData || null;
        const panelData = response.data.panel || null;
        const panelMatchData = response.data.panelMatch || null;

        // 提取对照水数据
        let controlWater = null;
        for (const sample of samplesData) {
          if (sample.sampleCode === 'H' || sample.displaySampleName === 'H') {
            controlWater = sample;
            break;
          }
        }
        if (controlWater) {
          setControlWaterData(controlWater);
        } else {
          setControlWaterData(null);
        }

        // 格式化文件时间
        const formattedFiles = filesData.map((file: any) => ({
          ...file,
          createdAt: formatDateTime(file.createdAt),
          batchStartTime: formatDateTime(file.batchStartTime),
          batchStopTime: formatDateTime(file.batchStopTime),
        }));
        
        const samplePanelMatchesData = response.data.samplePanelMatches || [];
        const sampleMatchMap = new Map(samplePanelMatchesData.map((item: any) => [item.sampleCode, item]));

        // 过滤掉H样本，只保留普通样本
        const filteredSamples = samplesData.filter(
          (sample: any) => sample.sampleCode !== 'H' && sample.displaySampleName !== 'H'
        ).map((sample: any) => {
          const sampleMatch: any = sampleMatchMap.get(sample.sampleCode);
          return sampleMatch ? {
            ...sample,
            patientId: sampleMatch.patientId || sample.patientId,
            patientCode: sampleMatch.patientCode || sample.patientCode,
            patientName: sampleMatch.patientName || sample.patientName,
            matchStatus: sampleMatch.matchStatus ?? sample.matchStatus,
            cancerTypeId: sampleMatch.cancerTypeId ?? sample.cancerTypeId,
            cancerTypeName: sampleMatch.cancerTypeName || sample.cancerTypeName,
          } : sample;
        });
        
        setIsMultiPlatform(true);
        setBatch(batchData);
        setPlatforms(Object.keys(platformsData));
        setFiles(formattedFiles);
        setSamples(filteredSamples);
        setMergedData(mergedDataData);
        setPanel(panelData);
        setPanelMatch(panelMatchData);
        setUnmatchedGenes([]);
        setSelectedSampleCodes([]);
        // 设置样本Panel匹配结果
        setSamplePanelMatches(samplePanelMatchesData);
      } else {
        // 如果没有多平台数据，回退到原有的获取方式
        await fetchOriginalDetail();
      }
      } catch (error) {
        console.error('获取多平台数据失败，回退到原有方式:', error);
        await fetchOriginalDetail();
      } finally {
        await fetchDuplicateSamples(Boolean(options.forceDuplicateRefresh));
        setLoading(false);
      }
    })();

    detailRequestRef.current = { key: requestKey, promise };
    try {
      await promise;
    } finally {
      if (detailRequestRef.current?.promise === promise) {
        detailRequestRef.current = null;
      }
    }
  };

  const fetchOriginalDetail = async () => {
    try {
      const response = await getBatchDetail(id!, { skipErrorHandler: true });
      const batchData = response.data.batch;
      const resultsData = response.data.results || [];
      const missingSamplesData = response.data.missingSamples || [];
      const unmatchedGenesData = response.data.unmatchedGenes || [];
      const medianData = response.data.medianData || [];
      const countData = response.data.countData || [];
      // 获取样本Panel匹配结果
      const samplePanelMatchesData = response.data.samplePanelMatches || [];
      
      // 构建样本数据映射
      const sampleMap = new Map();
      
      // 处理Median数据
      medianData.forEach((data: any) => {
        const sampleCode = data.Sample || data.sample_code;
        if (sampleCode) {
          if (!sampleMap.has(sampleCode)) {
            sampleMap.set(sampleCode, { sampleCode, median: data });
          } else {
            const sample = sampleMap.get(sampleCode);
            sample.median = data;
          }
        }
      });
      
      // 处理Count数据
      countData.forEach((data: any) => {
        const sampleCode = data.Sample || data.sample_code;
        if (sampleCode) {
          // 保存 Total Events 值
          const totalEvents = data['Total Events'] || data.totalEvents;
          // 移除 Total Events 字段，避免在表格中显示
          const { 'Total Events': _, ...countDataWithoutTotalEvents } = data;
          if (!sampleMap.has(sampleCode)) {
            sampleMap.set(sampleCode, { sampleCode, count: countDataWithoutTotalEvents, totalEvents });
          } else {
            const sample = sampleMap.get(sampleCode);
            sample.count = countDataWithoutTotalEvents;
            sample.totalEvents = totalEvents;
          }
        }
      });
      
      // 处理现有结果数据
      resultsData.forEach((result: any) => {
        const sampleCode = result.sampleCode;
        if (sampleCode && sampleMap.has(sampleCode)) {
          const sample = sampleMap.get(sampleCode);
          sample.id = result.id;
          sample.sampleId = result.sampleId;
          sample.patientId = result.patientId;
          sample.patientCode = result.patientCode;
          sample.patientName = result.patientName;
          sample.reportId = result.reportId;
          sample.hasReport = result.hasReport;
        }
      });
      
      // 转换为数组并处理
      const processedResults = Array.from(sampleMap.values()).map((sample: any) => {
        // 确保displaySampleName字段存在
        if (!sample.displaySampleName) {
          sample.displaySampleName = sample.sampleCode || '未知样本';
        }
        
        // 检查是否为对照水样本
        if (sample.displaySampleName === 'H' || sample.sampleCode === 'H') {
          sample.isControlWater = true;
        }
        
        // 提取孔位信息（处理大小写）
        sample.location = sample.median?.location || sample.median?.Location || sample.count?.location || sample.count?.Location || '-';
        
        // 提取总事件数
        sample.totalEvents = sample.totalEvents || sample.count?.['Total Events'] || sample.count?.totalEvents || '-';
        
        return sample;
      });
      
      // 将对照水样本移到第一行
      const controlWaterIndex = processedResults.findIndex(result => result.isControlWater);
      if (controlWaterIndex > 0) {
        const controlWater = processedResults.splice(controlWaterIndex, 1)[0];
        processedResults.unshift(controlWater);
      }
      const controlWater = processedResults.find(result => result.isControlWater);
      
      if (controlWater) {
        setControlWaterData({
          sampleCode: controlWater.sampleCode,
          location: controlWater.location,
          median: controlWater.median,
          count: controlWater.count,
          totalEvents: controlWater.totalEvents
        });
      }      
      // 提取基因列名
      let geneCols: string[] = [];
      if (processedResults.length > 0 && processedResults[0].median) {
        geneCols = Object.keys(processedResults[0].median).filter(key => 
          key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location' && key !== 'Total Events'
        );
      }
      
      // 检查磁珠计数小于10的问题
      const warnings: string[] = [];
      processedResults.forEach((sample: any) => {
        if (sample.count) {
          geneCols.forEach((gene) => {
            const count = sample.count[gene];
            if (typeof count === 'number' && count < 10) {
              warnings.push(`${sample.displaySampleName}样本${gene}基因磁珠数过少`);
            }
          });
        }
      });
      
      // 检查缺失样本
      const hasMissingSamples = missingSamplesData.filter((sample: string) => {
        const trimmedSample = sample.trim();
        return trimmedSample !== '无样本' && trimmedSample !== '';
      }).length > 0;
      
      // 检查表达值大于100的问题
      const hasHighExpression = processedResults.some((sample: { [key: string]: any }) => {
        if (sample.median) {
          return geneCols.some(gene => {
            const value = sample.median[gene];
            return typeof value === 'number' && value > 100;
          });
        }
        return false;
      });
      
      // 检查是否有验证问题
      const hasValidationIssues = warnings.length > 0 || hasHighExpression;
      
      // 确定校验状态和消息
      let status: 'success' | 'error' | 'warning' | null = null;
      let msg = '';
      
      // 对于已完成的批次，不进行缺失样本检查
      if (batchData.status === 'completed' || batchData.status === 'forced_completed') {
        status = 'success';
        msg = '批次已完成';
      } else if (hasMissingSamples) {
        status = 'error';
        msg = '有样本在系统中不存在，禁止提交';
      } else if (hasHighExpression || hasValidationIssues) {
        status = 'warning';
        if (hasHighExpression && hasValidationIssues) {
          msg = '样本编号对照水某一个Median（表达值）大于100，且存在磁珠计数过少的样本';
        } else if (hasHighExpression) {
          msg = '样本编号对照水某一个Median（表达值）大于100';
        } else {
          msg = warnings.join('；');
        }
      } else {
        status = 'success';
        msg = '校验通过';
      }
      
      // 从samplePanelMatches中获取检测类型信息，合并到processedResults中
      if (samplePanelMatchesData && samplePanelMatchesData.length > 0) {
        samplePanelMatchesData.forEach((sampleMatch: any) => {
          const sample = processedResults.find((s: any) => s.sampleCode === sampleMatch.sampleCode);
          if (sample) {
            sample.cancerTypeId = sampleMatch.cancerTypeId;
            sample.cancerTypeName = sampleMatch.cancerTypeName;
            sample.patientId = sampleMatch.patientId || sample.patientId;
            sample.patientCode = sampleMatch.patientCode || sample.patientCode;
            sample.patientName = sampleMatch.patientName || sample.patientName;
            sample.matchStatus = sampleMatch.matchStatus ?? sample.matchStatus;
          }
        });
      }

      setIsMultiPlatform(false);
      setOriginalBatchData(batchData);
      setOriginalResults(processedResults);
      setOriginalMissingSamples(missingSamplesData);
      setUnmatchedGenes(unmatchedGenesData);
      setGeneMatchValues((current) => {
        const next = { ...current };
        unmatchedGenesData.forEach((gene: string) => {
          if (!next[gene]) next[gene] = '';
        });
        return next;
      });
      setOriginalGeneColumns(geneCols);
      setOriginalBeadCountWarnings(warnings);
      setOriginalValidationStatus(status);
      setOriginalValidationMessage(msg);
      setBatch(batchData);
      setSamplePanelMatches(samplePanelMatchesData);
      setSelectedSampleCodes([]);
    } catch (error: any) {
      message.error({
        key: 'batch-detail-load-error',
        content: error?.response?.data?.message || error?.data?.message || error?.message || '获取批次详情失败',
      });
    }
  };

  const handleExport = async () => {
    setExporting(true);
    try {
      const response = await exportBatchResult(id!);
      
      // 创建下载链接
      const blob = new Blob([response], { type: 'application/vnd.openxmlformats-officedocument.spreadsheetml.sheet' });
      const url = URL.createObjectURL(blob);
      const link = document.createElement('a');
      link.href = url;
      link.download = `batch_${batch?.batchCode || id}_results.xlsx`;
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);
      URL.revokeObjectURL(url);
      
      message.success('导出成功');
    } catch (error: any) {
      message.error(error.message || '导出失败');
    } finally {
      setExporting(false);
    }
  };

  const handleDelete = async () => {
    Modal.confirm({
      title: '确认删除批次',
      content: '确定要彻底删除该批次吗？删除后无法恢复，样本状态将恢复为"已接收"。',
      centered: true,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await deleteBatch(id!);
          message.success(response.message || '批次已彻底删除');
          navigate('/result/center');
        } catch (error: any) {
          // 显示后端返回的错误消息（如"xx报告已发布，请退回后再删除"）
          message.error(error.message || '删除失败');
        }
      },
    });
  };

  const showReviewedReportsConfirm = (
    response: any,
    onConfirm: () => Promise<void>,
  ) => {
    const reviewedReports = Array.isArray(response?.data?.reviewedReports) ? response.data.reviewedReports : [];
    Modal.confirm({
      title: '已审核报告确认退回',
      content: (
        <div style={{ lineHeight: 1.8 }}>
          <div style={{ color: '#cf1322', fontWeight: 600, marginBottom: 8 }}>
            {response?.message || '存在已审核通过报告，是否退回该报告？'}
          </div>
          {reviewedReports.slice(0, 8).map((item: any) => (
            <div key={`${item.id}-${item.sampleCode}`}>
              {item.sampleCode || '-'} {item.reportNo ? `（${item.reportNo}）` : ''}
            </div>
          ))}
          {reviewedReports.length > 8 && <div>等 {reviewedReports.length} 份报告</div>}
        </div>
      ),
      centered: true,
      okText: '确认退回报告',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: onConfirm,
    });
  };

  const handleResetSubmittedBatch = async () => {
    Modal.confirm({
      title: '确认退回批次',
      content: (
        <div style={{ color: '#cf1322', fontWeight: 600, lineHeight: 1.8 }}>
          删除批次会清除批次状态，并且会清除报告。你确定要这样操作吗？
        </div>
      ),
      centered: true,
      okText: '确认退回',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const response = await resetSubmittedBatch(id!);
          if ((response as any).code === 409) {
            showReviewedReportsConfirm(response, async () => {
              const forcedResponse = await resetSubmittedBatch(id!, { force: true });
              message.success(forcedResponse.message || '批次已退回');
              await fetchDetail({ forceDuplicateRefresh: true });
            });
            return;
          }
          message.success(response.message || '批次已退回');
          await fetchDetail({ forceDuplicateRefresh: true });
        } catch (error: any) {
          message.error(error.message || '退回失败');
          throw error;
        }
      },
    });
  };

  const handlePartialResetSubmittedBatch = async () => {
    const sampleCodes = selectedSampleCodes.filter((code) => code && code !== 'H');
    if (sampleCodes.length === 0) {
      message.warning('请先选择要退回的样本');
      return;
    }
    Modal.confirm({
      title: '确认部分退回',
      content: (
        <div style={{ color: '#cf1322', fontWeight: 600, lineHeight: 1.8 }}>
          将清除所选 {sampleCodes.length} 个样本的结果和关联报告，批次状态保持不变。确定退回吗？
        </div>
      ),
      centered: true,
      okText: '确认退回',
      cancelText: '取消',
      okButtonProps: { danger: true },
      onOk: async () => {
        try {
          const response = await partialResetSubmittedBatch(id!, { sampleCodes });
          if ((response as any).code === 409) {
            showReviewedReportsConfirm(response, async () => {
              const forcedResponse = await partialResetSubmittedBatch(id!, { sampleCodes, force: true });
              message.success(forcedResponse.message || '所选样本已退回');
              setSelectedSampleCodes([]);
              await fetchDetail({ forceDuplicateRefresh: true });
            });
            return;
          }
          message.success(response.message || '所选样本已退回');
          setSelectedSampleCodes([]);
          await fetchDetail({ forceDuplicateRefresh: true });
        } catch (error: any) {
          message.error(error.message || '部分退回失败');
          throw error;
        }
      },
    });
  };

  const handleSubmitBatch = async () => {
    setSubmitting(true);
    try {
      const response = await submitBatch(id!);
      message.success(response.message || '提交成功');
      // 重新获取批次详情，更新状态
      await fetchDetail({ forceDuplicateRefresh: true });
    } catch (error: any) {
      message.error(error.message || '提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDeleteDuplicateBatch = async () => {
    if (resolvingDuplicateActionRef.current) return;
    resolvingDuplicateActionRef.current = 'deleteBatch';
    setResolvingDuplicateAction('deleteBatch');
    try {
      const response = await deleteBatch(id!, { skipErrorHandler: true });
      message.success(response.message || '批次已删除');
      setDuplicateModalVisible(false);
      navigate('/result/center');
    } catch (error: any) {
      message.error(error?.message || '删除批次失败');
    } finally {
      resolvingDuplicateActionRef.current = '';
      setResolvingDuplicateAction('');
    }
  };

  const handleSubmitDuplicateResolution = async () => {
    if (resolvingDuplicateActionRef.current) return;
    const actionGroups = duplicateSamples.reduce<Record<DuplicateSampleAction, string[]>>((groups, item: any) => {
      const sampleCode = String(item?.sampleCode || '').trim();
      if (!sampleCode) return groups;
      const action = duplicateActions[sampleCode] || 'overwrite';
      groups[action].push(sampleCode);
      return groups;
    }, { deleteSample: [], retest: [], overwrite: [] });

    const selectedCount = actionGroups.deleteSample.length + actionGroups.retest.length + actionGroups.overwrite.length;
    if (selectedCount === 0) {
      message.warning('请选择需要处理的重复样本');
      return;
    }

    resolvingDuplicateActionRef.current = 'submit';
    setResolvingDuplicateAction('submit');
    try {
      if (actionGroups.deleteSample.length > 0) {
        const response = await deleteSampleFromBatch({
          batchId: Number.isFinite(Number(id)) ? Number(id) : 0,
          batchCode: id!,
          sampleCodes: actionGroups.deleteSample,
        }, { skipErrorHandler: true });
        message.success(response.message || '已删除本批次中的重复样本');
        if (response.data?.batchDeleted) {
          setDuplicateModalVisible(false);
          navigate('/result/center');
          return;
        }
      }

      if (actionGroups.retest.length > 0) {
        const response = await createBatchRetestSamples({
          batchId: Number.isFinite(Number(id)) ? Number(id) : 0,
          batchCode: id!,
          sampleCodes: actionGroups.retest,
        }, { skipErrorHandler: true });
        const created = response.data?.samples?.map((item: any) => item.sampleCode).join('、');
        message.success(created ? `复检样本已创建：${created}` : response.message || '复检样本已创建');
      }

      const response = await submitBatch(id!, {
        forceOverwrite: actionGroups.overwrite.length > 0,
        skipErrorHandler: true,
      });
      message.success(response.message || '提交成功');
      setDuplicateModalVisible(false);
      setDuplicateActions({});
      resetDuplicateCache();
      await fetchDetail({ forceDuplicateRefresh: true });
    } catch (error: any) {
      message.error(error?.message || '提交处理方案失败');
    } finally {
      resolvingDuplicateActionRef.current = '';
      setResolvingDuplicateAction('');
    }
  };

  const handleSaveGeneMatches = async () => {
    const matches = unmatchedGenes
      .map((gene) => ({ source: gene, target: geneMatchValues[gene] }))
      .filter((item) => item.source && item.target);

    if (matches.length === 0) {
      message.warning('请先选择需要匹配的目标基因');
      return;
    }

    setSavingGeneMatches(true);
    try {
      await applyBatchGeneMatches({
        batchId: id!,
        matches,
      });
      message.success('基因匹配已保存');
      await fetchDetail();
    } catch (error: any) {
      message.error(error.message || '保存基因匹配失败');
    } finally {
      setSavingGeneMatches(false);
    }
  };

  const handleDeleteSample = async (sample: any) => {
    const sampleCode = getSampleCode(sample);
    const currentSampleCodes = getCurrentBatchSampleCodes();
    const isLastSample = currentSampleCodes.length === 1 && currentSampleCodes[0] === sampleCode;
    Modal.confirm({
      title: isLastSample ? '删除最后一个样本' : '确认删除结果',
      content: isLastSample
        ? `样本 ${sampleCode} 为该批次最后一个样本，删除将同时删除该批次。样本档案不会删除。`
        : `确定要删除样本 ${sampleCode} 在本批次中的检测结果吗？样本档案不会删除。`,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await deleteSampleFromBatch({
            batchId: Number.isFinite(Number(id)) ? Number(id) : 0,
            batchCode: id!,
            sampleCode,
          });
          message.success(response.message || '结果删除成功');
          if (response.data?.batchDeleted) {
            navigate('/result/center');
            return;
          }
          // 重新获取批次详情，更新状态
          await fetchDetail({ forceDuplicateRefresh: true });
        } catch (error: any) {
          message.error(error.message || '删除结果失败');
        }
      }
    });
  };

  const handleDeleteSelectedSamples = async () => {
    const sampleCodes = selectedSampleCodes.filter((code) => code && code !== 'H');
    if (sampleCodes.length === 0) {
      message.warning('请先选择要删除结果的样本');
      return;
    }
    const currentSampleCodes = getCurrentBatchSampleCodes();
    const selectedSet = new Set(sampleCodes);
    const deletesWholeBatch = currentSampleCodes.length > 0 && currentSampleCodes.every((sampleCode) => selectedSet.has(sampleCode));
    Modal.confirm({
      title: deletesWholeBatch ? '删除批次全部样本' : '确认批量删除结果',
      content: deletesWholeBatch
        ? `选中的样本为该批次全部剩余样本，删除将同时删除该批次。样本档案不会删除。`
        : `确定要删除选中 ${sampleCodes.length} 个样本在本批次中的检测结果吗？样本档案不会删除。`,
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await deleteSampleFromBatch({
            batchId: Number.isFinite(Number(id)) ? Number(id) : 0,
            batchCode: id!,
            sampleCodes,
          });
          message.success(response.message || '结果删除成功');
          setSelectedSampleCodes([]);
          if (response.data?.batchDeleted) {
            navigate('/result/center');
            return;
          }
          await fetchDetail({ forceDuplicateRefresh: true });
        } catch (error: any) {
          message.error(error.message || '批量删除结果失败');
        }
      },
    });
  };

  const handleEditSample = (sample: any) => {
    setEditingSample(sample);
    
    // 从mergedData数组中查找对应样本的数据
    let mergedSampleData: any = {};
    if (mergedData && Array.isArray(mergedData)) {
      const found = mergedData.find((m: any) => m.sampleCode === sample.sampleCode);
      if (found) {
        mergedSampleData = found;
      }
    }
    
    editForm.setFieldsValue({
      sampleCode: sample.sampleCode,
      // 填充合并后的数据到表单
      ...mergedSampleData
    });
  };

  const handleSaveMergedData = async () => {
    try {
      const values = await editForm.validateFields();
      const updatedMergedData = { ...mergedData };
      if (editingSample) {
        updatedMergedData[editingSample.sampleCode] = values;
      }
      
      await mergeBatchData({
        batchId: parseInt(id!),
        sampleData: updatedMergedData
      });
      
      message.success('保存成功');
      setMergedData(updatedMergedData);
      setEditingSample(null);
    } catch (error: any) {
      message.error(error.message || '保存失败');
    }
  };

  const renderMultiPlatformContent = () => {
    // 获取所有基因
    const allGenes = new Set<string>();
    samples.forEach(sample => {
      if (sample.platformData) {
        Object.values(sample.platformData).forEach((platformData: any) => {
          if (platformData.median) {
            Object.keys(platformData.median).forEach(gene => {
              if (gene !== 'Sample' && gene !== 'sample_code' && gene !== 'location' && gene !== 'Location') {
                allGenes.add(gene);
              }
            });
          }
        });
      }
    });
    const geneList = Array.from(allGenes);

    return (
      <>
        {files.length > 0 && (
          <Card title="上传文件" style={{ marginBottom: 16 }}>
            <Table
              dataSource={files}
              rowKey="id"
              pagination={false}
              columns={[
                { title: '文件名', dataIndex: 'fileName', key: 'fileName' },
                { title: '平台', dataIndex: 'platform', key: 'platform' },
                { title: '协议名称', dataIndex: 'protocolName', key: 'protocolName' },
                { title: '上传人', dataIndex: 'uploadedBy', key: 'uploadedBy' },
                { title: '上传时间', dataIndex: 'createdAt', key: 'createdAt' },
                { title: '开始时间', dataIndex: 'batchStartTime', key: 'batchStartTime' },
                { title: '结束时间', dataIndex: 'batchStopTime', key: 'batchStopTime' },
                { title: '仪器SN', dataIndex: 'instrumentSn', key: 'instrumentSn' },
              ]}
            />
          </Card>
        )}

        {/* 对照水数据展示 */}
        {controlWaterData && (
          <Card title="对照水数据" style={{ marginBottom: 16 }}>
            <Collapse defaultActiveKey={[]}>
              <Panel 
                header={
                  <Space>
                    <Tag color="blue">对照水</Tag>
                    <span>{controlWaterData.sampleCode || controlWaterData.displaySampleName}</span>
                    {controlWaterData.location && <span style={{ color: '#666' }}>孔位: {controlWaterData.location}</span>}
                  </Space>
                }
                key="control-water"
                style={{ background: '#fff' }}
              >
                {controlWaterData.platformData && Object.entries(controlWaterData.platformData).map(([platform, data]: [string, any]) => (
                  <Card 
                    title={`平台: ${platform}`} 
                    size="small" 
                    style={{ marginBottom: 8 }}
                    key={platform}
                  >
                    <Tabs defaultActiveKey="median">
                      <Tabs.TabPane tab="Median (表达值)" key="median">
                        {data.median && (
                          <Table
                            dataSource={[data.median]}
                            pagination={false}
                            rowKey={() => platform}
                            columns={Object.keys(data.median)
                              .filter(key => key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location')
                              .map(gene => ({
                                title: gene,
                                dataIndex: gene,
                                key: gene,
                                render: (value: any) => {
                                  if (typeof value === 'number' && value > 100) {
                                    return <span style={{ color: '#fa8c16' }}>{value}</span>;
                                  }
                                  return value;
                                }
                              }))}
                          />
                        )}
                      </Tabs.TabPane>
                      <Tabs.TabPane tab="Count (磁珠数)" key="count">
                        {data.count && (
                          <Table
                            dataSource={[data.count]}
                            pagination={false}
                            rowKey={() => platform}
                            columns={Object.keys(data.count)
                              .filter(key => key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location')
                              .map(gene => ({
                                title: gene,
                                dataIndex: gene,
                                key: gene,
                                render: (value: any) => {
                                  if (typeof value === 'number' && value < 10) {
                                    return <span style={{ color: '#fa8c16' }}>{value}</span>;
                                  }
                                  return value;
                                }
                              }))}
                          />
                        )}
                      </Tabs.TabPane>
                    </Tabs>
                  </Card>
                ))}
              </Panel>
            </Collapse>
          </Card>
        )}

        <Card
          title="样本数据"
          style={{ marginBottom: 16 }}
          extra={
            <Button
              danger
              icon={<DeleteOutlined />}
              disabled={selectedSampleCodes.length === 0}
              onClick={handleDeleteSelectedSamples}
            >
              批量删除结果{selectedSampleCodes.length > 0 ? `（${selectedSampleCodes.length}）` : ''}
            </Button>
          }
        >
          <Collapse defaultActiveKey={[]}>
            {samples
              .filter(sample => sample.sampleCode !== 'H' && sample.displaySampleName !== 'H')
              .map((sample) => (
              <Panel 
                header={
                  <Space style={{ width: '100%', justifyContent: 'space-between' }}>
                    <Space>
                      <Checkbox
                        checked={selectedSampleCodes.includes(getSampleCode(sample))}
                        onClick={(e) => e.stopPropagation()}
                        onChange={(e) => toggleSelectedSample(getSampleCode(sample), e.target.checked)}
                      />
                      <span>{sample.sampleCode}</span>
                      {sample.platformData && Object.keys(sample.platformData).length > 0 && (
                        <Tag color="purple">{Object.keys(sample.platformData).join(', ')}</Tag>
                      )}
                      {sample.patientName && <Tag color="blue">{sample.patientName}</Tag>}
                      <Tag color={isSampleMatched(sample) ? 'green' : 'orange'}>
                        {isSampleMatched(sample) ? '已匹配' : '未匹配'}
                      </Tag>
                      {mergedData && mergedData[sample.sampleCode] && <Tag color="green">已合并</Tag>}
                      {/* 检测类型显示 */}
                      {sample.cancerTypeName ? (
                        <Tag color="blue">检测类型: {sample.cancerTypeName}</Tag>
                      ) : (
                        <Tag color="red">检测类型: 未匹配</Tag>
                      )}
                      {sample.reportId && (
                        <Button
                          type="link"
                          size="small"
                          icon={<FileTextOutlined />}
                          onClick={(event) => {
                            event.stopPropagation();
                            navigate(`/report/view/${encodeURIComponent(getSampleCode(sample))}`);
                          }}
                        >
                          查看报告
                        </Button>
                      )}
                    </Space>
                    {!sample.isControlWater && (
                      <Space>
                        {!isSampleMatched(sample) && (
                          <Button 
                            type="primary" 
                            size="small" 
                            icon={<EditOutlined />}
                            onClick={(e) => {
                              e.stopPropagation();
                              navigate(`/sample/create?sampleCode=${sample.sampleCode}&cancerTypeId=${sample.cancerTypeId || ''}`);
                            }}
                          >
                            新建样本
                          </Button>
                        )}
                        {/* 检测类型手动选择按钮 */}
                        <Button
                          type="default"
                          size="small"
                          icon={<EditOutlined />}
                          onClick={(e) => {
                            e.stopPropagation();
                            handleUpdateCancerTypeForSample(sample.sampleCode);
                          }}
                        >
                          修改检测类型
                        </Button>
                        <Button 
                          danger 
                          size="small" 
                          icon={<DeleteOutlined />}
                          onClick={(e) => {
                            e.stopPropagation();
                            handleDeleteSample(sample);
                          }}
                        >
                          删除
                        </Button>
                      </Space>
                    )}
                  </Space>
                }
                key={sample.sampleCode}
                style={{ background: '#fff' }}
              >
                <Descriptions bordered column={1} style={{ marginBottom: 16 }}>
                  <Descriptions.Item label="样本编号">{sample.sampleCode}</Descriptions.Item>
                  <Descriptions.Item label="患者ID">{sample.patientCode || sample.patientId || '-'}</Descriptions.Item>
                  <Descriptions.Item label="患者姓名">{sample.patientName || '-'}</Descriptions.Item>
                  <Descriptions.Item label="匹配状态">
                    <Tag color={isSampleMatched(sample) ? 'green' : 'orange'}>
                      {isSampleMatched(sample) ? '已匹配' : '未匹配'}
                    </Tag>
                  </Descriptions.Item>
                </Descriptions>

                {sample.platformData && Object.entries(sample.platformData).map(([platform, data]: [string, any]) => (
                  <Card 
                    title={`平台: ${platform}`} 
                    size="small" 
                    style={{ marginBottom: 8 }}
                    key={platform}
                  >
                    <Tabs defaultActiveKey="median">
                      <Tabs.TabPane tab="Median (表达值)" key="median">
                        {data.median && (
                          <Table
                            dataSource={[data.median]}
                            pagination={false}
                            rowKey={() => platform}
                            columns={Object.keys(data.median)
                              .filter(key => key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location')
                              .map(gene => ({
                                title: gene,
                                dataIndex: gene,
                                key: gene,
                                render: (value: any) => {
                                  if (typeof value === 'number' && value > 100) {
                                    return <span style={{ color: '#fa8c16' }}>{value}</span>;
                                  }
                                  return value;
                                }
                              }))}
                          />
                        )}
                      </Tabs.TabPane>
                      <Tabs.TabPane tab="Count (磁珠数)" key="count">
                        {data.count && (
                          <Table
                            dataSource={[data.count]}
                            pagination={false}
                            rowKey={() => platform}
                            columns={Object.keys(data.count)
                              .filter(key => key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location')
                              .map(gene => ({
                                title: gene,
                                dataIndex: gene,
                                key: gene,
                                render: (value: any) => {
                                  if (typeof value === 'number' && value < 10) {
                                    return <span style={{ color: '#fa8c16' }}>{value}</span>;
                                  }
                                  return value;
                                }
                              }))}
                          />
                        )}
                      </Tabs.TabPane>
                    </Tabs>
                  </Card>
                ))}

                <div style={{ marginTop: 16, display: 'flex', gap: 8 }}>
                  <Button 
                    type="primary" 
                    icon={<EditOutlined />}
                    onClick={() => handleEditSample(sample)}
                  >
                    编辑合并数据
                  </Button>
                  {!sample.isControlWater && (
                    <Button 
                      danger 
                      icon={<DeleteOutlined />}
                      onClick={() => handleDeleteSample(sample)}
                    >
                      删除结果
                    </Button>
                  )}
                </div>

                {/* 显示已合并的数据 */}
                {mergedData && mergedData[sample.sampleCode] && (
                  <Card title="已合并数据" size="small" style={{ marginTop: 16 }}>
                    <Descriptions bordered column={2}>
                      {Object.entries(mergedData[sample.sampleCode]).map(([key, value]) => (
                        <Descriptions.Item key={key} label={key}>
                          {String(value)}
                        </Descriptions.Item>
                      ))}
                    </Descriptions>
                  </Card>
                )}
              </Panel>
            ))}
          </Collapse>
        </Card>
      </>
    );
  };

  const renderOriginalContent = () => {
    const buildColumns = (type: 'median' | 'count') => {
      return [
        {
          title: '样本编号',
          dataIndex: 'displaySampleName',
          key: 'displaySampleName',
          render: (_: any, record: any) => (
            <Space>
              <span>{record.displaySampleName}</span>
              {record.platformData && Object.keys(record.platformData).length > 0 && (
                <Tag color="purple">{Object.keys(record.platformData).join(', ')}</Tag>
              )}
              {record.isControlWater && <Tag color="blue">对照</Tag>}
              {!record.isControlWater && (
                <Tag color={isSampleMatched(record) ? 'green' : 'orange'}>
                  {isSampleMatched(record) ? '已匹配' : '未匹配'}
                </Tag>
              )}
            </Space>
          ),
        },
        {
          title: '患者姓名',
          dataIndex: 'patientName',
          key: 'patientName',
          render: (v: any) => v || '-',
        },
        {
          title: '孔位',
          dataIndex: 'location',
          key: 'location',
          render: (v: any) => v || '-',
        },
        ...(type === 'count' ? [{
          title: '总磁珠数',
          dataIndex: 'totalEvents',
          key: 'totalEvents',
          render: (v: any) => v || '-',
        }] : []),
        ...originalGeneColumns.map((gene) => ({
          title: gene,
          key: `${type}_${gene}`,
          render: (_: any, record: any) => {
            const value = record?.[type]?.[gene] ?? '-';
            if (type === 'median' && typeof value === 'number' && value > 100) {
              return <span style={{ color: '#fa8c16' }}>{value}</span>;
            }
            return value;
          },
        })),
        {
          title: '操作',
          key: 'action',
          render: (_: any, record: any) => (
            <Space size="small">
              {!record.isControlWater && (
                <>
                  {!record.patientName && (
                    <Button 
                      type="primary" 
                      size="small" 
                      icon={<EditOutlined />}
                      onClick={() => navigate('/sample/create')}
                    >
                      新建样本
                    </Button>
                  )}
                  <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDeleteSample(record)}>
                    删除
                  </Button>
                </>
              )}
            </Space>
          ),
        },
      ];
    };

    const medianColumns = useMemo(() => buildColumns('median'), [originalGeneColumns]);
    const countColumns = useMemo(() => buildColumns('count'), [originalGeneColumns]);
    const visibleOriginalResults = originalResults;

    return (
      <>
        <Card
          style={{ marginBottom: 16, background: '#fff' }}
          title="Median（表达值）"
          extra={
            <Button
              danger
              icon={<DeleteOutlined />}
              disabled={selectedSampleCodes.length === 0}
              onClick={handleDeleteSelectedSamples}
            >
              批量删除结果{selectedSampleCodes.length > 0 ? `（${selectedSampleCodes.length}）` : ''}
            </Button>
          }
        >
          <Table
            rowKey={(record) => record.sampleCode || record.id}
            loading={loading}
            columns={medianColumns}
            dataSource={visibleOriginalResults}
            scroll={{ x: 'max-content' }}
            rowSelection={{
              selectedRowKeys: selectedSampleCodes,
              getCheckboxProps: (record: any) => ({ disabled: record.isControlWater || getSampleCode(record) === 'H' }),
              onChange: (keys) => setSelectedSampleCodes(keys.map((key) => String(key))),
            }}
          />
        </Card>

        <Card style={{ background: '#fff' }} title="Count（磁珠数）">
          <Table
            rowKey={(record) => record.sampleCode || record.id}
            loading={loading}
            columns={countColumns}
            dataSource={visibleOriginalResults}
            scroll={{ x: 'max-content' }}
            rowSelection={{
              selectedRowKeys: selectedSampleCodes,
              getCheckboxProps: (record: any) => ({ disabled: record.isControlWater || getSampleCode(record) === 'H' }),
              onChange: (keys) => setSelectedSampleCodes(keys.map((key) => String(key))),
            }}
          />
        </Card>
      </>
    );
  };

  const statusTextMap: Record<string, string> = {
    completed: '已完成',
    forced_completed: '强制完成',
    import_blocked: '缺失样本',
    pending: '待处理',
    submitted: '已提交',
    verified: '已检验',
    withdrawn: '批次撤回'
  };

  if (loading) {
    return <div>加载中...</div>;
  }

  if (!batch) {
    return <div>批次不存在</div>;
  }

  // 获取当前选中的样本的当前检测类型
  const getCurrentCancerTypeForSample = (sampleCode: string): string => {
    const sample = samples.find(s => s.sampleCode === sampleCode);
    return sample?.cancerTypeName || '';
  };
  const getCurrentModelForSample = (sampleCode: string): string => {
    const sample = samples.find(s => s.sampleCode === sampleCode);
    const modelId = Number(sample?.modelId || sample?.model_id || 0);
    const model = models.find((item: any) => Number(item.id) === modelId);
    return model ? `${model.name || model.modelName}${model.version ? ` [${model.version}]` : ''}` : '';
  };
  const normalizedBatchStatus = normalizeBatchStatus(batch.status);
  const canSubmitBatch = !RESETTABLE_BATCH_STATUSES.includes(normalizedBatchStatus);
  const canDeleteBatch = !RESETTABLE_BATCH_STATUSES.includes(normalizedBatchStatus);
  const canResetSubmittedBatch = RESETTABLE_BATCH_STATUSES.includes(normalizedBatchStatus);
  const canGenerateReports = normalizedBatchStatus === 'submitted';

  return (
    <div data-build-revision={BATCH_DETAIL_BUILD_REVISION}>
      {/* 检测类型选择模态框 */}
      <Modal
        title={`为样本 ${selectedSampleForCancerType} 选择检测类型`}
        open={cancerTypeModalVisible}
        onCancel={() => {
          setCancerTypeModalVisible(false);
          setSelectedCancerTypeId('');
          setSelectedModelId('');
        }}
        onOk={handleConfirmCancerTypeChange}
        confirmLoading={updatingCancerType}
        okText="确认"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8 }}><strong>当前检测类型:</strong> {getCurrentCancerTypeForSample(selectedSampleForCancerType) || '未设置'}</p>
          <p style={{ marginBottom: 8 }}><strong>当前模型:</strong> {getCurrentModelForSample(selectedSampleForCancerType) || '未设置，将按样本基因自动首选'}</p>
        </div>
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8 }}><strong>样本可用基因:</strong></p>
          {getSampleGenes(selectedSampleForCancerType).length > 0 ? (
            <Space wrap>
              {getSampleGenes(selectedSampleForCancerType).map((gene: string) => (
                <Tag key={gene} color="green">{gene}</Tag>
              ))}
            </Space>
          ) : (
            <Alert message="未找到该样本的基因数据" type="warning" showIcon />
          )}
        </div>
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8 }}><strong>可以修改的检测类型:</strong></p>
          <Select
            value={selectedCancerTypeId}
            onChange={(value) => {
              setSelectedCancerTypeId(value);
              setSelectedModelId(chooseDefaultModelId(selectedSampleForCancerType, value));
            }}
            style={{ width: '100%' }}
            placeholder="请选择检测类型"
          >
            {selectableCancerTypes.map((ct) => (
              <Option key={ct.id} value={ct.id.toString()} disabled={ct.canMatch === false}>
                {ct.name} ({ct.geneCount || 0}个基因)
                {ct.canMatch === false ? ` - 缺少基因: ${ct.missingGenes?.join(', ') || '-'}` : ''}
              </Option>
            ))}
          </Select>
          <p style={{ marginTop: 8, fontSize: 12, color: '#999' }}>
            注意：检测类型所需基因必须被该样本基因数据覆盖；仅缺少基因时才不能选择。
          </p>
        </div>
        <div style={{ marginBottom: 16 }}>
          <p style={{ marginBottom: 8 }}><strong>检测模型:</strong></p>
          <Select
            value={selectedModelId || undefined}
            onChange={setSelectedModelId}
            style={{ width: '100%' }}
            placeholder="请选择该检测类型下的模型"
            optionLabelProp="label"
          >
            {getSelectableModelsForSample(selectedSampleForCancerType, selectedCancerTypeId).map((model: any) => {
              const version = model.version ? ` [${model.version}]` : '';
              const label = `${model.name || model.modelName}${version} (${model.modelGenes.length}个基因)`;
              return (
                <Option key={model.id} value={String(model.id)} label={label} disabled={!model.canMatch}>
                  {label}{!model.canMatch ? ` - 缺少基因: ${model.missingGenes.join(', ') || '-'}` : ''}
                </Option>
              );
            })}
          </Select>
        </div>
      </Modal>

      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <Button
            icon={<ArrowLeftOutlined />}
            onClick={() => navigate('/result/center')}
            style={{ marginRight: 16 }}
          >
            返回
          </Button>
          <h2>批次详情 {isMultiPlatform && <Tag color="blue">多平台</Tag>}</h2>
        </div>
        <div>
          <Button
            type="primary"
            icon={<DownloadOutlined />}
            onClick={handleExport}
            loading={exporting}
            style={{ marginRight: 8 }}
          >
            导出结果
          </Button>
          {canGenerateReports && (
            <Button
              type="default"
              onClick={() => navigate(`/report/batch/${batch.batchCode}`)}
              style={{ marginRight: 8 }}
            >
              报告生成
            </Button>
          )}
          {canSubmitBatch && (
            <Button
              type="primary"
              onClick={handleSubmitBatch}
              loading={submitting}
              style={{ marginRight: 8 }}
            >
              提交批次
            </Button>
          )}
          {canResetSubmittedBatch && (
            <Button
              danger
              style={{ marginRight: 8 }}
              onClick={handleResetSubmittedBatch}
            >
              退回批次
            </Button>
          )}
          {canResetSubmittedBatch && (
            <Button
              danger
              style={{ marginRight: 8 }}
              disabled={selectedSampleCodes.length === 0}
              onClick={handlePartialResetSubmittedBatch}
            >
              部分退回{selectedSampleCodes.length > 0 ? `（${selectedSampleCodes.length}）` : ''}
            </Button>
          )}
          {canDeleteBatch && (
            <Button
              danger
              onClick={handleDelete}
            >
              删除批次
            </Button>
          )}
        </div>
      </div>

      {!isMultiPlatform && unmatchedGenes.length > 0 && (
        <Card
          style={{ marginBottom: 16, borderColor: '#ff4d4f' }}
          title={<span style={{ color: '#cf1322' }}>存在无法识别的基因列，请手动匹配</span>}
        >
          <Space direction="vertical" style={{ width: '100%' }} size={12}>
            {unmatchedGenes.map((gene) => (
              <Space key={gene} style={{ width: '100%', justifyContent: 'space-between' }} align="center">
                <Tag color="red">{gene}</Tag>
                <Select
                  showSearch
                  placeholder="选择 setting_gene 中的基因"
                  value={geneMatchValues[gene] || undefined}
                  style={{ width: 360 }}
                  optionFilterProp="label"
                  onChange={(value) => setGeneMatchValues((current) => ({ ...current, [gene]: value }))}
                  options={geneOptions.map((item) => ({
                    value: item.geneSymbol,
                    label: `${item.geneSymbol}${item.name ? ` / ${item.name}` : ''}`,
                  }))}
                />
              </Space>
            ))}
            <Button type="primary" danger loading={savingGeneMatches} onClick={handleSaveGeneMatches}>
              保存基因匹配
            </Button>
          </Space>
        </Card>
      )}

      {/* 校验提示框 */}
      {!isMultiPlatform && originalValidationStatus && (
        <Alert
          type={originalValidationStatus}
          showIcon
          message={originalValidationMessage}
          style={{ marginBottom: 16 }}
        />
      )}

      {/* 核验未通过提示 */}
      {!isMultiPlatform && originalBatchData?.status !== 'completed' && originalBatchData?.status !== 'forced_completed' && ((originalMissingSamples.filter(sample => sample !== 'H' && !sample.includes('H')).length > 0 || originalMissingSamples.includes('无样本')) || originalBeadCountWarnings.length > 0) && (
        <Card style={{ marginBottom: 16 }} title="核验未通过">
          {/* 缺失样本提示 */}
          {(originalMissingSamples.filter(sample => sample !== 'H' && !sample.includes('H')).length > 0 || originalMissingSamples.includes('无样本')) && (
            <div style={{ marginBottom: 16 }}>
              <p>以下样本在系统中不存在，需要创建：</p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '12px', marginTop: 8 }}>
                {originalMissingSamples.filter(sample => sample !== 'H' && !sample.includes('H')).map((sample) => (
                  <div key={sample} style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
                    {sample === '无样本' ? (
                      <Tag color="green">{sample}</Tag>
                    ) : (
                      <>
                        <Tag color="orange">
                          <span>{sample}</span>
                          <Tag color="red" style={{ marginLeft: '4px' }}>无样本</Tag>
                        </Tag>
                        <Button
                          type="link"
                          icon={<EditOutlined />}
                          onClick={() => navigate('/sample/create')}
                        >
                          新建样本
                        </Button>
                      </>
                    )}
                  </div>
                ))}
              </div>
            </div>
          )}
          
          {/* 磁珠计数过少提示 */}
          {originalBeadCountWarnings.length > 0 && (
            <div style={{ marginBottom: 16 }}>
              <p>以下样本磁珠计数过少：</p>
              <div style={{ marginTop: 8 }}>
                {originalBeadCountWarnings.map((warning, index) => (
                  <Tag key={index} color="yellow">{warning}</Tag>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}

      {/* Panel信息和匹配结果 */}
      {panel && (
        <Card style={{ marginBottom: 16 }} title="Panel信息">
          <Descriptions bordered column={2}>
            <Descriptions.Item label="Panel名称">{panel.panelName}</Descriptions.Item>
            <Descriptions.Item label="Panel编号">{panel.panelCode}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      {panelMatch && (
        <Card style={{ marginBottom: 16 }} title="Panel匹配结果">
          <Descriptions bordered column={2}>
            <Descriptions.Item label="匹配状态">
              <Tag color={
                panelMatch.matchStatus === 'exact' ? 'green' :
                panelMatch.matchStatus === 'subset' ? 'orange' : 'red'
              }>
                {panelMatch.matchStatus === 'exact' ? '完全匹配' :
                 panelMatch.matchStatus === 'subset' ? '子集匹配' : '匹配不足'}
              </Tag>
            </Descriptions.Item>
            <Descriptions.Item label="匹配率">{(panelMatch.matchRate * 100).toFixed(1)}%</Descriptions.Item>
          </Descriptions>

          {panelMatch.matchedGenes && panelMatch.matchedGenes.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <p style={{ marginBottom: 8 }}><strong>匹配的基因:</strong></p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {panelMatch.matchedGenes.map((gene: string, index: number) => (
                  <Tag key={index} color="green">{gene}</Tag>
                ))}
              </div>
            </div>
          )}

          {panelMatch.missingGenes && panelMatch.missingGenes.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <p style={{ marginBottom: 8 }}><strong>缺失的基因:</strong></p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {panelMatch.missingGenes.map((gene: string, index: number) => (
                  <Tag key={index} color="red">{gene}</Tag>
                ))}
              </div>
            </div>
          )}

          {panelMatch.extraGenes && panelMatch.extraGenes.length > 0 && (
            <div style={{ marginTop: 16 }}>
              <p style={{ marginBottom: 8 }}><strong>额外的基因:</strong></p>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                {panelMatch.extraGenes.map((gene: string, index: number) => (
                  <Tag key={index} color="orange">{gene}</Tag>
                ))}
              </div>
            </div>
          )}
        </Card>
      )}

      <Card style={{ marginBottom: 16 }}>
        <Descriptions bordered column={2} title="批次基本信息">
          <Descriptions.Item label="批次编号">{batch.batchCode}</Descriptions.Item>
          <Descriptions.Item label="状态">{statusTextMap[batch.status] || batch.status}</Descriptions.Item>
          <Descriptions.Item label="样本数量">{formatBatchSampleCount((isMultiPlatform ? samples : originalResults).filter((item) => getSampleCode(item) !== 'H').length, batch.sampleCount)}</Descriptions.Item>
          <Descriptions.Item label="上传人">{batch.uploaderName || '-'}</Descriptions.Item>
          <Descriptions.Item label="提交人">{batch.submitterName || '-'}</Descriptions.Item>
          <Descriptions.Item label="检测人员">{batch.testerName || '-'}</Descriptions.Item>
          {batch.minEvents > 0 && <Descriptions.Item label="Min Events">{batch.minEvents}</Descriptions.Item>}
          {batch.perBead > 0 && <Descriptions.Item label="Per Bead">{batch.perBead}</Descriptions.Item>}
        </Descriptions>
      </Card>

      {isMultiPlatform ? renderMultiPlatformContent() : renderOriginalContent()}

      <Modal
        title="重复样本处理"
        open={duplicateModalVisible}
        onCancel={() => setDuplicateModalVisible(false)}
        footer={[
          <Button
            key="deleteBatch"
            danger
            loading={resolvingDuplicateAction === 'deleteBatch'}
            disabled={!!resolvingDuplicateAction && resolvingDuplicateAction !== 'deleteBatch'}
            onClick={handleDeleteDuplicateBatch}
          >
            删除本批次
          </Button>,
          <Button
            key="submit"
            type="primary"
            loading={resolvingDuplicateAction === 'submit'}
            disabled={!!resolvingDuplicateAction && resolvingDuplicateAction !== 'submit'}
            onClick={handleSubmitDuplicateResolution}
          >
            提交
          </Button>,
        ]}
        width={860}
      >
        <Alert
          type="warning"
          showIcon
          message="当前批次包含已有结果的样本，请为每个样本选择处理方式后统一提交。"
          style={{ marginBottom: 16 }}
        />
        <Table
          dataSource={duplicateSamples}
          pagination={false}
          rowKey="sampleCode"
          size="small"
          columns={[
            { title: '样本编号', dataIndex: 'sampleCode' },
            { title: '患者编号', dataIndex: 'patientCode', render: (value: any) => value || '-' },
            { title: '患者姓名', dataIndex: 'patientName', render: (value: any) => value || '-' },
            { title: '当前状态', dataIndex: 'status', width: 100, render: () => <Tag color="red">已有结果</Tag> },
            {
              title: '处理方式',
              dataIndex: 'action',
              width: 180,
              render: (_: any, record: any) => {
                const sampleCode = String(record?.sampleCode || '').trim();
                return (
                  <Select
                    size="small"
                    style={{ width: '100%' }}
                    value={duplicateActions[sampleCode] || 'overwrite'}
                    disabled={!!resolvingDuplicateAction}
                    onChange={(value: DuplicateSampleAction) => {
                      setDuplicateActions((current) => ({
                        ...current,
                        [sampleCode]: value,
                      }));
                    }}
                  >
                    <Option value="deleteSample">从本批次删除</Option>
                    <Option value="retest">去复检</Option>
                    <Option value="overwrite">覆盖原来数据</Option>
                  </Select>
                );
              },
            },
          ]}
        />
      </Modal>

      {/* 编辑合并数据模态框 */}
      <Modal
        title={`编辑合并数据 - ${editingSample?.sampleCode}`}
        open={!!editingSample}
        onCancel={() => setEditingSample(null)}
        footer={[
          <Button key="cancel" onClick={() => setEditingSample(null)}>
            取消
          </Button>,
          <Button key="save" type="primary" icon={<SaveOutlined />} onClick={handleSaveMergedData}>
            保存
          </Button>
        ]}
        width={600}
      >
        {editingSample && (
          <Form form={editForm} layout="vertical">
            <Form.Item name="sampleCode" label="样本编号">
              <Input disabled />
            </Form.Item>
            {/* 动态生成基因编辑字段 */}
            {isMultiPlatform && (() => {
              const allGenes = new Set<string>();
              if (editingSample.platformData) {
                Object.values(editingSample.platformData).forEach((platformData: any) => {
                  if (platformData.median) {
                    Object.keys(platformData.median).forEach(gene => {
                      if (gene !== 'Sample' && gene !== 'sample_code' && gene !== 'location' && gene !== 'Location') {
                        allGenes.add(gene);
                      }
                    });
                  }
                });
              }
              return Array.from(allGenes).map(gene => (
                <Form.Item key={gene} name={gene} label={gene}>
                  <Input type="number" placeholder={`请输入${gene}的值`} />
                </Form.Item>
              ));
            })()}
          </Form>
        )}
      </Modal>
    </div>
  );
};

export default Detail;
