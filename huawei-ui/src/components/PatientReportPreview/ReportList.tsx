import React, { useEffect, useMemo, useState } from 'react';
import { Empty, Skeleton, Table, Tag } from 'antd';
import PatientReportPreview, { patientReportFileName, PatientReportAnalysis } from './index';

type ReportFileItem = {
  file_url: string;
  file_name: string;
  upload_time?: string;
  status?: string;
  report_type?: string;
  hospital?: string;
  examination_time?: string;
  examination_item?: string;
};

const PatientReportList: React.FC<{ patientCode: string; reportFiles?: string }> = ({ patientCode, reportFiles }) => {
  const fallback = useMemo<ReportFileItem[]>(() => String(reportFiles || '').split(',').map((item) => item.trim()).filter(Boolean).map((file_url) => ({
    file_url,
    file_name: patientReportFileName(file_url),
    status: 'not_started',
  })), [reportFiles]);
  const [items, setItems] = useState<ReportFileItem[]>(fallback);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    setItems(fallback);
    if (!patientCode) return;
    const controller = new AbortController();
    setLoading(true);
    fetch(`/api/patients/${encodeURIComponent(patientCode)}/report-files`, {
      headers: { Authorization: `Bearer ${localStorage.getItem('token')}` },
      signal: controller.signal,
    }).then((response) => response.json().then((body) => ({ response, body })))
      .then(({ response, body }) => {
        if (response.ok && body.code === 200 && Array.isArray(body.data)) setItems(body.data);
      }).catch(() => undefined).finally(() => setLoading(false));
    return () => controller.abort();
  }, [patientCode, fallback]);

  const updateAnalysis = (fileURL: string, analysis: PatientReportAnalysis) => {
    setItems((current) => current.map((item) => item.file_url === fileURL ? {
      ...item,
      status: analysis.status,
      report_type: analysis.report_type,
      hospital: analysis.hospital,
      examination_time: analysis.examination_time,
      examination_item: analysis.examination_item,
    } : item));
  };

  if (loading && items.length === 0) return <Skeleton active paragraph={{ rows: 3 }} />;
  if (items.length === 0) return <Empty image={Empty.PRESENTED_IMAGE_SIMPLE} description="暂无报告文件" />;

  return <Table<ReportFileItem>
    size="small"
    rowKey="file_url"
    pagination={false}
    scroll={{ x: 900 }}
    dataSource={items}
    columns={[
      {
        title: '报告文件', dataIndex: 'file_name', width: 260, fixed: 'left',
        render: (_, item) => <PatientReportPreview patientCode={patientCode} fileUrl={item.file_url} label={item.file_name || patientReportFileName(item.file_url)} onAnalysisChange={(analysis) => updateAnalysis(item.file_url, analysis)} />,
      },
      { title: '上传时间', dataIndex: 'upload_time', width: 170, render: (value) => value || '-' },
      { title: '检查时间', dataIndex: 'examination_time', width: 170, render: (value) => value || '-' },
      { title: '报告类型', dataIndex: 'report_type', width: 150, render: (value, item) => value || (item.status === 'processing' ? <Tag color="processing">识别中</Tag> : '-') },
      { title: '检查项目', dataIndex: 'examination_item', ellipsis: true, render: (value) => value || '-' },
    ]}
  />;
};

export default PatientReportList;
