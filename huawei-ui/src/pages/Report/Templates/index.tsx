import React, { useCallback, useEffect, useState } from 'react';
import { Button, Card, Col, Form, Input, InputNumber, Modal, Row, Select, Space, Table, Tabs, App } from 'antd';
import { ArrowLeftOutlined, PlusOutlined } from '@ant-design/icons';
import { useNavigate } from '@umijs/max';
import { createTemplate, deleteTemplate, getTemplates, updateTemplate, listCancerTypes, getTreatmentStages } from '@/services/api';

const templateTypeText: Record<string, string> = {
  result_explanation: '结果说明',
  signal_explanation: '信号值说明',
};

const reportCategoryText: Record<string, string> = {
  normal: '普通',
  high: '高敏',
  screening: '早筛',
};

const valueTypeText: Record<string, string> = {
  signal: '信号值',
  delta: '术前-术后差值',
};

const SCREENING_DETECTION_TYPE = '早筛检查';

const splitListValue = (value?: string) => String(value || '').split(',').map((item) => item.trim()).filter(Boolean);
const joinListValue = (value?: string[] | string) => Array.isArray(value) ? value.join(',') : value;

const ReportTemplates: React.FC = () => {
  const navigate = useNavigate();
  const { message } = App.useApp();
  const [loading, setLoading] = useState(false);
  const [templates, setTemplates] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [treatmentStages, setTreatmentStages] = useState<any[]>([]);
  const [activeType, setActiveType] = useState<'signal_explanation' | 'result_explanation'>('signal_explanation');
  const [modalVisible, setModalVisible] = useState(false);
  const [editingTemplate, setEditingTemplate] = useState<any>(null);
  const [form] = Form.useForm();

  const fetchTemplates = useCallback(async () => {
    setLoading(true);
    try {
      const response = await getTemplates();
      setTemplates(response.data?.list || []);
    } catch (_error) {
      message.error('获取报告模板失败');
    } finally {
      setLoading(false);
    }
  }, [message]);

  const fetchCancerTypes = useCallback(async () => {
    try {
      const response = await listCancerTypes();
      setCancerTypes(Array.isArray(response.data) ? response.data : []);
    } catch (_error) {
      message.error('获取检测类型失败');
    }
  }, [message]);

  const fetchTreatmentStages = useCallback(async () => {
    try {
      const response = await getTreatmentStages();
      setTreatmentStages(Array.isArray(response.data) ? response.data : []);
    } catch (_error) {
      message.error('获取治疗阶段失败');
    }
  }, [message]);

  useEffect(() => {
    fetchTemplates();
    fetchCancerTypes();
    fetchTreatmentStages();
  }, [fetchTemplates, fetchCancerTypes, fetchTreatmentStages]);

  const cancerDetectionTypeNames = cancerTypes
    .map((item) => item.name)
    .filter((name) => name && name !== SCREENING_DETECTION_TYPE);

  const resolveDetectionTypesByCategory = (category?: string) => {
    if (category === 'screening') return cancerTypes.some((item) => item.name === SCREENING_DETECTION_TYPE) ? [SCREENING_DETECTION_TYPE] : [];
    if (category === 'normal' || category === 'high') return cancerDetectionTypeNames;
    return undefined;
  };

  const openCreate = () => {
    setEditingTemplate(null);
    const defaultCategory = 'normal';
    form.setFieldsValue({
      type: activeType,
      valueType: 'signal',
      reportCategory: defaultCategory,
      detectionType: resolveDetectionTypesByCategory(defaultCategory),
      project: undefined,
      title: '',
      content: '',
    });
    setModalVisible(true);
  };

  const openEdit = (record: any) => {
    setEditingTemplate(record);
    form.setFieldsValue({
      ...record,
      detectionType: splitListValue(record.detectionType),
      project: splitListValue(record.project),
    });
    setModalVisible(true);
  };

  const handleSubmit = async (values: any) => {
    try {
      const resolvedDetectionTypes = values.reportCategory
        ? resolveDetectionTypesByCategory(values.reportCategory)
        : values.detectionType;
      const payload = {
        ...values,
        detectionType: joinListValue(resolvedDetectionTypes),
        project: joinListValue(values.project),
        reportVersion: undefined,
      };
      if (editingTemplate?.id) {
        await updateTemplate(String(editingTemplate.id), payload);
        message.success('模板更新成功');
      } else {
        await createTemplate(payload);
        message.success('模板创建成功');
      }
      setModalVisible(false);
      fetchTemplates();
    } catch (_error) {
      message.error('模板保存失败');
    }
  };

  const handleDelete = async (record: any) => {
    try {
      await deleteTemplate(String(record.id));
      message.success('模板删除成功');
      fetchTemplates();
    } catch (_error) {
      message.error('模板删除失败');
    }
  };

  const columns = [
    { title: '标题', dataIndex: 'title', key: 'title', width: 180 },
    { title: '检测类型', dataIndex: 'detectionType', key: 'detectionType', width: 180, render: (value: string) => value || '-' },
    { title: '值的类型', dataIndex: 'valueType', key: 'valueType', width: 120, render: (value: string) => valueTypeText[value] || value || '-' },
    { title: '报告类别', dataIndex: 'reportCategory', key: 'reportCategory', width: 110, render: (value: string) => reportCategoryText[value] || value || '-' },
    { title: '治疗阶段', dataIndex: 'project', key: 'project', width: 180, render: (value: string) => value || '-' },
    { title: '最小值', dataIndex: 'minSignalValue', key: 'minSignalValue', width: 90, render: (value: number) => value ?? '-' },
    { title: '最大值', dataIndex: 'maxSignalValue', key: 'maxSignalValue', width: 90, render: (value: number) => value ?? '-' },
    { title: '内容', dataIndex: 'content', key: 'content', ellipsis: true },
    {
      title: '操作',
      key: 'action',
      fixed: 'right' as const,
      width: 130,
      render: (_text: unknown, record: any) => (
        <Space>
          <Button type="link" onClick={() => openEdit(record)}>编辑</Button>
          <Button type="link" danger onClick={() => handleDelete(record)}>删除</Button>
        </Space>
      ),
    },
  ];

  const renderTemplateTable = (type: 'signal_explanation' | 'result_explanation') => (
    <Table
      rowKey="id"
      columns={columns}
      dataSource={templates.filter((template) => template.type === type)}
      loading={loading}
      scroll={{ x: 1400 }}
    />
  );

  return (
    <div style={{ padding: 24 }}>
      <Card
        title={
          <Space>
            <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/report')}>返回</Button>
            <span>报告模板</span>
          </Space>
        }
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增模板</Button>}
      >
        <Tabs
          activeKey={activeType}
          onChange={(key) => setActiveType(key as 'signal_explanation' | 'result_explanation')}
          items={[
            {
              key: 'signal_explanation',
              label: '信号值说明',
              children: renderTemplateTable('signal_explanation'),
            },
            {
              key: 'result_explanation',
              label: '结果说明',
              children: renderTemplateTable('result_explanation'),
            },
          ]}
        />
      </Card>

      <Modal
        title={editingTemplate ? '编辑报告模板' : '新增报告模板'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
        width={860}
      >
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="title" label="模板标题" rules={[{ required: true, message: '请输入模板标题' }]}>
                <Input />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="type" label="模板类型" rules={[{ required: true, message: '请选择模板类型' }]}>
                <Select>
                  <Select.Option value="result_explanation">结果说明</Select.Option>
                  <Select.Option value="signal_explanation">信号值说明</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="detectionType" label="检测类型">
                <Select mode="multiple" allowClear showSearch optionFilterProp="children" placeholder="请选择检测类型">
                  {cancerTypes.map((item) => (
                    <Select.Option key={item.id} value={item.name}>{item.name}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="valueType" label="值的类型">
                <Select allowClear>
                  <Select.Option value="signal">信号值</Select.Option>
                  <Select.Option value="delta">术前-术后差值</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="project" label="治疗阶段">
                <Select mode="multiple" allowClear showSearch optionFilterProp="children" placeholder="请选择治疗阶段">
                  {treatmentStages.map((stage) => (
                    <Select.Option key={stage.id} value={stage.name}>{stage.name}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="reportCategory" label="报告类别">
                <Select
                  allowClear
                  onChange={(value) => {
                    form.setFieldValue('detectionType', resolveDetectionTypesByCategory(value));
                  }}
                >
                  <Select.Option value="normal">高敏</Select.Option>
                  <Select.Option value="high">高敏</Select.Option>
                  <Select.Option value="screening">早筛</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="modelId" label="模型ID">
                <InputNumber style={{ width: '100%' }} min={1} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="minSignalValue" label="最小值">
                <InputNumber style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item name="maxSignalValue" label="最大值">
                <InputNumber style={{ width: '100%' }} />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item name="content" label="模板内容" rules={[{ required: true, message: '请输入模板内容' }]}>
            <Input.TextArea rows={10} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>保存</Button>
            <Button onClick={() => setModalVisible(false)}>取消</Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default ReportTemplates;
