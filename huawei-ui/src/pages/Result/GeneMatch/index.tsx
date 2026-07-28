import React, { useState, useEffect } from 'react';
import { Table, Button, Form, Input, Row, Col, message, Card, Select, Modal, Tag, Alert } from 'antd';
import { ArrowLeftOutlined, SearchOutlined, EditOutlined } from '@ant-design/icons';
import { matchGenes, getModels, applyModel } from '@/services/api';
import { useNavigate, useParams } from '@umijs/max';

const { Option } = Select;

const GeneMatch: React.FC = () => {
  const { batchId } = useParams<{ batchId: string }>();
  const [form] = Form.useForm();
  const [matches, setMatches] = useState<any[]>([]);
  const [sampleMatches, setSampleMatches] = useState<any[]>([]);
  const [batchGenes, setBatchGenes] = useState<string[]>([]);
  const [allCancerTypes, setAllCancerTypes] = useState<any[]>([]);
  const [models, setModels] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [matching, setMatching] = useState(false);
  const [applying, setApplying] = useState(false);
  const [selectedModel, setSelectedModel] = useState<string>('');
  const [modelModalVisible, setModelModalVisible] = useState(false);
  const [selectedCancerType, setSelectedCancerType] = useState<string>('');
  const [cancerTypeModalVisible, setCancerTypeModalVisible] = useState(false);
  const [selectedSampleCode, setSelectedSampleCode] = useState<string>('');
  const navigate = useNavigate();

  useEffect(() => {
    fetchModels();
  }, []);

  const fetchModels = async () => {
    try {
      const response = await getModels();
      setModels(response.data.list);
    } catch (error: any) {
      message.error(error.message || '获取模型列表失败');
    }
  };

  const handleMatch = async (values: any) => {
    setMatching(true);
    try {
      const response = await matchGenes({
        batchId: batchId || values.batchId,
        ...values
      });
      
      // 新的按样本匹配结果
      if (response.data.sampleMatches) {
        setSampleMatches(response.data.sampleMatches);
      }
      if (response.data.batchGenes) {
        setBatchGenes(response.data.batchGenes);
      }
      if (response.data.allCancerTypes) {
        setAllCancerTypes(response.data.allCancerTypes);
      }
      
      // 兼容旧的模型匹配结果
      if (response.data.matches) {
        setMatches(response.data.matches);
      }
      
      message.success('基因匹配成功');
    } catch (error: any) {
      message.error(error.message || '基因匹配失败');
    } finally {
      setMatching(false);
    }
  };

  const handleApplyModel = async () => {
    if (!selectedModel) {
      message.error('请选择模型');
      return;
    }
    
    setApplying(true);
    try {
      const response = await applyModel({
        batchId: batchId || form.getFieldValue('batchId'),
        modelId: selectedModel
      });
      message.success('模型应用成功');
      setModelModalVisible(false);
    } catch (error: any) {
      message.error(error.message || '模型应用失败');
    } finally {
      setApplying(false);
    }
  };

  const handleSelectCancerType = (sampleCode: string) => {
    setSelectedSampleCode(sampleCode);
    setCancerTypeModalVisible(true);
  };

  const handleConfirmCancerType = () => {
    if (!selectedCancerType) {
      message.error('请选择检测类型');
      return;
    }
    message.success(`已为样本 ${selectedSampleCode} 选择检测类型`);
    setCancerTypeModalVisible(false);
    setSelectedCancerType('');
  };

  const getMatchStatusText = (status: string) => {
    switch (status) {
      case 'exact': return '完全匹配';
      case 'subset': return '子集匹配';
      default: return '匹配不足';
    }
  };

  const getMatchStatusColor = (status: string) => {
    switch (status) {
      case 'exact': return 'green';
      case 'subset': return 'orange';
      default: return 'red';
    }
  };

  const modelColumns = [
    {
      title: '模型名称',
      dataIndex: 'modelName',
      key: 'modelName'
    },
    {
      title: '癌种',
      dataIndex: 'cancerType',
      key: 'cancerType'
    },
    {
      title: '匹配基因数量',
      dataIndex: 'matchedGeneCount',
      key: 'matchedGeneCount'
    },
    {
      title: '匹配率',
      dataIndex: 'matchRate',
      key: 'matchRate',
      render: (rate: number) => `${(rate * 100).toFixed(2)}%`
    },
    {
      title: '操作',
      key: 'action',
      render: (_text: any, record: any) => (
        <Button
          type="link"
          onClick={() => {
            setSelectedModel(record.modelId);
            setModelModalVisible(true);
          }}
        >
          选择应用
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
        <h2>基因匹配</h2>
      </div>

      <Card style={{ marginBottom: 16 }}>
        <Form form={form} layout="inline" onFinish={handleMatch}>
          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="batchId" label="批次编号" initialValue={batchId}>
                <Input placeholder="请输入批次编号" prefix={<SearchOutlined />} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item>
                <Button type="primary" htmlType="submit" loading={matching}>
                  开始匹配
                </Button>
              </Form.Item>
            </Col>
          </Row>
        </Form>
      </Card>

      {/* 按样本匹配Panel结果 */}
      {sampleMatches.length > 0 && (
        <Card title="样本Panel匹配结果" style={{ marginBottom: 16 }}>
          <Alert
            message="按样本逐个匹配检测类型中的Panel"
            description="不同样本可能有不同的检测类型，系统会根据每个样本的检测类型自动匹配对应的Panel"
            type="info"
            showIcon
            style={{ marginBottom: 16 }}
          />
          
          {sampleMatches.map((sampleMatch, index) => (
            <Card
              key={index}
              size="small"
              style={{ marginBottom: 16, borderLeft: `4px solid ${sampleMatch.hasExactMatch ? '#52c41a' : sampleMatch.hasMatchingPanel ? '#faad14' : '#ff4d4f'}` }}
            >
              <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
                <div>
                  <strong>样本编号:</strong> {sampleMatch.sampleCode}
                  {sampleMatch.cancerTypeName && (
                    <span style={{ marginLeft: 12 }}>
                      <Tag color="blue">检测类型: {sampleMatch.cancerTypeName}</Tag>
                    </span>
                  )}
                  {!sampleMatch.cancerTypeName && (
                    <span style={{ marginLeft: 12 }}>
                      <Tag color="red">未关联检测类型</Tag>
                    </span>
                  )}
                </div>
                <Button
                  type="primary"
                  size="small"
                  icon={<EditOutlined />}
                  onClick={() => handleSelectCancerType(sampleMatch.sampleCode)}
                >
                  选择检测类型
                </Button>
              </div>

              {/* 样本基因列表 */}
              <div style={{ marginBottom: 12 }}>
                <p style={{ marginBottom: 8 }}><strong>样本基因 ({sampleMatch.sampleGenes?.length || 0}个):</strong></p>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: '8px' }}>
                  {sampleMatch.sampleGenes?.map((gene: string, idx: number) => (
                    <Tag key={idx}>{gene}</Tag>
                  ))}
                </div>
              </div>

              {/* Panel匹配结果 */}
              {sampleMatch.panelMatches && sampleMatch.panelMatches.length > 0 ? (
                sampleMatch.panelMatches.map((panelMatch: any, pIndex: number) => (
                  <div key={pIndex} style={{ marginBottom: 12, padding: '8px', backgroundColor: '#fafafa', borderRadius: '4px' }}>
                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 8 }}>
                      <span><strong>Panel:</strong> {panelMatch.panelName}</span>
                      <Tag color={getMatchStatusColor(panelMatch.matchStatus)}>
                        {getMatchStatusText(panelMatch.matchStatus)}
                      </Tag>
                    </div>
                    <div style={{ display: 'flex', gap: '24px', fontSize: '12px' }}>
                      <span>匹配数: {panelMatch.matchCount}/{panelMatch.totalGenes}</span>
                      <span>匹配率: {(panelMatch.matchRate * 100).toFixed(1)}%</span>
                    </div>
                    
                    {/* 缺失基因 */}
                    {panelMatch.missingGenes && panelMatch.missingGenes.length > 0 && (
                      <div style={{ marginTop: 8 }}>
                        <p style={{ marginBottom: 4, color: '#ff4d4f', fontSize: '12px' }}><strong>缺失基因:</strong></p>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                          {panelMatch.missingGenes.map((gene: string, gIdx: number) => (
                            <Tag key={gIdx} color="red">{gene}</Tag>
                          ))}
                        </div>
                      </div>
                    )}
                    
                    {/* 额外基因 */}
                    {panelMatch.extraGenes && panelMatch.extraGenes.length > 0 && (
                      <div style={{ marginTop: 8 }}>
                        <p style={{ marginBottom: 4, color: '#faad14', fontSize: '12px' }}><strong>额外基因:</strong></p>
                        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                          {panelMatch.extraGenes.map((gene: string, gIdx: number) => (
                            <Tag key={gIdx} color="orange">{gene}</Tag>
                          ))}
                        </div>
                      </div>
                    )}
                  </div>
                ))
              ) : (
                <Alert
                  message="未找到匹配的Panel"
                  description="该样本的检测类型未关联任何Panel，或检测类型未设置"
                  type="warning"
                  showIcon
                />
              )}
            </Card>
          ))}
        </Card>
      )}

      {/* 模型匹配结果（保留原有功能） */}
      {matches.length > 0 && (
        <Card>
          <div style={{ marginBottom: 16 }}>
            <h3>模型匹配结果</h3>
            <p>以下为模型基因与导入基因的匹配结果</p>
          </div>
          <Table
            columns={modelColumns}
            dataSource={matches}
            rowKey="modelId"
            pagination={{
              showSizeChanger: true,
              showQuickJumper: true,
              showTotal: (total) => `共 ${total} 条记录`
            }}
          />
        </Card>
      )}

      {/* 模型应用确认模态框 */}
      <Modal
        title="应用模型"
        open={modelModalVisible}
        onCancel={() => setModelModalVisible(false)}
        onOk={handleApplyModel}
        confirmLoading={applying}
      >
        <p>确定要将选中的模型应用到该批次吗？</p>
        <p>应用后，系统将使用该模型进行后续分析。</p>
      </Modal>

      {/* 检测类型选择模态框 */}
      <Modal
        title={`为样本 ${selectedSampleCode} 选择检测类型`}
        open={cancerTypeModalVisible}
        onCancel={() => {
          setCancerTypeModalVisible(false);
          setSelectedCancerType('');
        }}
        onOk={handleConfirmCancerType}
        confirmLoading={applying}
      >
        <p>请选择一个检测类型（支持选择基因少于检测类型的）:</p>
        <Form>
          <Form.Item label="检测类型">
            <Select
              value={selectedCancerType}
              onChange={(value) => setSelectedCancerType(value)}
              style={{ width: '100%' }}
              placeholder="请选择检测类型"
            >
              {allCancerTypes.map((ct) => (
                <Option key={ct.id} value={ct.id.toString()}>
                  {ct.name}
                </Option>
              ))}
            </Select>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default GeneMatch;
