import React, { useState, useEffect } from 'react';
import { Table, Button, Form, Input, Select, Row, Col, message } from 'antd';
import { getSamples } from '@/services/api';
import { useNavigate } from '@umijs/max';

const List: React.FC = () => {
  const [form] = Form.useForm();
  const [samples, setSamples] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState({});
  const navigate = useNavigate();

  const fetchSamples = async (params: any = {}) => {
    setLoading(true);
    try {
      // 默认只查询已检验的样本
      const response = await getSamples({ 
        ...searchParams, 
        status: 'tested',
        ...params 
      });
      setSamples(response.data.list);
      setPagination({
        ...pagination,
        total: response.data.total,
        current: params?.page || 1
      });
    } catch (_error) {
      message.error('获取已检验样本列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSamples();
  }, [searchParams]);

  const handleSearch = (values: any) => {
    setSearchParams(values);
    fetchSamples({ page: 1 });
  };

  const columns = [
    { 
      title: '样本编号', 
      dataIndex: 'sample_code', 
      width: 180,
      key: 'sample_code',
      render: (text: any) => <span>{text}</span>
    },
    { 
      title: '患者信息', 
      key: 'patient',
      width: 160,
      render: (_text: any, record: any) => (
        <div>
          <div>{record.patient_name || '未知'}</div>
          <div style={{ fontSize: '12px', color: '#999' }}>{record.patient_code || '未知'}</div>
        </div>
      )
    },
    { 
      title: '样本类型', 
      dataIndex: 'sample_type_name', 
      key: 'sample_type_name',
      width: 100,
      render: (sampleTypeName: any) => sampleTypeName || '-'
    },
    { 
      title: '采集日期', 
      dataIndex: 'collection_date', 
      key: 'collection_date',
      width: 220,
      render: (date: string) => {
        if (!date) return '-';
        const d = new Date(date);
        const year = d.getFullYear();
        const month = String(d.getMonth() + 1).padStart(2, '0');
        const day = String(d.getDate()).padStart(2, '0');
        const hours = String(d.getHours()).padStart(2, '0');
        const minutes = String(d.getMinutes()).padStart(2, '0');
        const seconds = String(d.getSeconds()).padStart(2, '0');
        return `${year}年${month}月${day}日 ${hours}:${minutes}:${seconds}`;
      }
    },
    { 
      title: '状态', 
      dataIndex: 'status', 
      key: 'status',
      width: 100,
      render: (status: string) => {
        const statusMap = {
          'created': { text: '已创建', color: 'default' },
          'collected': { text: '已采集', color: 'default' },
          'received': { text: '已接收', color: 'blue' },
          'processing': { text: '处理中', color: 'orange' },
          'tested': { text: '已检验', color: 'green' },
          'completed': { text: '已完成', color: 'green' }
        };
        const statusInfo = statusMap[status as keyof typeof statusMap] || { text: status, color: 'default' };
        return <span style={{ color: statusInfo.color }}>{statusInfo.text}</span>;
      }
    },
    { 
      title: '操作', 
      key: 'action',
      width: 100,
      render: (_text: any, record: any) => (
        <Button 
          type="link" 
          size="small"
          onClick={() => navigate(`/result/detail/${encodeURIComponent(record.sampleCode || record.sample_code)}`)}
        >
          查看详情
        </Button>
      ),
    },
  ];

  const handleTableChange = (paginationConfig: any) => {
    fetchSamples({
      page: paginationConfig.current,
      pageSize: paginationConfig.pageSize
    });
  };

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>已检验样本列表</h2>
      </div>

      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={5}>
            <Form.Item name="sampleCode">
              <Input placeholder="样本编号" />
            </Form.Item>
          </Col>
          <Col span={5}>
            <Form.Item name="patientName">
              <Input placeholder="患者姓名" />
            </Form.Item>
          </Col>
          <Col span={5}>
            <Form.Item name="sampleType">
              <Select placeholder="样本类型" allowClear>
                <Select.Option value="血液">血液</Select.Option>
                <Select.Option value="尿液">尿液</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={5}>
            <Form.Item name="status">
              <Select placeholder="样本状态" allowClear>
                <Select.Option value="tested">已检验</Select.Option>
                <Select.Option value="completed">已完成</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={4}>
            <Form.Item>
              <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                查询
              </Button>
              <Button type="default" onClick={() => form.resetFields()}>
                重置
              </Button>
            </Form.Item>
          </Col>
        </Row>
      </Form>

      <Table
        columns={columns}
        dataSource={samples}
        rowKey="id"
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条记录`
        }}
        onChange={handleTableChange}
      />
    </div>
  );
};

export default List;
