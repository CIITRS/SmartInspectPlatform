import React, { useState } from 'react';
import { App, Button, Modal, Spin } from 'antd';

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
  const kind = previewKind(fileUrl);
  const fileName = fileNameFromURL(fileUrl);

  const showPreview = async () => {
    setOpen(true);
    setLoading(true);
    setPreviewURL('');
    try {
      const response = await fetch(
        `/api/patients/${encodeURIComponent(patientCode)}/report-files/preview?file_url=${encodeURIComponent(fileUrl)}`,
        { headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` } },
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
      <Modal title={fileName} open={open} onCancel={() => setOpen(false)} footer={null} width="min(960px, 92vw)" destroyOnClose>
        <Spin spinning={loading}>
          {!loading && kind === 'image' && previewURL && (
            <div style={{ textAlign: 'center', maxHeight: '72vh', overflow: 'auto' }}>
              <img src={previewURL} alt={fileName} style={{ display: 'block', maxWidth: '100%', height: 'auto', margin: '0 auto' }} />
            </div>
          )}
          {!loading && kind === 'pdf' && previewURL && (
            <iframe src={previewURL} title={fileName} style={{ width: '100%', height: '72vh', border: 0 }} />
          )}
          {!loading && kind === 'other' && previewURL && (
            <div style={{ textAlign: 'center', padding: 32 }}>
              当前浏览器不支持直接预览此文件格式。
              <div style={{ marginTop: 16 }}>
                <Button type="primary" onClick={() => window.open(previewURL, '_blank', 'noopener,noreferrer')}>
                  打开文件
                </Button>
              </div>
            </div>
          )}
        </Spin>
      </Modal>
    </>
  );
};

export default PatientReportPreview;
