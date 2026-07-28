import React, { useState, useEffect } from 'react';
import { Form, Input, InputNumber, Button, message, Table, Space, Modal, Select, Row, Col, Tabs, DatePicker, Card } from 'antd';
import { PlusOutlined, EditOutlined } from '@ant-design/icons';
import { listPackages, createPackage, updatePackage, listPatientPackages, bindPatientPackage, listCancerTypes } from '@/services/api';

const { TextArea } = Input;

const SalesConfigure: React.FC = () => {
  const [form] = Form.useForm();
  const [editForm] = Form.useForm();
  const [packages, setPackages] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [editLoading, setEditLoading] = useState(false);
  const [refresh, setRefresh] = useState(false);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [currentPackage, setCurrentPackage] = useState<any>(null);
  const [createModalVisible, setCreateModalVisible] = useState(false);
  const [bindForm] = Form.useForm();
  const [bindModalVisible, setBindModalVisible] = useState(false);
  const [patientPackages, setPatientPackages] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [bindingLoading, setBindingLoading] = useState(false);

  // 加载套餐列表
  useEffect(() => {
    const loadPackages = async () => {
      try {
        const response = await listPackages();
        setPackages(response.data?.list || []);
      } catch (error) {
        message.error('加载套餐列表失败');
      }
    };

    loadPackages();
  }, [refresh]);

  useEffect(() => {
    const loadBindings = async () => {
      try {
        const response = await listPatientPackages();
        setPatientPackages(response.data?.list || []);
      } catch (error) {
        message.error('加载用户套餐绑定失败');
      }
    };
    const loadCancerTypes = async () => {
      try {
        const response = await listCancerTypes();
        setCancerTypes(Array.isArray(response.data) ? response.data : []);
      } catch (error) {
        message.error('加载检测类型失败');
      }
    };
    loadBindings();
    loadCancerTypes();
  }, [refresh]);

  // 处理创建套餐
  const handleCreatePackage = async (values: any) => {
    try {
      setLoading(true);
      const response = await createPackage({
        name: values.name,
        detectionCount: values.detectionCount,
        intervalDays: values.intervalDays,
        price: values.price,
        description: values.description,
        status: values.status,
      });
      message.success('套餐创建成功');
      form.resetFields();
      setCreateModalVisible(false);
      setRefresh(!refresh);
    } catch (error) {
      message.error('套餐创建失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  // 处理编辑套餐
  const handleEditPackage = async (values: any) => {
    try {
      setEditLoading(true);
      if (!currentPackage) return;
      
      const response = await updatePackage({
        id: currentPackage.id,
        name: values.name,
        detectionCount: values.detectionCount,
        intervalDays: values.intervalDays,
        price: values.price,
        description: values.description,
        status: values.status,
      });
      message.success('套餐编辑成功');
      editForm.resetFields();
      setEditModalVisible(false);
      setCurrentPackage(null);
      setRefresh(!refresh);
    } catch (error) {
      message.error('套餐编辑失败，请稍后重试');
    } finally {
      setEditLoading(false);
    }
  };

  // 打开编辑模态框
  const openEditModal = (record: any) => {
    setCurrentPackage(record);
    editForm.setFieldsValue({
      name: record.name,
      detectionCount: record.detectionCount,
      intervalDays: record.intervalDays,
      price: record.price,
      description: record.description,
      status: record.status,
    });
    setEditModalVisible(true);
  };

  const handleBindPatientPackage = async (values: any) => {
    try {
      setBindingLoading(true);
      await bindPatientPackage({
        patientIdCard: values.patientIdCard,
        packageId: values.packageId,
        cancerTypeId: values.cancerTypeId,
        firstDetectionDate: values.firstDetectionDate.format('YYYY-MM-DD'),
        paymentMethod: values.paymentMethod || 'offline',
        payment_status: values.paymentStatus || 'paid',
      });
      message.success('套餐绑定成功');
      bindForm.resetFields();
      setBindModalVisible(false);
      setRefresh(!refresh);
    } catch (error: any) {
      message.error(error?.message || '套餐绑定失败');
    } finally {
      setBindingLoading(false);
    }
  };

  // 表格列定义
  const columns = [
    {
      title: '套餐名称',
      dataIndex: 'name',
      key: 'name',
      render: (text: any) => text || '-'
    },
    {
      title: '检查次数',
      dataIndex: 'detectionCount',
      key: 'detectionCount',
      render: (detectionCount: any) => detectionCount || '-'
    },
    {
      title: '间隔天数',
      dataIndex: 'intervalDays',
      key: 'intervalDays',
      render: (intervalDays: any) => intervalDays || '-'
    },
    {
      title: '价格',
      dataIndex: 'price',
      key: 'price',
      render: (price: any) => price ? `¥${price}` : '-',
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (status === 'active' ? '启用' : '禁用'),
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
      render: (description: any) => description || '-'
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (createdAt: string) => createdAt ? new Date(createdAt).toLocaleString() : '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Space size="middle">
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => openEditModal(record)} 
          >
            编辑
          </Button>
        </Space>
      ),
    },
  ];

  const packageConfigView = (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>套餐配置</h2>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => setCreateModalVisible(true)}>
            创建套餐
          </Button>
        </Space>
      </div>

      <Form layout="inline" style={{ marginBottom: 16 }}>
        <Row gutter={16} align="middle">
          <Col span={6}>
            <Form.Item name="name">
              <Input placeholder="套餐名称" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="status">
              <Select placeholder="套餐状态" allowClear>
                <Select.Option value="active">启用</Select.Option>
                <Select.Option value="inactive">禁用</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={12}>
            <Space>
              <Button type="primary">查询</Button>
              <Button type="default">重置</Button>
            </Space>
          </Col>
        </Row>
      </Form>

      <Table
        dataSource={packages}
        columns={columns}
        rowKey="id"
        pagination={{
          pageSize: 10,
          showSizeChanger: true,
          pageSizeOptions: ['10', '20', '50'],
          showTotal: (total) => `共 ${total} 个套餐`,
        }}
        bordered
        rowHoverable
        loading={loading}
      />

      {/* 创建套餐模态框 */}
      <Modal
        title="创建套餐"
        open={createModalVisible}
        onCancel={() => setCreateModalVisible(false)}
        footer={null}
        width={800}
      >
        <Form
          form={form}
          layout="vertical"
          onFinish={handleCreatePackage}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label="套餐名称"
                name="name"
                rules={[{ required: true, message: '请输入套餐名称' }]}
              >
                <Input placeholder="请输入套餐名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="检查次数"
                name="detectionCount"
                rules={[{ required: true, message: '请输入检查次数' }]}
              >
                <InputNumber style={{ width: '100%' }} min={1} placeholder="请输入检查次数" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="间隔天数"
                name="intervalDays"
                rules={[{ required: true, message: '请输入每次检查相隔天数' }]}
              >
                <InputNumber style={{ width: '100%' }} min={1} placeholder="请输入每次检查相隔天数" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="套餐价格"
                name="price"
                rules={[{ required: true, message: '请输入套餐价格' }]}
              >
                <InputNumber style={{ width: '100%' }} min={0} step={0.01} placeholder="请输入套餐价格" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="套餐状态"
                name="status"
                rules={[{ required: true, message: '请选择套餐状态' }]}
              >
                <Select placeholder="请选择套餐状态">
                  <Select.Option value="active">启用</Select.Option>
                  <Select.Option value="inactive">禁用</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                label="套餐描述"
                name="description"
              >
                <TextArea rows={4} placeholder="请输入套餐描述" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item>
            <Space>
              <Button onClick={() => setCreateModalVisible(false)}>
                取消
              </Button>
              <Button type="primary" htmlType="submit" loading={loading}>
                创建套餐
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      {/* 编辑套餐模态框 */}
      <Modal
        title="编辑套餐"
        open={editModalVisible}
        onCancel={() => {
          setEditModalVisible(false);
          setCurrentPackage(null);
          editForm.resetFields();
        }}
        footer={null}
        width={800}
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={handleEditPackage}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                label="套餐名称"
                name="name"
                rules={[{ required: true, message: '请输入套餐名称' }]}
              >
                <Input placeholder="请输入套餐名称" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="检查次数"
                name="detectionCount"
                rules={[{ required: true, message: '请输入检查次数' }]}
              >
                <InputNumber style={{ width: '100%' }} min={1} placeholder="请输入检查次数" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="间隔天数"
                name="intervalDays"
                rules={[{ required: true, message: '请输入每次检查相隔天数' }]}
              >
                <InputNumber style={{ width: '100%' }} min={1} placeholder="请输入每次检查相隔天数" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="套餐价格"
                name="price"
                rules={[{ required: true, message: '请输入套餐价格' }]}
              >
                <InputNumber style={{ width: '100%' }} min={0} step={0.01} placeholder="请输入套餐价格" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                label="套餐状态"
                name="status"
                rules={[{ required: true, message: '请选择套餐状态' }]}
              >
                <Select placeholder="请选择套餐状态">
                  <Select.Option value="active">启用</Select.Option>
                  <Select.Option value="inactive">禁用</Select.Option>
                </Select>
              </Form.Item>
            </Col>
            <Col span={24}>
              <Form.Item
                label="套餐描述"
                name="description"
              >
                <TextArea rows={4} placeholder="请输入套餐描述" />
              </Form.Item>
            </Col>
          </Row>

          <Form.Item>
            <Space>
              <Button onClick={() => {
                setEditModalVisible(false);
                setCurrentPackage(null);
                editForm.resetFields();
              }}>
                取消
              </Button>
              <Button type="primary" htmlType="submit" loading={editLoading}>
                保存修改
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </>
  );

  const bindingView = (
    <>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>用户绑定套餐</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setBindModalVisible(true)}>
          绑定套餐
        </Button>
      </div>
      <Table
        dataSource={patientPackages}
        rowKey="id"
        bordered
        columns={[
          { title: '患者', dataIndex: 'patientName', key: 'patientName' },
          { title: '手机号', dataIndex: 'patientPhone', key: 'patientPhone', render: (v) => v || '-' },
          { title: '套餐', dataIndex: 'packageName', key: 'packageName' },
          { title: '检测类型', dataIndex: 'cancerTypeName', key: 'cancerTypeName' },
          { title: '首次检测日', dataIndex: 'firstDetectionDate', key: 'firstDetectionDate', render: (v) => v || '-' },
          { title: '销售', dataIndex: 'salesPersonName', key: 'salesPersonName' },
          { title: '状态', dataIndex: 'status', key: 'status', render: (v) => v || '-' },
        ]}
      />
      <Modal
        title="绑定用户套餐"
        open={bindModalVisible}
        onCancel={() => setBindModalVisible(false)}
        footer={null}
        width={760}
      >
        <Card size="small" bordered={false}>
          <Form form={bindForm} layout="vertical" onFinish={handleBindPatientPackage}>
            <Row gutter={16}>
              <Col span={12}>
                <Form.Item
                  label="患者身份证号"
                  name="patientIdCard"
                  rules={[{ required: true, message: '请输入患者身份证号' }]}
                >
                  <Input placeholder="请输入患者身份证号" />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="套餐" name="packageId" rules={[{ required: true, message: '请选择套餐' }]}>
                  <Select placeholder="请选择套餐">
                    {packages.map((item) => (
                      <Select.Option key={item.id} value={item.id}>
                        {item.name} / {item.detectionCount}次
                      </Select.Option>
                    ))}
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="检测类型" name="cancerTypeId" rules={[{ required: true, message: '请选择检测类型' }]}>
                  <Select placeholder="请选择检测类型">
                    {cancerTypes.map((item) => (
                      <Select.Option key={item.id} value={item.id}>{item.name}</Select.Option>
                    ))}
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="首次检测日期" name="firstDetectionDate" rules={[{ required: true, message: '请选择首次检测日期' }]}>
                  <DatePicker style={{ width: '100%' }} />
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="付款方式" name="paymentMethod" initialValue="offline">
                  <Select>
                    <Select.Option value="offline">线下支付</Select.Option>
                    <Select.Option value="online">在线支付</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col span={12}>
                <Form.Item label="付款状态" name="paymentStatus" initialValue="paid">
                  <Select>
                    <Select.Option value="paid">已付款</Select.Option>
                    <Select.Option value="pending">待付款</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
            <Form.Item>
              <Space>
                <Button onClick={() => setBindModalVisible(false)}>取消</Button>
                <Button type="primary" htmlType="submit" loading={bindingLoading}>保存绑定</Button>
              </Space>
            </Form.Item>
          </Form>
        </Card>
      </Modal>
    </>
  );

  return (
    <div>
      <Tabs
        defaultActiveKey="packages"
        items={[
          { key: 'packages', label: '套餐配置', children: packageConfigView },
          { key: 'bindings', label: '用户绑定套餐', children: bindingView },
        ]}
      />
    </div>
  );
};

export default SalesConfigure;
