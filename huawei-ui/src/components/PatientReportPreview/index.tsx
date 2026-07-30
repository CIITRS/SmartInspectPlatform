import React, { useState } from 'react';
import { Alert, App, Button, Empty, Modal, Skeleton, Space, Tag, Typography } from 'antd';
import { RobotOutlined, SyncOutlined } from '@ant-design/icons';
import './index.less';

type PreviewKind = 'image' | 'pdf' | 'other';

const previewKind = (fileUrl: string): PreviewKind => {
  const cleanURL = String(fileUrl || '').split('?')[0].toLowerCase();
  if (/\.(jpg|jpeg|png|gif|webp|bmp)$/.test(cleanURL)) return 'image';
  if (cleanURL.endsWith('.pdf')) return 'pdf';
  return 'other';
};

const fileNameFromURL = (fileUrl: string) => {
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
}> = ({ patientCode, fileUrl, label }) => {
  const { message } = App.useApp();
  const [open, setOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [previewURL, setPreviewURL] = useState('');
  const [analysisLoading, setAnalysisLoading] = useState(false);
  const [analysis, setAnalysis] = useState<any>(null);
  const kind = previewKind(fileUrl);
  const fileName = fileNameFromURL(fileUrl);

  const analysisEndpoint = `/api/patients/${encodeURIComponent(patientCode)}/report-files/analysis`;
  const authHeaders = () => ({
    'Authorization': `Bearer ${localStorage.getItem('token')}`,
  });

  const loadAnalysis = async (force = false) => {
    setAnalysisLoading(true);
    try {
      if (!force) {
        const query = `${analysisEndpoint}?file_url=${encodeURIComponent(fileUrl)}`;
        const response = await fetch(query, { headers: authHeaders() });
        const result = await response.json();
        if (!response.ok || result.code !== 200) {
          throw new Error(result.message || '读取AI分析失败');
        }
        if (result.data?.status === 'completed') {
          setAnalysis(result.data);
          return;
        }
      }
      const response = await fetch(`${analysisEndpoint}${force ? '?force=1' : ''}`, {
        method: 'POST',
        headers: { ...authHeaders(), 'Content-Type': 'application/json' },
        body: JSON.stringify({ file_url: fileUrl }),
      });
      const result = await response.json();
      if (!response.ok || result.code !== 200) {
        throw new Error(result.message || '报告分析失败');
      }
      setAnalysis(result.data);
    } catch (error: any) {
      setAnalysis({ status: 'failed', error_message: error?.message || '报告分析失败，请稍后重试' });
    } finally {
      setAnalysisLoading(false);
    }
  };

  const showPreview = async () => {
    setOpen(true);
    setLoading(true);
    setPreviewURL('');
    setAnalysis(null);
    void loadAnalysis();
    try {
      const response = await fetch(
        `/api/patients/${encodeURIComponent(patientCode)}/report-files/preview?file_url=${encodeURIComponent(fileUrl)}`,
        { headers: authHeaders() },
      );
      const result = await response.json();
      if (!response.ok || result.code !== 200 || !result.data?.preview_url) {
        throw new Error(result.message || '报告预览失败');
      }
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
      <Button type="link" style={{ padding: 0, height: 'auto' }} onClick={showPreview}>
        {label || fileName}
      </Button>
      <Modal
        title={fileName}
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width="min(1320px, 96vw)"
        className="patient-report-modal"
        destroyOnClose
      >
        <div className="patient-report-layout">
          <section className="patient-report-original" aria-label="报告原图">
            <div className="patient-report-panel-title">报告原图</div>
            {loading && <Skeleton.Image active className="patient-report-skeleton-image" />}
            {!loading && kind === 'image' && previewURL && (
              <div className="patient-report-media">
                <img src={previewURL} alt={`${fileName} 原图`} />
              </div>
            )}
            {!loading && kind === 'pdf' && previewURL && (
              <iframe src={previewURL} title={`${fileName} 原文`} className="patient-report-pdf" />
            )}
            {!loading && kind === 'other' && previewURL && (
              <Empty description="当前浏览器不支持直接预览此文件格式">
                <Button type="primary" onClick={() => window.open(previewURL, '_blank', 'noopener,noreferrer')}>
                  打开文件
                </Button>
              </Empty>
            )}
          </section>
          <section className="patient-report-analysis" aria-label="AI分析">
            <div className="patient-report-analysis-head">
              <Space>
                <RobotOutlined />
                <span className="patient-report-panel-title">AI 报告分析</span>
                {analysis?.status === 'completed' && <Tag color="success">已完成</Tag>}
              </Space>
              <Button
                type="link"
                icon={<SyncOutlined spin={analysisLoading} />}
                disabled={analysisLoading}
                onClick={() => loadAnalysis(true)}
              >
                重新分析
              </Button>
            </div>
            {analysisLoading && (
              <div className="patient-report-analysis-loading" aria-live="polite">
                <Skeleton active paragraph={{ rows: 8 }} />
                <Typography.Text type="secondary">AI 正在识别报告类型并整理内容，请稍候…</Typography.Text>
              </div>
            )}
            {!analysisLoading && analysis?.status === 'completed' && (
              <>
                <Typography.Paragraph className="patient-report-analysis-text">
                  {analysis.content}
                </Typography.Paragraph>
                <div className="patient-report-analysis-meta">
                  {analysis.model && <span>模型：{analysis.model}</span>}
                  {analysis.analyzed_at && <span>分析时间：{analysis.analyzed_at}</span>}
                </div>
              </>
            )}
            {!analysisLoading && analysis?.status === 'failed' && (
              <Alert
                type="error"
                showIcon
                message="AI 分析未完成"
                description={analysis.error_message || '请稍后重试'}
                action={<Button onClick={() => loadAnalysis(true)}>重试</Button>}
              />
            )}
            {!analysisLoading && !analysis && <Empty description="等待AI分析" />}
            <Alert
              className="patient-report-disclaimer"
              type="info"
              showIcon
              message="AI内容仅帮助阅读原报告，不能替代医生诊断，请以原报告和医生判断为准。"
            />
          </section>
        </div>
      </Modal>
    </>
  );
};

export default PatientReportPreview;
