import React, { useState, useEffect } from 'react';
import { Table, Button, Form, Input, Row, Col, message } from 'antd';
import { ArrowLeftOutlined, SearchOutlined } from '@ant-design/icons';
import { getSamples } from '@/services/api';
import { useNavigate } from '@umijs/max';

const SampleQuery: React.FC = () => {
  const [form] = Form.useForm();
  const [samples, setSamples] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState({});
  const navigate = useNavigate();

  const fetchSamples = async (params: any = {}) => {
    setLoading(true);
    try {
      const response = await getSamples({
        ...searchParams,
        ...params
      });
      // 处理数据字段映射，确保前端使用正确的字段名
      const processedSamples = response.data.list.map((sample: any) => ({
        id: sample.id || sample.id,
        sampleCode: sample.sampleCode || sample.sample_code || sample.sampleCode,
        patientName: sample.patientName || sample.patient_name || sample.patientName,
        collectionDate: sample.collectionDate || sample.collection_date || sample.collectionDate,
        status: sample.status || sample.status
      }));
      setSamples(processedSamples);
      setPagination({
        ...pagination,
        total: response.data.total,
        current: params?.page || 1
      });
    } catch (_error) {
      message.error('获取样本列表失败');
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

  const handleTableChange = (paginationConfig: any) => {
    fetchSamples({
      page: paginationConfig.current,
      pageSize: paginationConfig.pageSize
    });
  };

  const columns = [
    {
      title: '样本编号',
      dataIndex: 'sampleCode',
      key: 'sampleCode'
    },
    {
      title: '患者姓名',
      dataIndex: 'patientName',
      key: 'patientName'
    },
    {
      title: '采集日期',
      dataIndex: 'collectionDate',
      key: 'collectionDate',
      render: (date: string) => {
        if (!date) return '-';
        const d = new Date(date);
        if (isNaN(d.getTime())) return date;
        return `${d.getFullYear()}年${String(d.getMonth() + 1).padStart(2, '0')}月${String(d.getDate()).padStart(2, '0')}日 ${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}:${String(d.getSeconds()).padStart(2, '0')}`;
      }
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap = {
          'received': { text: '已接收', color: 'orange' },
          'tested': { text: '已检验', color: 'green' },
          'completed': { text: '已完成', color: 'blue' }
        };
        const statusInfo = statusMap[status as keyof typeof statusMap] || { text: status, color: 'default' };
        return <span style={{ color: statusInfo.color }}>{statusInfo.text}</span>;
      }
    },
    {
      title: '操作',
      key: 'action',
      render: (_text: any, record: any) => (
        <Button
          type="link"
          onClick={() => {
            navigate(`/result/detail/${encodeURIComponent(record.sampleCode)}`);
          }}
        >
          查看结果
        </Button>
      )
    }
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', alignItems: 'center' }}>
        <Button
          icon={<ArrowLeftOutlined />}
          onClick={() => navigate('/result/center')}
          style={{ marginRight: 16 }}
        >
          返回
        </Button>
        <h2>样本查询</h2>
      </div>

      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="patientName">
              <Input placeholder="患者姓名" prefix={<SearchOutlined />} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="sampleCode">
              <Input placeholder="样本编号" prefix={<SearchOutlined />} />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item>
              <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                搜索
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

export default SampleQuery;
