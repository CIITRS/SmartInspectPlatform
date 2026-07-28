import React, { useState, useEffect } from 'react';
import { Table, Button, Input, InputNumber, Form, Row, Col, Modal, Switch, Tabs, App, Select, Space, Tag, Tooltip, Descriptions, Alert, Transfer } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SettingOutlined } from '@ant-design/icons';
import { listCancerTypes, createCancerType, updateCancerType, deleteCancerType, listSampleTypes, createSampleType, updateSampleType, deleteSampleType, listGenes, createGene, updateGene, updateGenePanels, deleteGene, listModels, createModel, updateModel, deleteModel, listPanels, createPanel, updatePanel, deletePanel, getPanelGenes, updatePanelGenes } from '@/services/api';
import FormulaEditor from '../Model/FormulaEditor';

const { TextArea } = Input;

const DetectionType: React.FC = () => {
  // 检测类型状态
  const [form] = Form.useForm();
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] });
  const [cancerTypeSearchText, setCancerTypeSearchText] = useState('');
  const [searchParams, setSearchParams] = useState({});
  const [modalVisible, setModalVisible] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [modalForm] = Form.useForm();

  // 样本类型状态
  const [sampleTypeLoading, setSampleTypeLoading] = useState(false);
  const [sampleTypes, setSampleTypes] = useState<any[]>([]);
  const [sampleTypePagination, setSampleTypePagination] = useState({ current: 1, pageSize: 10, total: 0, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] });
  const [sampleTypeSearchText, setSampleTypeSearchText] = useState('');
  const [sampleTypeSearchParams, _setSampleTypeSearchParams] = useState({});
  const [sampleTypeModalVisible, setSampleTypeModalVisible] = useState(false);
  const [editingSampleType, setEditingSampleType] = useState<any | null>(null);
  const [sampleTypeForm] = Form.useForm();

  // 基因设置状态
  const [geneLoading, setGeneLoading] = useState(false);
  const [genes, setGenes] = useState<any[]>([]);
  const [genePagination, setGenePagination] = useState({ current: 1, pageSize: 10, total: 0, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] });
  const [geneSearchText, setGeneSearchText] = useState('');
  const [geneSearchParams, setGeneSearchParams] = useState({});
  const [geneModalVisible, setGeneModalVisible] = useState(false);
  const [editingGeneId, setEditingGeneId] = useState<string | null>(null);
  const [geneForm] = Form.useForm();
  // 基因详情模态框状态
  const [geneDetailModalVisible, setGeneDetailModalVisible] = useState(false);
  const [selectedGene, setSelectedGene] = useState<any>(null);
  // Panel列表状态
  const [panels, setPanels] = useState<any[]>([]);
  const [panelLoading, setPanelLoading] = useState(false);
  const [panelPagination, setPanelPagination] = useState({ current: 1, pageSize: 10, total: 0, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] });
  const [panelSearchText, setPanelSearchText] = useState('');
  const [panelModalVisible, setPanelModalVisible] = useState(false);
  const [editingPanel, setEditingPanel] = useState<any | null>(null);
  const [panelForm] = Form.useForm();
  const [panelGeneModalVisible, setPanelGeneModalVisible] = useState(false);
  const [selectedPanelId, setSelectedPanelId] = useState<string | null>(null);
  const [selectedPanelGenes, setSelectedPanelGenes] = useState<any[]>([]);
  const [allGenesForPanel, setAllGenesForPanel] = useState<any[]>([]);
  const [geneLoadingForPanel, setGeneLoadingForPanel] = useState(false);

  // 获取Panel列表
  const fetchPanels = async (params: any = {}) => {
    setPanelLoading(true);
    try {
      const response = await listPanels({ ...params });
      setPanels(response.data || []);
      setPanelPagination({
        ...panelPagination,
        total: response.data?.length || 0,
        current: params.page || 1,
        pageSize: params.pageSize || panelPagination.pageSize,
      });
    } catch (_error) {
      console.error('获取Panel列表失败');
    } finally {
      setPanelLoading(false);
    }
  };

  // 获取所有基因用于Panel管理
  const fetchAllGenesForPanel = async () => {
    try {
      const response = await listGenes();
      const genes = response.data || [];
      setAllGenesForPanel(genes.map((gene: any) => ({
        key: gene.id,
        title: `${gene.geneSymbol} (${gene.name})`,
        description: gene.description || '',
      })));
    } catch (_error) {
      console.error('获取基因列表失败');
    }
  };

  // 获取Panel基因列表
  const fetchPanelGenes = async (panelId: string) => {
    setGeneLoadingForPanel(true);
    try {
      const response = await getPanelGenes(panelId);
      setSelectedPanelGenes(response.data || []);
    } catch (_error) {
      console.error('获取Panel基因列表失败');
    } finally {
      setGeneLoadingForPanel(false);
    }
  };

  // Panel相关操作函数
  const handlePanelCreate = () => {
    setEditingPanel(null);
    panelForm.resetFields();
    setPanelModalVisible(true);
  };

  const handlePanelEdit = (record: any) => {
    setEditingPanel(record);
    panelForm.setFieldsValue({
      panelName: record.panelName,
      panelCode: record.panelCode,
      description: record.description,
      isActive: record.isActive === 1,
    });
    setPanelModalVisible(true);
  };

  const handlePanelDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该Panel吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deletePanel(id);
          appMessage.success('删除成功');
          fetchPanels();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handlePanelModalSubmit = async (values: any) => {
    try {
      const requestBody = {
        panelName: values.panelName,
        panelCode: values.panelCode,
        description: values.description,
        isActive: values.isActive ? 1 : 0,
      };

      if (editingPanel) {
        await updatePanel(editingPanel.id, requestBody);
        appMessage.success('更新成功');
      } else {
        await createPanel(requestBody);
        appMessage.success('创建成功');
      }
      setPanelModalVisible(false);
      fetchPanels();
    } catch (_error) {
      appMessage.error('操作失败');
    }
  };

  const handleGeneSettings = (record: any) => {
    setSelectedPanelId(record.id);
    fetchPanelGenes(record.id);
    setPanelGeneModalVisible(true);
  };

  const handleGeneTransferChange = (targetKeys: any[]) => {
    setSelectedPanelGenes(targetKeys.map((key: any) => {
      const gene = allGenesForPanel.find((g: any) => g.key === key);
      return {
        id: gene?.key,
        geneSymbol: gene?.title?.split(' ')[0] || '',
        name: gene?.title?.replace(/\s*\([^)]*\)/, '') || '',
      };
    }));
  };

  const handleSaveGenes = async () => {
    if (!selectedPanelId) return;
    try {
      const geneIds = selectedPanelGenes.map((gene: any) => gene.id);
      await updatePanelGenes(selectedPanelId, geneIds);
      appMessage.success('保存成功');
      setPanelGeneModalVisible(false);
    } catch (_error) {
      appMessage.error('保存失败');
    }
  };

  useEffect(() => {
    fetchPanels();
    fetchAllGenesForPanel();
  }, []);

  // 模型设置状态
  const [modelForm] = Form.useForm();
  const [models, setModels] = useState<any[]>([]);
  const [deprecatedModels, setDeprecatedModels] = useState<any[]>([]);
  const [modelLoading, setModelLoading] = useState(true);
  const [modelPagination, setModelPagination] = useState({ current: 1, pageSize: 10, total: 0, showSizeChanger: true, pageSizeOptions: ['10', '20', '50', '100'] });
  const [modelSearchText, setModelSearchText] = useState('');
  const [modelSearchParams, setModelSearchParams] = useState({});
  const [modelModalVisible, setModelModalVisible] = useState(false);
  const [modelViewModalVisible, setModelViewModalVisible] = useState(false);
  const [currentModel, setCurrentModel] = useState<any>(null);
  const [editingModelId, setEditingModelId] = useState<string | null>(null);
  const [fetchingCancerTypesForModel, _setFetchingCancerTypesForModel] = useState(false);
  const [selectedGenes, setSelectedGenes] = useState<number[]>([]);
  const [geneWeights, setGeneWeights] = useState<Record<string, number>>({});
  const [formula, setFormula] = useState('');
  const [modelMode, setModelMode] = useState('weighted');
  const [formulaEditorVisible, setFormulaEditorVisible] = useState(false);
  const [showDeprecated, setShowDeprecated] = useState(false);

  const { message: appMessage } = App.useApp();

  // 检测类型方法
  const fetchCancerTypes = async (params: any = {}) => {
    setLoading(true);
    try {
      const response = await listCancerTypes({ ...searchParams, ...params });
      setCancerTypes(response.data || []);
      setPagination({
        ...pagination,
        total: response.total || 0,
        current: params.page || 1,
        pageSize: params.pageSize || pagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取检测癌种列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 样本类型方法
  const fetchSampleTypes = async (params: any = {}) => {
    setSampleTypeLoading(true);
    try {
      // 处理API响应，从response.data中获取样本类型数组
      const response = await listSampleTypes({ ...sampleTypeSearchParams, ...params });
      setSampleTypes(response.data || []);
      setSampleTypePagination({
        ...sampleTypePagination,
        total: response.data?.length || 0,
        current: params.page || 1,
        pageSize: params.pageSize || sampleTypePagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取样本类型列表失败');
    } finally {
      setSampleTypeLoading(false);
    }
  };

  // 基因设置方法
  const fetchGenes = async (params: any = {}) => {
    setGeneLoading(true);
    try {
      // 处理API响应，从response.data中获取基因数组
      const response = await listGenes({ ...geneSearchParams, ...params }, { skipErrorHandler: true });
      const genesData = response.data || [];
      setGenes(genesData);
      setGenePagination({
        ...genePagination,
        total: genesData.length,
        current: params.page || 1,
        pageSize: params.pageSize || genePagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取基因列表失败');
    } finally {
      setGeneLoading(false);
    }
  };

  // 模型设置方法
  const fetchModels = async (params: any = {}) => {
    setModelLoading(true);
    try {
      // 处理API响应，从response.data中获取模型数组
      const response = await listModels({ ...modelSearchParams, ...params, includeDeprecated: false }, { skipErrorHandler: true });
      setModels(response.data || []);
      setModelPagination({
        ...modelPagination,
        total: response.data?.length || 0,
        current: params.page || 1,
      });
    } catch (_error) {
      appMessage.error('获取模型列表失败');
    } finally {
      setModelLoading(false);
    }
  };

  // 获取已弃用模型
  const fetchDeprecatedModels = async (params: any = {}) => {
    setModelLoading(true);
    try {
      // 处理API响应，从response.data中获取模型数组
      const response = await listModels({ ...modelSearchParams, ...params, includeDeprecated: true, activeOnly: false }, { skipErrorHandler: true });
      // 过滤出已弃用的模型
      const deprecatedModels = (response.data || []).filter((model: any) => model.isDeprecated === 1);
      return deprecatedModels;
    } catch (_error) {
      appMessage.error('获取已弃用模型列表失败');
      return [];
    } finally {
      setModelLoading(false);
    }
  };

  const handleModelSearch = (values: any) => {
    setModelSearchParams(values);
    fetchModels({ page: 1 });
  };

  const handleModelCreate = () => {
    setEditingModelId(null);
    modelForm.resetFields();
    setSelectedGenes([]);
    setGeneWeights({});
    setFormula('');
    setModelMode('weighted');
    setModelModalVisible(true);
  };

  const handleModelView = (record: any) => {
    setCurrentModel(record);
    setModelViewModalVisible(true);
  };

  const handleModelGenePanelsChange = async (geneId: number, panelIds: number[]) => {
    try {
      await updateGenePanels(geneId, panelIds);
      const nextPanels = panels.filter((panel: any) => panelIds.includes(panel.id)).map((panel: any) => ({
        id: panel.id,
        panelName: panel.panelName,
        panelCode: panel.panelCode,
      }));
      setGenes(prev => prev.map((gene: any) => gene.id === geneId ? { ...gene, panels: nextPanels } : gene));
      appMessage.success('基因Panel已更新');
    } catch (_error) {
      appMessage.error('基因Panel更新失败');
    }
  };

  const handleModelEdit = (record: any) => {
    setEditingModelId(record.id);
    // 处理基因选择和权重设置
    setSelectedGenes(record.selectedGenes || []);
    
    const weights: Record<string, number> = {};
    if (record.selectedGenes) {
      record.selectedGenes.forEach((geneId: number) => {
        weights[geneId.toString()] = 1.0; // 默认权重为1.0
      });
    }
    setGeneWeights(weights);
    setFormula(record.formula || '');
    setModelMode(record.modelMode || 'weighted');
    
    const formValues = {
      ...record,
      selectedGenes: record.selectedGenes || [],
      geneWeights: weights
    };
    modelForm.setFieldsValue(formValues);
    setModelModalVisible(true);
  };

  const handleGeneSelect = (values: number[]) => {
    setSelectedGenes(values);
    
    const newWeights: Record<string, number> = { ...geneWeights };
    values.forEach(geneId => {
      if (!newWeights[geneId.toString()]) {
        newWeights[geneId.toString()] = 1.0;
      }
    });
    setGeneWeights(newWeights);
  };

  const handleWeightChange = (geneId: number, value: number) => {
    setGeneWeights({
      ...geneWeights,
      [geneId.toString()]: value,
    });
  };

  const handleOpenFormulaEditor = () => {
    // 从表单中获取最新的selectedGenes值
    const formValues = modelForm.getFieldsValue();
    if (formValues.selectedGenes) {
      setSelectedGenes(formValues.selectedGenes);
      
      // 更新基因权重
      const newWeights: Record<string, number> = {};
      formValues.selectedGenes.forEach((geneId: number) => {
        if (!geneWeights[geneId.toString()]) {
          newWeights[geneId.toString()] = 1.0;
        } else {
          newWeights[geneId.toString()] = geneWeights[geneId.toString()];
        }
      });
      setGeneWeights(newWeights);
    }
    setFormulaEditorVisible(true);
  };

  const handleFormulaSave = (newFormula: string) => {
    setFormula(newFormula);
    modelForm.setFieldsValue({ formula: newFormula });
    setFormulaEditorVisible(false);
  };

  const handleModelModeChange = (mode: string) => {
    setModelMode(mode);
  };

  const handleModelDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该模型吗？删除后将标记为已弃用。',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteModel(id);
          appMessage.success('删除成功，模型已标记为已弃用');
          fetchModels();
          if (showDeprecated) {
            fetchDeprecatedModels().then(setDeprecatedModels);
          }
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleModelModalSubmit = async (values: any) => {
    try {
      // 如果是深度学习模型，无需输入参数
      if (values.modelType === 'deep_learning') {
        delete values.parameters;
        delete values.selectedGenes;
        delete values.geneWeights;
      }
      
      // 构建请求体，确保字段名称与后端期望的一致
      const requestBody = {
        ...values,
        // 确保is_active字段存在且为数字类型
        is_active: values.isActive || 0,
        // 删除isActive字段，避免后端解析冲突
        isActive: undefined
      };
      
      let response: any;
      if (editingModelId) {
        response = await updateModel(editingModelId, requestBody);
        appMessage.success('模型更新成功');
      } else {
        response = await createModel(requestBody);
        // 如果是深度学习模型，显示特殊提示
        if (values.modelType === 'deep_learning') {
          appMessage.success(`模型创建成功，模型ID为${response.data.id}，请提供给运维人员以配置模型`);
        } else {
          appMessage.success('模型创建成功');
        }
      }
      setModelModalVisible(false);
      fetchModels();
    } catch (_error) {
      appMessage.error(editingModelId ? '模型更新失败' : '模型创建失败');
    }
  };

  // 初始化加载数据
  useEffect(() => {
    fetchCancerTypes();
    fetchSampleTypes();
    fetchGenes();
    fetchModels();
  }, []);

  const handleCreate = () => {
    setEditingId(null);
    modalForm.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: any) => {
    setEditingId(record.id);
    // 获取已关联的 Panel IDs
    const panelIds = record.panels?.map((panel: any) => panel.id) || [];
    modalForm.setFieldsValue({
      name: record.name,
      description: record.description,
      // 将数字状态转换为布尔值供Switch组件使用
      is_active: record.is_active === 1,
      panelIds: panelIds,
    });
    setModalVisible(true);
  };

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该检测癌种吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteCancerType(id);
          appMessage.success('删除成功');
          fetchCancerTypes();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleStatusChange = async (id: string, checked: boolean) => {
    try {
      await updateCancerType(id, { is_active: checked ? 1 : 0 });
      appMessage.success('状态更新成功');
      fetchCancerTypes();
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  const handleModalSubmit = async (values: any) => {
    try {
      const panelIdsArray = values.panelIds || [];
      const requestBody = {
        ...values,
        panelIds: panelIdsArray.join(','),
      };
      
      if (editingId) {
        await updateCancerType(editingId, requestBody);
        appMessage.success('更新成功');
      } else {
        await createCancerType(requestBody);
        appMessage.success('创建成功');
      }
      setModalVisible(false);
      fetchCancerTypes();
    } catch (_error) {
      appMessage.error(editingId ? '更新失败' : '创建失败');
    }
  };

  // 样本类型相关方法
  const handleSampleTypeCreate = () => {
    setEditingSampleType(null);
    sampleTypeForm.resetFields();
    setSampleTypeModalVisible(true);
  };

  const handleSampleTypeEdit = (record: any) => {
    setEditingSampleType(record.id);
    sampleTypeForm.setFieldsValue({
      name: record.name,
      description: record.description,
      is_active: record.is_active,
    });
    setSampleTypeModalVisible(true);
  };

  const handleSampleTypeDelete = (record: any) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该样本类型吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteSampleType(record.id);
          appMessage.success('删除成功');
          fetchSampleTypes();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleSampleTypeModalSubmit = async (values: any) => {
    try {
      if (editingSampleType) {
        await updateSampleType(editingSampleType, values);
        appMessage.success('更新成功');
      } else {
        await createSampleType(values);
        appMessage.success('创建成功');
      }
      setSampleTypeModalVisible(false);
      fetchSampleTypes();
    } catch (_error) {
      appMessage.error(editingSampleType ? '更新失败' : '创建失败');
    }
  };

  // 基因设置相关方法
  const handleGeneCreate = async () => {
    setEditingGeneId(null);
    geneForm.resetFields();
    // 重新获取Panel列表，确保显示最新数据
    await fetchPanels();
    setGeneModalVisible(true);
  };

  const handleGeneEdit = async (gene: any) => {
    setEditingGeneId(gene.id);
    // 重新获取Panel列表，确保显示最新数据
    await fetchPanels();
    // 设置表单值，包括panelIds
    const panelIds = gene.panels?.map((panel: any) => panel.id) || [];
    geneForm.setFieldsValue({
      ...gene,
      panelIds
    });
    setGeneModalVisible(true);
  };

  const handleGeneView = (gene: any) => {
    setSelectedGene(gene);
    setGeneDetailModalVisible(true);
  };

  const handleGeneDelete = (gene: any) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除基因"${gene.geneSymbol}"吗？`,
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deleteGene(gene.id);
          appMessage.success('删除成功');
          fetchGenes();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleGeneModalSubmit = async (values: any) => {
    try {
      const geneData = {
        name: values.name,
        geneSymbol: values.geneSymbol,
        description: values.description,
        isActive: values.is_active,
        panelIds: values.panelIds || [],
      };
      
      if (editingGeneId) {
        await updateGene(editingGeneId, geneData);
        appMessage.success('基因更新成功');
      } else {
        await createGene(geneData);
        appMessage.success('基因创建成功');
      }
      setGeneModalVisible(false);
      fetchGenes();
    } catch (_error) {
      appMessage.error(editingGeneId ? '基因更新失败' : '基因创建失败');
    }
  };

  // 检测类型列定义
  const cancerTypeColumns = [
    { title: '检测癌种名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { 
      title: '关联Panel', 
      dataIndex: 'panels', 
      key: 'panels', 
      render: (panels: any[]) => {
        if (!panels || panels.length === 0) {
          return <span style={{ color: '#999' }}>无</span>;
        }
        return (
          <Space wrap size={[4, 4]}>
            {panels.map((panel: any) => (
              <Tag key={panel.id} color="blue">
                {panel.panelName}
              </Tag>
            ))}
          </Space>
        );
      }
    },
    { 
      title: '状态', 
      dataIndex: 'is_active', 
      key: 'is_active', 
      render: (status: number, record: any) => (
        <Switch 
          checked={status === 1} 
          onChange={(checked) => handleStatusChange(record.id, checked)} 
          checkedChildren="启用" 
          unCheckedChildren="禁用" 
        />
      ) 
    },
    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => handleEdit(record)} 
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // Panel列定义
  const panelColumns = [
    {
      title: 'Panel名称',
      dataIndex: 'panelName',
      key: 'panelName',
      render: (text: string, record: any) => (
        <a onClick={() => handlePanelEdit(record)}>{text}</a>
      ),
    },
    {
      title: 'Panel编码',
      dataIndex: 'panelCode',
      key: 'panelCode',
    },
    {
      title: '基因',
      dataIndex: 'geneNames',
      key: 'geneNames',
      ellipsis: true,
      render: (text: string) => text || '-',
    },
    {
      title: '状态',
      dataIndex: 'isActive',
      key: 'isActive',
      render: (status: number) => (
        <Tag color={status === 1 ? 'green' : 'red'}>
          {status === 1 ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: string) => text ? text.split('T')[0] : '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <>
          <Button
            type="link"
            icon={<SettingOutlined />}
            onClick={() => handleGeneSettings(record)}
            style={{ marginRight: 8 }}
          >
            基因设置
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handlePanelEdit(record)}
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handlePanelDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 样本类型列定义
  const sampleTypeColumns = [
    { title: '样本类型名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    { 
      title: '状态', 
      dataIndex: 'is_active', 
      key: 'is_active', 
      render: (status: number, record: any) => (
        <Switch 
          checked={status === 1} 
          onChange={async (checked) => {
            try {
              await updateSampleType(record.id, { is_active: checked ? 1 : 0 });
              appMessage.success('状态更新成功');
              fetchSampleTypes();
            } catch (_error) {
              appMessage.error('状态更新失败');
            }
          }} 
          checkedChildren="启用" 
          unCheckedChildren="禁用" 
        />
      ) 
    },
    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => handleSampleTypeEdit(record)} 
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleSampleTypeDelete(record)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 基因设置列定义
  const geneColumns = [
    { 
      title: '基因名称', 
      dataIndex: 'name', 
      key: 'name',
      render: (text: string, record: any) => (
        <a onClick={() => handleGeneView(record)} style={{ color: '#1890ff' }}>
          {text}
        </a>
      )
    },
    { title: '基因代码', dataIndex: 'geneSymbol', key: 'geneSymbol' },
    { title: '基因位点描述', dataIndex: 'description', key: 'description' },
    {
      title: '关联Panel',
      dataIndex: 'panels',
      key: 'panels',
      render: (panels: any[]) => {
        if (!panels || panels.length === 0) {
          return <span style={{ color: '#999' }}>无</span>;
        }
        return (
          <Space wrap size={[4, 4]}>
            {panels.map((panel: any) => (
              <Tag key={panel.id} color="green">
                {panel.panelName}
              </Tag>
            ))}
          </Space>
        );
      }
    },
    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => handleGeneEdit(record)} 
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleGeneDelete(record)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 使用从后端获取的样本类型数据
  const sampleTypeDataSource = sampleTypes;

  // 模型列定义
  const modelColumns = [
    { title: '模型名称', dataIndex: 'name', key: 'name', render: (text: string, record: any) => (
        <a onClick={() => handleModelView(record)}>{text}</a>
      )},
    { title: '模型类型', dataIndex: 'modelType', key: 'modelType', render: (text: string) => {
        const typeMap: Record<string, string> = {
          weighted_equation: '加权方程模型',
          deep_learning: '深度学习模型',
          random_forest: '随机森林模型',
          other: '其他模型'
        };
        return typeMap[text] || text;
      }},
    { title: '版本号', dataIndex: 'version', key: 'version' },
    { title: '适用检测', dataIndex: 'cancerTypeName', key: 'cancerTypeName' },
    { 
      title: '基因设置', 
      key: 'geneSettings', 
      render: (_text: any, record: any) => {
        const selectedGeneIds = record.selectedGenes || [];
        if (!selectedGeneIds || selectedGeneIds.length === 0) {
          return <span style={{ color: '#999' }}>无</span>;
        }
        return (
          <Space wrap size={[4, 4]}>
            {selectedGeneIds.map((geneId: number) => {
              const gene = genes.find(g => g.id === geneId);
              return (
                <Tag key={geneId} color="blue">
                  {gene?.geneSymbol || `基因${geneId}`}
                </Tag>
              );
            })}
          </Space>
        );
      }
    },
    { 
              title: '模型状态', 
              dataIndex: 'isActive', 
              key: 'isActive', 
              render: (status: number, record: any) => (
                <Switch 
                  checked={status === 1} 
                  onChange={async (checked) => {
                    try {
                      // 传递正确的参数格式
                      const updatedModel = {
                        name: record.name || record.modelName || '',
                        modelType: record.modelType || '',
                        cancerTypeId: record.cancerTypeId || 0,
                        version: record.version || '',
                        isActive: checked ? 1 : 0
                      };
                      await updateModel(String(record.id), updatedModel);
                      appMessage.success('状态更新成功');
                      fetchModels();
                    } catch (error: any) {
                      console.error('状态更新失败:', error);
                      appMessage.error('状态更新失败');
                    }
                  }} 
                  checkedChildren="启用" 
                  unCheckedChildren="禁用" 
                />
              ) 
            },
    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => handleModelEdit(record)} 
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleModelDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 定义tabs的items
  const tabsItems = [
    {
      key: 'detectionType',
          label: '检测癌种',
      children: (
        <>
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Input.Search
              placeholder="搜索检测癌种名称"
              onSearch={(value) => {
                setCancerTypeSearchText(value);
                fetchCancerTypes({ page: 1, search: value });
              }}
              style={{ width: 300 }}
              allowClear
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
            新增检测癌种
            </Button>
          </div>

          <Table
            columns={cancerTypeColumns}
            dataSource={cancerTypes}
            rowKey="id"
            loading={loading}
            pagination={pagination}
            onChange={(page) => fetchCancerTypes({ page: page.current, pageSize: page.pageSize })}
          />

          <Modal
        title={editingId ? '编辑检测癌种' : '新增检测癌种'}
            open={modalVisible}
            onCancel={() => setModalVisible(false)}
            footer={null}
          >
            <Form
              form={modalForm}
              layout="vertical"
              onFinish={handleModalSubmit}
            >
              <Form.Item
                name="name"
            label="检测癌种名称"
            rules={[{ required: true, message: '请输入检测癌种名称' }]}
              >
            <Input placeholder="请输入检测癌种名称" />
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={4} placeholder="请输入描述" />
              </Form.Item>

              <Form.Item
                name="is_active"
                label="状态"
                valuePropName="checked"
                getValueProps={(checked: boolean) => ({ checked })}
                getValueFromEvent={(checked) => checked ? 1 : 0}
              >
                <Switch checkedChildren="启用" unCheckedChildren="禁用" defaultChecked />
              </Form.Item>

              <Form.Item
                name="panelIds"
                label="关联Panel"
              >
                <Select
                  mode="multiple"
                  placeholder="请选择关联的Panel（可选）"
                  allowClear
                  style={{ width: '100%' }}
                >
                  {panels.map((panel: any) => (
                    <Select.Option key={panel.id} value={panel.id}>
                      {panel.panelName} ({panel.panelCode})
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
    {
      key: 'sampleType',
      label: '样本类型',
      children: (
        <>
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Input.Search
              placeholder="搜索样本类型名称"
              onSearch={(value) => {
                setSampleTypeSearchText(value);
                fetchSampleTypes({ page: 1, search: value });
              }}
              style={{ width: 300 }}
              allowClear
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={handleSampleTypeCreate}>
              新增样本类型
            </Button>
          </div>

          <Table
            columns={sampleTypeColumns}
            dataSource={sampleTypeDataSource}
            rowKey="id"
            loading={sampleTypeLoading}
            pagination={sampleTypePagination}
            onChange={(page) => fetchSampleTypes({ page: page.current, pageSize: page.pageSize })}
          />

          <Modal
            title={editingSampleType ? '编辑样本类型' : '新增样本类型'}
            open={sampleTypeModalVisible}
            onCancel={() => setSampleTypeModalVisible(false)}
            footer={null}
          >
            <Form
              form={sampleTypeForm}
              layout="vertical"
              onFinish={handleSampleTypeModalSubmit}
            >
              <Form.Item
                name="name"
                label="样本类型名称"
                rules={[{ required: true, message: '请输入样本类型名称' }]}
              >
                <Input placeholder="请输入样本类型名称" />
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={4} placeholder="请输入描述" />
              </Form.Item>

              <Form.Item
                name="is_active"
                label="状态"
                valuePropName="checked"
                getValueProps={(checked: boolean) => ({ checked })}
                getValueFromEvent={(checked) => checked ? 1 : 0}
              >
                <Switch checkedChildren="启用" unCheckedChildren="禁用" defaultChecked />
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setSampleTypeModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
    {
      key: 'panel',
      label: 'Panel管理',
      children: (
        <>
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Input.Search
              placeholder="搜索Panel名称或编码"
              onSearch={(value) => {
                setPanelSearchText(value);
                fetchPanels({ page: 1, search: value });
              }}
              style={{ width: 300 }}
              allowClear
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={handlePanelCreate}>
              新增Panel
            </Button>
          </div>

          <Table
            columns={panelColumns}
            dataSource={panels}
            rowKey="id"
            loading={panelLoading}
            pagination={panelPagination}
            onChange={(page) => fetchPanels({ page: page.current, pageSize: page.pageSize })}
          />

          <Modal
            title={editingPanel ? '编辑Panel' : '新增Panel'}
            open={panelModalVisible}
            onCancel={() => setPanelModalVisible(false)}
            footer={null}
          >
            <Form
              form={panelForm}
              layout="vertical"
              onFinish={handlePanelModalSubmit}
            >
              <Form.Item
                name="panelName"
                label="Panel名称"
                rules={[{ required: true, message: '请输入Panel名称' }]}
              >
                <Input placeholder="请输入Panel名称" />
              </Form.Item>

              <Form.Item
                name="panelCode"
                label="Panel编码"
                rules={[{ required: true, message: '请输入Panel编码' }]}
              >
                <Input placeholder="请输入Panel编码" />
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={3} placeholder="请输入描述" />
              </Form.Item>

              <Form.Item
                name="isActive"
                label="状态"
                valuePropName="checked"
                getValueProps={(checked: boolean) => ({ checked })}
                initialValue={true}
              >
                <Switch checkedChildren="启用" unCheckedChildren="禁用" />
              </Form.Item>

              <Form.Item>
                <Space>
                  <Button type="primary" htmlType="submit">
                    保存
                  </Button>
                  <Button onClick={() => setPanelModalVisible(false)}>
                    取消
                  </Button>
                </Space>
              </Form.Item>
            </Form>
          </Modal>

          <Modal
            title="Panel基因设置"
            open={panelGeneModalVisible}
            onCancel={() => setPanelGeneModalVisible(false)}
            onOk={handleSaveGenes}
            width={800}
            confirmLoading={geneLoadingForPanel}
          >
            <div style={{ marginBottom: 16 }}>
              <p>选择该Panel包含的基因：</p>
            </div>
            <Transfer
              dataSource={allGenesForPanel}
              titles={['可选择基因', '已选基因']}
              targetKeys={selectedPanelGenes.map((g: any) => g.id)}
              onChange={handleGeneTransferChange}
              render={(item: any) => item.title || item.geneSymbol}
              listStyle={{
                width: 350,
                height: 400,
              }}
              showSearch
              filterOption={(inputValue: string, item: any) =>
                item.title.toLowerCase().indexOf(inputValue.toLowerCase()) !== -1 ||
                (item.description && item.description.toLowerCase().indexOf(inputValue.toLowerCase()) !== -1)
              }
            />
          </Modal>
        </>
      ),
    },
    {
      key: 'gene',
      label: '基因设置',
      children: (
        <>
          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between' }}>
            <Input.Search
              placeholder="搜索基因名称或符号"
              onSearch={(value) => {
                setGeneSearchText(value);
                fetchGenes({ page: 1, search: value });
              }}
              style={{ width: 300 }}
              allowClear
            />
            <Button type="primary" icon={<PlusOutlined />} onClick={handleGeneCreate}>
              新增基因
            </Button>
          </div>

          <Table
            columns={geneColumns}
            dataSource={genes}
            rowKey="id"
            loading={geneLoading}
            pagination={genePagination}
            onChange={(page) => fetchGenes({ page: page.current, pageSize: page.pageSize })}
          />

          <Modal
            title={editingGeneId ? '编辑基因' : '新增基因'}
            open={geneModalVisible}
            onCancel={() => setGeneModalVisible(false)}
            footer={null}
          >
            <Form
              form={geneForm}
              layout="vertical"
              onFinish={handleGeneModalSubmit}
            >
              <Form.Item
                name="name"
                label="基因名称"
                rules={[{ required: true, message: '请输入基因名称' }]}
              >
                <Input placeholder="请输入基因名称" />
              </Form.Item>

              <Form.Item
                name="geneSymbol"
                label="基因代码"
                rules={[{ required: true, message: '请输入基因代码' }]}
              >
                <Input placeholder="请输入基因代码" />
              </Form.Item>

              <Form.Item
                name="description"
                label="基因位点描述"
                rules={[{ required: true, message: '请输入基因位点描述' }]}
              >
                <TextArea rows={4} placeholder="请输入基因位点描述" />
              </Form.Item>

              <Form.Item
                name="is_active"
                label="状态"
                initialValue={1}
                rules={[{ required: true, message: '请选择状态' }]}
              >
                <Select placeholder="请选择状态">
                  <Select.Option value={1}>启用</Select.Option>
                  <Select.Option value={0}>禁用</Select.Option>
                </Select>
              </Form.Item>

              <Form.Item
                name="panelIds"
                label="关联Panel"
              >
                <Select
                  mode="multiple"
                  placeholder="请选择关联的Panel（可选）"
                  allowClear
                >
                  {panels.map((panel: any) => (
                    <Select.Option key={panel.id} value={panel.id}>
                      {panel.panelName} ({panel.panelCode})
                    </Select.Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setGeneModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>

          {/* 基因详情模态框 */}
          <Modal
            title="基因详情"
            open={geneDetailModalVisible}
            onCancel={() => setGeneDetailModalVisible(false)}
            footer={[
              <Button key="close" onClick={() => setGeneDetailModalVisible(false)}>
                关闭
              </Button>
            ]}
            width={800}
          >
            {selectedGene && (
              <div>
                {/* 基因基本信息 */}
                <Descriptions column={2} bordered style={{ marginBottom: 24 }}>
                  <Descriptions.Item label="基因名称" span={2}>
                    {selectedGene.name}
                  </Descriptions.Item>
                  <Descriptions.Item label="基因编号">
                    {selectedGene.geneSymbol}
                  </Descriptions.Item>
                  <Descriptions.Item label="状态">
                    <Tag color={selectedGene.is_active === 1 ? 'green' : 'red'}>
                      {selectedGene.is_active === 1 ? '启用' : '禁用'}
                    </Tag>
                  </Descriptions.Item>
                  <Descriptions.Item label="详细描述" span={2}>
                    {selectedGene.description || '无'}
                  </Descriptions.Item>
                  <Descriptions.Item label="关联Panel" span={2}>
                    {selectedGene.panels && selectedGene.panels.length > 0 ? (
                      <Space wrap>
                        {selectedGene.panels.map((panel: any) => (
                          <Tag key={panel.id} color="green">
                            {panel.panelName}
                          </Tag>
                        ))}
                      </Space>
                    ) : '无'}
                  </Descriptions.Item>
                </Descriptions>

                {/* 已应用的模型列表 */}
                <div>
                  <h4 style={{ marginBottom: 16 }}>已应用的模型</h4>
                  {(() => {
                    const relatedModels = models.filter((model: any) => {
                      const selectedGenes = model.selectedGenes || [];
                      return Array.isArray(selectedGenes) && selectedGenes.includes(selectedGene.id);
                    });

                    if (relatedModels.length === 0) {
                      return (
                        <Alert
                          message="该基因暂未应用于任何模型"
                          type="info"
                          showIcon
                        />
                      );
                    }

                    return (
                      <Table
                        dataSource={relatedModels}
                        rowKey="id"
                        pagination={false}
                        columns={[
                          {
                            title: '模型名称',
                            dataIndex: 'name',
                            key: 'name',
                          },
                          {
                            title: '模型类型',
                            dataIndex: 'modelType',
                            key: 'modelType',
                            render: (text: string) => {
                              const typeMap: Record<string, string> = {
                                weighted_equation: '加权方程模型',
                                deep_learning: '深度学习模型',
                                random_forest: '随机森林模型',
                                other: '其他模型'
                              };
                              return typeMap[text] || text;
                            }
                          },
                          {
                            title: '版本号',
                            dataIndex: 'version',
                            key: 'version',
                          },
                          {
                            title: '适用癌种',
                            dataIndex: 'cancerTypeName',
                            key: 'cancerTypeName',
                            render: (text: string) => text || '无'
                          }
                        ]}
                      />
                    );
                  })()}
                </div>
              </div>
            )}
          </Modal>
        </>
      ),
    },
    {
      key: 'model',
      label: '模型设置',
      children: (
        <>
          <Form form={form} layout="inline" onFinish={handleModelSearch} style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="name">
                  <Input placeholder="模型名称" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="status">
                  <Input placeholder="状态" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item>
                  <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                    查询
                  </Button>
                  <Button type="default" onClick={() => form.resetFields()}>重置</Button>
                </Form.Item>
              </Col>
            </Row>
          </Form>

          <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <Input.Search
              placeholder="搜索模型名称"
              onSearch={(value) => {
                setModelSearchText(value);
                fetchModels({ page: 1, search: value });
              }}
              style={{ width: 300 }}
              allowClear
            />
            <div>
              <Button type="primary" icon={<PlusOutlined />} onClick={handleModelCreate} style={{ marginRight: 8 }}>
                新增模型
              </Button>
              <Button 
                type={showDeprecated ? "primary" : "default"} 
                onClick={() => setShowDeprecated(!showDeprecated)}
              >
                {showDeprecated ? '隐藏已弃用模型' : '显示已弃用模型'}
              </Button>
            </div>
          </div>

          <Table
            columns={modelColumns}
            dataSource={models}
            rowKey="id"
            loading={modelLoading}
            pagination={modelPagination}
            onChange={(page) => fetchModels({ page: page.current, pageSize: page.pageSize })}
          />

          {showDeprecated && (
            <div style={{ marginTop: 24 }}>
              <h3>已弃用模型</h3>
              <Table
                columns={[
                  { title: '模型名称', dataIndex: 'name', key: 'name' },
                  { title: '模型类型', dataIndex: 'modelType', key: 'modelType', render: (text: string) => {
                    const typeMap: Record<string, string> = {
                      weighted_equation: '加权方程模型',
                      deep_learning: '深度学习模型',
                      random_forest: '随机森林模型',
                      other: '其他模型'
                    };
                    return typeMap[text] || text;
                  }},
                  { title: '版本号', dataIndex: 'version', key: 'version' },
                  { title: '适用检测', dataIndex: 'cancerTypeName', key: 'cancerTypeName' },
                  { title: '弃用时间', dataIndex: 'deprecatedAt', key: 'deprecatedAt' },
                  { 
                    title: '操作', 
                    key: 'action', 
                    render: (_text: any, record: any) => (
                      <Button 
                        type="link" 
                        icon={<EditOutlined />} 
                        onClick={() => handleModelView(record)} 
                      >
                        查看
                      </Button>
                    ),
                  },
                ]}
                dataSource={deprecatedModels}
                rowKey="id"
                loading={modelLoading}
                pagination={{ pageSize: 10 }}
              />
            </div>
          )}

          {/* 查看模型模态框 */}
          <Modal
            title="查看模型"
            open={modelViewModalVisible}
            onCancel={() => setModelViewModalVisible(false)}
            footer={[
              <Button key="close" onClick={() => setModelViewModalVisible(false)}>
                关闭
              </Button>
            ]}
            width={800}
          >
            {currentModel && (
              <div>
                <Row gutter={[16, 16]}>
                  <Col span={12}>
                    <Form.Item label="模型名称">
                      <div>{currentModel.name}</div>
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="模型类型">
                      <div>
                        {{
                          weighted_equation: '加权方程模型',
                          deep_learning: '深度学习模型',
                          random_forest: '随机森林模型',
                          other: '其他模型'
                        }[currentModel.modelType as 'weighted_equation' | 'deep_learning' | 'random_forest' | 'other'] || currentModel.modelType}
                      </div>
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="版本号">
                      <div>{currentModel.version}</div>
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="适用检测">
                      <div>{currentModel.cancerTypeName || '无'}</div>
                    </Form.Item>
                  </Col>
                  <Col span={24}>
                    <Form.Item label="基因设置">
                      <div>
                        {currentModel.selectedGenes && currentModel.selectedGenes.length > 0 ? (
                          <div>
                            {currentModel.selectedGenes.map((geneId: number) => {
                              const gene = genes.find((item: any) => item.id === geneId);
                              return (
                              <Tag key={geneId} color="blue" style={{ marginRight: '8px', marginBottom: '8px' }}>
                                {gene?.geneSymbol || `基因${geneId}`}
                              </Tag>
                              );
                            })}
                          </div>
                        ) : (
                          '无'
                        )}
                      </div>
                    </Form.Item>
                  </Col>
                  <Col span={24}>
                    <Form.Item label="基因匹配Panel">
                      <Table
                        size="small"
                        rowKey="id"
                        pagination={false}
                        dataSource={(currentModel.selectedGenes || []).map((geneId: number) => {
                          const gene = genes.find((item: any) => item.id === geneId);
                          return {
                            id: geneId,
                            geneSymbol: gene?.geneSymbol || `基因${geneId}`,
                            geneName: gene?.name || '',
                            panelIds: (gene?.panels || []).map((panel: any) => panel.id),
                          };
                        })}
                        columns={[
                          {
                            title: '基因',
                            dataIndex: 'geneSymbol',
                            key: 'geneSymbol',
                            width: 180,
                          },
                          {
                            title: '匹配Panel',
                            dataIndex: 'panelIds',
                            key: 'panelIds',
                            render: (panelIds: number[], record: any) => (
                              <Select
                                mode="multiple"
                                allowClear
                                style={{ width: '100%' }}
                                value={panelIds}
                                placeholder="请选择Panel"
                                onChange={(nextPanelIds) => handleModelGenePanelsChange(record.id, nextPanelIds)}
                                options={panels.map((panel: any) => ({
                                  value: panel.id,
                                  label: `${panel.panelName} (${panel.panelCode})`,
                                }))}
                              />
                            ),
                          },
                        ]}
                      />
                    </Form.Item>
                  </Col>
                  <Col span={24}>
                    <Form.Item label="公式">
                      <div>
                        {currentModel.formula ? (
                          <pre style={{ whiteSpace: 'pre-wrap' }}>{currentModel.formula}</pre>
                        ) : (
                          '无'
                        )}
                      </div>
                    </Form.Item>
                  </Col>
                  <Col span={24}>
                    <Form.Item label="描述">
                      <div>{currentModel.description || '无'}</div>
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item label="状态">
                      <div>{currentModel.isActive === 1 ? '启用' : '禁用'}</div>
                    </Form.Item>
                  </Col>
                </Row>
              </div>
            )}
          </Modal>

          <Modal
            title={editingModelId ? '编辑模型' : '新增模型'}
            open={modelModalVisible}
            onCancel={() => setModelModalVisible(false)}
            footer={null}
            width={900}
          >
            <Form
              form={modelForm}
              layout="vertical"
              onFinish={handleModelModalSubmit}
            >
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="name"
                    label="模型名称"
                    rules={[{ required: true, message: '请输入模型名称' }]}
                  >
                    {editingModelId ? (
                      <Select
                        placeholder="请选择模型名称"
                        showSearch
                        optionFilterProp="children"
                      >
                        {[...models, ...deprecatedModels]
                          .filter((model: any) => model.name)
                          .map((model: any) => (
                            <Select.Option key={`${model.id}-${model.name}`} value={model.name}>
                              {model.name}
                            </Select.Option>
                          ))}
                      </Select>
                    ) : (
                      <Input placeholder="请输入模型名称" />
                    )}
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="modelType"
                    label="模型类型"
                    rules={[{ required: true, message: '请选择模型类型' }]}
                  >
                    <Select 
                      placeholder="请选择模型类型"
                      onChange={handleModelModeChange}
                    >
                      <Select.Option value="weighted_equation">加权方程模型</Select.Option>
                      <Select.Option value="random_forest">随机森林模型</Select.Option>
                      <Select.Option value="deep_learning">深度学习模型</Select.Option>
                      <Select.Option value="other">其他模型</Select.Option>
                    </Select>
                  </Form.Item>
                </Col>
              </Row>
              
              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="version"
                    label="版本号"
                  >
                    <Input placeholder="请输入版本号" />
                  </Form.Item>
                </Col>
                <Col span={12}>
                  <Form.Item
                    name="cancerTypeId"
                    label="适用检测"
                  >
                    <Select placeholder="请选择适用检测" allowClear>
                      {cancerTypes.map(ct => (
                        <Select.Option key={ct.id} value={ct.id}>{ct.name}</Select.Option>
                      ))}
                    </Select>
                  </Form.Item>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col span={12}>
                  <Form.Item
                    name="isActive"
                    label="状态"
                    initialValue={1}
                  >
                    <Select placeholder="请选择状态">
                      <Select.Option value={1}>启用</Select.Option>
                      <Select.Option value={0}>禁用</Select.Option>
                    </Select>
                  </Form.Item>
                </Col>
              </Row>

              {modelMode !== 'deep_learning' && (
                <>
                  <Form.Item
                    name="selectedGenes"
                    label="选择基因"
                  >
                    <Select
                      mode="multiple"
                      placeholder="请选择基因"
                      onChange={handleGeneSelect}
                      style={{ width: '100%' }}
                    >
                      {genes.map(gene => (
                        <Select.Option key={gene.id} value={gene.id}>
                          {gene.geneSymbol} - {gene.name}
                        </Select.Option>
                      ))}
                    </Select>
                  </Form.Item>

                  {selectedGenes.length > 0 && (
                    <Form.Item
                      label="基因权重"
                    >
                      <Row gutter={[8, 8]}>
                        {selectedGenes.map(geneId => {
                          const gene = genes.find(g => g.id === geneId);
                          return (
                            <Col span={8} key={geneId}>
                              <Form.Item
                                name={['geneWeights', geneId.toString()]}
                                style={{ marginBottom: 0 }}
                              >
                                <InputNumber
                                  placeholder={gene?.geneSymbol || '基因'}
                                  min={0}
                                  max={10}
                                  step={0.1}
                                  onChange={(value) => handleWeightChange(geneId, value || 0)}
                                  addonAfter={gene?.geneSymbol || ''}
                                />
                              </Form.Item>
                            </Col>
                          );
                        })}
                      </Row>
                    </Form.Item>
                  )}

                  <Form.Item
                    name="formula"
                    label="公式"
                  >
                    <Input.TextArea 
                      rows={4} 
                      placeholder="请输入公式" 
                      readOnly
                    />
                  </Form.Item>

                  <Button 
                    type="primary" 
                    onClick={handleOpenFormulaEditor}
                    style={{ marginBottom: 16 }}
                  >
                    编辑公式
                  </Button>

                  {formulaEditorVisible && (
                    <FormulaEditor
                      genes={genes.filter(g => selectedGenes.includes(g.id))}
                      geneWeights={geneWeights}
                      formula={formula}
                      onSave={handleFormulaSave}
                      onCancel={() => setFormulaEditorVisible(false)}
                    />
                  )}
                </>
              )}

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={4} placeholder="请输入描述" />
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setModelModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
  ];

  return (
    <div>
      <Tabs defaultActiveKey="detectionType" items={tabsItems} />
    </div>
  );
};

export default DetectionType;
