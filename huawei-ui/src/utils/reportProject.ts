export type ReportSensitivityType = 'normal' | 'high' | 'screening' | string;

export const formatReportProject = (project?: string, reportType: ReportSensitivityType = 'normal') => {
  const name = String(project || '').trim();
  if (!name || /MePlex.*CpG/i.test(name)) return name;

  const normalizedType = String(reportType || '').trim().toLowerCase();
  const isUltraSensitive = name.includes('超敏') || normalizedType === 'high';
  const sensitivity = isUltraSensitive ? '超敏' : '高敏';
  const cpgCount = isUltraSensitive ? 180 : 98;
  return `${name}(MePlex${sensitivity}${cpgCount}CpG)`;
};
