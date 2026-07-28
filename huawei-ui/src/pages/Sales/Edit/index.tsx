import React, { useState, useEffect } from 'react';
import {
  Card,
  Form,
  Input,
  Select,
  DatePicker,
  Button,
  Table,
  message,
  Modal,
  Space,
} from 'antd';
import { EditOutlined } from '@ant-design/icons';
import { getOrders, getDetectionPlans, updateDetectionPlan } from '@/services/api';
import dayjs from 'dayjs';

const EditPage: React.FC = () => {
  const [form] = Form.useForm();
  const [orders, setOrders] = useState<any[]>([]);
  const [detectionPlans, setDetectionPlans] = useState<any[]>([]);
  const [selectedOrder, setSelectedOrder] = useState<number | null>(null);
  const [editingPlan, setEditingPlan] = useState<any | null>(null);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [loading, setLoading] = useState(false);

  // 获取订单列表
  useEffect(() => {
    fetchOrders();
  }, []);

  const fetchOrders = async () => {
    try {
      setLoading(true);
      const response = await getOrders();
      // 全面处理后端返回的数据格式，确保获取到正确的数组
      if (response && response.data) {
        if (Array.isArray((response.data as any).list)) {
          setOrders((response.data as any).list);
        } else if (Array.isArray(response.data)) {
          setOrders(response.data);
        } else {
          setOrders([]);
        }
      } else {
        setOrders([]);
      }
    } catch (error) {
      console.error('获取订单列表失败:', error);
      message.error('获取订单列表失败');
      setOrders([]);
    } finally {
      setLoading(false);
    }
  };

  // 获取检测计划
  const fetchDetectionPlans = async (orderId: number) => {
    try {
      setLoading(true);
      const response = await getDetectionPlans(orderId);
      // 全面处理后端返回的数据格式，确保获取到正确的数组
      if (response && response.data) {
        if (Array.isArray((response.data as any).list)) {
          setDetectionPlans((response.data as any).list);
        } else if (Array.isArray(response.data)) {
          setDetectionPlans(response.data);
        } else {
          setDetectionPlans([]);
        }
      } else {
        setDetectionPlans([]);
      }
    } catch (error) {
      console.error('获取检测计划失败:', error);
      message.error('获取检测计划失败');
      setDetectionPlans([]);
    } finally {
      setLoading(false);
    }
  };

  // 选择订单
  const handleOrderSelect = (orderId: number) => {
    setSelectedOrder(orderId);
    fetchDetectionPlans(orderId);
  };

  // 打开编辑模态框
  const handleEditPlan = (plan: any) => {
    setEditingPlan(plan);
    setEditModalVisible(true);
  };

  // 保存检测计划
  const handleSavePlan = async (values: any) => {
    try {
      setLoading(true);
      const response = await updateDetectionPlan({
        id: editingPlan.id,
        detectionDate: values.detection_date.format('YYYY-MM-DD'),
      });
      message.success('保存成功');
      setEditModalVisible(false);
      fetchDetectionPlans(selectedOrder!);
    } catch (error) {
      message.error('网络错误');
    } finally {
      setLoading(false);
    }
  };

  // 订单列
  const orderColumns = [
    {
      title: '订单编号',
      dataIndex: 'id',
      key: 'id',
    },
    {
      title: '患者身份证号',
      dataIndex: 'detect_patientIdCard',
      key: 'detect_patientIdCard',
    },
    {
      title: '套餐名称',
      dataIndex: 'sale_packageName',
      key: 'sale_packageName',
    },
    {
      title: '癌型',
      dataIndex: 'cancerTypeName',
      key: 'cancerTypeName',
    },
    {
      title: '购买日期',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => dayjs(text).format('YYYY-MM-DD'),
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Button
          type="primary"
          onClick={() => handleOrderSelect(record.id)}
        >
          查看检测计划
        </Button>
      ),
    },
  ];

  // 检测计划列
  const planColumns = [
    {
      title: '序号',
      dataIndex: 'detectionNumber',
      key: 'detectionNumber',
    },
    {
      title: '检测日期',
      dataIndex: 'detectionDate',
      key: 'detectionDate',
      render: (text: string) => dayjs(text).format('YYYY-MM-DD'),
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap: { [key: string]: string } = {
          'pending': '待检测',
          'completed': '已完成',
          'canceled': '已取消',
        };
        return statusMap[status] || status;
      },
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <Button
          icon={<EditOutlined />}
          onClick={() => handleEditPlan(record)}
        >
          编辑
        </Button>
      ),
    },
  ];

  return (
    <div className="sales-edit-page">
      <Card title="个性化编辑" className="mb-4">
        <h3 className="mb-4">选择订单</h3>
        <Table
          columns={orderColumns}
          dataSource={orders}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      {selectedOrder && (
        <Card title="检测计划编辑" className="mb-4">
          <Table
            columns={planColumns}
            dataSource={detectionPlans}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10 }}
          />
        </Card>
      )}

      {/* 编辑检测计划模态框 */}
      <Modal
        title="编辑检测计划"
        open={editModalVisible}
        onCancel={() => setEditModalVisible(false)}
        footer={null}
      >
        <Form
          form={form}
          initialValues={{
            detection_date: editingPlan ? dayjs(editingPlan.detectionDate) : null,
          }}
          onFinish={handleSavePlan}
        >
          <Form.Item
            name="detection_date"
            label="检测日期"
            rules={[{ required: true, message: '请选择检测日期' }]}
          >
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item>
            <Space style={{ width: '100%', justifyContent: 'flex-end' }}>
              <Button onClick={() => setEditModalVisible(false)}>
                取消
              </Button>
              <Button type="primary" htmlType="submit" loading={loading}>
                保存
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default EditPage;
