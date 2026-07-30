import React, { useEffect, useState } from 'react';
import { Button, Card, Col, Row, Statistic, Table, Typography, message } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { getSalesStatistics } from '@/services/api';

const SalesStatistics: React.FC = () => {
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<any>({ list: [] });

  const load = async () => {
    setLoading(true);
    try {
      const response = await getSalesStatistics();
      setData(response.data || { list: [] });
    } catch (error: any) {
      message.error(error.message || '销售统计加载失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    load();
  }, []);

  return (
    <Card title="销售统计" extra={<Button icon={<ReloadOutlined />} onClick={load}>刷新</Button>}>
      <Typography.Paragraph type="secondary">
        付款在线下完成，本页只统计患者、套餐与配置进度，不统计线上金额。
      </Typography.Paragraph>
      <Row gutter={16} style={{ marginBottom: 20 }}>
        <Col xs={24} md={6}><Card size="small"><Statistic title="套餐总数" value={data.totalOrderCount || 0} /></Card></Col>
        <Col xs={24} md={6}><Card size="small"><Statistic title="待配置" value={data.totalPendingCount || 0} /></Card></Col>
        <Col xs={24} md={6}><Card size="small"><Statistic title="进行中" value={data.totalActiveCount || 0} /></Card></Col>
        <Col xs={24} md={6}><Card size="small"><Statistic title="服务患者数" value={data.totalPatientCount || 0} /></Card></Col>
      </Row>
      <Table
        rowKey="salesPersonId"
        loading={loading}
        dataSource={data.list || []}
        pagination={false}
        columns={[
          { title: '销售人员', dataIndex: 'salesPersonName' },
          { title: '服务患者数', dataIndex: 'patientCount' },
          { title: '套餐数', dataIndex: 'sale_orderCount' },
          { title: '待配置', dataIndex: 'pendingConfigCount' },
          { title: '进行中', dataIndex: 'activeCount' },
        ]}
      />
    </Card>
  );
};

export default SalesStatistics;
