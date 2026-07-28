import React, { useEffect, useMemo, useState } from 'react';
import { App, Button, Card, Form, Input, Select, Space, Table, Tag } from 'antd';
import { ReloadOutlined, SearchOutlined, UserSwitchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { assignSalesToPatient, listSalesAssignmentPatients, listUsers } from '@/services/api';

const getSalesPersonCode = (user: any) => String(user?.employee_id || '').trim();

const SalesAssignment: React.FC = () => {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [assignForm] = Form.useForm();
  const [patients, setPatients] = useState<any[]>([]);
  const [salesUsers, setSalesUsers] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [assigningId, setAssigningId] = useState<number | null>(null);
  const [pagination, setPagination] = useState({
    current: 1,
    pageSize: 10,
    total: 0,
    showSizeChanger: true,
  });

  const salesOptions = useMemo(
    () =>
      salesUsers.map((user) => ({
        value: getSalesPersonCode(user),
        label: `${user.real_name || user.username}${user.employee_id ? ` (${user.employee_id})` : ''}`,
      })),
    [salesUsers],
  );

  const fetchSalesUsers = async () => {
    try {
      const response = await listUsers();
      const data: any = response.data;
      const list = Array.isArray(data?.list) ? data.list : data || [];
      setSalesUsers(
        list.filter((user: any) => user.role_name === '销售' && getSalesPersonCode(user)),
      );
    } catch (_error) {
      message.error('获取销售列表失败');
    }
  };

  const fetchPatients = async (params: any = {}) => {
    setLoading(true);
    try {
      const values = form.getFieldsValue();
      const response = await listSalesAssignmentPatients({
        page: params.page || pagination.current,
        pageSize: params.pageSize || pagination.pageSize,
        keyword: values.keyword,
      });
      setPatients(response.data?.list || []);
      setPagination({
        ...pagination,
        current: params.page || pagination.current,
        pageSize: params.pageSize || pagination.pageSize,
        total: response.data?.total || 0,
      });
    } catch (_error) {
      message.error('获取待分配患者失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSalesUsers();
    fetchPatients({ page: 1 });
  }, []);

  const handleAssign = async (record: any) => {
    const salesPerson = assignForm.getFieldValue(['sales', record.id]);
    if (!salesPerson) {
      message.warning('请选择销售');
      return;
    }
    setAssigningId(record.id);
    try {
      await assignSalesToPatient({
        patient_id: record.id,
        sales_person: salesPerson,
      });
      message.success('分配销售成功');
      assignForm.setFieldValue(['sales', record.id], undefined);
      fetchPatients({ page: pagination.current });
    } catch (_error) {
      message.error('分配销售失败');
    } finally {
      setAssigningId(null);
    }
  };

  const columns = [
    { title: '患者编号', dataIndex: 'patientCode', key: 'patientCode', width: 150 },
    { title: '姓名', dataIndex: 'name', key: 'name', width: 120 },
    { title: '性别', dataIndex: 'gender', key: 'gender', width: 80 },
    { title: '身份证号', dataIndex: 'idCard', key: 'idCard', width: 190 },
    { title: '联系电话', dataIndex: 'phone', key: 'phone', width: 130 },
    {
      title: '来源',
      dataIndex: 'patientSource',
      key: 'patientSource',
      width: 110,
      render: () => <Tag color="blue">自主注册</Tag>,
    },
    {
      title: '注册时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      width: 140,
      render: (value: any) => (value ? dayjs(value).format('YYYY-MM-DD') : '-'),
    },
    {
      title: '分配销售',
      key: 'assign',
      width: 300,
      render: (_: any, record: any) => (
        <Space.Compact style={{ width: '100%' }}>
          <Form.Item name={['sales', record.id]} noStyle>
            <Select
              showSearch
              allowClear
              placeholder="选择销售"
              options={salesOptions}
              optionFilterProp="label"
              style={{ minWidth: 190 }}
            />
          </Form.Item>
          <Button
            type="primary"
            icon={<UserSwitchOutlined />}
            loading={assigningId === record.id}
            onClick={() => handleAssign(record)}
          >
            分配
          </Button>
        </Space.Compact>
      ),
    },
  ];

  return (
    <Card
      title="自主注册销售分配"
      extra={
        <Button icon={<ReloadOutlined />} onClick={() => fetchPatients({ page: 1 })}>
          刷新
        </Button>
      }
    >
      <Form form={form} layout="inline" onFinish={() => fetchPatients({ page: 1 })} style={{ marginBottom: 16 }}>
        <Form.Item name="keyword" label="患者">
          <Input placeholder="姓名/电话/身份证/编号" allowClear style={{ width: 240 }} />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" htmlType="submit" icon={<SearchOutlined />}>
              搜索
            </Button>
            <Button
              onClick={() => {
                form.resetFields();
                fetchPatients({ page: 1 });
              }}
            >
              重置
            </Button>
          </Space>
        </Form.Item>
      </Form>
      <Form form={assignForm}>
        <Table
          columns={columns}
          dataSource={patients}
          loading={loading}
          rowKey="id"
          pagination={pagination}
          scroll={{ x: 1220 }}
          onChange={(nextPagination) => {
            fetchPatients({
              page: nextPagination.current || 1,
              pageSize: nextPagination.pageSize || 10,
            });
          }}
        />
      </Form>
    </Card>
  );
};

export default SalesAssignment;
