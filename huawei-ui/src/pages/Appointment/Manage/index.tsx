import React, { useEffect, useState } from 'react';
import {
  Button,
  Card,
  Form,
  Input,
  Modal,
  Select,
  Space,
  Table,
  Tag,
  Tabs,
  Upload,
  message,
} from 'antd';
import { DownloadOutlined, ReloadOutlined, SearchOutlined, UploadOutlined } from '@ant-design/icons';
import { listAppointments, updateAppointment, uploadAppointmentTracking } from '@/services/api';

const statusMeta: Record<string, { text: string; color: string }> = {
  requested: { text: '待邮寄', color: 'orange' },
  shipped: { text: '已邮寄', color: 'green' },
};

const expressCompanyOptions = [
  { label: '顺丰速运', value: '顺丰速运' },
  { label: '京东快递', value: '京东快递' },
];

const AppointmentManage: React.FC = () => {
  const [form] = Form.useForm();
  const [shipForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [list, setList] = useState<any[]>([]);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [filters, setFilters] = useState<any>({});
  const [activeStatus, setActiveStatus] = useState('requested');
  const [currentRecord, setCurrentRecord] = useState<any>(null);

  const fetchList = async (extra: any = {}) => {
    setLoading(true);
    try {
      const current = extra.current || extra.page || pagination.current;
      const pageSize = extra.pageSize || pagination.pageSize;
      const response = await listAppointments({
        ...filters,
        ...extra,
        status: activeStatus,
        current,
        pageSize,
      });
      const data = response.data || { list: [], total: 0 };
      setList(data.list || []);
      setPagination({ current, pageSize, total: data.total || 0 });
    } catch (error: any) {
      message.error(error.message || '获取预约列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchList({ current: 1 });
  }, [filters, activeStatus]);

  const openShipModal = (record: any) => {
    setCurrentRecord(record);
    shipForm.setFieldsValue({
      expressCompany: record.express_company || '顺丰速运',
      trackingNumber: record.tracking_number || '',
      status: 'shipped',
      notes: record.notes || '',
    });
  };

  const saveShipping = async () => {
    if (!currentRecord) return;
    const values = await shipForm.validateFields();
    setSaving(true);
    try {
      await updateAppointment(currentRecord.id, values);
      message.success('保存成功');
      setCurrentRecord(null);
      fetchList();
    } catch (error: any) {
      message.error(error.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const downloadTemplate = () => {
    const header = '预约ID,患者编号,患者电话,收件人电话,快递公司,运单号\n';
    const example = '1,HWP2026060001,13800000000,13800000000,顺丰速运,SF1234567890\n';
    const blob = new Blob([`\uFEFF${header}${example}`], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = '预约快递单号导入模板.csv';
    link.click();
    URL.revokeObjectURL(link.href);
  };

  const exportAppointments = async () => {
    try {
      const response = await listAppointments({
        ...filters,
        status: activeStatus,
        current: 1,
        pageSize: 200,
      });
      const rows = response.data?.list || [];
      const headers = ['预约ID', '提交时间', '患者姓名', '患者编号', '患者电话', '收件人', '收件人电话', '邮寄地址', '快递公司', '运单号', '状态'];
      const escapeCsv = (value: any) => `"${String(value ?? '').replace(/"/g, '""')}"`;
      const body = rows.map((item: any) => [
        item.id,
        item.created_at,
        item.patient_name,
        item.patient_code,
        item.patient_phone,
        item.receiver_name,
        item.receiver_phone,
        item.full_address,
        item.express_company,
        item.tracking_number,
        item.status_text || statusMeta[item.status]?.text || item.status,
      ].map(escapeCsv).join(',')).join('\n');
      const blob = new Blob([`\uFEFF${headers.join(',')}\n${body}`], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `预约邮寄_${activeStatus === 'shipped' ? '已寄出' : '未寄出'}.csv`;
      link.click();
      URL.revokeObjectURL(link.href);
      message.success('导出成功');
    } catch (error: any) {
      message.error(error.message || '导出失败');
    }
  };

  const uploadTrackingFile = async (file: File) => {
    const formData = new FormData();
    formData.append('file', file);
    setUploading(true);
    try {
      const response = await uploadAppointmentTracking(formData);
      const failedCount = response.data?.failed_count || 0;
      if (failedCount > 0) {
        Modal.warning({
          title: response.message || '导入完成',
          content: (
            <div>
              {(response.data?.failed || []).slice(0, 8).map((item: any) => (
                <div key={`${item.row}-${item.reason}`}>第{item.row}行：{item.reason}</div>
              ))}
              {failedCount > 8 && <div>还有 {failedCount - 8} 条失败记录未显示</div>}
            </div>
          ),
        });
      } else {
        message.success(response.message || '批量导入成功');
      }
      fetchList();
    } catch (error: any) {
      message.error(error.message || '批量导入失败');
    } finally {
      setUploading(false);
    }
    return false;
  };

  const columns = [
    {
      title: '提交时间',
      dataIndex: 'created_at',
      width: 170,
    },
    {
      title: '患者',
      dataIndex: 'patient_name',
      width: 180,
      render: (_: any, record: any) => (
        <div>
          <div>{record.patient_name || '-'}</div>
          <div style={{ color: '#8c8c8c', fontSize: 12 }}>{record.patient_code || record.patient_phone || '-'}</div>
        </div>
      ),
    },
    {
      title: '收件人',
      dataIndex: 'receiver_name',
      width: 130,
      render: (_: any, record: any) => (
        <div>
          <div>{record.receiver_name || '-'}</div>
          <div style={{ color: '#8c8c8c', fontSize: 12 }}>{record.receiver_phone || '-'}</div>
        </div>
      ),
    },
    {
      title: '邮寄地址',
      dataIndex: 'full_address',
      width: 360,
      render: (text: string) => (
        <div style={{ whiteSpace: 'normal', wordBreak: 'break-all', lineHeight: 1.5 }}>{text || '-'}</div>
      ),
    },
    {
      title: '套餐/订单',
      width: 190,
      render: (_: any, record: any) => (
        <div>
          <div>{record.package_name || '-'}</div>
          <div style={{ color: '#8c8c8c', fontSize: 12 }}>{record.order_no || '-'}</div>
        </div>
      ),
    },
    {
      title: '状态',
      dataIndex: 'status',
      width: 100,
      render: (status: string) => {
        const meta = statusMeta[status] || { text: status || '待邮寄', color: 'orange' };
        return <Tag color={meta.color}>{meta.text}</Tag>;
      },
    },
    {
      title: '运单号',
      width: 180,
      render: (_: any, record: any) => (
        <div>
          <div>{record.tracking_number || '-'}</div>
          {record.express_company && <div style={{ color: '#8c8c8c', fontSize: 12 }}>{record.express_company}</div>}
        </div>
      ),
    },
    {
      title: '操作',
      width: 120,
      fixed: 'right' as const,
      render: (_: any, record: any) => (
        <Button type="link" onClick={() => openShipModal(record)}>
          录入运单
        </Button>
      ),
    },
  ];

  return (
    <Card
      title="预约管理"
      extra={
        <Space>
          <Button icon={<DownloadOutlined />} onClick={downloadTemplate}>
            下载模板
          </Button>
          <Button icon={<DownloadOutlined />} onClick={exportAppointments}>
            批量导出
          </Button>
          <Upload
            accept=".csv,.xlsx,.xls"
            showUploadList={false}
            beforeUpload={(file) => uploadTrackingFile(file)}
            disabled={uploading}
          >
            <Button type="primary" icon={<UploadOutlined />} loading={uploading}>
              批量上传快递单号
            </Button>
          </Upload>
        </Space>
      }
    >
      <Tabs
        activeKey={activeStatus}
        onChange={(key) => {
          setActiveStatus(key);
          setPagination((prev) => ({ ...prev, current: 1 }));
        }}
        items={[
          { key: 'requested', label: '未寄出' },
          { key: 'shipped', label: '已寄出' },
        ]}
      />

      <Form
        form={form}
        layout="inline"
        onFinish={(values) => {
          setFilters(values);
          setPagination((prev) => ({ ...prev, current: 1 }));
        }}
        style={{ marginBottom: 16 }}
      >
        <Form.Item name="keyword">
          <Input allowClear placeholder="患者/电话/运单号" prefix={<SearchOutlined />} />
        </Form.Item>
        <Form.Item>
          <Space>
            <Button type="primary" htmlType="submit">
              搜索
            </Button>
            <Button
              onClick={() => {
                form.resetFields();
                setFilters({});
              }}
            >
              重置
            </Button>
            <Button icon={<ReloadOutlined />} onClick={() => fetchList()}>
              刷新
            </Button>
          </Space>
        </Form.Item>
      </Form>

      <Table
        columns={columns}
        dataSource={list}
        rowKey="id"
        loading={loading}
        scroll={{ x: 1320 }}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showTotal: (total) => `共 ${total} 条记录`,
        }}
        onChange={(pager) => fetchList({ current: pager.current, pageSize: pager.pageSize })}
      />

      <Modal
        title="录入运单号"
        open={!!currentRecord}
        onCancel={() => setCurrentRecord(null)}
        onOk={saveShipping}
        confirmLoading={saving}
        destroyOnClose
      >
        <Form form={shipForm} layout="vertical" preserve={false}>
          <Form.Item name="expressCompany" label="快递公司">
            <Select options={expressCompanyOptions} />
          </Form.Item>
          <Form.Item name="trackingNumber" label="运单号">
            <Input placeholder="请输入运单号" />
          </Form.Item>
          <Form.Item name="notes" label="备注">
            <Input.TextArea rows={3} placeholder="可填写邮寄备注" />
          </Form.Item>
        </Form>
      </Modal>
    </Card>
  );
};

export default AppointmentManage;
