import React, { useState } from 'react';
import { Alert, App, Button, Descriptions, Empty, Form, Input, Modal, Skeleton, Space, Tag, Typography } from 'antd';
import {
  CompressOutlined,
  EditOutlined,
  RobotOutlined,
  SaveOutlined,
  SyncOutlined,
  ZoomInOutlined,
  ZoomOutOutlined,
} from '@ant-design/icons';
import './index.less';

export type PatientReportAnalysis = {
  status?: string;
  content?: string;
  model?: string;
  file_name?: string;
  file_type?: string;
  report_type?: string;
  hospital?: string;
  examination_time?: string;
  examination_item?: string;
  error_message?: string;
  analyzed_at?: string;
  edited_at?: string;
};

type PreviewKind = 'image' | 'pdf' | 'other';

const previewKind = (fileUrl: string): PreviewKind => {
  const cleanURL = String(fileUrl || '').split('?')[0].toLowerCase();
  if (/\.(jpg|jpeg|png|gif|webp|bmp)$/.test(cleanURL)) return 'image';
  if (cleanURL.endsWith('.pdf')) return 'pdf';
  return 'other';
};

export const patientReportFileName = (fileUrl: string) => {
  const encodedName = String(fileUrl || '').split('?')[0].split('/').pop();
  if (!encodedName) return '查看报告';
  try {
    return decodeURIComponent(encodedName);
  } catch (_error) {
    return encodedName;
  }
};

const PatientReportPreview: React.FC<{
  patientCode: string;
  fileUrl: string;
  label?: React.ReactNode;
  onAnalysisChange?: (analysis: PatientReportAnalysis) => void;
}> = ({ patientCode, fileUrl, label, onAnalysisChange }) => {
  const { message } = App.useApp();
  const [form] = Form.useForm();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [previewURL, setPreviewURL] = useState('');
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [editing, setEditing] = useState(false);
  const [analysis, setAnalysis] = useState<PatientReportAnalysis | null>(null);
  const [imageScale, setImageScale] = useState(1);
  const kind = previewKind(fileUrl);
  const fileName = patientReportFileName(fileUrl);

  const analysisEndpoint = `/api/patients/${encodeURIComponent(patientCode)}/report-files/analysis`;
  const authHeaders = () => ({ Authorization: `Bearer ${localStorage.getItem('token')}` });

  const applyAnalysis = (next: PatientReportAnalysis) => {
    setAnalysis(next);
    onAnalysisChange?.(next);
  };

  const loadAnalysis = async (force = false) => {
    setAnalysisLoading(true);
    setEditing(false);
    try {
      if (!force) {
        const response = await fetch(`${analysisEndpoint}?file_url=${encodeURIComponent(fileUrl)}`, { headers: authHeaders() });
        const result = await response.json();
        if (!response.ok || result.code !== 200) throw new Error(result.message || '读取AI分析失败');
        if (result.data?.status === 'completed') {
          applyAnalysis(result.data);
          return;
        }
      }
      const response = await fetch(`${analysisEndpoint}${force ? '?force=1' : ''}`, {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ file_url: fileUrl }),
      });
      const result = await response.json();
      if (!response.ok || result.code !== 200) throw new Error(result.message || '报告分析失败');
      applyAnalysis(result.data);
    } catch (error: any) {
      setAnalysis({ status: 'failed', error_message: error?.message || '报告分析失败，请稍后重试' });
    } finally {
      setAnalysisLoading(false);
    }
  };

  const startEditing = () => {
    form.setFieldsValue({
      report_type: analysis?.report_type,
      hospital: analysis?.hospital,
      examination_time: analysis?.examination_time,
      examination_item: analysis?.examination_item,
      content: analysis?.content,
    });
    setEditing(true);
  };

  const saveAnalysis = async () => {
    const values = await form.validateFields();
    setSaving(true);
    try {
      const response = await fetch(`${analysisEndpoint}?file_url=${encodeURIComponent(fileUrl)}`, {
        method: 'PUT',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ file_url: fileUrl, ...values }),
      });
      const result = await response.json();
      if (!response.ok || result.code !== 200) throw new Error(result.message || '保存失败');
      applyAnalysis(result.data);
      setEditing(false);
      message.success('报告信息已保存');
    } catch (error: any) {
      message.error(error?.message || '保存失败');
    } finally {
      setSaving(false);
    }
  };

  const showPreview = async () => {
    setOpen(true);
    setLoading(true);
    setPreviewURL('');
    setAnalysis(null);
    setEditing(false);
    setImageScale(1);
    void loadAnalysis();
    try {
      const response = await fetch(
        `/api/patients/${encodeURIComponent(patientCode)}/report-files/preview?file_url=${encodeURIComponent(fileUrl)}`,
        { headers: authHeaders() },
      );
      const result = await response.json();
      if (!response.ok || result.code !== 200 || !result.data?.preview_url) throw new Error(result.message || '报告预览失败');
      setPreviewURL(result.data.preview_url);
    } catch (error: any) {
      setOpen(false);
      message.error(error?.message || '报告预览失败');
    } finally {
      setLoading(false);
    }
  };

  return (
    <>
      <Button type="link" className="patient-report-trigger" onClick={showPreview}>
        {label || fileName}
      </Button>
      <Modal
        title={fileName}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width="min(1500px, 97vw)"
        className="patient-report-modal"
        destroyOnClose
      >
        <div className="patient-report-layout">
          <section className="patient-report-original" aria-label="报告原件预览">
            <div className="patient-report-original-head">
              <span className="patient-report-panel-title">报告原件</span>
              {kind === 'image' && previewURL && (
                <Space.Compact aria-label="图片缩放控制">
                  <Button aria-label="缩小图片" icon={<ZoomOutOutlined />} disabled={imageScale <= 0.25} onClick={() => setImageScale((value) => Math.max(0.25, value - 0.25))} />
                  <Button aria-label="图片实际大小" onClick={() => setImageScale(1)}>{Math.round(imageScale * 100)}%</Button>
                  <Button aria-label="放大图片" icon={<ZoomInOutlined />} disabled={imageScale >= 3} onClick={() => setImageScale((value) => Math.min(3, value + 0.25))} />
                  <Button aria-label="适应窗口" icon={<CompressOutlined />} onClick={() => setImageScale(1)} />
                </Space.Compact>
              )}
            </div>
            {loading && <Skeleton.Image active className="patient-report-skeleton-image" />}
            {!loading && kind === 'image' && previewURL && (
              <div className="patient-report-media">
                <img
                  src={previewURL}
                  alt={`${fileName} 原图`}
                  style={{ width: imageScale === 1 ? 'auto' : `${imageScale * 100}%`, maxWidth: imageScale === 1 ? '100%' : 'none' }}
                />
              </div>
            )}
            {!loading && kind === 'pdf' && previewURL && (
              <iframe src={previewURL} title={`${fileName} PDF原文`} className="patient-report-pdf" />
            )}
            {!loading && kind === 'other' && previewURL && (
              <Empty description="当前浏览器不支持直接预览此文件格式">
                <Button type="primary" onClick={() => window.open(previewURL, '_blank', 'noopener,noreferrer')}>打开文件</Button>
              </Empty>
            )}
          </section>
          <section className="patient-report-analysis" aria-label="AI分析内容">
            <div className="patient-report-analysis-head">
              <Space>
                <RobotOutlined />
                <span className="patient-report-panel-title">AI 报告分析</span>
                {analysis?.status === 'completed' && <Tag color="success">已完成</Tag>}
              </Space>
              <Space>
                {analysis?.status === 'completed' && !editing && <Button icon={<EditOutlined />} onClick={startEditing}>编辑</Button>}
                <Button icon={<SyncOutlined spin={analysisLoading} />} disabled={analysisLoading || saving} onClick={() => loadAnalysis(true)}>重新分析</Button>
              </Space>
            </div>
            {analysisLoading && <div className="patient-report-analysis-loading" aria-live="polite"><Skeleton active paragraph={{ rows: 8 }} /><Typography.Text type="secondary">AI 正在识别并整理报告，请稍候…</Typography.Text></div>}
            {!analysisLoading && analysis?.status === 'completed' && editing && (
              <Form form={form} layout="vertical" className="patient-report-edit-form">
                <div className="patient-report-field-grid">
                  <Form.Item name="report_type" label="报告类型"><Input maxLength={50} /></Form.Item>
                  <Form.Item name="hospital" label="医院"><Input maxLength={255} /></Form.Item>
                  <Form.Item name="examination_time" label="检查时间"><Input maxLength={100} /></Form.Item>
                  <Form.Item name="examination_item" label="检查项目"><Input maxLength={255} /></Form.Item>
                </div>
                <Form.Item name="content" label="内容摘要"><Input.TextArea autoSize={{ minRows: 10, maxRows: 24 }} maxLength={50000} showCount /></Form.Item>
                <Space><Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={saveAnalysis}>保存</Button><Button disabled={saving} onClick={() => setEditing(false)}>取消</Button></Space>
              </Form>
            )}
            {!analysisLoading && analysis?.status === 'completed' && !editing && (
              <div className="patient-report-analysis-result">
                <Descriptions bordered size="small" column={1} className="patient-report-fields">
                  <Descriptions.Item label="报告类型">{analysis.report_type || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="医院">{analysis.hospital || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="检查时间">{analysis.examination_time || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="检查项目">{analysis.examination_item || '未识别'}</Descriptions.Item>
                </Descriptions>
                <Typography.Title level={5}>内容摘要</Typography.Title>
                <Typography.Paragraph className="patient-report-analysis-text">{analysis.content || '暂无摘要'}</Typography.Paragraph>
                <div className="patient-report-analysis-meta">{analysis.model && <span>模型：{analysis.model}</span>}{analysis.analyzed_at && <span>分析时间：{analysis.analyzed_at}</span>}{analysis.edited_at && <span>编辑时间：{analysis.edited_at}</span>}</div>
              </div>
            )}
            {!analysisLoading && analysis?.status === 'failed' && <Alert type="error" showIcon message="AI 分析未完成" description={analysis.error_message || '请稍后重试'} action={<Button onClick={() => loadAnalysis(true)}>重试</Button>} />}
            {!analysisLoading && !analysis && <Empty description="等待AI分析" />}
          </section>
        </div>
      </Modal>
    </>
  );
};

export default PatientReportPreview;
