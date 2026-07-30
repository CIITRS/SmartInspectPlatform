import React, { useEffect, useMemo, useState } from 'react';
import { Button, Card, DatePicker, Drawer, Input, Progress, Space, Table, Tag, Typography, message } from 'antd';
import { CalendarOutlined, ReloadOutlined, SearchOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import { getDetectionPlans, listPatientPackages, updateDetectionPlan } from '@/services/api';

const { Text } = Typography;

const orderStatus: Record<string, { text: string; color: string }> = {
  pending: { text: '待确认', color: 'orange' },
  pending_config: { text: '待配置', color: 'gold' },
  active: { text: '进行中', color: 'blue' },
  completed: { text: '已完成', color: 'green' },
  cancelled: { text: '已取消', color: 'default' },
};

const PatientPackageManage: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [plansLoading, setPlansLoading] = useState(false);
  const [orders, setOrders] = useState<any[]>([]);
  const [plans, setPlans] = useState<any[]>([]);
  const [current, setCurrent] = useState<any>();
  const [keyword, setKeyword] = useState('');
  const [savingID, setSavingID] = useState<number>();

  const loadOrders = async () => {
    setLoading(true);
    try {
      const response = await listPatientPackages();
      setOrders(response.data?.list || []);
    } catch (error: any) {
      message.error(error.message || '患者套餐加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadOrders();
  }, []);

  const openPlans = async (record: any) => {
    setCurrent(record);
    setPlansLoading(true);
    try {
      const response = await getDetectionPlans(record.id);
      setPlans(response.data?.list || []);
    } catch (error: any) {
      message.error(error.message || '检测计划加载失败');
    } finally {
      setPlansLoading(false);
    }
  };

  const savePlan = async (plan: any, value: dayjs.Dayjs | null) => {
    if (!value) {
      message.warning('请选择检测时间');
      return;
    }
    setSavingID(plan.id);
    try {
      await updateDetectionPlan({ id: plan.id, detectionDate: value.format('YYYY-MM-DD') });
      message.success(`第 ${plan.detectionNumber} 次检测时间已保存`);
      await openPlans(current);
      await loadOrders();
    } catch (error: any) {
      message.error(error.message || '保存失败');
    } finally {
      setSavingID(undefined);
    }
  };

  const visibleOrders = useMemo(() => {
    const value = keyword.trim().toLowerCase();
    if (!value) return orders;
    return orders.filter((item) =>
      [item.patientName, item.patientPhone, item.packageName, item.sale_orderNo, item.cancerTypeName]
        .some((field) => String(field || '').toLowerCase().includes(value)),
    );
  }, [orders, keyword]);

  const columns = [
    {
      title: '患者',
      width: 180,
      render: (_: any, row: any) => (
        <div>
          <div>{row.patientName || '-'}</div>
          <Text type="secondary">{row.patientPhone || '-'}</Text>
        </div>
      ),
    },
    {
      title: '套餐',
      width: 190,
      render: (_: any, row: any) => (
        <div>
          <div>{row.packageName || '-'}</div>
          <Text type="secondary">{row.cancerTypeName || '未配置癌型'}</Text>
        </div>
      ),
    },
    {
      title: '首次扫码时间',
      dataIndex: 'firstScanAt',
      width: 180,
      render: (value: string) => value || <Text type="secondary">尚未扫码</Text>,
    },
    {
      title: '检测计划',
      width: 190,
      render: (_: any, row: any) => {
        const total = row.planCount || 0;
        const configured = row.configuredCount || 0;
        return (
          <div style={{ minWidth: 150 }}>
            <Progress percent={total ? Math.round((configured / total) * 100) : 0} size="small" showInfo={false} />
            <Text type="secondary">已配置 {configured}/{total} 次</Text>
          </div>
        );
      },
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (value: string) => {
        const meta = orderStatus[value] || { text: value || '-', color: 'default' };
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: '销售',
      dataIndex: 'salesPersonName',
      width: 120,
      render: (value: string) => value || '待分配',
    },
    {
      title: '操作',
      width: 130,
      fixed: 'right' as const,
      render: (_: any, row: any) => (
        <Button type="link" icon={<CalendarOutlined />} onClick={() => openPlans(row)}>
          配置时间
        </Button>
      ),
    },
  ];

  return (
    <>
      <Card
        title="患者套餐管理"
        extra={<Button icon={<ReloadOutlined />} onClick={loadOrders}>刷新</Button>}
      >
        <Space style={{ marginBottom: 16 }}>
          <Input
            allowClear
            value={keyword}
            onChange={(event) => setKeyword(event.target.value)}
            prefix={<SearchOutlined />}
            placeholder="患者、电话、套餐、癌型"
            style={{ width: 320 }}
          />
          <Text type="secondary">患者提交套餐后，由所属销售配置每一次检测时间；付款在线下完成。</Text>
        </Space>
        <Table
          rowKey="id"
          columns={columns}
          dataSource={visibleOrders}
          loading={loading}
          scroll={{ x: 1120 }}
          pagination={{ pageSize: 10, showTotal: (total) => `共 ${total} 个患者套餐` }}
        />
      </Card>

      <Drawer
        width={620}
        open={!!current}
        title={`${current?.patientName || ''} · ${current?.packageName || ''}`}
        onClose={() => {
          setCurrent(undefined);
          setPlans([]);
        }}
      >
        <Card size="small" style={{ marginBottom: 16 }}>
          <Space direction="vertical" size={4}>
            <Text>订单：{current?.sale_orderNo || '-'}</Text>
            <Text>癌型：{current?.cancerTypeName || '-'}</Text>
            <Text strong>第一次扫码时间：{current?.firstScanAt || '尚未扫码'}</Text>
          </Space>
        </Card>
        <Table
          rowKey="id"
          loading={plansLoading}
          pagination={false}
          dataSource={plans}
          columns={[
            {
              title: '检测次数',
              dataIndex: 'detectionNumber',
              width: 100,
              render: (value) => `第 ${value} 次`,
            },
            {
              title: '时间',
              render: (_: any, plan: any) => (
                <DatePicker
                  defaultValue={plan.detectionDate ? dayjs(plan.detectionDate) : null}
                  disabled={plan.detectionNumber === 1 && !!plan.firstScanAt}
                  placeholder={plan.detectionNumber === 1 && plan.firstScanAt ? plan.firstScanAt : '选择检测日期'}
                  onChange={(value) => { plan.pendingDate = value; }}
                  style={{ width: '100%' }}
                />
              ),
            },
            {
              title: '操作',
              width: 90,
              render: (_: any, plan: any) => (
                plan.detectionNumber === 1 && plan.firstScanAt
                  ? <Tag color="green">扫码已记录</Tag>
                  : (
                    <Button
                      type="primary"
                      size="small"
                      loading={savingID === plan.id}
                      onClick={() => savePlan(plan, plan.pendingDate || (plan.detectionDate ? dayjs(plan.detectionDate) : null))}
                    >
                      保存
                    </Button>
                  )
              ),
            },
          ]}
        />
      </Drawer>
    </>
  );
};

export default PatientPackageManage;
