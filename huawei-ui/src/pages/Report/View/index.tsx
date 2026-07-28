import React, { useEffect, useState, useRef } from 'react';
import { useParams, useNavigate } from 'react-router-dom';
import { useModel } from '@umijs/max';
import { Card, Spin, message, Descriptions, Typography, Button, Modal, Form, Input, InputNumber, Select, Collapse, Table, Progress, Drawer, List, App, Row, Col, Space, Tag, Result } from 'antd';
import { getReportById, reviewReport, updateReportStatus, updateReport, listModels, listCancerTypes, getTemplates, createTemplate, listUsers, updateSample, getTreatmentStages, getPatientHistoricalReports } from '@/services/api';
import { ArrowDownOutlined, ArrowUpOutlined, DownloadOutlined } from '@ant-design/icons';
import { CartesianGrid, Line, LineChart, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';



const { Panel } = Collapse;

const toArray = <T,>(value: any): T[] => {
  if (Array.isArray(value)) return value;
  if (typeof value === 'string' && value.trim()) {
    try {
      const parsed = JSON.parse(value);
      return Array.isArray(parsed) ? parsed : [];
    } catch (_error) {
      return [];
    }
  }
  return [];
};

const isValidReviewerUser = (user: any) => {
  const employeeId = String(user?.employee_id || user?.employeeId || user?.username || '').trim().toLowerCase();
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || '');
  return employeeId !== 'admin' && /管理员|管理|IT/.test(roleText);
};

const mustSelectRealReviewer = (user: any) => {
  const employeeId = String(user?.employee_id || user?.employeeId || '').trim().toLowerCase();
  const username = String(user?.username || '').trim().toLowerCase();
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || '');
  return employeeId === 'admin' || username === 'admin' || /实验室/.test(roleText);
};

const formatOneDecimal = (value: any) => {
  const num = Number(value);
  return Number.isFinite(num) ? num.toFixed(1) : '-';
};

const normalizeReportData = (data: any): Report => ({
  ...data,
  reporter: data?.reporter || data?.generatedBy || data?.detect_reporter || '',
  selectedHistoricalReports: toArray(data?.selectedHistoricalReports),
  historicalReports: toArray(data?.historicalReports),
  geneData: data?.geneData && typeof data.geneData === 'object' && !Array.isArray(data.geneData) ? data.geneData : {},
});

interface Report {
  id: string;
  sampleId: string;
  patientId?: string | number;
  sampleCode: string;
  reportType: string;
  reportTypeLabel?: string;
  calculationResult: number;
  originalCalculationResult?: number;
  calculationModified?: boolean;
  selectedModelId: string;
  geneData: any;
  resultExplanation: string;
  signalValueExplanation: string;
  organization?: string;
  createdAt?: string;
  updatedAt?: string;
  patientName?: string;
  gender?: string;
  patientAge?: number;
  sampleType?: string;
  sampleCollectedAt?: string;
  status?: string;
  filePath?: string;
  previewUrl?: string;
  inspector?: string;
  reporter?: string;
  reviewer?: string;
  pdfGenerationStatus?: string;
  historicalReports?: any[];
  mergeReportCandidates?: {
    id: number;
    sampleCode: string;
    time: string;
    signal: number;
    trend?: string;
    type?: string;
    note?: string;
  }[];
  selectedHistoricalReports?: {
    time: string;
    signal: number;
    trend?: string;
    type?: string;
    note?: string;
  }[];
  trend?: string;
  treatmentStageName?: string;
  time1?: string;
  signal1?: number;
  trend1?: string;
  type1?: string;
  note1?: string;
  time2?: string;
  signal2?: number;
  trend2?: string;
  type2?: string;
  note2?: string;
  time3?: string;
  signal3?: number;
  trend3?: string;
  type3?: string;
  note3?: string;
  time4?: string;
  signal4?: number;
  trend4?: string;
  type4?: string;
  note4?: string;
  remarks?: string;
}

const { Title, Paragraph } = Typography;

const ReportView: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [report, setReport] = useState<Report | null>(null);
  const [loading, setLoading] = useState<boolean>(true);
  const [loadError, setLoadError] = useState<{ status: '403' | '404' | '500'; message: string } | null>(null);
  const [editing, setEditing] = useState<boolean>(false);
  const [previewVisible, setPreviewVisible] = useState<boolean>(false);
  const [pdfLoading, setPdfLoading] = useState<boolean>(false);
  const [pdfUrl, setPdfUrl] = useState<string>('');
  const [pdfInstances, setPdfInstances] = useState<any[]>([]);
  const [pdfGenerationStatus, setPdfGenerationStatus] = useState<boolean>(false);
  const [pdfGenerationLoading, setPdfGenerationLoading] = useState<boolean>(false);
  const [pdfGenerationProgress, setPdfGenerationProgress] = useState<number>(0);
  const [pollingInterval, setPollingInterval] = useState<NodeJS.Timeout | null>(null);
  const [models, setModels] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [form] = Form.useForm();
  const [templateDrawerVisible, setTemplateDrawerVisible] = useState<boolean>(false);
  const [currentTemplateType, setCurrentTemplateType] = useState<string>('');
  const [templates, setTemplates] = useState<any[]>([]);
  const [createTemplateModalVisible, setCreateTemplateModalVisible] = useState<boolean>(false);
  const [newTemplate, setNewTemplate] = useState<{ title: string; content: string }>({ title: '', content: '' });
  const { message: appMessage } = App.useApp();
  const [editReportForm] = Form.useForm();
  const [addHistoryForm] = Form.useForm();
  const [addHistoryModalVisible, setAddHistoryModalVisible] = useState<boolean>(false);
  const [users, setUsers] = useState<any[]>([]);
  const [isLoadingUsers, setIsLoadingUsers] = useState<boolean>(false);
  const [treatmentStages, setTreatmentStages] = useState<any[]>([]);
  const [availableHistoryReports, setAvailableHistoryReports] = useState<any[]>([]);
  const [selectedExistingHistoryIds, setSelectedExistingHistoryIds] = useState<React.Key[]>([]);
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  
  // 报告预览模态框相关状态
  const [reportPreviewVisible, setReportPreviewVisible] = useState<boolean>(false);
  const [previewLoading, setPreviewLoading] = useState<boolean>(false);
  const [previewData, setPreviewData] = useState<any>(null);

  useEffect(() => {
    const fetchReport = async () => {
      if (id) {
        try {
          setLoadError(null);
          const response = await getReportById(id, { skipErrorHandler: true });
          const reportData = normalizeReportData(response.data);
          setReport(reportData);
          form.setFieldsValue({
            reportType: reportData.reportType,
            calculationResult: reportData.calculationResult,
            selectedModelId: reportData.selectedModelId,
            resultExplanation: reportData.resultExplanation,
            signalValueExplanation: reportData.signalValueExplanation,
            organization: reportData.organization,
            inspector: reportData.inspector,
            reporter: reportData.reporter,
            reviewer: reportData.reviewer,
          });
        } catch (error: any) {
          const status = Number(error?.response?.status || error?.data?.code || error?.info?.errorCode || 0);
          const messageText = status === 403
            ? '无权限查看此报告'
            : status === 404
              ? '报告不存在'
              : (error?.response?.data?.message || error?.data?.message || '获取报告失败');
          setLoadError({ status: status === 403 ? '403' : status === 404 ? '404' : '500', message: messageText });
        } finally {
          setLoading(false);
        }
      }
    };

    const fetchModelsAndCancerTypes = async () => {
      try {
        // 获取模型列表
        const modelsResponse = await listModels();
        setModels(toArray(modelsResponse.data));
        
        // 获取癌种列表
        const cancerTypesResponse = await listCancerTypes();
        setCancerTypes(toArray(cancerTypesResponse.data));
      } catch (error) {
        console.error('获取模型和癌种信息失败:', error);
      }
    };

    const fetchUsers = async () => {
      try {
        setIsLoadingUsers(true);
        const response = await listUsers();
        const data: any = response.data;
        setUsers(Array.isArray(data) ? data : (Array.isArray(data?.list) ? data.list : []));
      } catch (error) {
        console.error('获取用户列表失败:', error);
        message.error('获取用户列表失败');
      } finally {
        setIsLoadingUsers(false);
      }
    };

    const fetchTreatmentStages = async () => {
      try {
        const response = await getTreatmentStages();
        setTreatmentStages(toArray(response.data));
      } catch (error) {
        console.error('获取治疗阶段失败:', error);
      }
    };

    fetchReport();
    fetchModelsAndCancerTypes();
    fetchUsers();
    fetchTreatmentStages();
  }, [id, form]);

  // 获取模板列表
  const fetchTemplates = async (templateType: string) => {
    try {
      const response = await getTemplates({ type: templateType });
      if (response.data) {
        setTemplates(response.data.list || []);
      } else {
        appMessage.error('获取模板列表失败');
      }
    } catch (error) {
      console.error('获取模板列表失败:', error);
      appMessage.error('获取模板列表失败');
    }
  };

  // 打开模板抽屉
  const handleOpenTemplateDrawer = (templateType: string) => {
    setCurrentTemplateType(templateType);
    setTemplateDrawerVisible(true);
    fetchTemplates(templateType);
  };

  // 创建模板
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
    } catch (error) {
      console.error('创建模板失败:', error);
      appMessage.error('创建模板失败');
    }
  };

  // 使用模板
  const handleUseTemplate = (template: any) => {
    if (currentTemplateType === 'result_explanation') {
      form.setFieldsValue({ resultExplanation: template.content });
    } else if (currentTemplateType === 'signal_explanation') {
      form.setFieldsValue({ signalValueExplanation: template.content });
    }
    setTemplateDrawerVisible(false);
    appMessage.success('使用模板成功');
  };

  // 根据模型ID获取模型名字
  const getModelName = (modelId: string) => {
    if (!modelId) return '-';
    const model = models.find(m => m.id === modelId);
    return model ? model.name : modelId;
  };

  // 根据模型ID获取对应的癌种
  const getCancerTypeByModelId = (modelId: string) => {
    if (!modelId) return '-';
    const model = models.find(m => m.id === modelId);
    if (!model) return '-';
    
    const cancerType = cancerTypes.find(ct => ct.id === model.cancerTypeId);
    return cancerType ? cancerType.name : '-';
  };

  const renderTreatmentStageSelect = (placeholder = '请选择检测类型（治疗阶段）') => (
    <Select showSearch optionFilterProp="children" placeholder={placeholder}>
      {treatmentStages.map((stage: any) => (
        <Select.Option key={stage.id || stage.name} value={stage.name}>
          {stage.name}
        </Select.Option>
      ))}
    </Select>
  );

  const formatReportDate = (date?: string) => {
    if (!date) return '';
    const parsed = new Date(date);
    if (Number.isNaN(parsed.getTime())) return String(date).slice(0, 10);
    const year = parsed.getFullYear();
    const month = String(parsed.getMonth() + 1).padStart(2, '0');
    const day = String(parsed.getDate()).padStart(2, '0');
    return `${year}-${month}-${day}`;
  };

  const currentHistoryRows = () => {
    if (!report) return [];
    if (Array.isArray(report.selectedHistoricalReports) && report.selectedHistoricalReports.length > 0) {
      return report.selectedHistoricalReports.map((item) => ({
        time: formatReportDate(item.time),
        signal: Number(item.signal || 0),
        trend: item.trend || '-',
        type: item.type || '-',
        note: item.note || '',
      })).filter(item => item.time && item.signal !== 0 && item.type);
    }
    return (report.historicalReports || []).map((item) => ({
      time: formatReportDate(item.createdAt || item.generatedTime || ''),
      signal: Number(item.signalValue || 0),
      trend: item.trend || '-',
      type: item.treatmentStageName || '-',
      note: item.remarks || '',
    })).filter(item => item.time && item.signal !== 0 && item.type);
  };

  const persistHistoryRows = async (historyRows: any[]) => {
    if (!report || !id) return;
    const updateData = {
      ...form.getFieldsValue(),
      ResultExplanation: report.resultExplanation || '',
      SignalValueExplanation: report.signalValueExplanation || '',
      CalculationResult: report.calculationResult || 0,
      signal1: report.calculationResult || 0,
      SelectedHistoricalReports: historyRows,
      treatmentStageName: report.treatmentStageName || '',
      sampleType: report.sampleType || '',
      remarks: report.remarks || '',
      trend: report.trend || '-',
    };
    await updateReport(String(report.id), updateData);
    const response = await getReportById(id);
    setReport(normalizeReportData(response.data));
  };

  const moveHistoryRow = async (historyIndex: number, direction: -1 | 1) => {
    const rows = currentHistoryRows();
    const target = historyIndex + direction;
    if (historyIndex < 0 || target < 0 || target >= rows.length) return;
    const nextRows = [...rows];
    [nextRows[historyIndex], nextRows[target]] = [nextRows[target], nextRows[historyIndex]];
    await persistHistoryRows(nextRows);
    message.success('显示顺序已更新');
  };

  const handleEdit = () => {
    setEditing(true);
  };

  const handleSave = async () => {
    try {
      const values = await form.validateFields();
      
      // 检查送检单位是否有变化
      if (values.organization && values.organization !== report?.organization) {
        // 更新样本表的送检单位
        await updateSample(report?.sampleId!, { organization: values.organization });
        message.success('送检单位更新成功');
      }
      
      // 准备更新数据，包含所有新字段
      const updateData = {
        ...values,
        // 确保包含所有必要的字段
        time1: report?.time1 || '',
        signal1: typeof values.calculationResult === 'number' ? values.calculationResult : (report?.signal1 || report?.calculationResult || 0),
        trend1: report?.trend1 || '',
        type1: report?.type1 || '',
        note1: report?.note1 || '',
        time2: report?.time2 || '',
        signal2: report?.signal2 || 0,
        trend2: report?.trend2 || '',
        type2: report?.type2 || '',
        note2: report?.note2 || '',
        time3: report?.time3 || '',
        signal3: report?.signal3 || 0,
        trend3: report?.trend3 || '',
        type3: report?.type3 || '',
        note3: report?.note3 || '',
        time4: report?.time4 || '',
        signal4: report?.signal4 || 0,
        trend4: report?.trend4 || '',
        type4: report?.type4 || '',
        note4: report?.note4 || '',
        treatmentStageName: report?.treatmentStageName || '',
        sampleType: report?.sampleType || '',
        remarks: report?.remarks || '',
        trend: report?.trend || ''
      };
      console.log('更新报告数据:', updateData);
      await updateReport(String(report!.id), updateData);
      message.success('编辑成功');
      setEditing(false);
      if (id) {
        const response = await getReportById(id);
        setReport(normalizeReportData(response.data));
      }
    } catch (error) {
      console.error('保存失败:', error);
      message.error('保存失败');
    }
  };

  // 渲染PDF页面的组件
  const PdfPage = ({ pageData, pageNumber }: { pageData: any; pageNumber: number }) => {
    const canvasRef = useRef<HTMLCanvasElement>(null);

    useEffect(() => {
      const renderPage = async () => {
        if (!canvasRef.current || !pageData) return;

        try {
          const canvas = canvasRef.current;
          const context = canvas.getContext('2d');
          if (!context) return;

          // 设置渲染参数
          const pixelRatio = 2;
          const viewport = pageData.getViewport({ scale: 1.5 });

          // 设置canvas尺寸
          canvas.width = viewport.width * pixelRatio;
          canvas.height = viewport.height * pixelRatio;
          canvas.style.width = '100%';
          canvas.style.height = 'auto';

          // 渲染页面
          const renderContext = {
            canvasContext: context,
            viewport: viewport,
            transform: [pixelRatio, 0, 0, pixelRatio, 0, 0]
          };

          await pageData.render(renderContext).promise;
          console.log(`PDF页面 ${pageNumber} 渲染完成`);
        } catch (error) {
          console.error(`渲染PDF页面 ${pageNumber} 错误:`, error);
        }
      };

      renderPage();

      // 清理函数
      return () => {
        try {
          if (pageData && typeof pageData.cleanup === 'function') {
            pageData.cleanup();
          }
        } catch (error) {
          console.error('清理PDF页面错误:', error);
        }
      };
    }, [pageData, pageNumber]);

    return (
      <div style={{ width: '100%', textAlign: 'center' }}>
        <canvas ref={canvasRef} style={{ maxWidth: '100%' }} />
      </div>
    );
  };

  // 初始化PDF查看器
  const initPDFViewer = async (pdfUrl: string) => {
    if (!pdfUrl) return;

    setPdfLoading(true);
    setPdfInstances([]);

    try {
      // 动态导入pdfjs-dist，使用 @ts-ignore 忽略类型检查
      // @ts-ignore
      const PDFJS = await import('pdfjs-dist/build/pdf.js');
      
      // 设置worker
      if (typeof window !== 'undefined' && 'Worker' in window) {
        try {
          PDFJS.GlobalWorkerOptions.workerSrc = '//cdnjs.cloudflare.com/ajax/libs/pdf.js/2.16.105/pdf.worker.min.js';
        } catch (workerError) {
          console.error('Worker设置错误:', workerError);
        }
      }

      // 加载PDF文档
      const loadingTask = PDFJS.getDocument({
        url: pdfUrl,
        // 启用字体加载
        disableFontFace: false,
        // 字体加载超时
        fontLoadingTimeout: 30000,
        // 字体加载重试次数
        fontLoadingRetry: 3
      });
      const pdf = await loadingTask.promise;
      
      console.log('PDF加载成功，总页数:', pdf.numPages);

      // 只预加载第1页和第3页
      const pagesToLoad = [1, 3];
      const pages: Array<any> = [];
      
      for (const pageNum of pagesToLoad) {
        try {
          if (pageNum <= pdf.numPages) {
            const page = await pdf.getPage(pageNum);
            pages.push(page);
          } else {
            pages.push(null); // 添加null作为占位符
          }
        } catch (pageError) {
          console.error(`加载PDF页面 ${pageNum} 错误:`, pageError);
          pages.push(null); // 添加null作为占位符
        }
      }

      setPdfInstances(pages);
      console.log('PDF页面预加载完成');
    } catch (error) {
      console.error('PDF加载或渲染失败:', error);
      const errorMessage = (error as Error).message || '未知错误';
      message.error(`PDF加载失败: ${errorMessage}`);
    } finally {
      setPdfLoading(false);
    }
  };

  // 获取新的PDF预览URL
  const getNewPDFPreviewUrl = async (reportId: string) => {
    try {
      // 从后端获取新的PDF预览URL和token
      const response = await fetch(`/api/reports/${reportId}/pdf/preview`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      
      const result = await response.json();
      if (result.code === 200 && result.data && result.data.previewUrl) {
        return result.data.previewUrl;
      } else {
        throw new Error('获取PDF预览URL失败');
      }
    } catch (error) {
      console.error('获取PDF预览URL失败:', error);
      throw error;
    }
  };

  // 获取报告预览数据
  const fetchReportPreviewData = async (reportId: string) => {
    try {
      setPreviewLoading(true);
      const response = await fetch(`/api/reports/${reportId}/preview-data`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });
      
      const result = await response.json();
      if (result.code === 200 && result.data) {
        setPreviewData(result.data);
      } else {
        throw new Error('获取预览数据失败');
      }
    } catch (error) {
      console.error('获取报告预览数据失败:', error);
      message.error('获取预览数据失败');
    } finally {
      setPreviewLoading(false);
    }
  };

  // 打开报告预览模态框
  const handleOpenReportPreview = async () => {
    if (!report) return;
    
    await fetchReportPreviewData(String(report.id));
    setReportPreviewVisible(true);
  };

  // 渲染预览页面（使用背景图片+文字）
  // 日期格式化函数 - 统一使用 YYYY-MM-DD 格式
  const formatDate = (dateStr: string | undefined): string => {
    if (!dateStr) return '-';
    try {
      const date = new Date(dateStr);
      return date.toISOString().split('T')[0];
    } catch {
      return dateStr;
    }
  };

  const getPreviewNumber = (value: any, fallback = 0): number => {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : fallback;
  };

  const renderPreviewPage = (pageNum?: number) => {
    const pages = Array.isArray(previewData?.pages) ? previewData.pages : [];
    const previewPage = pageNum
      ? pages.find((page: any) => Number(page?.pageNumber) === pageNum)
      : pages.find((page: any) => page?.backgroundPath) || pages[0];
    if (!previewPage) return null;

    const mmToPx = 96 / 25.4;
    const pageWidthMm = getPreviewNumber(previewPage.pageWidth, 210);
    const pageHeightMm = getPreviewNumber(previewPage.pageHeight, 297);
    const previewBackground = previewPage.backgroundPath || '/Template/Template_Report/Blood_Normal.jpg';
    const elements = Array.isArray(previewPage.elements) ? previewPage.elements : [];
    const webAdjust = previewPage.webAdjust || {};
    const adjustX = getPreviewNumber(webAdjust.x, 0);
    const adjustY = getPreviewNumber(webAdjust.y, -1.0);

    return (
      <div
        className="report-preview-page"
        style={{
          width: `${pageWidthMm * mmToPx}px`,
          height: `${pageHeightMm * mmToPx}px`,
          position: 'relative',
          backgroundImage: `url("${previewBackground}")`,
          backgroundSize: '100% 100%',
          backgroundPosition: 'center',
          backgroundRepeat: 'no-repeat',
          overflow: 'hidden',
        }}
      >
        {elements.map((element: any, index: number) => {
          const elementType = String(element?.type || 'text');
          if (elementType !== 'text' && elementType !== 'multilineText' && elementType !== 'multiline') {
            return null;
          }
          const content = String(element?.content ?? '');
          const x = getPreviewNumber(element?.x) + adjustX;
          const elementAdjustY = element?.webAdjustY !== undefined ? getPreviewNumber(element.webAdjustY, adjustY) : adjustY;
          const y = getPreviewNumber(element?.y) + elementAdjustY;
          const width = getPreviewNumber(element?.width, 80);
          const height = getPreviewNumber(element?.height, elementType === 'text' ? 6 : 24);
          const fontSize = getPreviewNumber(element?.fontSize, 10);
          const isMultiline = elementType === 'multilineText' || elementType === 'multiline';
          const align = element?.align === 'center' ? 'center' : 'left';

          return (
            <div
              key={`${element?.key || 'element'}-${index}`}
              style={{
                position: 'absolute',
                left: `${x * mmToPx}px`,
                top: `${y * mmToPx}px`,
                width: `${width * mmToPx}px`,
                minHeight: `${height * mmToPx}px`,
                fontSize: `${fontSize * mmToPx * 0.3528}px`,
                lineHeight: isMultiline ? `${5 * mmToPx}px` : `${height * mmToPx}px`,
                fontFamily: 'NotoSansSC, SimSun, sans-serif',
                color: '#000',
                textAlign: align as any,
                whiteSpace: isMultiline ? 'pre-wrap' : 'nowrap',
                wordBreak: isMultiline ? 'break-all' : 'normal',
                overflow: 'visible',
              }}
            >
              {content}
            </div>
          );
        })}
      </div>
    );
  };

  // 重新生成PDF
  const handleRegeneratePDF = async () => {
    if (!report) return;

    setPdfGenerationLoading(true);
    setPdfGenerationStatus(false);
    setPdfGenerationProgress(0);

    try {
      // 直接调用后端的重新生成PDF接口
      await fetch(`/api/reports/${report.id}/pdf/regenerate`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({
          remarks: '重新生成PDF报告',
        }),
      });

      // 开始轮询PDF生成状态
      let progress = 0;
      const interval = setInterval(async () => {
        progress += 10;
        setPdfGenerationProgress(progress > 90 ? 90 : progress);

        try {
          // 获取报告详情，检查PDF是否生成成功
          const response = await getReportById(id!);
          const updatedReport = response.data;
          
          if (updatedReport.pdfGenerationStatus === 'success') {
            // 获取新的PDF预览URL
            const newPreviewUrl = await getNewPDFPreviewUrl(String(report.id));
            clearInterval(interval);
            setPdfGenerationProgress(100);
            setPdfGenerationStatus(true);
            setPdfGenerationLoading(false);
            
            // 重新加载PDF预览
            setPdfUrl(newPreviewUrl);
            initPDFViewer(newPreviewUrl);
            message.success('PDF生成成功');
          }
        } catch (error) {
          console.error('轮询PDF生成状态失败:', error);
        }
      }, 1000);

      setPollingInterval(interval);
    } catch (error) {
      console.error('重新生成PDF失败:', error);
      message.error('PDF重新生成失败，请稍后重试');
      setPdfGenerationLoading(false);
    }
  };

  const handlePreviewPDF = async () => {
    if (!report) return;

    setPreviewVisible(true);
    setPdfLoading(true);

    try {
      // 每次打开PDF预览时，从后端请求新的文件连接和token
      const pdfUrl = await getNewPDFPreviewUrl(String(report.id));

      console.log('尝试加载PDF URL:', pdfUrl);

      if (!pdfUrl) {
        throw new Error('PDF URL无效，无法获取PDF文件');
      }

      // 设置pdfUrl状态，初始化PDF查看器
      setPdfUrl(pdfUrl);
      initPDFViewer(pdfUrl);
    } catch (error) {
      console.error('PDF预览失败:', error);
      
      // 显示更详细的错误信息
      let errorMessage = 'PDF加载失败';
      if (error instanceof Error) {
        errorMessage += `: ${error.message}`;
      }
      
      // 提供多种解决方案的确认框
      Modal.confirm({
        title: 'PDF预览失败',
        content: (
          <div>
            <p>无法加载PDF文件，请尝试以下解决方案：</p>
            <ol>
              <li>检查PDF文件是否已正确生成</li>
              <li>确认您的网络连接正常</li>
              <li>尝试重新生成PDF文件</li>
            </ol>
            <p>错误详情: {errorMessage}</p>
          </div>
        ),
        okText: '重新生成PDF',
        cancelText: '关闭预览',
        onOk: async () => {
          try {
            setPreviewVisible(false);
            handleRegeneratePDF();
          } catch (regenerateError) {
            message.error('PDF重新生成失败，请稍后重试');
          }
        },
        onCancel: () => {
          setPreviewVisible(false);
        }
      });
    } finally {
      setPdfLoading(false);
    }
  };



  const handleReview = async (status: string) => {
    try {
      let reviewerId: number | undefined;
      if (status === 'reviewed' && mustSelectRealReviewer(currentUser)) {
        reviewerId = await new Promise<number | undefined>((resolve) => {
          let selectedReviewerId: number | undefined;
          Modal.confirm({
            title: '选择真实审核人',
            content: (
              <Select
                style={{ width: '100%', marginTop: 12 }}
                placeholder="请选择审核人"
                loading={isLoadingUsers}
                showSearch
                optionFilterProp="children"
                onChange={(value) => {
                  selectedReviewerId = Number(value);
                }}
              >
                {users.filter(isValidReviewerUser).map((user) => (
                  <Select.Option key={user.id} value={user.id}>
                    {user.real_name || user.name || user.username}（{user.employee_id || user.username || '-'}）
                  </Select.Option>
                ))}
              </Select>
            ),
            okText: '确认审核',
            cancelText: '取消',
            onOk: () => {
              if (!selectedReviewerId) {
                message.error('请选择审核人');
                return Promise.reject();
              }
              resolve(selectedReviewerId);
              return Promise.resolve();
            },
            onCancel: () => resolve(undefined),
          });
        });
        if (!reviewerId) return;
      }
      setPdfGenerationLoading(true);
      setPdfGenerationProgress(0);
      
      await reviewReport(String(report!.id), {
        status,
        rejectedReason: status === 'rejected' ? '审核退回' : '',
        remarks: status === 'reviewed' ? '审核通过' : '审核退回',
        reviewer_id: reviewerId,
      });
      
      if (status === 'reviewed') {
        message.success('审核通过，正在生成PDF');
        
        // 开始轮询PDF生成状态
        let progress = 0;
        const interval = setInterval(async () => {
          progress += 10;
          setPdfGenerationProgress(progress > 90 ? 90 : progress);

          try {
            // 获取报告详情，检查PDF是否生成成功
            const response = await getReportById(id!);
            const updatedReport = normalizeReportData(response.data);
            
            if (updatedReport.previewUrl) {
              clearInterval(interval);
              setPdfGenerationProgress(100);
              setPdfGenerationLoading(false);
              setReport(updatedReport);
              message.success('PDF生成成功');
            }
          } catch (error) {
            console.error('轮询PDF生成状态失败:', error);
          }
        }, 1000);

        // 设置超时
        setTimeout(() => {
          clearInterval(interval);
          setPdfGenerationLoading(false);
          // 无论如何都更新报告状态
          if (id) {
            getReportById(id).then(response => {
              setReport(normalizeReportData(response.data));
            });
          }
        }, 30000); // 30秒超时
      } else {
        message.success('已退回');
        setPdfGenerationLoading(false);
        if (id) {
          const response = await getReportById(id);
          setReport(normalizeReportData(response.data));
        }
      }
    } catch (error) {
      message.error('操作失败');
      setPdfGenerationLoading(false);
    }
  };

  // 添加下载PDF的功能
  const handleDownloadPDF = async (version: 'concise' | 'full' = 'full') => {
    if (!report) return;

    try {
      message.info('正在下载...');
      
      const response = await fetch(`/api/reports/${report.id}/pdf/download?version=${version}&_=${Date.now()}`, {
        method: 'GET',
        cache: 'no-store',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        throw new Error('下载失败');
      }

      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `报告_${report.sampleCode}_${report.patientName || 'unknown'}_${version === 'concise' ? '简洁版' : '完整版'}.pdf`;
      document.body.appendChild(a);
      a.click();
      document.body.removeChild(a);
      window.URL.revokeObjectURL(url);

      message.success('下载成功');
    } catch (error) {
      console.error('下载PDF失败:', error);
      message.error('下载失败，请重试');
    }
  };

  const handleDownloadInstruction = () => {
    const a = document.createElement('a');
    a.href = '/Template/ReportInstruction.pdf';
    a.download = 'ReportInstruction.pdf';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  // 编辑检测报告记录
  const handleEditReportRecord = (record: any) => {
    editReportForm.setFieldsValue({
      testDate: record.testDate,
      signalValue: record.signalValue,
      trend: record.trend,
      testType: record.testType,
      note: record.note
    });
    
    Modal.confirm({
      title: `编辑${record.isCurrent ? '本次' : '过往'}检测报告`,
      content: (
        <Form form={editReportForm} layout="vertical">
          <Form.Item name="testDate" label="检测时间" rules={[{ required: true, message: '请输入检测时间' }]}>
            <Input disabled={record.isCurrent} />
          </Form.Item>
          <Form.Item name="signalValue" label="信号值" rules={[{ required: true, message: '请输入信号值' }]}>
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="trend" label="趋势" rules={[{ required: true, message: '请选择趋势' }]}>
            <Select>
              <Select.Option value="↑">上升</Select.Option>
              <Select.Option value="↓">下降</Select.Option>
              <Select.Option value="-">稳定</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="testType" label="检测类型（治疗阶段）" rules={[{ required: true, message: '请选择检测类型' }]}>
            {renderTreatmentStageSelect()}
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      ),
      okText: '保存',
      cancelText: '取消',
      onOk: async () => {
        try {
          const values = await editReportForm.validateFields();
          
          // 准备更新数据
          const updateData = {
            ...form.getFieldsValue(),
            ResultExplanation: report?.resultExplanation || '',
            SignalValueExplanation: report?.signalValueExplanation || '',
            SelectedHistoricalReports: report?.selectedHistoricalReports || [],
            treatmentStageName: report?.treatmentStageName || '',
            sampleType: report?.sampleType || '',
            remarks: report?.remarks || ''
          };
          
          // 对于当前报告，更新主报告数据
          if (record.isCurrent) {
            updateData.CalculationResult = values.signalValue;
            updateData.signal1 = values.signalValue;
            updateData.trend = values.trend;
            updateData.trend1 = values.trend;
            updateData.time1 = values.testDate;
            updateData.type1 = values.testType;
            updateData.note1 = values.note || '';
            updateData.remarks = values.note || '';
          } else {
            // 对于历史报告，更新SelectedHistoricalReports
            const historicalReports = currentHistoryRows();
            const historyIndex = parseInt(String(record.key).replace('history-', ''), 10);
            
            if (historicalReports[historyIndex]) {
              historicalReports[historyIndex] = {
                ...historicalReports[historyIndex],
                time: values.testDate,
                signal: values.signalValue,
                trend: values.trend,
                type: values.testType,
                note: values.note
              };
              updateData.SelectedHistoricalReports = historicalReports;
            }
          }
          
          await updateReport(String(report!.id), updateData);
          message.success('编辑成功');
          
          // 重新获取报告详情
          if (id) {
            const response = await getReportById(id);
            setReport(normalizeReportData(response.data));
          }
        } catch (error) {
          console.error('编辑失败:', error);
          message.error('编辑失败');
        }
      }
    });
  };

  // 添加历史记录
  const handleAddHistory = async () => {
    addHistoryForm.resetFields();
    setSelectedExistingHistoryIds([]);
    if (report?.patientId) {
      try {
        const response = await getPatientHistoricalReports(report.patientId, {
          exclude_sample_id: report.sampleId,
          limit: 100,
        });
        setAvailableHistoryReports(toArray(response.data));
      } catch (error) {
        console.error('读取既往报告失败:', error);
        setAvailableHistoryReports([]);
        message.error('读取既往报告失败，可手工添加');
      }
    }
    setAddHistoryModalVisible(true);
  };

  // 保存历史记录
  const handleSaveHistory = async () => {
    try {
      const values = await addHistoryForm.validateFields();
      const rows = currentHistoryRows();
      const hasManualInput = Boolean(values.testDate || values.signalValue || values.testType || values.note);
      if (selectedExistingHistoryIds.length === 0 && !hasManualInput) {
        message.warning('请选择已有报告或手工填写一条记录');
        return;
      }
      if (hasManualInput && (!values.testDate || values.signalValue === undefined || values.signalValue === null || !values.testType)) {
        message.warning('手工添加时请填写检测时间、信号值和检测类型');
        return;
      }
      const existingRows = availableHistoryReports
        .filter(item => selectedExistingHistoryIds.includes(item.id))
        .map(item => ({
          time: formatReportDate(item.generatedTime || item.createdAt || ''),
          signal: Number(item.signalValue || 0),
          trend: item.trend || '-',
          type: item.treatmentStageName || '-',
          note: item.remarks || item.sampleCode || '',
        }));

      if (hasManualInput) {
        rows.push({
          time: formatReportDate(values.testDate),
          signal: Number(values.signalValue || 0),
          trend: values.trend || '-',
          type: values.testType || '-',
          note: values.note || '',
        });
      }

      const mergedRows = [...rows, ...existingRows]
        .filter(item => item.time && item.signal !== 0 && item.type)
        .filter((item, index, array) => array.findIndex(other =>
          other.time === item.time && Number(other.signal) === Number(item.signal) && other.type === item.type
        ) === index);

      await persistHistoryRows(mergedRows);
      message.success('添加历史记录成功');
      setAddHistoryModalVisible(false);
    } catch (error) {
      console.error('添加历史记录失败:', error);
      message.error('添加历史记录失败');
    }
  };

  if (loading) {
    return <Spin size="large" style={{ textAlign: 'center', marginTop: '50px' }} />;
  }

  if (loadError || !report) {
    const error = loadError || { status: '404' as const, message: '报告不存在' };
    return <Result status={error.status} title={error.message} />;
  }

  const signalTrendHistory = report.selectedHistoricalReports?.length
    ? report.selectedHistoricalReports.map((item) => ({
      date: formatReportDate(item.time),
      signalValue: Number(item.signal),
      treatmentStage: item.type || '-',
      recordType: '过往',
    }))
    : (report.historicalReports || []).map((item) => ({
      date: formatReportDate(item.createdAt || item.generatedTime),
      signalValue: Number(item.signalValue),
      treatmentStage: item.treatmentStageName || '-',
      recordType: '过往',
    }));
  const signalTrendData = [
    ...signalTrendHistory,
    {
      date: formatReportDate(report.sampleCollectedAt || report.createdAt),
      signalValue: Number(report.calculationResult || 0),
      treatmentStage: report.treatmentStageName || '-',
      recordType: '本次',
    },
  ]
    .filter((item) => item.date && Number.isFinite(item.signalValue))
    .sort((left, right) => new Date(left.date).getTime() - new Date(right.date).getTime());

  return (
    <div style={{ padding: '20px' }}>
      <Card title="报告详情">
        <Form form={form} layout="vertical">
          <Descriptions bordered column={3} style={{ marginBottom: '20px' }}>
            <Descriptions.Item label="患者姓名">{report.patientName || '-'}</Descriptions.Item>
            <Descriptions.Item label="性别">{report.gender || '-'}</Descriptions.Item>
            <Descriptions.Item label="年龄">{report.patientAge || '-'}</Descriptions.Item>
            <Descriptions.Item label="样本编号">{report.sampleCode}</Descriptions.Item>
            <Descriptions.Item label="样本类型">{report.sampleType || '-'}</Descriptions.Item>
            <Descriptions.Item label="报告类型">{editing ? (
              <Form.Item name="reportType" noStyle>
                <Select style={{ width: '100%' }}>
                  <Select.Option value="normal">高敏</Select.Option>
                  <Select.Option value="high">高敏</Select.Option>
                  <Select.Option value="screening">早筛</Select.Option>
                </Select>
              </Form.Item>
            ) : (
              report.reportTypeLabel || report.reportType || '-'
            )}</Descriptions.Item>
            <Descriptions.Item label="采样日期">{report.sampleCollectedAt || '-'}</Descriptions.Item>
            <Descriptions.Item label="送检单位" span={3}>{editing ? (
              <Form.Item name="organization" noStyle>
                <Input style={{ width: '100%' }} />
              </Form.Item>
            ) : (
              report.organization || '-'
            )}</Descriptions.Item>
            <Descriptions.Item label="计算结果" span={2}>{editing ? (
              <Form.Item name="calculationResult" noStyle>
                <InputNumber style={{ width: '100%' }} />
              </Form.Item>
            ) : (
              <Space>
                <span>{formatOneDecimal(report.calculationResult)}</span>
                {report.calculationModified && Number(report.originalCalculationResult) !== Number(report.calculationResult) && (
                  <Tag color="red">已修改【原始数值为：{Number(report.originalCalculationResult).toFixed(1)}】</Tag>
                )}
              </Space>
            )}</Descriptions.Item>
            <Descriptions.Item label="模型名称">{editing ? (
              <Form.Item name="selectedModelId" noStyle>
                <Select style={{ width: '100%' }} placeholder="请选择模型" showSearch optionFilterProp="children">
                  {models.map(model => (
                    <Select.Option key={model.id} value={model.id}>
                      {model.name || `模型${model.id}`} {model.version ? `[${String(model.version).startsWith('V') ? '' : 'V'}${model.version}]` : ''}
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>
            ) : (
              getModelName(report.selectedModelId)
            )}</Descriptions.Item>
            <Descriptions.Item label="对应癌种">{getCancerTypeByModelId(report.selectedModelId)}</Descriptions.Item>
            <Descriptions.Item label="趋势">{report.trend || '-'}</Descriptions.Item>
            <Descriptions.Item label="报告状态" span={3}>
              <span style={{
                color: report.status === 'reviewed' ? 'green' : 
                       report.status === 'rejected' ? 'red' : 
                       report.status === 'published' ? 'blue' : 'orange',
                fontWeight: 'bold'
              }}>
                {report.status === 'reviewed' ? '已审核' :
                 report.status === 'rejected' ? '已退回' :
                 report.status === 'published' ? '已发布' : '待审核'}
              </span>
            </Descriptions.Item>
          </Descriptions>

          {/* 检测报告表格 */}
          <Card title="检测报告" style={{ marginBottom: '20px' }}>
            <Table
              dataSource={[
                // 首先添加本次检测记录
                {
                  key: 'current',
                  type: '本次',
                  testDate: report.sampleCollectedAt || '',
                  signalValue: report.calculationResult || 0,
                  trend: report.trend || '-',
                  testType: report.treatmentStageName || '-',
                  note: report.remarks || '',
                  isCurrent: true
                },
                // 然后添加历史记录
                ...((report.selectedHistoricalReports && Array.isArray(report.selectedHistoricalReports) && report.selectedHistoricalReports.length > 0) ? 
                  // 使用存储的selectedHistoricalReports数据
                  report.selectedHistoricalReports.map((item, index) => ({
                    key: `history-${index}`,
                    type: '过往',
                    testDate: item.time,
                    signalValue: item.signal,
                    trend: item.trend || '-',
                    testType: item.type || '-',
                    note: item.note || '',
                    historyIndex: index,
                    isCurrent: false
                  })).filter(item => item.testDate && item.signalValue !== 0 && item.testType)
                : 
                  // 当selectedHistoricalReports为null或不是数组时，使用historicalReports作为后备
                  (report.historicalReports || []).map((item, index) => ({
                    key: `history-${index}`,
                    type: '过往',
                    testDate: item.createdAt || item.generatedTime || '',
                    signalValue: item.signalValue || 0,
                    trend: '-',
                    testType: item.treatmentStageName || '-',
                    note: item.remarks || '',
                    historyIndex: index,
                    isCurrent: false
                  })).filter(item => item.testDate && item.signalValue !== 0 && item.testType)
                )
              ]}
              columns={[
                {
                  title: '本次/过往',
                  dataIndex: 'type',
                  key: 'type',
                  render: (type: string, record: any) => (
                    <span style={{ fontWeight: record.isCurrent ? 'bold' : 'normal' }}>
                      {type}
                    </span>
                  )
                },
                {
                  title: '检测时间',
                  dataIndex: 'testDate',
                  key: 'testDate',
                  render: (date: string) => {
                    if (!date) return '-';
                    const d = new Date(date);
                    const year = d.getFullYear();
                    const month = String(d.getMonth() + 1).padStart(2, '0');
                    const day = String(d.getDate()).padStart(2, '0');
                    return `${year}-${month}-${day}`;
                  }
                },
                {
                  title: '信号值',
                  dataIndex: 'signalValue',
                  key: 'signalValue',
                  render: (value: number) => value.toFixed(1)
                },
                {
                  title: '趋势',
                  dataIndex: 'trend',
                  key: 'trend',
                  render: (trend: string) => (
                    <span style={{ 
                      color: trend === '↑' ? 'red' : trend === '↓' ? 'green' : 'black'
                    }}>
                      {trend}
                    </span>
                  )
                },
                {
                  title: '检测类型（治疗阶段）',
                  dataIndex: 'testType',
                  key: 'testType',
                },
                {
                  title: '备注',
                  dataIndex: 'note',
                  key: 'note',
                },
                {
                  title: '操作',
                  key: 'action',
                  render: (_: any, record: any) => (
                    editing ? (
                      <>
                        <Button size="small" onClick={() => handleEditReportRecord(record)} style={{ marginRight: 8 }}>
                          编辑
                        </Button>
                        {!record.isCurrent && (
                          <>
                            <Button
                              size="small"
                              icon={<ArrowUpOutlined />}
                              disabled={record.historyIndex <= 0}
                              onClick={() => moveHistoryRow(record.historyIndex, -1)}
                              style={{ marginRight: 8 }}
                            />
                            <Button
                              size="small"
                              icon={<ArrowDownOutlined />}
                              disabled={record.historyIndex >= currentHistoryRows().length - 1}
                              onClick={() => moveHistoryRow(record.historyIndex, 1)}
                              style={{ marginRight: 8 }}
                            />
                          </>
                        )}
                        {record.isCurrent && (
                          <Button size="small" type="primary" onClick={handleAddHistory}>
                            添加历史记录
                          </Button>
                        )}
                      </>
                    ) : null
                  )
                }
              ]}
              rowKey="key"
              size="small"
            />
          </Card>

          <Title level={4} style={{ marginTop: '20px' }}>患者检查波动</Title>
          <Card>
            {signalTrendData.length > 0 ? (
              <div style={{ width: '100%', height: 320 }}>
                <ResponsiveContainer width="100%" height="100%">
                  <LineChart data={signalTrendData} margin={{ top: 16, right: 24, left: 8, bottom: 8 }}>
                    <CartesianGrid strokeDasharray="3 3" />
                    <XAxis dataKey="date" />
                    <YAxis dataKey="signalValue" domain={['auto', 'auto']} />
                    <Tooltip
                      content={({ active, payload }) => {
                        if (!active || !payload?.length) return null;
                        const point = payload[0]?.payload;
                        return (
                          <div style={{ padding: '10px 12px', background: '#fff', border: '1px solid #d9d9d9', borderRadius: 6, boxShadow: '0 4px 12px rgba(0,0,0,0.12)' }}>
                            <div><strong>{point.recordType}检测</strong></div>
                            <div>检测日期：{point.date}</div>
                            <div>信号值：{formatOneDecimal(point.signalValue)}</div>
                            <div>检测阶段：{point.treatmentStage}</div>
                          </div>
                        );
                      }}
                    />
                    <Line
                      type="monotone"
                      dataKey="signalValue"
                      name="信号值"
                      stroke="#1677ff"
                      strokeWidth={3}
                      dot={{ r: 5, fill: '#1677ff' }}
                      activeDot={{ r: 8 }}
                    />
                  </LineChart>
                </ResponsiveContainer>
              </div>
            ) : (
              <div style={{ textAlign: 'center', color: '#8c8c8c', padding: 40 }}>暂无检查波动数据</div>
            )}
          </Card>

          <Title level={4} style={{ marginTop: '20px' }}>信号值说明</Title>
          <Card>
            {editing && (
              <Row align="middle" justify="space-between" style={{ marginBottom: 8 }}>
                <Col>
                  <h4>信号值说明</h4>
                </Col>
                <Col>
                  <a href="#" onClick={() => handleOpenTemplateDrawer('signal_explanation')} style={{ fontSize: '12px', color: '#1890ff' }}>使用模板</a>
                </Col>
              </Row>
            )}
            {editing ? (
              <Form.Item name="signalValueExplanation" noStyle>
                <Input.TextArea rows={4} />
              </Form.Item>
            ) : (
              <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{report.signalValueExplanation}</Paragraph>
            )}
          </Card>

          <Title level={4} style={{ marginTop: '20px' }}>结果说明</Title>
          <Card>
            {editing && (
              <Row align="middle" justify="space-between" style={{ marginBottom: 8 }}>
                <Col>
                  <h4>结果说明</h4>
                </Col>
                <Col>
                  <a href="#" onClick={() => handleOpenTemplateDrawer('result_explanation')} style={{ fontSize: '12px', color: '#1890ff' }}>使用模板</a>
                </Col>
              </Row>
            )}
            {editing ? (
              <Form.Item name="resultExplanation" noStyle>
                <Input.TextArea rows={4} />
              </Form.Item>
            ) : (
              <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{report.resultExplanation}</Paragraph>
            )}
          </Card>
        </Form>

        <Collapse defaultActiveKey={[]}>
          <Panel header="基因数据" key="geneData">
            <Card>
              {report.geneData ? (
                <Table 
                  dataSource={Object.entries(report.geneData).map(([key, value]) => ({
                    key,
                    gene: key,
                    value: value
                  }))}
                  columns={[
                    {
                      title: '基因名称',
                      dataIndex: 'gene',
                      key: 'gene',
                    },
                    {
                      title: '信号值',
                      dataIndex: 'value',
                      key: 'value',
                    },
                  ]}
                  pagination={false}
                  size="small"
                />
              ) : (
                <div style={{ textAlign: 'center', padding: '20px' }}>无基因数据</div>
              )}
            </Card>
          </Panel>
        </Collapse>

        <Descriptions bordered column={3} style={{ marginTop: '20px' }}>
          <Descriptions.Item label="检验者">
            {editing ? (
              <Form.Item name="inspector" noStyle>
                <Select style={{ width: '100%' }} placeholder="请选择检验者">
                  {users.map(user => (
                    <Select.Option key={user.id} value={user.username}>{user.username}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            ) : (
              report.inspector || '-'
            )}
          </Descriptions.Item>
          <Descriptions.Item label="报告人">
            {editing ? (
              <Form.Item name="reporter" noStyle>
                <Select style={{ width: '100%' }} placeholder="请选择报告人">
                  {users.map(user => (
                    <Select.Option key={user.id} value={user.username}>{user.username}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            ) : (
              report.reporter || '-'
            )}
          </Descriptions.Item>
          <Descriptions.Item label="审核人">
            {editing ? (
              <Form.Item name="reviewer" noStyle>
                <Select style={{ width: '100%' }} placeholder="请选择审核人">
                  {users.filter(isValidReviewerUser).map(user => (
                    <Select.Option key={user.id} value={user.username}>{user.real_name || user.name || user.username}（{user.employee_id || user.username || '-'}）</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            ) : (
              report.reviewer || '-'
            )}
          </Descriptions.Item>
        </Descriptions>

        <div style={{ marginTop: '30px', textAlign: 'right' }}>
          {editing ? (
            <>
              <Button onClick={() => setEditing(false)} style={{ marginRight: '10px' }}>
                取消
              </Button>
              <Button type="primary" onClick={handleSave}>
                保存
              </Button>
            </>
          ) : (
            <>
              <Button onClick={handleEdit} style={{ marginRight: '10px' }}>
                编辑
              </Button>
              {report.status !== 'reviewed' && report.status !== 'published' && (
                <Button 
                  type="primary" 
                  style={{ marginRight: '10px' }}
                  onClick={() => handleReview('reviewed')}
                >
                  审核通过
                </Button>
              )}
              {(report.status === 'reviewed' || report.status === 'published') && (
                <Button 
                  style={{ marginRight: '10px' }}
                  onClick={() => handleReview('pending')}
                >
                  反审核
                </Button>
              )}
              {report.status !== 'rejected' && (
                <Button 
                  danger
                  style={{ marginRight: '10px' }}
                  onClick={() => handleReview('rejected')}
                >
                  退回
                </Button>
              )}
              {(report.status === 'reviewed' || report.status === 'published') && (
                <>
                  <Button 
                    style={{ marginRight: '10px' }}
                    onClick={handleOpenReportPreview}
                  >
                    预览报告
                  </Button>
                  <Button 
                    style={{ marginRight: '10px' }}
                    onClick={() => handleDownloadPDF('concise')}
                  >
                    下载简洁报告
                  </Button>
                  <Button 
                    style={{ marginRight: '10px' }}
                    onClick={handleDownloadInstruction}
                  >
                    下载说明书
                  </Button>
                  <Button 
                    type="primary"
                    style={{ marginRight: '10px' }}
                    onClick={() => handleDownloadPDF('full')}
                  >
                    下载完整报告
                  </Button>
                </>
      )}
            </>
          )}
        </div>
      </Card>

      <Modal
        title={
          <div style={{ display: 'flex', alignItems: 'center' }}>
            <span>PDF预览</span>
            <Button 
              size="small" 
              onClick={handleRegeneratePDF}
              disabled={pdfGenerationLoading}
              style={{ marginLeft: '20px' }}
            >
              {pdfGenerationLoading ? '生成中...' : '重新生成PDF'}
            </Button>
          </div>
        }
        open={previewVisible}
        onCancel={() => setPreviewVisible(false)}
        footer={[
          <Button key="close" onClick={() => setPreviewVisible(false)}>
            关闭
          </Button>,
          <Space key="downloads">
            <Button onClick={() => handleDownloadPDF('concise')} disabled={pdfLoading}>
              下载简洁报告
            </Button>
            <Button onClick={handleDownloadInstruction} disabled={pdfLoading}>
              下载说明书
            </Button>
            <Button type="primary" onClick={() => handleDownloadPDF('full')} disabled={pdfLoading}>
              下载完整报告
            </Button>
          </Space>
        ]}
        width={800}
        style={{ top: 20 }}
      >
        <div style={{ height: '600px', overflow: 'auto', position: 'relative' }}>
          {pdfLoading ? (
            <div style={{ textAlign: 'center', padding: '50px' }}>
              <Spin size="large" />
              <p style={{ marginTop: '20px' }}>加载PDF中，请稍候...</p>
            </div>
          ) : pdfUrl ? (
            <div style={{ width: '100%' }}>
              {pdfInstances.map((pageData, index) => {
                const pageNumber = [1, 3][index];
                return pageData ? (
                  <div key={pageNumber} style={{
                    marginBottom: '20px',
                    boxShadow: '0 2px 8px rgba(0, 0, 0, 0.1)',
                    padding: '10px',
                    backgroundColor: '#fff',
                    textAlign: 'center'
                  }}>
                    <h4>第 {pageNumber} 页</h4>
                    <PdfPage pageData={pageData} pageNumber={pageNumber} />
                  </div>
                ) : (
                  <div key={pageNumber} style={{
                    marginBottom: '20px',
                    padding: '20px',
                    backgroundColor: '#f5f5f5',
                    textAlign: 'center'
                  }}>
                    <p>第 {pageNumber} 页不存在</p>
                  </div>
                );
              })}
            </div>
          ) : (
            <div style={{ textAlign: 'center', padding: '50px' }}>
              <p>请等待PDF文件加载...</p>
            </div>
          )}
          {pdfGenerationLoading && (
            <div style={{
              position: 'absolute',
              top: 0,
              left: 0,
              right: 0,
              bottom: 0,
              backgroundColor: 'rgba(255, 255, 255, 0.8)',
              display: 'flex',
              flexDirection: 'column',
              justifyContent: 'center',
              alignItems: 'center',
              zIndex: 10
            }}>
              <Spin size="large" />
              <p style={{ marginTop: '20px' }}>PDF生成中，请稍候...</p>
              <Progress 
                percent={pdfGenerationProgress} 
                style={{ width: '80%', marginTop: '20px' }}
              />
            </div>
          )}
        </div>
      </Modal>

      {/* 模板抽屉 */}
      <Drawer
        title={`${currentTemplateType === 'result_explanation' ? '结果说明' : '信号值说明'}模板`}
        placement="right"
        onClose={() => setTemplateDrawerVisible(false)}
        open={templateDrawerVisible}
        width={500}
      >
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <h4>模板列表</h4>
          <Button type="primary" size="small" onClick={() => setCreateTemplateModalVisible(true)}>
            新建模板
          </Button>
        </div>
        <List
          dataSource={templates}
          renderItem={(item) => (
            <List.Item
              actions={[
                <Button size="small" onClick={() => handleUseTemplate(item)}>
                  使用
                </Button>,
              ]}
            >
              <List.Item.Meta
                title={item.title}
                description={item.content.substring(0, 100) + (item.content.length > 100 ? '...' : '')}
              />
            </List.Item>
          )}
          locale={{ emptyText: '暂无模板' }}
        />
      </Drawer>

      {/* 创建模板模态框 */}
      <Modal
        title={`创建${currentTemplateType === 'result_explanation' ? '结果说明' : '信号值说明'}模板`}
        open={createTemplateModalVisible}
        onCancel={() => setCreateTemplateModalVisible(false)}
        onOk={handleCreateTemplate}
      >
        <Form>
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

      {/* 添加历史记录模态框 */}
      <Modal
        title="添加历史检测记录"
        open={addHistoryModalVisible}
        onCancel={() => setAddHistoryModalVisible(false)}
        onOk={handleSaveHistory}
      >
        <Form form={addHistoryForm} layout="vertical">
          {availableHistoryReports.length > 0 && (
            <Form.Item label="从已有报告添加">
              <Table
                rowKey="id"
                size="small"
                pagination={false}
                dataSource={availableHistoryReports}
                rowSelection={{
                  selectedRowKeys: selectedExistingHistoryIds,
                  onChange: setSelectedExistingHistoryIds,
                }}
                columns={[
                  { title: '样本编号', dataIndex: 'sampleCode', key: 'sampleCode' },
                  { title: '检测时间', key: 'time', render: (_: any, record: any) => formatReportDate(record.generatedTime || record.createdAt) },
                  { title: '信号值', dataIndex: 'signalValue', key: 'signalValue', render: (value: any) => Number(value || 0).toFixed(1) },
                  { title: '治疗阶段', dataIndex: 'treatmentStageName', key: 'treatmentStageName' },
                ]}
              />
            </Form.Item>
          )}
          <Form.Item label="手工添加">
            <Typography.Text type="secondary">可从已有报告选择，也可以在下面手工录入一条。</Typography.Text>
          </Form.Item>
          <Form.Item name="testDate" label="检测时间">
            <Input placeholder="例如：2026-02-10" />
          </Form.Item>
          <Form.Item name="signalValue" label="信号值">
            <InputNumber style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name="trend" label="趋势">
            <Select>
              <Select.Option value="↑">上升</Select.Option>
              <Select.Option value="↓">下降</Select.Option>
              <Select.Option value="-">稳定</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="testType" label="检测类型（治疗阶段）">
            {renderTreatmentStageSelect()}
          </Form.Item>
          <Form.Item name="note" label="备注">
            <Input.TextArea rows={2} />
          </Form.Item>
        </Form>
      </Modal>

      {/* 报告预览模态框 */}
      <Modal
        title="报告预览"
        open={reportPreviewVisible}
        onCancel={() => setReportPreviewVisible(false)}
        footer={[
          <Button key="close" onClick={() => setReportPreviewVisible(false)}>
            关闭
          </Button>,
          <Space key="downloads">
            <Button icon={<DownloadOutlined />} onClick={() => handleDownloadPDF('concise')}>
              下载简洁报告
            </Button>
            <Button onClick={handleDownloadInstruction}>
              下载说明书
            </Button>
            <Button type="primary" icon={<DownloadOutlined />} onClick={() => handleDownloadPDF('full')}>
              下载完整报告
            </Button>
          </Space>,
        ]}
        width={900}
        style={{ top: 20 }}
      >
        <div style={{ height: '600px', overflow: 'auto', position: 'relative' }}>
          {previewLoading ? (
            <div style={{ textAlign: 'center', padding: '50px' }}>
              <Spin size="large" />
              <p style={{ marginTop: '20px' }}>加载预览数据中...</p>
            </div>
          ) : (
            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center' }}>
              <div style={{ overflow: 'auto', maxHeight: '550px' }}>
                {renderPreviewPage()}
              </div>
            </div>
          )}
        </div>
      </Modal>
    </div>
  );
};

export default ReportView;
