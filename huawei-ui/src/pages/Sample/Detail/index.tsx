import React, { useState, useEffect } from 'react';
import { Card, Descriptions, Button, Spin, message, Tag, Steps, Table, Collapse, Row, Col, Empty, Modal, Form, Input, Select, DatePicker, Space, Timeline, Alert } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, FileTextOutlined, ExperimentOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from '@umijs/max';
import { getSamples, listModels, listCancerTypes, listGenes, updateSampleGeneData, getSampleExpress, saveSampleExpress, updateSampleExpress, refreshSampleExpress } from '@/services/api';
import EChartsHeatmap from '@/components/EChartsHeatmap';
import dayjs from 'dayjs';

const formatDateTime = (value?: string) => value && dayjs(value).isValid() ? dayjs(value).format('YYYY/MM/DD HH:mm') : '-';
const buildBarcodeImageUrl = (code?: string) => code ? `/api/samples/barcode?sample_code=${encodeURIComponent(code)}` : '';

const Detail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [sample, setSample] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [_models, setModels] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [_genes, setGenes] = useState<any[]>([]);
  const [applicableModels, setApplicableModels] = useState<any[]>([]);
  const [expressModalVisible, setExpressModalVisible] = useState(false);
  const [expressData, setExpressData] = useState<Record<'inbound' | 'outbound', any>>({ inbound: null, outbound: null });
  const [expressDirection, setExpressDirection] = useState<'inbound' | 'outbound'>('inbound');
  const [expressLoading, setExpressLoading] = useState(false);
  const [expressForm] = Form.useForm();
  const navigate = useNavigate();

  useEffect(() => {
    if (id) {
      fetchSampleDetail();
      fetchExpressData();
    }
  }, [id]);

  const fetchExpressData = async () => {
    if (!id) return;
    try {
      const [inbound, outbound] = await Promise.all([
        getSampleExpress(id, 'inbound'),
        getSampleExpress(id, 'outbound'),
      ]);
      setExpressData({ inbound: inbound.data || null, outbound: outbound.data || null });
    } catch (error) {
      console.error('获取快递运单失败:', error);
    }
  };

  const handleOpenExpressModal = (direction: 'inbound' | 'outbound') => {
    setExpressDirection(direction);
    const current = expressData[direction];
    if (current) {
      expressForm.setFieldsValue({
        trackingNumber: current.tracking_number,
        expressType: current.express_type || 'auto',
        expressCompany: current.express_company,
        queryMobile: current.query_mobile,
        sendTime: current.send_time ? dayjs(current.send_time) : null,
        receiveTime: current.receive_time ? dayjs(current.receive_time) : null,
        status: current.status,
        notes: current.notes,
      });
    } else {
      expressForm.resetFields();
      expressForm.setFieldsValue({ expressType: 'auto', status: 'in_transit' });
    }
    setExpressModalVisible(true);
  };

  const handleSaveExpress = async () => {
    try {
      const values = await expressForm.validateFields();
      setExpressLoading(true);
      
      const expressInfo = {
        trackingNumber: values.trackingNumber,
        direction: expressDirection,
        expressType: values.expressType || 'auto',
        expressCompany: values.expressCompany,
        queryMobile: values.queryMobile,
        sendTime: values.sendTime ? values.sendTime.format('YYYY-MM-DD HH:mm:ss') : null,
        receiveTime: values.receiveTime ? values.receiveTime.format('YYYY-MM-DD HH:mm:ss') : null,
        status: values.status,
        notes: values.notes,
      };
      
      const current = expressData[expressDirection];
      if (current) {
        await updateSampleExpress(String(current.id), expressInfo);
        message.success('快递运单已更新');
      } else {
        await saveSampleExpress(String(sample.id), expressInfo);
        message.success('快递运单已保存');
      }
      
      await fetchExpressData();
      setExpressModalVisible(false);
    } catch (error: any) {
      if (error.errorFields) {
        return;
      }
      message.error(error.message || '保存快递运单失败');
    } finally {
      setExpressLoading(false);
    }
  };

  const handleRefreshExpress = async (direction: 'inbound' | 'outbound') => {
    const current = expressData[direction];
    if (!current?.id) return;
    setExpressLoading(true);
    try {
      const response = await refreshSampleExpress(current.id);
      message.success(response.message || '物流状态已更新');
      await fetchExpressData();
    } catch (error: any) {
      message.error(error.message || '物流查询失败');
    } finally {
      setExpressLoading(false);
    }
  };

  const fetchSampleDetail = async () => {
    setLoading(true);
    try {
      // 获取样本详情
      const response = await getSamples({ id });
      if (response.data.list && response.data.list.length > 0) {
        const sampleData = response.data.list[0];
        setSample(sampleData);
        
        // 并行获取模型、癌症类型和基因数据
        const [modelsResponse, cancerTypesResponse, genesResponse] = await Promise.all([
          listModels(),
          listCancerTypes(),
          listGenes()
        ]);
        
        // 处理模型数据
        if (modelsResponse.data) {
          setModels(modelsResponse.data);
          // 确定适用的模型（这里简化处理，实际应根据样本信息和模型配置判断）
          const applicable = modelsResponse.data.filter((model: any) => 
            model.status === 'active' // 假设只考虑激活状态的模型
          );
          setApplicableModels(applicable);
        }
        
        // 处理癌症类型数据
        if (cancerTypesResponse.data) {
          setCancerTypes(cancerTypesResponse.data);
        }
        
        // 处理基因数据
        if (genesResponse.data) {
          setGenes(genesResponse.data);
        }
      } else {
        message.error('样本不存在');
      }
    } catch (_error) {
      message.error('获取样本详情失败');
    } finally {
      setLoading(false);
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <Spin size="large" tip="加载中..." />
      </div>
    );
  }

  if (!sample) {
    return (
      <div style={{ textAlign: 'center', padding: '100px 0' }}>
        <p>样本不存在</p>
        <Button onClick={() => navigate('/sample/list')}>返回列表</Button>
      </div>
    );
  }

  const renderExpressPanel = (direction: 'inbound' | 'outbound') => {
    const current = expressData[direction];
    const directionName = direction === 'inbound' ? '患者寄出 → 实验室' : '公司发货 → 患者';
    const statusMap: Record<string, { color: string; text: string }> = {
      pending: { color: 'orange', text: '待揽件' },
      picked_up: { color: 'blue', text: '已揽件' },
      sent: { color: 'blue', text: '已发货' },
      in_transit: { color: 'cyan', text: '运输中' },
      delivered: { color: 'green', text: '已签收' },
      exception: { color: 'red', text: '物流异常' },
      returned: { color: 'red', text: '已退回' },
    };
    return (
      <Card
        type="inner"
        title={directionName}
        extra={(
          <Space>
            {current?.id && current.status !== 'delivered' && (
              <Button loading={expressLoading} onClick={() => handleRefreshExpress(direction)}>查询物流</Button>
            )}
            <Button type="primary" onClick={() => handleOpenExpressModal(direction)}>
              {current ? '更换当前运单' : '录入运单'}
            </Button>
          </Space>
        )}
      >
        {!current ? (
          <Empty description="暂无当前运单" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        ) : (
          <>
            {current.last_query_error && (
              <Alert type="warning" showIcon message={current.last_query_error} style={{ marginBottom: 16 }} />
            )}
            <Descriptions column={2} bordered size="small">
              <Descriptions.Item label="快递单号">{current.tracking_number || '-'}</Descriptions.Item>
              <Descriptions.Item label="快递公司">{current.express_company || current.express_type || '自动识别'}</Descriptions.Item>
              <Descriptions.Item label="状态">
                <Tag color={(statusMap[current.status] || {}).color || 'default'}>
                  {(statusMap[current.status] || {}).text || current.status || '-'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="签收时间">{current.delivered_at || '-'}</Descriptions.Item>
              <Descriptions.Item label="最新动态" span={2}>{current.latest_event_status || '-'}</Descriptions.Item>
              <Descriptions.Item label="最后查询">{current.last_query_at || '-'}</Descriptions.Item>
              <Descriptions.Item label="备注">{current.notes || '-'}</Descriptions.Item>
            </Descriptions>
            {current.status === 'delivered' ? (
              <Alert
                type="success"
                showIcon
                message="运单已签收，中间物流路径已清除，仅保留签收状态与时间。"
                style={{ marginTop: 16 }}
              />
            ) : Array.isArray(current.route) && current.route.length > 0 ? (
              <Timeline
                style={{ marginTop: 20 }}
                items={current.route.map((event: any) => ({
                  children: <><div>{event.status}</div><div style={{ color: '#8c8c8c' }}>{event.time}</div></>,
                }))}
              />
            ) : null}
          </>
        )}
      </Card>
    );
  };

  return (
    <div style={{ maxWidth: 1400, margin: '0 auto', padding: '0 24px' }}>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>样本详细信息</h2>
        <Button onClick={() => navigate('/result/sample-query')}>返回列表</Button>
      </div>

      <Card>
        <Descriptions column={2} bordered>
          <Descriptions.Item label="样本编号">
            <Space direction="vertical" size={4}>
              <span>{sample.sample_code}</span>
              <img
                src={buildBarcodeImageUrl(sample.sample_code)}
                alt={`${sample.sample_code} 条形码`}
                style={{ width: 240, maxWidth: '100%', height: 68, objectFit: 'contain', border: '1px solid #f0f0f0', borderRadius: 4, background: '#fff', padding: 4 }}
              />
            </Space>
          </Descriptions.Item>
          <Descriptions.Item label="患者姓名">{sample.patient_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="患者身份证号">{sample.id_card || sample.patient_id_card || '-'}</Descriptions.Item>
          <Descriptions.Item label="样本类型">{sample.sample_type_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="采集日期">
            {sample.collection_date ? (
              function() {
                const d = new Date(sample.collection_date);
                const year = d.getFullYear();
                const month = String(d.getMonth() + 1).padStart(2, '0');
                const day = String(d.getDate()).padStart(2, '0');
                const hours = String(d.getHours()).padStart(2, '0');
                const minutes = String(d.getMinutes()).padStart(2, '0');
                const seconds = String(d.getSeconds()).padStart(2, '0');
                return `${year}年${month}月${day}日 ${hours}:${minutes}:${seconds}`;
              }()
            ) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="当前状态">
            {(() => {
              const statusMap: any = {
                'created': <Tag color="default">已创建</Tag>,
                'collected': <Tag color="default">已采集</Tag>,
                'received': <Tag color="blue">已接收</Tag>,
                'processing': <Tag color="orange">处理中</Tag>,
                'tested': <Tag color="green">已检验</Tag>,
                'completed': <Tag color="green">已完成</Tag>,
              };
              return statusMap[sample.status] || sample.status;
            })()}
          </Descriptions.Item>
          <Descriptions.Item label="创建人员">{sample.collection_user_name || sample.collection_operator || '-'}</Descriptions.Item>
          <Descriptions.Item label="接收人员">{sample.receive_user_name || sample.receive_operator || '-'}</Descriptions.Item>
          <Descriptions.Item label="癌种">{sample.cancer_type_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="模型">{sample.model_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="备注" span={2}>{sample.notes || '-'}</Descriptions.Item>
        </Descriptions>
      </Card>

      {/* 样本状态时间线 */}
      <Card title="样本状态" style={{ marginTop: 16 }}>
        <div style={{ overflowX: 'auto', paddingBottom: 4 }}>
          <Steps
            responsive
            current={['created', 'collected'].includes(sample.status) ? (expressData.inbound ? 2 : sample.status === 'collected' ? 1 : 0) : sample.has_report ? (sample.report_reviewed_time ? (sample.patient_viewed ? 7 : 6) : 5) : ['tested', 'completed'].includes(sample.status) ? 4 : 3}
            status={sample.status === 'processing' ? 'process' : undefined}
            items={[
              {
                title: '样本创建',
                description: sample.created_at ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{formatDateTime(sample.created_at)}</div>
                    <div>{sample.collection_user_name || sample.collection_operator || '-'}</div>
                  </div>
                ) : '未设置',
              },
              {
                title: '样本采集',
                description: ['collected', 'sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>已采集</div>
                    <div>{formatDateTime(sample.collection_date)}</div>
                    <div>{sample.collection_user_name || sample.collection_operator || '-'}</div>
                  </div>
                ) : '待采集',
              },
              {
                title: '运输中',
                description: expressData.inbound ? (
                  <div style={{ fontSize: 12, maxWidth: 180 }}>
                    <div>{expressData.inbound.status === 'delivered' ? '已送达' : '送检中'}</div>
                    <div>单号：{expressData.inbound.tracking_number || '-'}</div>
                    <div>{expressData.inbound.latest_event_status || formatDateTime(expressData.inbound.latest_event_time)}</div>
                  </div>
                ) : '暂无运单',
              },
              {
                title: '样本接收',
                description: ['received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{formatDateTime(sample.receive_date)}</div>
                    <div>{sample.receive_user_name || sample.receive_operator || '-'}</div>
                  </div>
                ) : '待接收',
              },
              {
                title: '样本检测',
                description: ['tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{formatDateTime(sample.test_completed_at)}</div>
                    <div>{sample.test_user_name || sample.test_operator || '-'}</div>
                  </div>
                ) : '待检测',
              },
              {
                title: sample.has_report ? '已出报告' : '未出报告',
                description: sample.has_report ? <div style={{ fontSize: 12 }}><div>{formatDateTime(sample.report_generated_time)}</div><div>{sample.report_generated_by_name || '-'}</div></div> : '待生成',
              },
              {
                title: '报告审核',
                description: sample.report_reviewed_time ? <div style={{ fontSize: 12 }}><div>已审核</div><div>{formatDateTime(sample.report_reviewed_time)}</div><div>{sample.report_reviewed_by_name || '-'}</div></div> : sample.has_report ? '待审核' : '未生成报告',
              },
              {
                title: '患者查看',
                description: sample.patient_viewed ? <div style={{ fontSize: 12 }}><div>已查看</div><div>{formatDateTime(sample.patient_viewed_at)}</div></div> : '未查看',
              },
            ]}
          />
          {Array.isArray(expressData.inbound?.route) && expressData.inbound.route.length > 0 && expressData.inbound.status !== 'delivered' && (
            <Timeline
              style={{ marginTop: 24 }}
              items={expressData.inbound.route.map((event: any) => ({ children: <><div>{event.status}</div><div style={{ color: '#8c8c8c' }}>{event.time}</div></> }))}
            />
          )}
        </div>
      </Card>

      {/* 快递运单信息 */}
      <Card title="快递全周期管理" style={{ marginTop: 16 }}>
        <Row gutter={[16, 16]}>
          <Col xs={24} xl={12}>{renderExpressPanel('inbound')}</Col>
          <Col xs={24} xl={12}>{renderExpressPanel('outbound')}</Col>
        </Row>
      </Card>

      {/* 检测情况展示区域 */}
      {sample.status !== 'received' && sample.status !== 'sent' && (
        <Card 
          title={
            <span>
              <ExperimentOutlined style={{ marginRight: 8 }} />
              检测情况
            </span>
          } 
          style={{ marginTop: 16 }}
        >
          {sample.gene_data ? (
          <Collapse
            defaultActiveKey={['heatmap', 'table']}
            items={[
              {
                key: 'heatmap',
                label: '基因表达热力图',
                children: (
                  <EChartsHeatmap 
                    data={Object.entries(sample.gene_data).map(([gene, value]) => ({
                      gene,
                      value: parseFloat(value as string) || 0
                    }))} 
                    onDataChange={async (newData) => {
                      try {
                        const geneDataMap: any = {};
                        newData.forEach(item => {
                          geneDataMap[item.gene] = item.value;
                        });
                        
                        if (id) {
                          await updateSampleGeneData(id, geneDataMap);
                        }
                        
                        const updatedSample = {
                          ...sample,
                          gene_data: geneDataMap
                        };
                        setSample(updatedSample);
                        
                        message.success('检测数据已更新');
                      } catch (error) {
                        console.error('更新检测数据失败:', error);
                        message.error('更新检测数据失败，请重试');
                      }
                    }}
                  />
                )
              },
              {
                key: 'table',
                label: '基因值列表',
                children: (
                  <Table
                    dataSource={Object.entries(sample.gene_data).map(([gene, value], index) => {
                      const geneInfo = _genes.find((g: any) => g.geneSymbol === gene);
                      return {
                        key: index,
                        gene,
                        value: parseFloat(value as string) || 0,
                        threshold: geneInfo?.threshold || 0
                      };
                    })}
                    columns={[
                      { title: '基因', dataIndex: 'gene', width: 150 },
                      { title: '表达值', dataIndex: 'value', width: 120 },
                      { title: '阈值', dataIndex: 'threshold', width: 120 }
                    ]}
                    pagination={{ pageSize: 5, size: 'small' }}
                    size="small"
                  />
                )
              }
            ]}
          />
        ) : (
          <Empty description="暂无检测数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}
        </Card>
      )}

      {/* 报告情况展示区域 */}
      <Card 
        title={
          <span>
            <FileTextOutlined style={{ marginRight: 8 }} />
            报告情况
          </span>
        } 
        style={{ marginTop: 16 }}
      >
        {sample.report_id || sample.report_status ? (
          <Row gutter={[16, 16]}>
            <Col span={8}>
              <Card size="small" bordered={false} style={{ background: '#fafafa' }}>
                <div style={{ color: '#666', marginBottom: 4 }}>报告编号</div>
                <div style={{ fontSize: 16, fontWeight: 500 }}>{sample.report_code || sample.report_id || '-'}</div>
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small" bordered={false} style={{ background: '#fafafa' }}>
                <div style={{ color: '#666', marginBottom: 4 }}>报告状态</div>
                <div style={{ fontSize: 16, fontWeight: 500 }}>
                  {(() => {
                    const reportStatusMap: Record<string, { color: string; text: string }> = {
                      'pending': { color: 'orange', text: '生成中' },
                      'completed': { color: 'green', text: '已完成' },
                      'failed': { color: 'red', text: '生成失败' }
                    };
                    const status = reportStatusMap[sample.report_status];
                    return status ? <Tag color={status.color}>{status.text}</Tag> : sample.report_status || '-';
                  })()}
                </div>
              </Card>
            </Col>
            <Col span={8}>
              <Card size="small" bordered={false} style={{ background: '#fafafa' }}>
                <div style={{ color: '#666', marginBottom: 4 }}>生成时间</div>
                <div style={{ fontSize: 16, fontWeight: 500 }}>
                  {sample.report_generated_at ? (() => {
                    const d = new Date(sample.report_generated_at);
                    const year = d.getFullYear();
                    const month = String(d.getMonth() + 1).padStart(2, '0');
                    const day = String(d.getDate()).padStart(2, '0');
                    const hours = String(d.getHours()).padStart(2, '0');
                    const minutes = String(d.getMinutes()).padStart(2, '0');
                    return `${year}/${month}/${day} ${hours}:${minutes}`;
                  })() : '-'}
                </div>
              </Card>
            </Col>
          </Row>
        ) : (
          <Empty description="暂无报告" image={Empty.PRESENTED_IMAGE_SIMPLE} />
        )}
      </Card>

      {/* 快递运单编辑弹窗 */}
      <Modal
        title={`${expressDirection === 'inbound' ? '患者寄出' : '公司发货'}运单`}
        open={expressModalVisible}
        onCancel={() => setExpressModalVisible(false)}
        onOk={handleSaveExpress}
        confirmLoading={expressLoading}
        width={500}
        okText="保存"
        cancelText="取消"
      >
        <Form
          form={expressForm}
          layout="vertical"
          style={{ marginTop: 20 }}
        >
          <Form.Item
            name="trackingNumber"
            label="快递单号"
            rules={[{ required: true, message: '请输入快递单号' }]}
          >
            <Input placeholder="请输入快递单号" />
          </Form.Item>
          <Form.Item
            name="expressType"
            label="快递公司"
            rules={[{ required: true, message: '请选择快递公司' }]}
          >
            <Select placeholder="请选择快递公司">
              <Select.Option value="auto">自动识别</Select.Option>
              <Select.Option value="sfexpress">顺丰速运</Select.Option>
              <Select.Option value="yuantong">圆通快递</Select.Option>
              <Select.Option value="zhongtong">中通快递</Select.Option>
              <Select.Option value="yunda">韵达快递</Select.Option>
              <Select.Option value="shentong">申通快递</Select.Option>
              <Select.Option value="jd">京东物流</Select.Option>
              <Select.Option value="ems">邮政 EMS</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="expressCompany" label="快递公司显示名称">
            <Input placeholder="自动识别时可留空" />
          </Form.Item>
          <Form.Item
            name="queryMobile"
            label="收/寄件手机号"
            tooltip="顺丰查询必须填写收件人或寄件人手机号。"
          >
            <Input maxLength={20} placeholder="顺丰必填，其他快递选填" />
          </Form.Item>
          <Space style={{ width: '100%' }} size="middle">
            <Form.Item
              name="sendTime"
              label="寄件时间"
              style={{ flex: 1 }}
            >
              <DatePicker style={{ width: '100%' }} showTime format="YYYY-MM-DD HH:mm:ss" />
            </Form.Item>
            <Form.Item
              name="receiveTime"
              label="收件时间"
              style={{ flex: 1 }}
            >
              <DatePicker style={{ width: '100%' }} showTime format="YYYY-MM-DD HH:mm:ss" />
            </Form.Item>
          </Space>
          <Form.Item
            name="status"
            label="状态"
            rules={[{ required: true, message: '请选择状态' }]}
          >
            <Select placeholder="请选择状态">
              <Select.Option value="pending">待发货</Select.Option>
              <Select.Option value="sent">已发货</Select.Option>
              <Select.Option value="in_transit">运输中</Select.Option>
              <Select.Option value="delivered">已签收</Select.Option>
              <Select.Option value="returned">已退回</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item
            name="notes"
            label="备注"
          >
            <Input.TextArea rows={3} placeholder="请输入备注信息" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Detail;
