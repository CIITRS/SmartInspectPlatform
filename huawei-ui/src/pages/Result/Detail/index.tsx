import React, { useState, useEffect } from 'react';
import { Card, Descriptions, Button, Spin, message, Tag, Steps, Table, Collapse, Row, Col, Empty, Modal, Form, Input, Select, DatePicker, Space } from 'antd';
import { CheckCircleOutlined, ClockCircleOutlined, FileTextOutlined, ExperimentOutlined } from '@ant-design/icons';
import { useParams, useNavigate } from '@umijs/max';
import { getSamples, listModels, listCancerTypes, listGenes, updateSampleGeneData, getSampleExpress, saveSampleExpress, updateSampleExpress } from '@/services/api';
import EChartsHeatmap from '@/components/EChartsHeatmap';
import dayjs from 'dayjs';

const Detail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const [sample, setSample] = useState<any>(null);
  const [loading, setLoading] = useState(true);
  const [_models, setModels] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [_genes, setGenes] = useState<any[]>([]);
  const [applicableModels, setApplicableModels] = useState<any[]>([]);
  const [expressModalVisible, setExpressModalVisible] = useState(false);
  const [expressData, setExpressData] = useState<any>(null);
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
      const response = await getSampleExpress(id);
      if (response.data) {
        setExpressData(response.data);
      }
    } catch (error) {
      console.error('获取快递运单失败:', error);
    }
  };

  const handleOpenExpressModal = () => {
    if (expressData) {
      expressForm.setFieldsValue({
        trackingNumber: expressData.trackingNumber,
        expressCompany: expressData.expressCompany,
        sendTime: expressData.sendTime ? dayjs(expressData.sendTime) : null,
        receiveTime: expressData.receiveTime ? dayjs(expressData.receiveTime) : null,
        status: expressData.status,
        notes: expressData.notes,
      });
    } else {
      expressForm.resetFields();
    }
    setExpressModalVisible(true);
  };

  const handleSaveExpress = async () => {
    try {
      const values = await expressForm.validateFields();
      setExpressLoading(true);
      
      const expressInfo = {
        trackingNumber: values.trackingNumber,
        expressCompany: values.expressCompany,
        sendTime: values.sendTime ? values.sendTime.format('YYYY-MM-DD HH:mm:ss') : null,
        receiveTime: values.receiveTime ? values.receiveTime.format('YYYY-MM-DD HH:mm:ss') : null,
        status: values.status,
        notes: values.notes,
      };
      
      if (expressData) {
        await updateSampleExpress(id!, expressInfo);
        message.success('快递运单已更新');
      } else {
        await saveSampleExpress(id!, expressInfo);
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
        <Button onClick={() => navigate('/result/sample-query')}>返回列表</Button>
      </div>
    );
  }

  return (
    <div style={{ maxWidth: 1400, margin: '0 auto', padding: '0 24px' }}>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>样本详细信息</h2>
        <Button onClick={() => navigate('/result/sample-query')}>返回列表</Button>
      </div>

      <Card>
        <Descriptions column={2} bordered>
          <Descriptions.Item label="样本编号">{sample.sample_code}</Descriptions.Item>
          <Descriptions.Item label="患者姓名">{sample.patient_name || '-'}</Descriptions.Item>
          <Descriptions.Item label="患者身份证号">{sample.patient_id_card || '-'}</Descriptions.Item>
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

      {/* 样本状态时间线 - 横向 Steps */}
      <Card title="样本状态" style={{ marginTop: 16 }}>
        <div style={{ maxWidth: 1200, overflowX: 'auto' }}>
          <Steps
            current={(() => {
              const statusIndex: Record<string, number> = {
                'created': 0,
                'collected': 1,
                'sent': 2,
                'received': 3,
                'processing': 3,
                'tested': 4,
                'completed': 4
              };
              return statusIndex[sample.status] || 0;
            })()}
            status={sample.status === 'processing' ? 'process' : undefined}
            items={[
              {
                title: '样本创建',
                description: sample.collection_date ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{(() => {
                      const d = new Date(sample.collection_date);
                      const year = d.getFullYear();
                      const month = String(d.getMonth() + 1).padStart(2, '0');
                      const day = String(d.getDate()).padStart(2, '0');
                      const hours = String(d.getHours()).padStart(2, '0');
                      const minutes = String(d.getMinutes()).padStart(2, '0');
                      return `${year}/${month}/${day} ${hours}:${minutes}`;
                    })()}</div>
                    <div>{sample.collection_user_name || sample.collection_operator || '-'}</div>
                  </div>
                ) : '未设置',
                icon: (['collected', 'sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status)) ? (
                  <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
                ) : (
                  <ClockCircleOutlined style={{ color: '#d9d9d9', fontSize: 20 }} />
                )
              },
              {
                title: '样本采集',
                description: ['collected', 'sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>已采集</div>
                    <div>{sample.collection_date ? (() => {
                      const d = new Date(sample.collection_date);
                      return `${d.getFullYear()}/${String(d.getMonth() + 1).padStart(2, '0')}/${String(d.getDate()).padStart(2, '0')}`;
                    })() : ''}</div>
                  </div>
                ) : '待采集',
                icon: ['collected', 'sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
                ) : (
                  <ClockCircleOutlined style={{ color: '#d9d9d9', fontSize: 20 }} />
                )
              },
              {
                title: '送检中',
                description: ['sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}><div>已送检</div></div>
                ) : '待送检',
                icon: ['sent', 'received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
                ) : (
                  <ClockCircleOutlined style={{ color: '#d9d9d9', fontSize: 20 }} />
                )
              },
              {
                title: '样本接收',
                description: ['received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{sample.receive_date ? (() => {
                      const d = new Date(sample.receive_date);
                      const year = d.getFullYear();
                      const month = String(d.getMonth() + 1).padStart(2, '0');
                      const day = String(d.getDate()).padStart(2, '0');
                      const hours = String(d.getHours()).padStart(2, '0');
                      const minutes = String(d.getMinutes()).padStart(2, '0');
                      return `${year}/${month}/${day} ${hours}:${minutes}`;
                    })() : '-'}</div>
                    <div>{sample.receive_user_name || sample.receive_operator || '-'}</div>
                  </div>
                ) : '待接收',
                icon: ['received', 'processing', 'tested', 'completed'].includes(sample.status) ? (
                  sample.status === 'processing' ? (
                    <CheckCircleOutlined style={{ color: '#1890ff', fontSize: 20 }} />
                  ) : (
                    <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
                  )
                ) : (
                  <ClockCircleOutlined style={{ color: '#d9d9d9', fontSize: 20 }} />
                )
              },
              {
                title: '样本检测',
                description: ['tested', 'completed'].includes(sample.status) ? (
                  <div style={{ fontSize: 12 }}>
                    <div>{sample.test_completed_at ? (() => {
                      const d = new Date(sample.test_completed_at);
                      const year = d.getFullYear();
                      const month = String(d.getMonth() + 1).padStart(2, '0');
                      const day = String(d.getDate()).padStart(2, '0');
                      const hours = String(d.getHours()).padStart(2, '0');
                      const minutes = String(d.getMinutes()).padStart(2, '0');
                      return `${year}/${month}/${day} ${hours}:${minutes}`;
                    })() : '-'}</div>
                    <div>{sample.test_user_name || sample.test_operator || '-'}</div>
                  </div>
                ) : '待检测',
                icon: ['tested', 'completed'].includes(sample.status) ? (
                  <CheckCircleOutlined style={{ color: '#52c41a', fontSize: 20 }} />
                ) : sample.status === 'processing' ? (
                  <ClockCircleOutlined style={{ color: '#1890ff', fontSize: 20 }} />
                ) : (
                  <ClockCircleOutlined style={{ color: '#d9d9d9', fontSize: 20 }} />
                )
              }
            ]}
          />
        </div>
      </Card>

      {/* 快递运单信息 */}
      <Card 
        title="快递运单信息" 
        style={{ marginTop: 16 }}
        extra={
          <Button type="primary" onClick={handleOpenExpressModal}>
            {expressData ? '编辑快递运单' : '录入快递运单'}
          </Button>
        }
      >
        {expressData ? (
          <Descriptions column={2} bordered>
            <Descriptions.Item label="快递单号">{expressData.trackingNumber || '-'}</Descriptions.Item>
            <Descriptions.Item label="快递公司">{expressData.expressCompany || '-'}</Descriptions.Item>
            <Descriptions.Item label="寄件时间">
              {expressData.sendTime ? (() => {
                const d = new Date(expressData.sendTime);
                const year = d.getFullYear();
                const month = String(d.getMonth() + 1).padStart(2, '0');
                const day = String(d.getDate()).padStart(2, '0');
                const hours = String(d.getHours()).padStart(2, '0');
                const minutes = String(d.getMinutes()).padStart(2, '0');
                return `${year}/${month}/${day} ${hours}:${minutes}`;
              })() : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="收件时间">
              {expressData.receiveTime ? (() => {
                const d = new Date(expressData.receiveTime);
                const year = d.getFullYear();
                const month = String(d.getMonth() + 1).padStart(2, '0');
                const day = String(d.getDate()).padStart(2, '0');
                const hours = String(d.getHours()).padStart(2, '0');
                const minutes = String(d.getMinutes()).padStart(2, '0');
                return `${year}/${month}/${day} ${hours}:${minutes}`;
              })() : '-'}
            </Descriptions.Item>
            <Descriptions.Item label="状态">
              {(() => {
                const statusMap: Record<string, { color: string; text: string }> = {
                  'pending': { color: 'orange', text: '待发货' },
                  'sent': { color: 'blue', text: '已发货' },
                  'in_transit': { color: 'cyan', text: '运输中' },
                  'delivered': { color: 'green', text: '已签收' },
                  'returned': { color: 'red', text: '已退回' }
                };
                const status = statusMap[expressData.status];
                return status ? <Tag color={status.color}>{status.text}</Tag> : expressData.status || '-';
              })()}
            </Descriptions.Item>
            <Descriptions.Item label="备注" span={2}>{expressData.notes || '-'}</Descriptions.Item>
          </Descriptions>
        ) : (
          <Empty description="暂无快递运单信息" image={Empty.PRESENTED_IMAGE_SIMPLE}>
            <Button type="primary" onClick={handleOpenExpressModal}>录入快递运单</Button>
          </Empty>
        )}
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
        title={expressData ? '编辑快递运单' : '录入快递运单'}
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
            name="expressCompany"
            label="快递公司"
            rules={[{ required: true, message: '请选择快递公司' }]}
          >
            <Select placeholder="请选择快递公司">
              <Select.Option value="顺丰速运">顺丰速运</Select.Option>
              <Select.Option value="圆通快递">圆通快递</Select.Option>
              <Select.Option value="中通快递">中通快递</Select.Option>
              <Select.Option value="韵达快递">韵达快递</Select.Option>
              <Select.Option value="申通快递">申通快递</Select.Option>
              <Select.Option value="京东物流">京东物流</Select.Option>
              <Select.Option value="邮政EMS">邮政EMS</Select.Option>
              <Select.Option value="其他">其他</Select.Option>
            </Select>
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