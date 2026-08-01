import React, { useEffect, useState } from 'react';
import { Button, Card, Collapse, Descriptions, Empty, Row, Col, Spin, Table, Tag, message } from 'antd';
import { ExperimentOutlined, FileTextOutlined } from '@ant-design/icons';
import { useNavigate, useParams } from '@umijs/max';
import { getSamples, listGenes, updateSampleGeneData } from '@/services/api';
import EChartsHeatmap from '@/components/EChartsHeatmap';

const reportStatus = (status?: string) => {
  const states: Record<string, { color: string; text: string }> = {
    pending: { color: 'orange', text: '生成中' },
    completed: { color: 'green', text: '已完成' },
    reviewed: { color: 'green', text: '已审核' },
    failed: { color: 'red', text: '生成失败' },
  };
  const current = states[String(status || '').toLowerCase()];
  return current ? <Tag color={current.color}>{current.text}</Tag> : <Tag>{status || '未出报告'}</Tag>;
};

const ResultDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [sample, setSample] = useState<any>();
  const [genes, setGenes] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);

  const load = async () => {
    if (!id) return;
    setLoading(true);
    try {
      const [sampleResponse, genesResponse] = await Promise.all([getSamples({ id }), listGenes()]);
      const item = sampleResponse.data?.list?.[0];
      if (!item) {
        message.error('样本不存在');
        return;
      }
      setSample(item);
      setGenes(Array.isArray(genesResponse.data) ? genesResponse.data : []);
    } catch (_error) {
      message.error('获取样本结果失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, [id]);

  if (loading) return <div style={{ textAlign: 'center', padding: '100px 0' }}><Spin size="large" /></div>;
  if (!sample) return <Empty description="样本不存在" />;

  const geneEntries = Object.entries(sample.gene_data || {}).map(([gene, value]) => ({
    key: gene,
    gene,
    value: Number(value) || 0,
    threshold: genes.find((item: any) => item.geneSymbol === gene)?.threshold || 0,
  }));
  const panelMatches = Array.isArray(sample.panel_matches) ? sample.panel_matches : [];

  return (
    <div style={{ maxWidth: 1400, margin: '0 auto', padding: '0 24px' }}>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>样本结果详情</h2>
        <Button onClick={() => navigate('/result/sample-query')}>返回列表</Button>
      </div>

      <Card size="small">
        <Descriptions column={{ xs: 1, sm: 3 }} size="small">
          <Descriptions.Item label="癌种">{sample.cancer_type_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="检测情况">
            {geneEntries.length > 0 ? <Tag color="green">已上传检测结果</Tag> : <Tag>暂无检测结果</Tag>}
          </Descriptions.Item>
          <Descriptions.Item label="报告情况">{reportStatus(sample.report_status)}</Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title={<><ExperimentOutlined style={{ marginRight: 8 }} />检测情况</>} style={{ marginTop: 16 }}>
        {geneEntries.length > 0 ? (
          <Collapse
            defaultActiveKey={['heatmap', 'table', 'panel']}
            items={[
              {
                key: 'heatmap', label: '基因表达热力图',
                children: <EChartsHeatmap data={geneEntries.map(({ gene, value }) => ({ gene, value }))} onDataChange={async (data) => {
                  const geneData = data.reduce((result: Record<string, number>, item: any) => ({ ...result, [item.gene]: item.value }), {});
                  try {
                    await updateSampleGeneData(id!, geneData);
                    setSample({ ...sample, gene_data: geneData });
                    message.success('检测数据已更新');
                  } catch (_error) { message.error('检测数据更新失败'); }
                }} />,
              },
              {
                key: 'table', label: '基因值列表',
                children: <Table size="small" dataSource={geneEntries} pagination={{ pageSize: 10, size: 'small' }} columns={[
                  { title: '基因', dataIndex: 'gene' },
                  { title: '表达值', dataIndex: 'value' },
                  { title: '阈值', dataIndex: 'threshold' },
                ]} />,
              },
              {
                key: 'panel', label: '按 Panel 分布',
                children: panelMatches.length === 0 ? <Empty description="暂无 Panel 匹配数据" image={Empty.PRESENTED_IMAGE_SIMPLE} /> : (
                  <Row gutter={[12, 12]}>{panelMatches.map((panel: any) => (
                    <Col xs={24} md={12} xl={8} key={panel.panelId || panel.panelCode}>
                      <Card size="small" title={panel.panelName || panel.panelCode || '未命名 Panel'}>
                        <Descriptions size="small" column={1}>
                          <Descriptions.Item label="匹配状态"><Tag color={panel.matchColor || 'default'}>{panel.matchStatus === 'exact' ? '完全匹配' : '基因不足'}</Tag></Descriptions.Item>
                          <Descriptions.Item label="匹配率">{Math.round(Number(panel.matchRate || 0) * 100)}% ({panel.matchCount || 0}/{panel.totalGenes || 0})</Descriptions.Item>
                          <Descriptions.Item label="缺失基因">{Array.isArray(panel.missingGenes) && panel.missingGenes.length ? panel.missingGenes.join(', ') : '-'}</Descriptions.Item>
                        </Descriptions>
                      </Card>
                    </Col>
                  ))}</Row>
                ),
              },
            ]}
          />
        ) : <Empty description="暂无检测数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />}
      </Card>

      <Card title={<><FileTextOutlined style={{ marginRight: 8 }} />报告情况</>} style={{ marginTop: 16 }}>
        {sample.has_report ? <Descriptions column={{ xs: 1, sm: 3 }} size="small">
          <Descriptions.Item label="报告状态">{reportStatus(sample.report_status)}</Descriptions.Item>
          <Descriptions.Item label="出具时间">{sample.report_generated_time || '-'}</Descriptions.Item>
          <Descriptions.Item label="审核时间">{sample.report_reviewed_time || '待审核'}</Descriptions.Item>
          <Descriptions.Item label="患者查看">{sample.patient_viewed ? `已查看 ${sample.patient_viewed_at || ''}` : '未查看'}</Descriptions.Item>
        </Descriptions> : <Empty description="未出报告" image={Empty.PRESENTED_IMAGE_SIMPLE} />}
      </Card>
    </div>
  );
};

export default ResultDetail;
