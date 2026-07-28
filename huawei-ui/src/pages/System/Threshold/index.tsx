import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  InputNumber,
  Button,
  message,
  Space,
  Typography,
  Modal,
  Descriptions,
  Select,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { Box } from '@ant-design/plots';
import { listCancerTypes, listModels, getModelGeneThresholds, updateModelGeneThresholds, getBoxplotData } from '@/services/api';

const { Title } = Typography;

const ThresholdSetting: React.FC = () => {
  const [genes, setGenes] = useState<any[]>([]);
  const [loading, setLoading] = useState<boolean>(false);
  const [editingGenes, setEditingGenes] = useState<Record<string, number>>({});
  const [selectedGene, setSelectedGene] = useState<any>(null);
  const [modalVisible, setModalVisible] = useState<boolean>(false);
  const [modalThreshold, setModalThreshold] = useState<number>(0);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [selectedCancerType, setSelectedCancerType] = useState<string>('');
  const [boxplotData, setBoxplotData] = useState<any[]>([]);
  const [models, setModels] = useState<any[]>([]);
  const [selectedModelId, setSelectedModelId] = useState<number | undefined>();

  // 获取模型列表
  const loadModels = async () => {
    try {
      const response = await listModels({ activeOnly: 1, includeDeprecated: 0 });
      const nextModels = response.data || [];
      setModels(nextModels);
      if (nextModels.length > 0) {
        setSelectedModelId(prev => prev || nextModels[0].id);
      }
    } catch (error) {
      message.error('获取模型列表失败');
    }
  };

  // 获取当前模型的基因阈值
  const loadGenes = async (modelId = selectedModelId) => {
    if (!modelId) {
      setGenes([]);
      setEditingGenes({});
      return;
    }
    setLoading(true);
    try {
      const response = await getModelGeneThresholds(modelId);
      if (response.success) {
        setGenes(response.data || []);
        // 初始化编辑状态
        const initialEditing: Record<string, number> = {};
        (response.data || []).forEach((gene: any) => {
          initialEditing[gene.id] = gene.threshold || 0;
        });
        setEditingGenes(initialEditing);
      } else {
        message.error('获取基因列表失败');
      }
    } catch (error) {
      message.error('网络错误，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  // 获取癌种列表
  const loadCancerTypes = async () => {
    try {
      const response = await listCancerTypes();
      if (response.data) {
        setCancerTypes(response.data);
        if (response.data.length > 0) {
          setSelectedCancerType(response.data[0].id.toString());
        }
      }
    } catch (error) {
      message.error('获取癌种列表失败');
    }
  };

  // 获取箱线图数据
  const loadBoxplotData = async (geneSymbol: string, cancerTypeId?: string) => {
    try {
      const response = await getBoxplotData(geneSymbol, cancerTypeId);
      if (response.data && response.data.Data) {
        // 转换后端返回的数据格式为前端期望的格式
        const formattedData = [];
        for (const item of response.data.Data) {
          // 为每个箱线图数据生成多个点，以便前端绘制箱线图
          // 这里简化处理，实际项目中可能需要根据后端返回的数据结构进行调整
          if (item.Outliers) {
            for (const outlier of item.Outliers) {
              formattedData.push({
                cancerType: response.data.CancerType || '未知',
                treatment: item.TreatmentStage,
                gene: response.data.GeneSymbol,
                value: outlier,
                isOutlier: true,
              });
            }
          }
          // 添加四分位数点
          formattedData.push({
            cancerType: response.data.CancerType || '未知',
            treatment: item.TreatmentStage,
            gene: response.data.GeneSymbol,
            value: item.Min,
            isOutlier: false,
          });
          formattedData.push({
            cancerType: response.data.CancerType || '未知',
            treatment: item.TreatmentStage,
            gene: response.data.GeneSymbol,
            value: item.Q1,
            isOutlier: false,
          });
          formattedData.push({
            cancerType: response.data.CancerType || '未知',
            treatment: item.TreatmentStage,
            gene: response.data.GeneSymbol,
            value: item.Median,
            isOutlier: false,
          });
          formattedData.push({
            cancerType: response.data.CancerType || '未知',
            treatment: item.TreatmentStage,
            gene: response.data.GeneSymbol,
            value: item.Q3,
            isOutlier: false,
          });
          formattedData.push({
            cancerType: response.data.CancerType || '未知',
            treatment: item.TreatmentStage,
            gene: response.data.GeneSymbol,
            value: item.Max,
            isOutlier: false,
          });
        }
        setBoxplotData(formattedData);
      } else {
        // 没有数据时，设置为空数组
        setBoxplotData([]);
      }
    } catch (error) {
      console.error('获取箱线图数据失败:', error);
      message.error('获取箱线图数据失败');
      setBoxplotData([]);
    }
  };

  // 保存单个基因的阈值修改
  const saveThreshold = async (geneId: string, threshold: number) => {
    if (!selectedModelId) {
      message.warning('请先选择模型');
      return;
    }
    try {
      await updateModelGeneThresholds(selectedModelId, {
        thresholds: [{ geneId: Number(geneId), threshold }],
      });
      setGenes(prev => prev.map(gene =>
        gene.id === Number(geneId) ? { ...gene, threshold } : gene
      ));
    } catch (error) {
      message.error('保存失败，请稍后重试');
    }
  };

  // 处理阈值变化
  const handleThresholdChange = (geneId: number, value: number | null) => {
    const threshold = value || 0;
    setEditingGenes(prev => ({
      ...prev,
      [geneId]: threshold,
    }));
    // 自动保存
    saveThreshold(geneId.toString(), threshold);
  };

  // 处理基因点击
  const handleGeneClick = (gene: any) => {
    setSelectedGene(gene);
    setModalThreshold(gene.threshold || 0);
    setModalVisible(true);
    // 加载箱线图数据
    loadBoxplotData(gene.geneSymbol, selectedCancerType);
  };

  // 处理模态框阈值变化
  const handleModalThresholdChange = (value: number | null) => {
    const threshold = value || 0;
    setModalThreshold(threshold);
  };

  // 处理模态框保存
  const handleModalSave = () => {
    if (selectedGene) {
      setEditingGenes(prev => ({
        ...prev,
        [selectedGene.id]: modalThreshold,
      }));
      saveThreshold(selectedGene.id.toString(), modalThreshold);
      // 更新基因列表中的阈值
      setGenes(prev => prev.map(gene => 
        gene.id === selectedGene.id ? { ...gene, threshold: modalThreshold } : gene
      ));
      setModalVisible(false);
    }
  };

  useEffect(() => {
    loadModels();
    loadCancerTypes();
  }, []);

  useEffect(() => {
    loadGenes(selectedModelId);
  }, [selectedModelId]);

  // 处理癌种选择变化
  useEffect(() => {
    if (selectedGene) {
      loadBoxplotData(selectedGene.geneSymbol, selectedCancerType);
    }
  }, [selectedCancerType]);

  const columns = [
    {
      title: '基因符号',
      dataIndex: 'geneSymbol',
      key: 'geneSymbol',
      render: (text: string, record: any) => (
        <a onClick={() => handleGeneClick(record)}>{text}</a>
      ),
    },
    {
      title: '基因描述',
      dataIndex: 'description',
      key: 'description',
    },
    {
      title: '阈值',
      dataIndex: 'threshold',
      key: 'threshold',
      render: (text: number, record: any) => (
        <InputNumber
          min={0}
          step={0.01}
          value={editingGenes[record.id]}
          onChange={(value) => handleThresholdChange(record.id, value)}
          style={{ width: 120 }}
        />
      ),
    },
  ];

  return (
    <Card>
      <Space direction="vertical" size="large" style={{ width: '100%' }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <Title level={4}>基因阈值设置</Title>
          <Space>
            <Select
              showSearch
              placeholder="选择模型"
              style={{ width: 260 }}
              value={selectedModelId}
              onChange={setSelectedModelId}
              optionFilterProp="label"
              options={models.map(model => ({
                value: model.id,
                label: `${model.name || model.modelName}${model.version ? ` [${model.version}]` : ''}`,
              }))}
            />
            <Button
              icon={<ReloadOutlined />}
              onClick={() => loadGenes(selectedModelId)}
              loading={loading}
            >
              刷新
            </Button>
          </Space>
        </div>
        <Table
          columns={columns}
          dataSource={genes.map(gene => ({
            ...gene,
            key: gene.id,
          }))}
          pagination={false}
          loading={loading}
        />
        <div style={{ color: '#666', fontSize: '12px' }}>
          说明：阈值按模型分别保存，公式中的 count_ge_threshold(...) 会按当前模型逐个基因判断是否达到阈值。
        </div>
      </Space>

      {/* 基因详情模态框 */}
      <Modal
        title="基因详情"
        open={modalVisible}
        onOk={handleModalSave}
        onCancel={() => setModalVisible(false)}
        width={800}
      >
        {selectedGene && (
          <Space direction="vertical" size="large" style={{ width: '100%' }}>
            <Descriptions bordered column={2}>
              <Descriptions.Item label="基因符号">{selectedGene.geneSymbol}</Descriptions.Item>
              <Descriptions.Item label="基因描述">{selectedGene.description}</Descriptions.Item>
              <Descriptions.Item label="当前模型">
                {models.find(model => model.id === selectedModelId)?.name || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="阈值">
                <InputNumber
                  min={0}
                  step={0.01}
                  value={modalThreshold}
                  onChange={handleModalThresholdChange}
                  style={{ width: 120 }}
                />
              </Descriptions.Item>
              <Descriptions.Item label="癌种">
                <Select
                  style={{ width: 120 }}
                  value={selectedCancerType}
                  onChange={setSelectedCancerType}
                  options={cancerTypes.map(type => ({
                    value: type.id.toString(),
                    label: type.name,
                  }))}
                />
              </Descriptions.Item>
            </Descriptions>
            <div>
              <h4>历史测试数据箱线图</h4>
              {boxplotData.length > 0 ? (
                <div style={{ height: 400 }}>
                  <Box
                    data={boxplotData.filter(item => item.gene === selectedGene.geneSymbol)}
                    boxType="boxplot"
                    xField="treatment"
                    yField="value"
                    colorField="treatment"
                    seriesField="treatment"
                    outliers={{
                      visible: true,
                    }}
                    tooltip={{
                      formatter: (datum: any) => {
                        return {
                          name: datum.treatment,
                          value: datum.value,
                          outliers: datum.isOutlier ? '异常点' : '正常点',
                        };
                      },
                    }}
                  />
                </div>
              ) : (
                <div style={{ height: 200, display: 'flex', alignItems: 'center', justifyContent: 'center', backgroundColor: '#f5f5f5', borderRadius: 4 }}>
                  无历史信息
                </div>
              )}
            </div>
          </Space>
        )}
      </Modal>
    </Card>
  );
};

export default ThresholdSetting;
