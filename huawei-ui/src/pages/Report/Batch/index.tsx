import React, { useEffect, useMemo, useRef, useState } from 'react';
import { Alert, Button, Card, Checkbox, DatePicker, Form, Input, InputNumber, Modal, Select, Space, Spin, Table, Tag, Typography, message } from 'antd';
import { CheckCircleOutlined, DownloadOutlined, ExclamationCircleOutlined, PlusOutlined, UnorderedListOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from '@umijs/max';
import { batchGenerateReports, calculateModelFormula, getBatchDetail, getPatientHistoricalReports, getSystemBootstrap } from '@/services/api';
import { formatReportProject } from '@/utils/reportProject';

const { Text } = Typography;

const metaKeys = new Set(['Sample', 'sample_code', 'location', 'Location', 'Total Events']);

type MatchInfo = {
  color: 'green' | 'orange' | 'red';
  label: string;
  selectable: boolean;
  missingGenes: string[];
  modelGenes: string[];
  exactCancerTypeMatched: boolean;
  cancerTypeMatched: boolean;
  sampleTypeMatched: boolean;
};

type SampleRow = {
  key: string;
  sampleId?: number;
  patientId?: number;
  patientName?: string;
  gender?: string;
  patientAge?: number | string;
  sampleCode: string;
  sampleCollectedAt?: string;
  organization?: string;
  cancerTypeId?: number;
  cancerTypeName?: string;
  sampleTypeId?: number;
  sampleType?: string;
  treatmentStageName?: string;
  geneData: Record<string, any>;
  score?: number;
  originalScore?: number;
  calculated?: boolean;
  selectedModelId?: number;
  reportCategory?: ReportCategory;
  signalValueExplanation?: string;
  resultExplanation?: string;
  mergeHistorical?: boolean;
  historicalReports?: any[];
  selectedHistoricalReportIds?: number[];
  remarks?: string;
};

type PreviewTrendRow = {
  id?: number;
  sampleId?: number;
  time: string;
  rawTime?: string;
  signal: number;
  trend: string;
  type: string;
  note?: string;
  sampleCode?: string;
};

type ReportTemplate = {
  id: number;
  title: string;
  content: string;
  type: 'result_explanation' | 'signal_explanation';
  modelId?: number;
  minSignalValue?: number | null;
  maxSignalValue?: number | null;
  detectionType?: string;
  valueType?: string;
  reportCategory?: string;
  project?: string;
};

type ReportCategory = 'normal' | 'high' | 'screening';

type ReportPosition = {
  x: number;
  y: number;
  width?: number;
  height?: number;
  fontSize?: number;
  align?: 'center' | 'left' | 'right';
};

type ReportPositionTemplate = {
  id: number;
  sampleTypeId?: number;
  reportType?: string;
  backgroundPath?: string;
  positions?: Record<string, ReportPosition>;
  isActive?: number;
};

const batchDetailCache = new Map<string, { promise?: Promise<any>; data?: any }>();
const patientHistoryCache = new Map<string, Promise<any[]>>();

const normalizeText = (value?: string) => String(value || '').trim().toLowerCase();
const roundScore = (value: number) => Math.round(Number(value || 0) * 10) / 10;
const formatScore = (value?: number) => {
  const numeric = Number(value);
  return Number.isFinite(numeric) ? numeric.toFixed(1) : '***';
};
const isScoreModified = (row: SampleRow) => (
  row.originalScore !== undefined
  && Number.isFinite(Number(row.originalScore))
  && Number(formatScore(row.score)) !== Number(formatScore(row.originalScore))
);

const formatDate = (value?: string) => {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return String(value).slice(0, 10);
  return date.toISOString().slice(0, 10);
};

const formatBatchSampleCount = (actualCount: number, declaredCount?: number) => {
  const declared = Number(declaredCount || 0);
  if (actualCount > 0 && declared > 0 && actualCount !== declared) {
    return `实际 ${actualCount} / 登记 ${declared}`;
  }
  return String(actualCount || declared || 0);
};

const getTimeValue = (value?: string) => {
  if (!value) return 0;
  const time = new Date(value).getTime();
  return Number.isNaN(time) ? 0 : time;
};

const parseMaybeJson = (value: any) => {
  if (!value) return null;
  if (typeof value === 'object') return value;
  try {
    return JSON.parse(value);
  } catch (_error) {
    return null;
  }
};

const getHistorySignalValue = (item: any) => {
  const directValue = item.signalValue ?? item.signal_value ?? item.signal ?? item.calculationResult ?? item.calculation_result;
  const parsedDirect = Number(directValue);
  if (directValue !== undefined && directValue !== null && directValue !== '' && Number.isFinite(parsedDirect)) {
    return parsedDirect;
  }
  const resultData = parseMaybeJson(item.resultData || item.result_data || item.reportData || item.report_data);
  const resultValue = resultData?.signalValue ?? resultData?.signal_value ?? resultData?.signal ?? resultData?.calculationResult ?? resultData?.CalculationResult ?? resultData?.score;
  const parsedResult = Number(resultValue);
  return Number.isFinite(parsedResult) ? parsedResult : 0;
};

const normalizeHistoryForReport = (item: any) => ({
  id: item.id,
  sampleId: item.sampleId || item.sample_id,
  time: item.receiveDate || item.receive_date || item.sampleReceivedAt || item.createdAt || item.generatedTime || item.time || '',
  rawTime: item.receiveDate || item.receive_date || item.sampleReceivedAt || item.createdAt || item.generatedTime || item.time || '',
  signal: getHistorySignalValue(item),
  trend: item.trend || '-',
  type: item.treatmentStageName || item.type || '-',
  note: item.remarks || item.note || '',
  sampleCode: item.sampleCode || item.sample_code || '',
});

const getSignalTrend = (current?: number, previous?: number) => {
  const currentValue = Number(current);
  const previousValue = Number(previous);
  if (!Number.isFinite(currentValue) || !Number.isFinite(previousValue)) return '-';
  if (currentValue > previousValue) return '↑';
  if (currentValue < previousValue) return '↓';
  return '-';
};

const stableStringify = (value: any): string => {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`;
};

const treatmentStageOrder = ['健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'];

const normalizeTreatmentStageName = (stageName?: string) => {
  const stage = String(stageName || '').trim();
  if (stage === '健康筛查') return '健康体检';
  if (stage === '术后' || stage === '手术后') return '术后检测';
  if (stage === '残留检测（术前中后）') return '残留检测';
  return stage;
};

const isPreoperativeStage = (stageName?: string) => normalizeTreatmentStageName(stageName).includes('术前');
const isPostoperativeStage = (stageName?: string) => normalizeTreatmentStageName(stageName) === '术后检测';

const getTreatmentStageRank = (stageName?: string) => {
  const stage = normalizeTreatmentStageName(stageName);
  const index = treatmentStageOrder.findIndex((item) => stage.includes(item));
  return index >= 0 ? index : 999;
};

const getDefaultReportCategory = (_detectionType?: string): ReportCategory => 'normal';
const normalizeReportCategory = (value?: string): ReportCategory => {
  const normalized = String(value || '').trim().toLowerCase();
  if (['high', 'super', 'supersensitive', '超敏', '超敏报告'].includes(normalized)) return 'high';
  if (['screening', 'physical', '体检', '健康筛查', '早筛', '早筛检查'].includes(normalized)) return 'screening';
  return 'normal';
};
const splitListValue = (value?: string) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean);
const listValueMatches = (actual?: string, expected?: string) => {
  const actualValues = splitListValue(actual);
  if (!actualValues.length || !expected) return true;
  return actualValues.some((item) => normalizeText(normalizeTreatmentStageName(item)) === normalizeText(normalizeTreatmentStageName(expected)));
};

const reportPreviewPositions: Record<string, { x: number; y: number; width?: number; fontSize?: number; align?: 'center' }> = {
  NameP2: { x: 30, y: 72.5, width: 28, fontSize: 10 },
  SexP2: { x: 62, y: 72.5, width: 18, fontSize: 10 },
  AgeP2: { x: 92, y: 72.5, width: 18, fontSize: 10 },
  SampleType: { x: 149, y: 71.5, width: 42, fontSize: 10 },
  SampleTime: { x: 149, y: 81, width: 42, fontSize: 10 },
  Project: { x: 30, y: 87.3, width: 50, fontSize: 10 },
  NumberID: { x: 92, y: 87.3, width: 48, fontSize: 10 },
  Organization: { x: 149, y: 90, width: 42, fontSize: 10 },
  SignalInstructions: { x: 42.5, y: 136.5, width: 150, fontSize: 10 },
  ResultInstructions: { x: 42.5, y: 154, width: 150, fontSize: 10 },
};
for (let row = 1; row <= 4; row += 1) {
  const y = 110.9 + (row - 1) * 7.25;
  reportPreviewPositions[`Time${row}`] = { x: 61.5, y, width: 27.35, fontSize: 10, align: 'center' };
  reportPreviewPositions[`Signal${row}`] = { x: 88, y, width: 17.61, fontSize: 10, align: 'center' };
  reportPreviewPositions[`Trend${row}`] = { x: 100.8, y, width: 30.48, fontSize: 10, align: 'center' };
  reportPreviewPositions[`Type${row}`] = { x: 129.2, y, width: 19.6, fontSize: 10, align: 'center' };
}

const getPreviewBackground = (category: ReportCategory) => {
  if (category === 'high') return '/Template/Template_Report/Blood_Sensitivity.jpg';
  if (category === 'screening') return '/Template/Template_Report/Physical_examination.jpg';
  return '/Template/Template_Report/Blood_Normal.jpg';
};

const getLegacyProjectByTreatmentStage = (stageName?: string): string => {
  const stage = normalizeTreatmentStageName(stageName);
  if (stage.includes('术后') || stage.includes('手术后')) return 'postoperative';
  if (stage.includes('残留')) return 'residual';
  if (stage.includes('化疗')) return 'chemo';
  if (stage.includes('复发')) return 'recurrence';
  if (stage.includes('术前') || stage.includes('健康') || stage.includes('辅助')) return 'auxiliary';
  return 'auxiliary';
};

const parseModelParameters = (model: any) => {
  if (!model?.parameters || typeof model.parameters !== 'string') return {};
  try {
    return JSON.parse(model.parameters);
  } catch (_error) {
    return {};
  }
};

const getModelName = (model: any) => model?.modelName || model?.name || '未知模型';

const getRiskText = (score?: number) => {
  const value = Number(score || 0);
  if (value < 25) return '低风险';
  if (value <= 50) return '中风险';
  return '高风险';
};

const getHistoryStage = (history: any) => String(history?.type || history?.treatmentStageName || '').trim();
const getHistorySignal = (history: any) => Number(history?.signal ?? history?.signalValue ?? 0);
const findReferenceSignal = (stageName: string | undefined, histories: any[] = []) => {
  const stage = normalizeTreatmentStageName(stageName);
  const preferred = histories.find((item) => {
    const historyStage = normalizeTreatmentStageName(getHistoryStage(item));
    if ((stage.includes('术后') || stage.includes('手术后')) && (historyStage.includes('术前') || historyStage.includes('辅助'))) return true;
    if (stage.includes('化疗后') && historyStage.includes('化疗前')) return true;
    return false;
  });
  return preferred ? getHistorySignal(preferred) : undefined;
};

const getPostoperativeResultExplanation = (diff: number) => {
  if (diff >= 10) {
    return '术后ctDNA甲基化检测信号值明显下降，表明患者体内肿瘤负荷降低，进而反映出治疗效果显著。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。';
  }
  if (diff >= 5) {
    return '术后ctDNA甲基化检测信号值有所下降，表明患者体内肿瘤负荷降低，进而反映出有一定治疗效果。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。';
  }
  if (diff >= 1) {
    return '术后ctDNA甲基化检测信号值略有下降，表明患者体内肿瘤负荷降低，进而反映出有一定治疗效果。为有效管理患者病情，建议除临床常规监测外，定期复检ctDNA甲基化。';
  }
  return '';
};

const getSignalExplanation = (category: ReportCategory) => {
  if (category === 'screening') {
    return '（a）阴性：信号值≤25，说明信号弱或未检测到标记信号，发病或复发风险低；\n（b）预警：25<信号值≤50，说明检测到少量阳性标记信号，中等发病风险，需随诊检查；\n（c）阳性：信号值>50，说明检测到较强的标记信号，发病或复发风险较高；';
  }
  const cpgCount = category === 'high' ? 180 : 98;
  return `基于多中心队列研究，本检测通过检测ctDNA甲基化异常模式(覆盖${cpgCount}个CpG甲基化位点)，结合机器学习模型生成信号值风险评分(阈值25)。分子级检测可提前影像学技术一年左右发现肿瘤病灶，但仍有可能由于病灶细胞数过少而无法检出情况。数据显示检测结果为:[低风险】(评分<25)的患者，24个月内复发率小于8%(95%CI5-12%)。[中风险】(25-50)为35%(95%CI 28-42%)【高风险】(>50)为91%(95%CI84-97%)。`;
};

const getResultExplanation = (stageName: string | undefined, score?: number, histories: any[] = []) => {
  const project = getLegacyProjectByTreatmentStage(stageName);
  const signal = Number(score || 0);
  const risk = getRiskText(signal);
  const scoreText = formatScore(signal);
  const targetMap: Record<string, string> = {
    auxiliary: '肿瘤发生',
    recurrence: '肿瘤复发',
    residual: '肿瘤残留',
    chemo: '肿瘤残留',
  };
  const target = targetMap[project] || '肿瘤发生';

  if (project === 'postoperative') {
    const referenceSignal = findReferenceSignal(stageName, histories);
    if (Number.isFinite(Number(referenceSignal))) {
      const explanation = getPostoperativeResultExplanation(Number(referenceSignal) - signal);
      if (explanation) return explanation;
    }
    return getResultExplanation('残留检测', score, histories);
  }

  if (signal < 25) {
    return `血液游离肿瘤DNA的分析信号值低于检测下限，检测信号值${scoreText}为【低风险】结果，这表明当前受检者${target}的风险较低。但应注意，此结果不排除所有风险，建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。`;
  }
  if (signal <= 30) {
    return project === 'chemo'
      ? 'MePlex检出阳性标记信号，肿瘤负荷略高。建议除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。'
      : project === 'residual' || project === 'recurrence'
        ? 'MePlex检出少量阳性标记信号，检测信号值略高于正常水平。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。'
      : `MePlex检出少量阳性标记信号，检测信号值${scoreText}为【中风险】结果，这可能暗示存在${target}的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。`;
  }
  if (signal <= 45) {
    return project === 'chemo'
      ? 'MePlex检出阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。'
      : `MePlex检出阳性标记信号，检测信号值${scoreText}为【中风险】结果，这可能暗示存在${target}的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。`;
  }
  if (signal <= 50) {
    return project === 'chemo'
      ? 'MePlex检出多个阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。'
      : `MePlex检出多个阳性标记信号，检测信号值${scoreText}为【中风险】结果，这可能暗示存在${target}的风险。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。`;
  }
  return project === 'chemo'
    ? 'MePlex检出多个阳性标记信号，肿瘤负荷较高，建议进一步进行相关治疗。除临床常规监测外，定期复检ctDNA甲基化，观察信号进行性变化情况，密切监控病情变化，以便实施早期干预。'
    : `MePlex检出多个阳性标记信号，检测信号值${scoreText}为【高风险】结果，这可能暗示${target}的风险较高。建议除临床常规监测外，定期复检ctDNA甲基化观察信号进行性变化情况。`;
};

const BatchReport: React.FC = () => {
  const { batchCode } = useParams<{ batchCode: string }>();
  const navigate = useNavigate();
  const [batch, setBatch] = useState<any>(null);
  const [models, setModels] = useState<any[]>([]);
  const [reportTemplates, setReportTemplates] = useState<ReportTemplate[]>([]);
  const [loading, setLoading] = useState(false);
  const [submitting, setSubmitting] = useState(false);
  const [sampleRows, setSampleRows] = useState<SampleRow[]>([]);
  const [reportPositionTemplates, setReportPositionTemplates] = useState<ReportPositionTemplate[]>([]);
  const [scoreLoading, setScoreLoading] = useState(false);
  const [manualHistoryVisible, setManualHistoryVisible] = useState(false);
  const [manualHistorySampleCode, setManualHistorySampleCode] = useState<string>();
  const [testPdfLoading, setTestPdfLoading] = useState(false);
  const [primarySampleByPatient, setPrimarySampleByPatient] = useState<Record<string, string>>({});
  const [manualHistoryForm] = Form.useForm();
  const loadedBatchCodeRef = useRef<string | undefined>(undefined);
  const referenceDataLoadedRef = useRef(false);
  const lastCalculationKeyRef = useRef('');

  useEffect(() => {
    if (!batchCode) return;
    if (loadedBatchCodeRef.current !== batchCode) {
      loadedBatchCodeRef.current = batchCode;
      void fetchBatchDetail();
    }
    if (!referenceDataLoadedRef.current) {
      referenceDataLoadedRef.current = true;
      void fetchReferenceData();
    }
  }, [batchCode]);

  const fetchBatchDetail = async () => {
    if (!batchCode) return;

    try {
      setLoading(true);
      const cached = batchDetailCache.get(batchCode);
      let data = cached?.data;
      if (!data) {
        const promise = cached?.promise || getBatchDetail(batchCode).then((response) => response.data);
        batchDetailCache.set(batchCode, { promise });
        data = await promise;
        batchDetailCache.set(batchCode, { data });
      }
      if (!data) return;

      setBatch(data.batch);
      const resultMap = new Map<string, any>();
      (data.results || []).forEach((result: any) => {
        if (result.sampleCode) resultMap.set(String(result.sampleCode), result);
      });

      const rows = (data.medianData || [])
        .filter((median: any) => {
          const sampleCode = median.Sample || median.sample_code;
          return sampleCode && sampleCode !== 'H';
        })
        .map((median: any, index: number) => {
          const sampleCode = String(median.Sample || median.sample_code || `样本${index + 1}`);
          const result = resultMap.get(sampleCode) || {};
          const geneData = Object.fromEntries(
            Object.entries(median).filter(([key]) => !metaKeys.has(key)),
          );
          return {
            key: sampleCode,
            sampleId: result.sampleId || result.id,
            patientId: result.patientId,
            patientName: result.patientName,
            gender: result.gender,
            patientAge: result.patientAge,
            sampleCode,
            sampleCollectedAt: result.sampleCollectedAt,
            organization: result.organization,
            cancerTypeId: Number(result.cancerTypeId || result.cancer_type_id || 0),
            cancerTypeName: result.cancerTypeName,
            sampleTypeId: Number(result.sampleTypeId || result.sample_type_id || 0),
            sampleType: result.sampleType || result.sampleTypeName,
            treatmentStageName: result.treatmentStageName,
            geneData,
            selectedModelId: Number(result.selectedModelId || result.modelId || result.model_id || 0) || undefined,
            reportCategory: normalizeReportCategory(
              result.reportType || result.report_type || getDefaultReportCategory(result.cancerTypeName),
            ),
            mergeHistorical: true,
          };
        });

      setSampleRows(rows);
      void loadHistoricalReports(rows);
    } catch (_error) {
      loadedBatchCodeRef.current = undefined;
      if (batchCode) batchDetailCache.delete(batchCode);
      message.error('获取批次详情失败');
    } finally {
      setLoading(false);
    }
  };

  const loadHistoricalReports = async (rows: SampleRow[]) => {
    const rowsWithPatient = rows.filter((row) => row.patientId);
    if (!rowsWithPatient.length) return;
    const historiesByPatient = new Map<string, any[]>();
    const patientIds = Array.from(new Set(rowsWithPatient.map((row) => String(row.patientId))));
    await Promise.all(patientIds.map(async (patientId) => {
      try {
        let promise = patientHistoryCache.get(patientId);
        if (!promise) {
          promise = getPatientHistoricalReports(patientId, {
            limit: 20,
          }, { skipErrorHandler: true }).then((response) => (Array.isArray(response.data) ? response.data : []));
          patientHistoryCache.set(patientId, promise);
        }
        const reports = await promise;
        historiesByPatient.set(patientId, reports);
      } catch (_error) {
        historiesByPatient.set(patientId, []);
      } finally {
        patientHistoryCache.delete(patientId);
      }
    }));
    setSampleRows((currentRows) => currentRows.map((row) => {
      const allReports = row.patientId ? historiesByPatient.get(String(row.patientId)) : undefined;
      const historicalReports = allReports?.filter((item: any) => {
        const itemSampleId = Number(item.sampleId || item.sample_id || 0);
        const sameSampleId = row.sampleId && itemSampleId === Number(row.sampleId);
        const sameSampleCode = item.sampleCode && String(item.sampleCode) === row.sampleCode;
        return !sameSampleId && !sameSampleCode;
      });
      if (!historicalReports) return row;
      return {
        ...row,
        historicalReports,
        selectedHistoricalReportIds: row.selectedHistoricalReportIds || historicalReports.map((item) => Number(item.id)).filter(Boolean).slice(0, 3),
      };
    }));
  };

  const fetchReferenceData = async () => {
    try {
      const response = await getSystemBootstrap({
        resources: 'models,templates,reportPositions',
        activeOnly: 1,
        includeDeprecated: 0,
      }, { skipErrorHandler: true });
      const data = response.data || {};
      setModels(Array.isArray(data.models) ? data.models : []);
      setReportTemplates(data.templates?.list || []);
      setReportPositionTemplates(Array.isArray(data.reportPositions?.list) ? data.reportPositions.list : []);
    } catch (_error) {
      referenceDataLoadedRef.current = false;
      message.error('获取报告初始化数据失败');
      setReportPositionTemplates([]);
    }
  };

  const fillScore = (content: string, score?: number) => {
    return String(content || '').replace(/\*\*\*/g, formatScore(score));
  };

  const templateMatches = (
    template: ReportTemplate,
    type: ReportTemplate['type'],
    score: number | undefined,
    options: {
      category?: string;
      project?: string;
      detectionType?: string;
      valueType?: string;
    },
  ) => {
    if (template.type !== type) return false;

    const value = Number(score);
    if ((template.minSignalValue !== undefined && template.minSignalValue !== null
      || template.maxSignalValue !== undefined && template.maxSignalValue !== null) && !Number.isFinite(value)) {
      return false;
    }
    if (template.minSignalValue !== undefined && template.minSignalValue !== null && Number.isFinite(value) && value < Number(template.minSignalValue)) {
      return false;
    }
    if (template.maxSignalValue !== undefined && template.maxSignalValue !== null && Number.isFinite(value) && value > Number(template.maxSignalValue)) {
      return false;
    }

    const checks: Array<[string | undefined, string | undefined]> = [
      [template.valueType, options.valueType],
    ];
    return checks.every(([actual, expected]) => !actual || !expected || normalizeText(actual) === normalizeText(expected))
      && listValueMatches(template.project, options.project)
      && listValueMatches(template.detectionType, options.detectionType)
      && listValueMatches(template.reportCategory, options.category);
  };

  const pickTemplate = (
    type: ReportTemplate['type'],
    score: number | undefined,
    options: {
      category?: string;
      project?: string;
      detectionType?: string;
      valueType?: string;
    },
  ) => {
    const matches = reportTemplates.filter((template) => templateMatches(template, type, score, options));
    return matches.sort((a, b) => {
      const scoreSpecificity = Number(b.minSignalValue != null) + Number(b.maxSignalValue != null)
        - Number(a.minSignalValue != null) - Number(a.maxSignalValue != null);
      if (scoreSpecificity !== 0) return scoreSpecificity;
      const bSpecificity = [b.reportCategory, b.project, b.detectionType, b.valueType].filter(Boolean).length;
      const aSpecificity = [a.reportCategory, a.project, a.detectionType, a.valueType].filter(Boolean).length;
      return bSpecificity - aSpecificity;
    })[0];
  };

  const getDefaultSignalExplanation = (category: ReportCategory, detectionType?: string) => {
    const template = pickTemplate('signal_explanation', undefined, {
      category,
      detectionType,
      valueType: 'signal',
    });
    return template?.content || getSignalExplanation(category);
  };

  const getDefaultResultExplanation = (
    stageName: string | undefined,
    detectionType?: string,
    score?: number,
    category: ReportCategory = 'normal',
    histories: any[] = [],
  ) => {
    const template = pickTemplate('result_explanation', score, {
      category,
      project: stageName,
      detectionType,
      valueType: 'signal',
    });
    return template ? fillScore(template.content, score) : getResultExplanation(stageName, score, histories);
  };

  const getModelGenes = (model: any): string[] => {
    if (Array.isArray(model?.geneSymbols)) return model.geneSymbols.filter(Boolean);
    if (typeof model?.genes === 'string' && model.genes.trim()) {
      return model.genes.split(',').map((gene: string) => gene.trim()).filter(Boolean);
    }
    return [];
  };

  const getModelSampleTypes = (model: any): string[] => {
    const params = parseModelParameters(model);
    const rawTypes = model.sampleTypes || model.applicableSampleTypes || params.sampleTypes || params.applicableSampleTypes;
    if (Array.isArray(rawTypes)) return rawTypes.map((item: any) => String(item).trim()).filter(Boolean);
    if (typeof rawTypes === 'string') return rawTypes.split(',').map((item) => item.trim()).filter(Boolean);
    return [];
  };

  const getMatchInfo = (model: any, sample: SampleRow): MatchInfo => {
    const modelGenes = getModelGenes(model);
    const sampleGenes = Object.keys(sample.geneData).filter((gene) => !metaKeys.has(gene));
    const sampleGeneSet = new Set(sampleGenes.map((gene) => normalizeText(gene)));
    const missingModelGenes = modelGenes.filter((gene) => !sampleGeneSet.has(normalizeText(gene)));
    const sampleCancerTypeId = Number((sample as any).cancerTypeId || (sample as any).cancer_type_id || 0);
    const exactCancerTypeMatched = sampleCancerTypeId > 0
      ? Number(model?.cancerTypeId || model?.cancer_type_id || 0) === sampleCancerTypeId
      : Boolean(sample.cancerTypeName) && normalizeText(model.cancerTypeName) === normalizeText(sample.cancerTypeName);
    const cancerTypeMatched = !sample.cancerTypeName || exactCancerTypeMatched
      || (Array.isArray(model.applicableItems) && model.applicableItems.some((item: string) => normalizeText(item) === normalizeText(sample.cancerTypeName)));
    const modelSampleTypes = getModelSampleTypes(model);
    const sampleTypeMatched = modelSampleTypes.length === 0 || !sample.sampleType
      || modelSampleTypes.some((item) => normalizeText(item) === normalizeText(sample.sampleType));

    if (missingModelGenes.length > 0) {
      return {
        color: 'red',
        label: '缺少模型基因',
        selectable: false,
        missingGenes: missingModelGenes,
        modelGenes,
        exactCancerTypeMatched,
        cancerTypeMatched,
        sampleTypeMatched,
      };
    }

    if (cancerTypeMatched && sampleTypeMatched) {
      return {
        color: 'green',
        label: '匹配',
        selectable: true,
        missingGenes: [],
        modelGenes,
        exactCancerTypeMatched,
        cancerTypeMatched,
        sampleTypeMatched,
      };
    }

    return {
      color: 'orange',
      label: cancerTypeMatched ? '样本类型不匹配' : '检测类型不匹配',
      selectable: true,
      missingGenes: [],
      modelGenes,
      exactCancerTypeMatched,
      cancerTypeMatched,
      sampleTypeMatched,
    };
  };

  const availableModels = useMemo(() => {
    return models.filter((model) => (
      Number(model?.isActive ?? model?.is_active) === 1 && !Number(model?.isDeprecated ?? model?.is_deprecated)
    ));
  }, [models]);
  const canBatchGenerate = String(batch?.status || '').toLowerCase() === 'submitted';
  const getPatientGroupRows = (row: SampleRow, rows: SampleRow[] = sampleRows) => {
    if (!row.patientId) return [row];
    return rows
      .filter((item) => item.patientId === row.patientId)
      .sort((a, b) => {
        const timeDiff = getTimeValue(b.sampleCollectedAt) - getTimeValue(a.sampleCollectedAt);
        if (timeDiff !== 0) return timeDiff;
        return String(b.sampleCode).localeCompare(String(a.sampleCode));
      });
  };
  const getPrimarySampleCode = (row: SampleRow, rows: SampleRow[] = sampleRows) => {
    if (!row.patientId) return row.sampleCode;
    const groupRows = getPatientGroupRows(row, rows);
    const selected = primarySampleByPatient[String(row.patientId)];
    return groupRows.some((item) => item.sampleCode === selected) ? selected : groupRows[0]?.sampleCode || row.sampleCode;
  };
  const getReportIssueRows = (rows: SampleRow[]) => rows.filter((row) => {
    if (!row.patientId) return true;
    return row.sampleCode === getPrimarySampleCode(row, rows);
  });

  const getDefaultModel = (sample: SampleRow) => {
    const matchedModels = availableModels
      .map((model) => ({ model, matchInfo: getMatchInfo(model, sample) }))
      .filter((item) => item.matchInfo.selectable)
      .sort((a, b) => {
        const exactCancerDiff = Number(b.matchInfo.exactCancerTypeMatched) - Number(a.matchInfo.exactCancerTypeMatched);
        if (exactCancerDiff) return exactCancerDiff;
        const greenDiff = Number(b.matchInfo.color === 'green') - Number(a.matchInfo.color === 'green');
        if (greenDiff) return greenDiff;
        const cancerDiff = Number(b.matchInfo.cancerTypeMatched) - Number(a.matchInfo.cancerTypeMatched);
        if (cancerDiff) return cancerDiff;
        const sampleTypeDiff = Number(b.matchInfo.sampleTypeMatched) - Number(a.matchInfo.sampleTypeMatched);
        if (sampleTypeDiff) return sampleTypeDiff;
        return getModelGenes(b.model).length - getModelGenes(a.model).length;
      });
    return matchedModels[0]?.model;
  };

  useEffect(() => {
    if (!availableModels.length || !sampleRows.length) return;

    setSampleRows((currentRows) => currentRows.map((row) => {
      if (row.selectedModelId) {
        const savedModel = availableModels.find((model) => model.id === row.selectedModelId);
        if (savedModel) {
          const savedMatch = getMatchInfo(savedModel, row);
          const hasAssignedCancerType = Boolean(row.cancerTypeName || row.cancerTypeId);
          if (savedMatch.selectable && (!hasAssignedCancerType || savedMatch.exactCancerTypeMatched)) {
            return row;
          }
        }
      }
      const defaultModel = getDefaultModel(row);
      return { ...row, selectedModelId: defaultModel?.id };
    }));
  }, [availableModels.length, sampleRows.length]);

  useEffect(() => {
    const selectedPairs = sampleRows
      .filter((row) => row.selectedModelId)
      .map((row) => `${row.sampleCode}:${row.selectedModelId}`)
      .join('|');
    if (!selectedPairs) return;
    const calculationKey = sampleRows
      .filter((row) => row.selectedModelId)
      .map((row) => `${row.sampleCode}:${row.selectedModelId}:${stableStringify(row.geneData)}`)
      .join('|');
    if (lastCalculationKeyRef.current === calculationKey) return;
    lastCalculationKeyRef.current = calculationKey;

    const calculateScores = async () => {
      try {
        setScoreLoading(true);
        const scoreBySample = new Map<string, { score?: number; originalScore?: number; calculated?: boolean }>();
        const modelIds = Array.from(new Set(sampleRows.map((row) => row.selectedModelId).filter(Boolean)));

        for (const modelId of modelIds) {
          const rowsForModel = sampleRows.filter((row) => row.selectedModelId === modelId);
          const requestBody = {
            modelId,
            rows: rowsForModel.map((row) => ({ Sample: row.sampleCode, ...row.geneData })),
          };
          const response = await calculateModelFormula(requestBody, { timeout: 15000 });
          (response.data?.results || []).forEach((result: any, index: number) => {
            const score = roundScore(Number(result.score || 0));
            scoreBySample.set(rowsForModel[index].sampleCode, {
              score,
              originalScore: score,
              calculated: result.calculated,
            });
          });
        }

        setSampleRows((currentRows) => currentRows.map((row) => ({
          ...row,
          ...scoreBySample.get(row.sampleCode),
          signalValueExplanation: getDefaultSignalExplanation(row.reportCategory || getDefaultReportCategory(row.cancerTypeName), row.cancerTypeName),
          resultExplanation: getDefaultResultExplanation(
            row.treatmentStageName,
            row.cancerTypeName,
            scoreBySample.get(row.sampleCode)?.score ?? row.score,
            row.reportCategory || getDefaultReportCategory(row.cancerTypeName),
          ),
        })));
      } catch (_error) {
        lastCalculationKeyRef.current = '';
        message.error('模型计算预览失败');
      } finally {
        setScoreLoading(false);
      }
    };

    void calculateScores();
  }, [sampleRows.map((row) => `${row.sampleCode}:${row.selectedModelId}`).join('|')]);

  const handleBatchGenerate = async () => {
    if (!batch?.id) {
      message.error('批次信息不存在');
      return;
    }
    if (!canBatchGenerate) {
      message.warning('只有批次提交后才能生成报告');
      return;
    }

    const rowsToGenerate = getReportIssueRows(sampleRows);

    const invalidRow = rowsToGenerate.find((row) => {
      const model = availableModels.find((item) => item.id === row.selectedModelId);
      return !model || !getMatchInfo(model, row).selectable;
    });
    if (invalidRow) {
      message.error(`样本 ${invalidRow.sampleCode} 未选择可用模型`);
      return;
    }

    try {
      setSubmitting(true);
      await batchGenerateReports({
        batchId: batch.id,
        reportType: 'normal',
        rows: rowsToGenerate.map((row) => {
          const previewTrendRows = getPreviewTrendRows(row);
          const selectedHistoricalReports = previewTrendRows.slice(1).map((item) => ({
            time: item.time,
            signal: item.signal,
            trend: item.trend,
            type: item.type,
            note: item.note || '',
            sampleCode: item.sampleCode || '',
            sampleId: item.sampleId,
          }));
          const reportRow = (index: number) => previewTrendRows[index] || {};
          return {
            sampleId: row.sampleId,
            sampleCode: row.sampleCode,
            patientId: row.patientId,
            selectedModelId: row.selectedModelId,
            calculationResult: roundScore(Number(row.score || 0)),
            originalCalculationResult: row.originalScore,
            mergeHistorical: row.mergeHistorical !== false,
            selectedHistoricalReports,
            geneData: row.geneData,
            sampleType: row.sampleType,
            organization: row.organization,
            reportType: row.reportCategory || 'normal',
            time1: reportRow(0).time || formatDate(row.sampleCollectedAt || new Date().toISOString()),
            signal1: Number(reportRow(0).signal ?? row.score ?? 0),
            trend1: reportRow(0).trend || '-',
            type1: reportRow(0).type || row.treatmentStageName || '-',
            note1: reportRow(0).note || '',
            remarks: row.remarks || '',
            time2: reportRow(1).time || '',
            signal2: Number(reportRow(1).signal || 0),
            trend2: reportRow(1).trend || '',
            type2: reportRow(1).type || '',
            note2: reportRow(1).note || '',
            time3: reportRow(2).time || '',
            signal3: Number(reportRow(2).signal || 0),
            trend3: reportRow(2).trend || '',
            type3: reportRow(2).type || '',
            note3: reportRow(2).note || '',
            time4: reportRow(3).time || '',
            signal4: Number(reportRow(3).signal || 0),
            trend4: reportRow(3).trend || '',
            type4: reportRow(3).type || '',
            note4: reportRow(3).note || '',
            trend: reportRow(0).trend || '-',
            resultExplanation: row.resultExplanation || getDefaultResultExplanation(
              row.treatmentStageName,
              row.cancerTypeName,
              row.score,
              row.reportCategory || getDefaultReportCategory(row.cancerTypeName),
              selectedHistoricalReports,
            ),
            signalValueExplanation: row.signalValueExplanation || getDefaultSignalExplanation(
              row.reportCategory || getDefaultReportCategory(row.cancerTypeName),
              row.cancerTypeName,
            ),
            treatmentStageName: row.treatmentStageName,
          };
        }),
      });
      message.success('批量报告生成任务已启动');
      navigate('/report/review');
    } catch (_error) {
      message.error('批量报告生成失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDownloadConciseTestPdf = async () => {
    try {
      setTestPdfLoading(true);
      const firstRow = sampleRows[0];
      const params = new URLSearchParams({
        reportType: 'normal',
        sampleTypeId: String(firstRow?.sampleTypeId || 1),
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
      message.success('测试PDF已导出');
    } catch (error) {
      console.error('导出测试PDF失败:', error);
      message.error('导出测试PDF失败');
    } finally {
      setTestPdfLoading(false);
    }
  };

  const updateRowScore = (sampleCode: string, value: number | null) => {
    setSampleRows((rows) => rows.map((row) => {
      if (row.sampleCode !== sampleCode) return row;
      const score = roundScore(Number(value || 0));
      return {
        ...row,
        score,
        originalScore: row.originalScore,
        calculated: true,
        resultExplanation: getDefaultResultExplanation(
          row.treatmentStageName,
          row.cancerTypeName,
          score,
          row.reportCategory || getDefaultReportCategory(row.cancerTypeName),
        ),
      };
    }));
  };

  const getMergeCandidates = (record: SampleRow) => {
    if (!record.patientId) return [];
    return getPatientGroupRows(record)
      .filter((row) => row.sampleCode !== record.sampleCode);
  };

  const getSelectedHistoryRows = (row: SampleRow) => {
    const sameBatchRows = row.mergeHistorical === false ? [] : getMergeCandidates(row).map((item) => ({
      time: formatDate(item.sampleCollectedAt || new Date().toISOString()),
      rawTime: item.sampleCollectedAt || new Date().toISOString(),
      signal: Number(item.score || 0),
      trend: '-',
      type: item.treatmentStageName || '-',
      note: '',
      sampleCode: item.sampleCode,
      sampleId: item.sampleId,
    }));
    const selectedRows = (row.historicalReports || [])
      .filter((item) => (row.selectedHistoricalReportIds || []).includes(Number(item.id)))
      .map(normalizeHistoryForReport);
    return [...sameBatchRows, ...selectedRows].slice(0, 3);
  };

  const getPreviewTrendRows = (row: SampleRow) => {
    const rows = [
      {
        time: formatDate(row.sampleCollectedAt || new Date().toISOString()),
        rawTime: row.sampleCollectedAt || new Date().toISOString(),
        signal: Number(row.score || 0),
        trend: '-',
        type: row.treatmentStageName || '-',
        note: row.remarks || '',
        sampleCode: row.sampleCode,
        sampleId: row.sampleId,
      },
      ...getSelectedHistoryRows(row),
    ]
      .sort((a, b) => {
        const timeDiff = getTimeValue(b.rawTime || b.time) - getTimeValue(a.rawTime || a.time);
        if (timeDiff !== 0) return timeDiff;
        return String(b.sampleCode || '').localeCompare(String(a.sampleCode || ''));
      })
      .slice(0, 4);

    return rows.map((item, index) => ({
      ...item,
      trend: index === rows.length - 1 ? '-' : getSignalTrend(item.signal, rows[index + 1]?.signal),
    }));
  };

  const openManualHistory = (sampleCode: string) => {
    setManualHistorySampleCode(sampleCode);
    manualHistoryForm.resetFields();
    setManualHistoryVisible(true);
  };

  const handleAddManualHistory = async () => {
    const values = await manualHistoryForm.validateFields();
    const sampleCode = manualHistorySampleCode;
    if (!sampleCode) return;
    const record = {
      id: -Date.now(),
      source: 'manual',
      sampleCode: values.sampleCode || '手动添加',
      createdAt: values.createdAt ? values.createdAt.format('YYYY-MM-DD') : '',
      generatedTime: values.createdAt ? values.createdAt.format('YYYY-MM-DD') : '',
      signalValue: Number(values.signalValue || 0),
      trend: values.trend || '',
      treatmentStageName: values.treatmentStageName || '',
      remarks: values.remarks || '',
    };
    setSampleRows((rows) => rows.map((row) => {
      if (row.sampleCode !== sampleCode) return row;
      const historicalReports = [record, ...(row.historicalReports || [])];
      const selectedHistoricalReportIds = [Number(record.id), ...(row.selectedHistoricalReportIds || [])].slice(0, 3);
      return { ...row, historicalReports, selectedHistoricalReportIds };
    }));
    setManualHistoryVisible(false);
    setManualHistorySampleCode(undefined);
    message.success('已添加历史记录');
  };

  const getReportPositionTemplate = (row: SampleRow) => {
    const reportType = row.reportCategory || getDefaultReportCategory(row.cancerTypeName);
    const sampleTypeId = Number(row.sampleTypeId || 0);
    const activeTemplates = reportPositionTemplates.filter((item) => Number(item.isActive ?? 1) === 1);
    return activeTemplates.find((item) => (
      normalizeText(item.reportType) === normalizeText(reportType)
      && Number(item.sampleTypeId || 0) === sampleTypeId
    )) || activeTemplates.find((item) => (
      normalizeText(item.reportType) === normalizeText(reportType)
      && Number(item.sampleTypeId || 0) === 0
    ));
  };

  const getReportPreviewPosition = (row: SampleRow, key: string) => {
    const template = getReportPositionTemplate(row);
    if (template?.positions && !(key in template.positions) && key === 'Organization') {
      return undefined;
    }
    return template?.positions?.[key] || reportPreviewPositions[key];
  };

  const getReportPreviewBackground = (row: SampleRow) => {
    const template = getReportPositionTemplate(row);
    const backgroundPath = template?.backgroundPath;
    if (backgroundPath) {
      return backgroundPath.startsWith('/') ? backgroundPath : `/${backgroundPath}`;
    }
    return getPreviewBackground(row.reportCategory || getDefaultReportCategory(row.cancerTypeName));
  };

  const renderBatchPreviewPage = (row: SampleRow) => {
    const mmToPx = 96 / 25.4;
    const scale = 0.72;
    const px = (mm: number) => `${mm * mmToPx}px`;
    const getNumber = (value: any, fallback = 0) => {
      const parsed = Number(value);
      return Number.isFinite(parsed) ? parsed : fallback;
    };
    const adjusted = (key: string) => {
      const position = getReportPreviewPosition(row, key);
      if (!position) return undefined;
      const yAdjust = key === 'SignalInstructions' || key === 'ResultInstructions' ? -1.5 : -2.0;
      const height = key === 'SignalInstructions' ? 15 : key === 'ResultInstructions' ? 28 : getNumber(position?.height, 6);
      return {
        left: px(getNumber(position?.x, 0)),
        top: px(getNumber(position?.y, 0) + yAdjust),
        width: position?.width ? px(position.width) : undefined,
        minHeight: px(height),
        fontSize: `${getNumber(position?.fontSize, 10) * mmToPx * 0.3528}px`,
        lineHeight: key === 'SignalInstructions' || key === 'ResultInstructions' ? `${5 * mmToPx}px` : px(height),
        textAlign: position?.align,
      } as React.CSSProperties;
    };
    const textStyle: React.CSSProperties = {
      position: 'absolute',
      fontSize: `${10 * mmToPx * 0.3528}px`,
      fontFamily: 'NotoSansSC, SimSun, sans-serif',
      color: '#000',
      whiteSpace: 'nowrap',
      overflow: 'visible',
    };
    const blockStyle: React.CSSProperties = {
      ...textStyle,
      whiteSpace: 'pre-wrap',
      wordBreak: 'break-all',
    };
    const previewTrendRows = getPreviewTrendRows(row);
    const selectedHistories = previewTrendRows.slice(1);
    const renderText = (key: string, value: React.ReactNode) => {
      const style = adjusted(key);
      if (!style) return null;
      return <div style={{ ...textStyle, ...style }}>{value || '-'}</div>;
    };
    const renderBlock = (key: string, value: React.ReactNode) => {
      const style = adjusted(key);
      if (!style) return null;
      return <div style={{ ...blockStyle, ...style }}>{value}</div>;
    };

    return (
      <div style={{ overflowX: 'auto', width: px(210 * scale) }}>
        <div
          style={{
            width: px(210 * scale),
            height: px(297 * scale),
            position: 'relative',
            overflow: 'hidden',
            boxShadow: '0 2px 12px rgba(0,0,0,0.12)',
          }}
        >
          <div
            style={{
              width: px(210),
              height: px(297),
              position: 'relative',
              transform: `scale(${scale})`,
              transformOrigin: 'top left',
              backgroundImage: `url("${getReportPreviewBackground(row)}")`,
              backgroundSize: '100% 100%',
              backgroundPosition: 'center',
              backgroundRepeat: 'no-repeat',
              overflow: 'hidden',
            }}
          >
            {renderText('NameP2', row.patientName || '-')}
            {renderText('SexP2', row.gender || '-')}
            {renderText('AgeP2', row.patientAge || '-')}
            {renderText('Project', formatReportProject(row.cancerTypeName, row.reportCategory || getDefaultReportCategory(row.cancerTypeName)) || '-')}
            {renderText('NumberID', row.sampleCode || '-')}
            {renderText('SampleType', row.sampleType || '-')}
            {renderText('SampleTime', formatDate(row.sampleCollectedAt))}
            {renderText('Organization', row.organization || '-')}
            {renderText('Time1', previewTrendRows[0]?.time || formatDate(row.sampleCollectedAt || new Date().toISOString()))}
            {renderText('Signal1', formatScore(previewTrendRows[0]?.signal))}
            {renderText('Trend1', previewTrendRows[0]?.trend || '-')}
            {renderText('Type1', previewTrendRows[0]?.type || row.treatmentStageName || '-')}
            {renderText('Note1', previewTrendRows[0]?.note || '-')}
            {selectedHistories.map((item, index) => {
              const rowIndex = index + 2;
              return (
                <React.Fragment key={`${row.sampleCode}-history-${index}`}>
                  {renderText(`Time${rowIndex}`, formatDate(item.time))}
                  {renderText(`Signal${rowIndex}`, formatScore(item.signal))}
                  {renderText(`Trend${rowIndex}`, item.trend || '-')}
                  {renderText(`Type${rowIndex}`, item.type || '-')}
                  {renderText(`Note${rowIndex}`, item.note || '-')}
                </React.Fragment>
              );
            })}
            {renderBlock('SignalInstructions', row.signalValueExplanation || getDefaultSignalExplanation(row.reportCategory || 'normal', row.cancerTypeName))}
            {renderBlock('ResultInstructions', row.resultExplanation || getDefaultResultExplanation(row.treatmentStageName, row.cancerTypeName, row.score, row.reportCategory || 'normal', getSelectedHistoryRows(row)))}
          </div>
        </div>
      </div>
    );
  };

  const columns = [
    { title: '样本编号', dataIndex: 'sampleCode', key: 'sampleCode', fixed: 'left' as const, width: 140 },
    { title: '检测类型', dataIndex: 'cancerTypeName', key: 'cancerTypeName', width: 140, render: (value: string) => value || '-' },
    {
      title: '报告类别',
      dataIndex: 'reportCategory',
      key: 'reportCategory',
      width: 130,
      render: (_value: ReportCategory, record: SampleRow) => {
        return (
          <Select
            style={{ width: '100%' }}
            value={record.reportCategory || getDefaultReportCategory(record.cancerTypeName)}
            onChange={(value: ReportCategory) => {
              setSampleRows((rows) => rows.map((row) => (
                row.sampleCode === record.sampleCode
                  ? {
                    ...row,
                    reportCategory: value,
                    signalValueExplanation: getDefaultSignalExplanation(value, row.cancerTypeName),
                    resultExplanation: getDefaultResultExplanation(row.treatmentStageName, row.cancerTypeName, row.score, value),
                  }
                  : row
              )));
            }}
          >
            <Select.Option value="normal">高敏</Select.Option>
            <Select.Option value="high">超敏</Select.Option>
            <Select.Option value="screening">早筛</Select.Option>
          </Select>
        );
      },
    },
    { title: '样本类型', dataIndex: 'sampleType', key: 'sampleType', width: 120, render: (value: string) => value || '-' },
    {
      title: '模型',
      dataIndex: 'selectedModelId',
      key: 'selectedModelId',
      width: 320,
      render: (_value: number, record: SampleRow) => (
        <Select
          style={{ width: '100%' }}
          placeholder="请选择模型"
          value={record.selectedModelId}
          optionLabelProp="label"
          onChange={(value) => {
            setSampleRows((rows) => rows.map((row) => (
              row.sampleCode === record.sampleCode
                ? { ...row, selectedModelId: value, score: undefined, originalScore: undefined, calculated: undefined }
                : row
            )));
          }}
        >
          {availableModels.map((model) => {
            const matchInfo = getMatchInfo(model, record);
            return (
              <Select.Option
                key={model.id}
                value={model.id}
                disabled={!matchInfo.selectable}
                label={getModelName(model)}
              >
                <Space style={{ width: '100%', justifyContent: 'space-between', display: 'flex' }}>
                  <span>{getModelName(model)}</span>
                  <Tag color={matchInfo.color}>{matchInfo.label}</Tag>
                </Space>
              </Select.Option>
            );
          })}
        </Select>
      ),
    },
    {
      title: '匹配状态',
      key: 'matchStatus',
      width: 150,
      render: (_value: any, record: SampleRow) => {
        const model = availableModels.find((item) => item.id === record.selectedModelId);
        if (!model) return <Tag>未选择</Tag>;
        const matchInfo = getMatchInfo(model, record);
        return (
          <Tag color={matchInfo.color} icon={matchInfo.color === 'green' ? <CheckCircleOutlined /> : <ExclamationCircleOutlined />}>
            {matchInfo.label}
          </Tag>
        );
      },
    },
    {
      title: '计算值',
      dataIndex: 'score',
      key: 'score',
      width: 150,
      render: (value: number, record: SampleRow) => {
        const modified = isScoreModified(record);
        return (
          <Space direction="vertical" size={2} style={{ width: '100%' }}>
            <InputNumber
              style={{ width: '100%', borderColor: modified ? '#ff4d4f' : undefined }}
              precision={1}
              value={record.calculated ? Number(formatScore(value)) : undefined}
              placeholder="-"
              status={modified ? 'error' : undefined}
              onChange={(nextValue) => updateRowScore(record.sampleCode, nextValue)}
            />
            {modified && <Text type="danger" style={{ fontSize: 12 }}>该数值已经过修改</Text>}
          </Space>
        );
      },
    },
    { title: '治疗阶段', dataIndex: 'treatmentStageName', key: 'treatmentStageName', width: 130, render: (value: string) => value || '-' },
    {
      title: '信号值说明',
      dataIndex: 'signalValueExplanation',
      key: 'signalValueExplanation',
      width: 360,
      render: (_value: string, record: SampleRow) => (
        <Input.TextArea
          rows={4}
          value={record.signalValueExplanation || getDefaultSignalExplanation(record.reportCategory || getDefaultReportCategory(record.cancerTypeName), record.cancerTypeName)}
          onChange={(event) => {
            const value = event.target.value;
            setSampleRows((rows) => rows.map((row) => (
              row.sampleCode === record.sampleCode ? { ...row, signalValueExplanation: value } : row
            )));
          }}
        />
      ),
    },
    {
      title: '结果说明',
      dataIndex: 'resultExplanation',
      key: 'resultExplanation',
      width: 420,
      render: (_value: string, record: SampleRow) => (
        <Input.TextArea
          rows={4}
          value={record.resultExplanation || getDefaultResultExplanation(
            record.treatmentStageName,
            record.cancerTypeName,
            record.score,
            record.reportCategory || getDefaultReportCategory(record.cancerTypeName),
          )}
          onChange={(event) => {
            const value = event.target.value;
            setSampleRows((rows) => rows.map((row) => (
              row.sampleCode === record.sampleCode ? { ...row, resultExplanation: value } : row
            )));
          }}
        />
      ),
    },
  ];

  const reportIssueRows = getReportIssueRows(sampleRows);

  return (
    <div style={{ padding: 24 }}>
      <Card title={`报告生成-${batchCode || ''}`}>
        {loading ? (
          <Spin size="large" style={{ display: 'block', margin: '48px auto' }} />
        ) : !batch ? (
          <Alert message="未找到批次信息" type="error" />
        ) : (
          <Space direction="vertical" size={16} style={{ width: '100%' }}>
            <div>
              <h3>批次信息</h3>
              <p>批次编号: {batch.batchCode}</p>
              <p>样本数量: {formatBatchSampleCount(sampleRows.length, batch.sampleCount)}</p>
              <p>创建时间: {batch.createdAt}</p>
            </div>
            {!canBatchGenerate && (
              <Alert message="当前批次未提交或已退回，不能生成报告" type="warning" showIcon />
            )}

            <div>
              <Text strong>报告内容预览（全部展开）</Text>
              <Spin spinning={scoreLoading}>
                <Space direction="vertical" size={20} style={{ width: '100%', marginTop: 12 }}>
                  {reportIssueRows.map((row, index) => {
                    const selectedModel = availableModels.find((model) => model.id === row.selectedModelId);
                    const matchInfo = selectedModel ? getMatchInfo(selectedModel, row) : undefined;
                    const groupRows = getPatientGroupRows(row);
                    const mergeCandidates = getMergeCandidates(row);
                    const modified = isScoreModified(row);
                    return (
                      <Card
                        key={row.sampleCode}
                        title={`报告 ${index + 1}：${row.sampleCode}`}
                        extra={matchInfo ? <Tag color={matchInfo.color}>{matchInfo.label}</Tag> : <Tag>未选择模型</Tag>}
                      >
                        <Space direction="vertical" size={16} style={{ width: '100%' }}>
                          <Space wrap size={24}>
                            <Text>样本编号：{row.sampleCode}</Text>
                            <Text>检测类型：{row.cancerTypeName || '-'}</Text>
                            <Text>样本类型：{row.sampleType || '-'}</Text>
                            <Text>治疗阶段：{row.treatmentStageName || '-'}</Text>
                            <Space size={8}>
                              <Text strong>计算值：</Text>
                              <Space direction="vertical" size={2}>
                                <InputNumber
                                  style={{ width: 120, borderColor: modified ? '#ff4d4f' : undefined }}
                                  precision={1}
                                  value={row.calculated ? Number(formatScore(row.score)) : undefined}
                                  placeholder="-"
                                  status={modified ? 'error' : undefined}
                                  onChange={(nextValue) => updateRowScore(row.sampleCode, nextValue)}
                                />
                                {modified && <Text type="danger" style={{ fontSize: 12 }}>该数值已经过修改</Text>}
                              </Space>
                            </Space>
                          </Space>
                          <Space wrap style={{ width: '100%' }}>
                            {row.patientId && groupRows.length > 1 && (
                              <Space size={8}>
                                <Text strong>主报告样本：</Text>
                                <Select
                                  style={{ minWidth: 280 }}
                                  value={getPrimarySampleCode(row)}
                                  optionLabelProp="label"
                                  onChange={(value) => setPrimarySampleByPatient((current) => ({
                                    ...current,
                                    [String(row.patientId)]: String(value),
                                  }))}
                                >
                                  {groupRows.map((item) => {
                                    const label = `${item.sampleCode}（${item.treatmentStageName || '-'}，${formatDate(item.sampleCollectedAt)}，${formatScore(item.score)}）`;
                                    return (
                                      <Select.Option key={item.sampleCode} value={item.sampleCode} label={label}>
                                        {label}
                                      </Select.Option>
                                    );
                                  })}
                                </Select>
                              </Space>
                            )}
                            <Select
                              style={{ width: 180 }}
                              value={row.reportCategory || getDefaultReportCategory(row.cancerTypeName)}
                              onChange={(value: ReportCategory) => {
                                setSampleRows((rows) => rows.map((item) => item.sampleCode === row.sampleCode ? {
                                  ...item,
                                  reportCategory: value,
                                  signalValueExplanation: getDefaultSignalExplanation(value, item.cancerTypeName),
                                  resultExplanation: getDefaultResultExplanation(item.treatmentStageName, item.cancerTypeName, item.score, value),
                                } : item));
                              }}
                            >
                              <Select.Option value="normal">高敏报告</Select.Option>
                              <Select.Option value="high">超敏报告</Select.Option>
                              <Select.Option value="screening">早筛报告</Select.Option>
                            </Select>
                            <Select
                              style={{ minWidth: 320 }}
                              placeholder="请选择模型"
                              value={row.selectedModelId}
                              onChange={(value) => {
                                setSampleRows((rows) => rows.map((item) => (
                                  item.sampleCode === row.sampleCode
                                    ? { ...item, selectedModelId: value, score: undefined, originalScore: undefined, calculated: undefined }
                                    : item
                                )));
                              }}
                            >
                              {availableModels.map((model) => {
                                const info = getMatchInfo(model, row);
                                return (
                                  <Select.Option key={model.id} value={model.id} disabled={!info.selectable}>
                                    {getModelName(model)} - {info.label}
                                  </Select.Option>
                                );
                              })}
                            </Select>
                          </Space>
                          {mergeCandidates.length > 0 && (
                            <Alert
                              type="info"
                              showIcon
                              message={
                                <Space direction="vertical" size={4}>
                                  <Checkbox
                                    checked={row.mergeHistorical !== false}
                                    onChange={(event) => setSampleRows((rows) => rows.map((item) => (
                                      item.sampleCode === row.sampleCode
                                        ? { ...item, mergeHistorical: event.target.checked }
                                        : item
                                    )))}
                                  >
                                    同批次同患者报告合并显示
                                  </Checkbox>
                                  <Text type="secondary">
                                    将带入：{mergeCandidates.map((item) => `${item.sampleCode}（${item.treatmentStageName || '-'}，${formatScore(item.score)}）`).join('；')}
                                  </Text>
                                </Space>
                              }
                            />
                          )}
                          {groupRows.length > 1 && (
                            <Card size="small" title="合并样本信号值">
                              <Space direction="vertical" size={8} style={{ width: '100%' }}>
                                {groupRows.map((item) => {
                                  const itemModified = isScoreModified(item);
                                  return (
                                    <Space key={item.sampleCode} wrap size={12}>
                                      <Text style={{ minWidth: 110 }}>{item.sampleCode}</Text>
                                      <Tag color={item.sampleCode === row.sampleCode ? 'red' : 'blue'}>
                                        {item.sampleCode === row.sampleCode ? '主报告' : '子报告'}
                                      </Tag>
                                      <Text type="secondary">{item.treatmentStageName || '-'}</Text>
                                      <InputNumber
                                        style={{ width: 120, borderColor: itemModified ? '#ff4d4f' : undefined }}
                                        precision={1}
                                        step={0.1}
                                        min={0}
                                        value={item.calculated ? Number(formatScore(item.score)) : undefined}
                                        placeholder="-"
                                        status={itemModified ? 'error' : undefined}
                                        onChange={(nextValue) => updateRowScore(item.sampleCode, nextValue)}
                                      />
                                      {itemModified && <Text type="danger" style={{ fontSize: 12 }}>已修改</Text>}
                                    </Space>
                                  );
                                })}
                              </Space>
                            </Card>
                          )}
                          <Card
                            size="small"
                            title="报告中显示的历史检测"
                            extra={(
                              <Button size="small" icon={<PlusOutlined />} onClick={() => openManualHistory(row.sampleCode)}>
                                手动添加
                              </Button>
                            )}
                          >
                            {(row.historicalReports || []).length > 0 ? (
                              <Checkbox.Group
                                value={row.selectedHistoricalReportIds || []}
                                onChange={(checkedValues) => setSampleRows((rows) => rows.map((item) => (
                                  item.sampleCode === row.sampleCode
                                    ? { ...item, selectedHistoricalReportIds: checkedValues.map((value) => Number(value)).slice(0, 3) }
                                    : item
                                )))}
                              >
                                <Space direction="vertical" size={6}>
                                  {(row.historicalReports || []).map((history) => (
                                    <Checkbox key={history.id} value={Number(history.id)}>
                                      {formatDate(history.receiveDate || history.receive_date || history.sampleReceivedAt || history.createdAt || history.generatedTime)}
                                      {'  '}
                                      {history.treatmentStageName || '-'}
                                      {'  '}
                                      信号值 {formatScore(getHistorySignalValue(history))}
                                      {history.sampleCode ? `  ${history.sampleCode}` : ''}
                                      {history.source === 'manual' ? '  手动' : ''}
                                    </Checkbox>
                                  ))}
                                </Space>
                              </Checkbox.Group>
                            ) : (
                              <Text type="secondary">暂无可选历史检测，可手动添加。</Text>
                            )}
                          </Card>
                          <Card size="small" title="报告预览">
                            {renderBatchPreviewPage(row)}
                          </Card>
                          <div>
                            <Text strong>本次备注</Text>
                            <Input.TextArea
                              rows={2}
                              maxLength={100}
                              showCount
                              style={{ marginTop: 8 }}
                              placeholder="填写后显示在报告趋势表的本次备注列"
                              value={row.remarks || ''}
                              onChange={(event) => setSampleRows((rows) => rows.map((item) => (
                                item.sampleCode === row.sampleCode ? { ...item, remarks: event.target.value } : item
                              )))}
                            />
                          </div>
                          <div>
                            <Text strong>信号值说明</Text>
                            <Input.TextArea
                              rows={5}
                              style={{ marginTop: 8 }}
                              value={row.signalValueExplanation || getDefaultSignalExplanation(row.reportCategory || 'normal', row.cancerTypeName)}
                              onChange={(event) => setSampleRows((rows) => rows.map((item) => (
                                item.sampleCode === row.sampleCode ? { ...item, signalValueExplanation: event.target.value } : item
                              )))}
                            />
                          </div>
                          <div>
                            <Text strong>结果说明</Text>
                            <Input.TextArea
                              rows={6}
                              style={{ marginTop: 8 }}
                              value={row.resultExplanation || getDefaultResultExplanation(row.treatmentStageName, row.cancerTypeName, row.score, row.reportCategory || 'normal', getSelectedHistoryRows(row))}
                              onChange={(event) => setSampleRows((rows) => rows.map((item) => (
                                item.sampleCode === row.sampleCode ? { ...item, resultExplanation: event.target.value } : item
                              )))}
                            />
                          </div>
                        </Space>
                      </Card>
                    );
                  })}
                </Space>
              </Spin>
            </div>

            {canBatchGenerate && (
              <div style={{ textAlign: 'center' }}>
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
                    icon={<UnorderedListOutlined />}
                    loading={submitting}
                    onClick={handleBatchGenerate}
                  >
                    批量生成报告
                  </Button>
                </Space>
              </div>
            )}
          </Space>
        )}
      </Card>
      <Modal
        title={`手动添加历史检测${manualHistorySampleCode ? ` - ${manualHistorySampleCode}` : ''}`}
        open={manualHistoryVisible}
        onOk={handleAddManualHistory}
        onCancel={() => {
          setManualHistoryVisible(false);
          setManualHistorySampleCode(undefined);
        }}
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
            <Input placeholder="如 术前评估、残留检测、复发监测" />
          </Form.Item>
          <Form.Item name="trend" label="趋势">
            <Select allowClear placeholder="可选">
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
    </div>
  );
};

export default BatchReport;
