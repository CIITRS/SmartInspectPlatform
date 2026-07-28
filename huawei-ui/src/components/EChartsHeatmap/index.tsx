import React, { useEffect, useRef, useState } from 'react';
import * as echarts from 'echarts';
import { Modal, Input } from 'antd';

interface HeatmapData {
  gene: string;
  value: number;
}

interface EChartsHeatmapProps {
  data: HeatmapData[];
  onDataChange?: (data: HeatmapData[]) => void;
}

const EChartsHeatmap: React.FC<EChartsHeatmapProps> = ({ data, onDataChange }) => {
  const chartRef = useRef<HTMLDivElement>(null);
  const chartInstance = useRef<echarts.ECharts | null>(null);
  const [heatmapData, setHeatmapData] = useState<HeatmapData[]>(data);
  const [editModalVisible, setEditModalVisible] = useState(false);
  const [editingItem, setEditingItem] = useState<HeatmapData | null>(null);
  const [editValue, setEditValue] = useState('');

  // 当外部数据变化时更新内部状态
  useEffect(() => {
    setHeatmapData(data);
  }, [data]);

  // 初始化和更新图表
  useEffect(() => {
    if (chartRef.current) {
      // 初始化图表实例
      if (!chartInstance.current) {
        chartInstance.current = echarts.init(chartRef.current);
      }

      // 准备图表数据（横向布局）
      const chartData = heatmapData.map((item, index) => [
        index, // x轴为基因索引
        0, // y轴固定为0（只有一行）
        item.value
      ]);

      // 图表配置
      const option = {
        tooltip: {
          position: 'top',
          formatter: (params: any) => {
            const gene = heatmapData[params.value[0]]?.gene;
            const value = params.value[2];
            return `${gene}: ${value}`;
          }
        },
        grid: {
          height: '60%', // 调整高度
          top: '30%',
          left: '10%',
          right: '10%'
        },
        xAxis: {
          type: 'category',
          data: heatmapData.map(item => item.gene),
          splitArea: {
            show: true
          },
          axisLabel: {
            rotate: 45, // 旋转标签避免重叠
            fontSize: 12 // 增大字体大小
          }
        },
        yAxis: {
          type: 'category',
          data: [], // 移除表达值标签
          splitArea: {
            show: true
          }
        },
        visualMap: {
          min: 0,
          max: 3000,
          calculable: true,
          orient: 'horizontal',
          left: 'center',
          top: '5%', // 移到上面
          inRange: {
            color: ['#00ff00', '#99ff00', '#ffff00', '#ff9900', '#ff0000']
          },
          textStyle: {
            fontSize: 12 // 增大图例字体大小
          }
        },
        series: [
          {
            name: '基因表达值',
            type: 'heatmap',
            data: chartData,
            label: {
              show: true,
              fontSize: 20 // 增大标签字体大小
            },
            emphasis: {
              itemStyle: {
                shadowBlur: 10,
                shadowColor: 'rgba(0, 0, 0, 0.5)'
              }
            }
          }
        ]
      };

      // 设置图表配置
      chartInstance.current.setOption(option);

      // 绑定点击事件
      chartInstance.current.off('click');
      chartInstance.current.on('click', (params: any) => {
        const geneIndex = params.value[0]; // 横向布局，基因索引在x轴
        const gene = heatmapData[geneIndex];
        if (gene) {
          setEditingItem(gene);
          setEditValue(gene.value.toString());
          setEditModalVisible(true);
        }
      });

      // 响应窗口大小变化
      const handleResize = () => {
        chartInstance.current?.resize();
      };

      window.addEventListener('resize', handleResize);

      // 清理函数
      return () => {
        window.removeEventListener('resize', handleResize);
      };
    }
    return undefined;
  }, [heatmapData]);

  // 保存修改的值
  const handleSaveEdit = () => {
    if (editingItem) {
      const newValue = parseFloat(editValue);
      if (!isNaN(newValue)) {
        const updatedData = heatmapData.map((item) => 
          item.gene === editingItem.gene ? { ...item, value: newValue } : item
        );
        setHeatmapData(updatedData);
        if (onDataChange) {
          onDataChange(updatedData);
        }
      }
    }
    setEditModalVisible(false);
    setEditingItem(null);
    setEditValue('');
  };

  return (
    <div>
      <div ref={chartRef} style={{ width: '100%', height: 300 }} />
      
      {/* 编辑值的模态框 */}
      <Modal
        title="修改表达值"
        open={editModalVisible}
        onOk={handleSaveEdit}
        onCancel={() => setEditModalVisible(false)}
        okText="保存"
        cancelText="取消"
      >
        <div style={{ marginBottom: 16 }}>
          <label>基因: </label>
          <span>{editingItem?.gene}</span>
        </div>
        <div>
          <label>表达值: </label>
          <Input
            value={editValue}
            onChange={(e) => setEditValue(e.target.value)}
            type="number"
            style={{ width: 200, marginLeft: 8 }}
          />
        </div>
      </Modal>
    </div>
  );
};

export default EChartsHeatmap;