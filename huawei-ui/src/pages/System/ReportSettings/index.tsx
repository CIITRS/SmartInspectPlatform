import React, { useEffect, useMemo, useRef, useState } from 'react';
import { App, Button, Card, Col, Input, InputNumber, Row, Select, Space, Spin, Switch, Typography } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { getReportPositions, updateReportPosition } from '@/services/api';

const PAGE_WIDTH_MM = 210;
const PAGE_HEIGHT_MM = 297;
const SCALE = 3;

type Position = {
  x: number;
  y: number;
  width: number;
  height: number;
  fontSize?: number;
};

type ReportPosition = {
  id: number;
  positionKey: string;
  positionName: string;
  sampleTypeId: number;
  reportType: string;
  pageNumber: number;
  backgroundPath: string;
  positions: Record<string, Position>;
  isActive: number;
};

const fieldLabels: Record<string, string> = {
  NameP2: '姓名',
  SexP2: '性别',
  AgeP2: '年龄',
  SampleType: '样本类型',
  SampleTime: '采样时间',
  Project: '检测项目',
  NumberID: '样本编号',
  Organization: '送检单位',
  Inspector: '检验者',
  Reviewer: '审核者',
  ReportTime: '报告时间',
  SignalInstructions: '信号值说明',
  ResultInstructions: '结果说明',
};

const labelFor = (key: string) => fieldLabels[key] || key
  .replace('Time', '检测时间')
  .replace('Signal', '信号值')
  .replace('Trend', '趋势')
  .replace('Type', '类型')
  .replace('Note', '备注');

const round = (value: number) => Math.round(value * 10) / 10;
const templateUrl = (path?: string) => `/${String(path || '').replaceAll('\\', '/').replace(/^\/+/, '')}`;

const ReportSettings: React.FC = () => {
  const { message } = App.useApp();
  const [templates, setTemplates] = useState<ReportPosition[]>([]);
  const [activeId, setActiveId] = useState<number>();
  const [selectedKey, setSelectedKey] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const dragRef = useRef<{ key: string; mode: 'move' | 'resize'; startX: number; startY: number; origin: Position } | undefined>(undefined);

  const activeTemplate = useMemo(
    () => templates.find((item) => item.id === activeId),
    [activeId, templates],
  );
  const visiblePositions = useMemo(() => {
    if (!activeTemplate) return {};
    if (activeTemplate.reportType !== 'high') return activeTemplate.positions;
    return Object.fromEntries(Object.entries(activeTemplate.positions).filter(([key]) => key !== 'Organization'));
  }, [activeTemplate]);

  const fetchTemplates = async () => {
    setLoading(true);
    try {
      const response = await getReportPositions();
      const list = response.data?.list || [];
      setTemplates(list);
      setActiveId((current) => current || list[0]?.id);
      setSelectedKey((current) => current || Object.keys(list[0]?.positions || {})[0]);
    } catch {
      message.error('获取报告设置失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchTemplates();
  }, []);

  const updateActive = (updater: (template: ReportPosition) => ReportPosition) => {
    setTemplates((items) => items.map((item) => item.id === activeId ? updater(item) : item));
  };

  const updatePosition = (key: string, patch: Partial<Position>) => {
    updateActive((template) => ({
      ...template,
      positions: {
        ...template.positions,
        [key]: { ...template.positions[key], ...patch },
      },
    }));
  };

  const startPointer = (event: React.PointerEvent, key: string, mode: 'move' | 'resize') => {
    event.preventDefault();
    event.stopPropagation();
    setSelectedKey(key);
    const position = activeTemplate?.positions[key];
    if (!position) return;
    dragRef.current = { key, mode, startX: event.clientX, startY: event.clientY, origin: { ...position } };
    event.currentTarget.setPointerCapture(event.pointerId);
  };

  const movePointer = (event: React.PointerEvent) => {
    const drag = dragRef.current;
    if (!drag) return;
    const dx = (event.clientX - drag.startX) / SCALE;
    const dy = (event.clientY - drag.startY) / SCALE;
    if (drag.mode === 'move') {
      updatePosition(drag.key, {
        x: round(Math.max(0, Math.min(PAGE_WIDTH_MM - drag.origin.width, drag.origin.x + dx))),
        y: round(Math.max(0, Math.min(PAGE_HEIGHT_MM - drag.origin.height, drag.origin.y + dy))),
      });
    } else {
      updatePosition(drag.key, {
        width: round(Math.max(3, Math.min(PAGE_WIDTH_MM - drag.origin.x, drag.origin.width + dx))),
        height: round(Math.max(3, Math.min(PAGE_HEIGHT_MM - drag.origin.y, drag.origin.height + dy))),
      });
    }
  };

  const save = async () => {
    if (!activeTemplate) return;
    setSaving(true);
    try {
      await updateReportPosition(activeTemplate.id, {
        positionName: activeTemplate.positionName,
        backgroundPath: activeTemplate.backgroundPath,
        positions: activeTemplate.positions,
        isActive: activeTemplate.isActive,
      });
      message.success('报告定位保存成功');
    } catch {
      message.error('保存报告设置失败');
    } finally {
      setSaving(false);
    }
  };

  if (loading) return <Spin />;
  if (!activeTemplate) return <Card>暂无报告定位配置</Card>;
  const selected = selectedKey ? visiblePositions[selectedKey] : undefined;

  return (
    <Row gutter={16}>
      <Col flex="660px">
        <Card
          title="第三页定位画布"
          extra={<Typography.Text type="secondary">坐标单位：毫米，PDF 将直接使用这些数值</Typography.Text>}
        >
          <div
            onPointerMove={movePointer}
            onPointerUp={() => { dragRef.current = undefined; }}
            onPointerCancel={() => { dragRef.current = undefined; }}
            style={{
              width: PAGE_WIDTH_MM * SCALE,
              height: PAGE_HEIGHT_MM * SCALE,
              position: 'relative',
              overflow: 'hidden',
              background: '#fff',
              boxShadow: '0 0 8px rgba(0,0,0,.18)',
              userSelect: 'none',
            }}
          >
            <img
              src={templateUrl(activeTemplate.backgroundPath)}
              alt={activeTemplate.positionName}
              draggable={false}
              style={{ position: 'absolute', inset: 0, width: '100%', height: '100%' }}
            />
            {Object.entries(visiblePositions).map(([key, position]) => {
              const selectedBox = key === selectedKey;
              return (
                <div
                  key={key}
                  onPointerDown={(event) => startPointer(event, key, 'move')}
                  style={{
                    position: 'absolute',
                    left: position.x * SCALE,
                    top: position.y * SCALE,
                    width: position.width * SCALE,
                    height: position.height * SCALE,
                    border: `1px solid ${selectedBox ? '#ff4d4f' : '#1677ff'}`,
                    background: selectedBox ? 'rgba(255,77,79,.18)' : 'rgba(22,119,255,.10)',
                    color: '#111',
                    fontSize: 11,
                    cursor: 'move',
                    overflow: 'hidden',
                  }}
                >
                  {labelFor(key)}
                  {selectedBox && (
                    <span
                      onPointerDown={(event) => startPointer(event, key, 'resize')}
                      style={{
                        position: 'absolute',
                        right: -1,
                        bottom: -1,
                        width: 9,
                        height: 9,
                        background: '#ff4d4f',
                        cursor: 'nwse-resize',
                      }}
                    />
                  )}
                </div>
              );
            })}
          </div>
        </Card>
      </Col>
      <Col flex="420px">
        <Card title="报告定位方案" style={{ marginBottom: 16 }}>
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Select
              value={activeId}
              style={{ width: '100%' }}
              options={templates.map((item) => ({ value: item.id, label: item.positionName }))}
              onChange={(id) => {
                setActiveId(id);
                const template = templates.find((item) => item.id === id);
                const keys = Object.keys(template?.positions || {}).filter((key) => template?.reportType !== 'high' || key !== 'Organization');
                setSelectedKey(keys[0]);
              }}
            />
            <Input
              addonBefore="名称"
              value={activeTemplate.positionName}
              onChange={(event) => updateActive((item) => ({ ...item, positionName: event.target.value }))}
            />
            <Input
              addonBefore="背景"
              value={activeTemplate.backgroundPath}
              onChange={(event) => updateActive((item) => ({ ...item, backgroundPath: event.target.value }))}
            />
            <Typography.Text type="secondary">
              样本类型ID：{activeTemplate.sampleTypeId || '全部'}，报告赋值：{activeTemplate.reportType}
            </Typography.Text>
            <Space>
              <span>启用</span>
              <Switch
                checked={activeTemplate.isActive === 1}
                onChange={(checked) => updateActive((item) => ({ ...item, isActive: checked ? 1 : 0 }))}
              />
              <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={save}>保存</Button>
            </Space>
          </Space>
        </Card>
        <Card title="字段定位">
          <Select
            showSearch
            value={selectedKey}
            style={{ width: '100%', marginBottom: 16 }}
            options={Object.keys(visiblePositions).map((key) => ({ value: key, label: `${labelFor(key)} (${key})` }))}
            onChange={setSelectedKey}
          />
          {selected && selectedKey && (
            <Row gutter={[12, 12]}>
              {(['x', 'y', 'width', 'height', 'fontSize'] as const).map((key) => (
                <Col span={key === 'fontSize' ? 24 : 12} key={key}>
                  <InputNumber
                    addonBefore={{ x: 'X', y: 'Y', width: '宽', height: '高', fontSize: '字号' }[key]}
                    value={selected[key]}
                    min={key === 'fontSize' ? 6 : 0}
                    step={0.1}
                    style={{ width: '100%' }}
                    onChange={(value) => updatePosition(selectedKey, { [key]: Number(value || 0) })}
                  />
                </Col>
              ))}
            </Row>
          )}
        </Card>
      </Col>
    </Row>
  );
};

export default ReportSettings;
