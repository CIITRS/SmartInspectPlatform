import React, { useMemo, useRef, useState } from 'react';
import { App, Button, Card, Input, Modal, Space, Table, Tag, Typography } from 'antd';
import { ArrowLeftOutlined, DeleteOutlined, DownloadOutlined, ScanOutlined, SendOutlined } from '@ant-design/icons';
import { useNavigate } from '@umijs/max';
import * as XLSX from 'xlsx';
import { batchReceiveSamples, getSampleReceivePreview, sampleReceived } from '@/services/api';

const panelText = (record: any) => record.panel_summary || (record.panels || []).map((item: any) => item.panel_code || item.panel_name).filter(Boolean).join('，') || '-';

const SampleReceive: React.FC = () => {
  const navigate = useNavigate();
  const { message, modal } = App.useApp();
  const inputRef = useRef<any>(null);
  const [sampleCode, setSampleCode] = useState('');
  const [scannedSamples, setScannedSamples] = useState<any[]>([]);
  const [panelGroups, setPanelGroups] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);

  const addSample = async (code: string) => {
    const normalized = code.trim();
    if (!normalized) return;
    if (scannedSamples.some((item) => item.sample_code === normalized)) {
      message.warning('该样本已在列表中');
      setSampleCode('');
      return;
    }
    try {
      const response = await getSampleReceivePreview(normalized, { skipErrorHandler: true });
      const sample = response.data;
      if (!sample) {
        message.warning('样本不存在');
        return;
      }
      const panels = sample.panels || [];
      const next = {
        ...sample,
        sample_status: sample.status,
        panel_summary: sample.panel_summary || panels.map((item: any) => item.panel_code || item.panel_name).filter(Boolean).join('，'),
      };
      setScannedSamples((prev) => [...prev, next]);
      setSampleCode('');
      inputRef.current?.focus?.();
    } catch (error: any) {
      message.error(error?.response?.data?.message || error?.message || '读取样本失败');
    }
  };

  const handleSingleReceive = async () => {
    const normalized = sampleCode.trim();
    if (!normalized) {
      message.warning('请输入或扫描样本编号');
      return;
    }
    setLoading(true);
    try {
      const response = await sampleReceived({ sample_code: normalized }, { skipErrorHandler: true });
      const data = response.data;
      modal.info({
        title: '样本接收成功',
        content: (
          <div>
            <p>该样本的检测癌种为：{data?.cancer_type_name || '-'}</p>
            <p>需要测试的Panel为：{data?.panel_summary || '-'}</p>
          </div>
        ),
      });
      setSampleCode('');
    } catch (error: any) {
      message.error(error?.response?.data?.message || error?.message || '样本接收失败');
    } finally {
      setLoading(false);
    }
  };

  const handleBatchReceive = async () => {
    if (scannedSamples.length === 0) {
      message.warning('请先扫描样本');
      return;
    }
    setLoading(true);
    try {
      const response = await batchReceiveSamples({
        sample_codes: scannedSamples.map((item) => item.sample_code),
      }, { skipErrorHandler: true });
      const groups = response.data?.panel_groups || [];
      setPanelGroups(groups);
      message.success(`批量接收成功，共更新 ${response.data?.updated || 0} 个样本`);
    } catch (error: any) {
      message.error(error?.response?.data?.message || error?.message || '批量接收失败');
    } finally {
      setLoading(false);
    }
  };

  const exportPanelExcel = () => {
    if (panelGroups.length === 0) {
      message.warning('暂无Panel汇总可导出');
      return;
    }
    const rows: any[] = [];
    panelGroups.forEach((group) => {
      (group.sample_codes || []).forEach((code: string) => {
        rows.push({ Panel: group.panel, 样本编号: code });
      });
    });
    const sheet = XLSX.utils.json_to_sheet(rows);
    const workbook = XLSX.utils.book_new();
    XLSX.utils.book_append_sheet(workbook, sheet, 'Panel汇总');
    XLSX.writeFile(workbook, `样本接收Panel汇总_${Date.now()}.xlsx`);
  };

  const panelSummaryText = useMemo(() => {
    if (panelGroups.length === 0) return '';
    return panelGroups.map((group) => `${group.panel} 需要检测的样本为：${(group.sample_codes || []).join('，')}`).join('\n');
  }, [panelGroups]);

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button icon={<ArrowLeftOutlined />} onClick={() => navigate('/sample/list')}>
          返回样本中心
        </Button>
      </div>

      <Card title="批量采集">
        <Space.Compact style={{ width: '100%', marginBottom: 16 }}>
          <Input
            ref={inputRef}
            autoFocus
            value={sampleCode}
            prefix={<ScanOutlined />}
            placeholder="扫描或输入样本编号后回车"
            onChange={(event) => setSampleCode(event.target.value)}
            onPressEnter={() => addSample(sampleCode)}
          />
          <Button onClick={() => addSample(sampleCode)}>加入列表</Button>
          <Button type="primary" loading={loading} onClick={handleSingleReceive}>单个接收</Button>
        </Space.Compact>

        <Table
          rowKey="sample_code"
          size="small"
          pagination={false}
          dataSource={scannedSamples}
          columns={[
            { title: '样本编号', dataIndex: 'sample_code' },
            { title: '患者姓名', dataIndex: 'patient_name', render: (value) => value || '-' },
            { title: '检测癌种', dataIndex: 'cancer_type_name', render: (value) => value || '-' },
            { title: 'Panel', render: (_value, record) => panelText(record) },
            { title: '状态', dataIndex: 'sample_status', render: (value) => <Tag>{value || '-'}</Tag> },
            {
              title: '操作',
              width: 80,
              render: (_value, record) => (
                <Button
                  type="text"
                  danger
                  icon={<DeleteOutlined />}
                  onClick={() => setScannedSamples((prev) => prev.filter((item) => item.sample_code !== record.sample_code))}
                />
              ),
            },
          ]}
        />

        <Space style={{ marginTop: 16 }}>
          <Button type="primary" icon={<SendOutlined />} loading={loading} onClick={handleBatchReceive}>
            一键提交
          </Button>
          <Button onClick={() => setScannedSamples([])}>清空</Button>
          <Button icon={<DownloadOutlined />} onClick={exportPanelExcel}>
            导出EXCEL
          </Button>
        </Space>
      </Card>

      {panelGroups.length > 0 && (
        <Card title="Panel汇总" style={{ marginTop: 16 }}>
          <Typography.Paragraph style={{ whiteSpace: 'pre-line' }}>{panelSummaryText}</Typography.Paragraph>
          <Table
            rowKey="panel"
            size="small"
            pagination={false}
            dataSource={panelGroups}
            columns={[
              { title: 'Panel', dataIndex: 'panel' },
              { title: '需要检测的样本', dataIndex: 'sample_codes', render: (value) => (value || []).join('，') },
            ]}
          />
        </Card>
      )}
    </div>
  );
};

export default SampleReceive;
