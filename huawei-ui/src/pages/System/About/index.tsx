import { CheckCircleOutlined, ClockCircleOutlined, CloudDownloadOutlined, CodeOutlined, CopyrightOutlined, CloseCircleOutlined, InfoCircleOutlined, LoadingOutlined, ReloadOutlined } from '@ant-design/icons';
import { Alert, App, Button, Card, Divider, List, Modal, Progress, Space, Spin, Steps, Tag, Typography } from 'antd';
import React, { useCallback, useEffect, useRef, useState } from 'react';
import './index.less';

const { Title, Text, Paragraph } = Typography;

interface VersionInfo {
  current_version: string;
  release_date: string;
  build_commit: string;
  latest_version?: string;
  latest_published_at?: string;
  latest_notes?: string;
  update_available?: boolean;
  upgrade_supported?: boolean;
  upgrade_running?: boolean;
  upgrade_status?: UpgradeStatus;
}

interface UpgradeStatus {
  version?: string;
  state: 'idle' | 'running' | 'completed' | 'failed';
  current_step: number;
  total_steps: number;
  progress: number;
  message: string;
  started_at?: string;
  updated_at?: string;
  download_name?: string;
  download_bytes?: number;
  download_total_bytes?: number;
  download_speed_bps?: number;
  download_percent?: number;
}

interface UpgradeLogEntry {
  time: string;
  title: string;
  detail?: string;
  level: 'info' | 'running' | 'success' | 'error';
}

const upgradeSteps = [
  '检查环境',
  '获取发布包',
  '备份当前版本',
  '替换系统文件',
  '重启服务',
  '健康检查',
  '升级完成',
];

const idleUpgradeStatus: UpgradeStatus = {
  state: 'idle', current_step: 0, total_steps: upgradeSteps.length, progress: 0, message: '尚未开始升级',
};

const localVersionInfo: VersionInfo = {
  current_version: __APP_VERSION__,
  release_date: __APP_RELEASE_DATE__,
  build_commit: __APP_BUILD_COMMIT__,
};

const formatBytes = (value = 0) => {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB'];
  const unitIndex = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  const amount = value / (1024 ** unitIndex);
  return `${amount >= 10 || unitIndex === 0 ? amount.toFixed(0) : amount.toFixed(1)} ${units[unitIndex]}`;
};

const parseUpgradeLog = (line: string): UpgradeLogEntry => {
  const match = line.match(/^\[([^\]]+)]\s*(.*)$/);
  const time = match?.[1] || '';
  const content = (match?.[2] || line).trim();

  if (/curl:|failed|failure|mismatch|not found|invalid|error/i.test(content)) {
    return { time, title: '执行出错', detail: content, level: 'error' };
  }
  if (/completed successfully|Check completed/i.test(content)) {
    return { time, title: '升级已完成', detail: content, level: 'success' };
  }
  if (/(: OK$|checksum.*valid|through verification)/i.test(content)) {
    return { time, title: '发布包校验通过', detail: content, level: 'success' };
  }
  if (content.startsWith('Application directory:')) {
    return { time, title: '确认应用目录', detail: content.replace('Application directory:', '').trim(), level: 'info' };
  }
  if (content.startsWith('Binary target:')) {
    return { time, title: '确认后端程序', detail: content.replace('Binary target:', '').trim(), level: 'info' };
  }
  if (content.startsWith('Service manager:')) {
    return { time, title: '确认服务管理器', detail: content.replace('Service manager:', '').trim(), level: 'info' };
  }
  if (content === 'Upgrade mode: verified GitHub Release') {
    return { time, title: '升级方式', detail: '已验证的 GitHub Release 发布包', level: 'info' };
  }
  if (content.startsWith('Reading GitHub Release metadata for')) {
    return { time, title: '读取 GitHub 发布信息', detail: content.replace('Reading GitHub Release metadata for', '').replace(/\.$/, '').trim(), level: 'running' };
  }
  if (content.startsWith('Downloading verified GitHub Release asset')) {
    return { time, title: '下载已验证的发布包', detail: content.replace('Downloading verified GitHub Release asset', '').replace(/\.$/, '').trim(), level: 'running' };
  }
  return { time, title: content || '升级任务更新', level: 'info' };
};

const About: React.FC = () => {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState(false);
  const [error, setError] = useState('');
  const [versionInfo, setVersionInfo] = useState<VersionInfo>(localVersionInfo);
  const [upgradeModalOpen, setUpgradeModalOpen] = useState(false);
  const [upgradeStatus, setUpgradeStatus] = useState<UpgradeStatus>(idleUpgradeStatus);
  const [upgradeLog, setUpgradeLog] = useState<string[]>([]);
  const [upgradeConnectionHint, setUpgradeConnectionHint] = useState('');
  const terminalNoticeRef = useRef('');

  const openSourceComponents = [
    { name: 'Go', url: 'https://go.dev/' },
    { name: 'CloudWeGo Hertz', url: 'https://www.cloudwego.io/docs/hertz/' },
    { name: 'React', url: 'https://react.dev/' },
    { name: 'Ant Design', url: 'https://ant.design/' },
    { name: 'Umi Max', url: 'https://umijs.org/' },
    { name: 'UniApp', url: 'https://uniapp.dcloud.net.cn/' },
    { name: 'MySQL', url: 'https://www.mysql.com/' },
    { name: '七牛云 Kodo', url: 'https://developer.qiniu.com/kodo' },
  ];

  const fetchVersion = async () => {
    setLoading(true);
    setError('');
    try {
      const response = await fetch('/api/system/version');
      const result = await response.json();
      if (!response.ok || result.code !== 200) {
        throw new Error(result.message || 'GitHub 版本读取失败');
      }
      setVersionInfo((current) => ({
        ...current,
        ...(result.data || {}),
        current_version: result.data?.current_version || current.current_version,
        release_date: result.data?.release_date || current.release_date,
        build_commit: result.data?.build_commit || current.build_commit,
      }));
      if (result.data?.upgrade_status) {
        setUpgradeStatus(result.data.upgrade_status);
        if (result.data.upgrade_status.state === 'running') {
          setUpgrading(true);
          setUpgradeModalOpen(true);
        }
      }
    } catch (requestError: any) {
      setError(requestError?.message || 'GitHub 版本读取失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchVersion();
  }, []);

  const fetchUpgradeStatus = useCallback(async () => {
    try {
      const response = await fetch('/api/system/version/upgrade/status', { cache: 'no-store' });
      const result = await response.json();
      if (!response.ok || result.code !== 200 || !result.data?.status) {
        throw new Error(result.message || '升级进度读取失败');
      }
      const nextStatus = result.data.status as UpgradeStatus;
      setUpgradeStatus(nextStatus);
      setUpgradeLog(Array.isArray(result.data.log_tail) ? result.data.log_tail : []);
      setUpgradeConnectionHint('');
      if (nextStatus.state === 'completed' || nextStatus.state === 'failed') {
        setUpgrading(false);
        const noticeKey = `${nextStatus.version}-${nextStatus.state}-${nextStatus.updated_at}`;
        if (terminalNoticeRef.current !== noticeKey) {
          terminalNoticeRef.current = noticeKey;
          if (nextStatus.state === 'completed') {
            message.success('系统升级完成，当前服务运行正常');
            window.setTimeout(() => fetchVersion(), 1200);
          } else {
            message.error('系统升级失败，请根据当前步骤和日志处理后重试');
          }
        }
      }
    } catch (_requestError) {
      // 服务重启期间短暂断连是正常现象，保留当前步骤并继续轮询。
      setUpgradeConnectionHint('服务正在切换，暂时无法读取进度；页面会自动继续尝试连接。');
    }
  }, [message]);

  useEffect(() => {
    if (!upgrading) return undefined;
    void fetchUpgradeStatus();
    const timer = window.setInterval(fetchUpgradeStatus, 2000);
    return () => window.clearInterval(timer);
  }, [fetchUpgradeStatus, upgrading]);

  const startUpgrade = () => {
    if (!versionInfo?.latest_version) return;
    Modal.confirm({
      title: `升级到 ${versionInfo.latest_version}`,
      content: '升级过程会构建新版本、备份当前服务并自动重启。请确认当前没有正在进行的批次导入或报告生成任务。',
      centered: true,
      okText: '开始升级',
      cancelText: '取消',
      onOk: async () => {
        setUpgradeModalOpen(true);
        setUpgradeStatus({
          state: 'running', current_step: 1, total_steps: upgradeSteps.length, progress: 5,
          version: versionInfo.latest_version, message: '正在启动升级任务',
        });
        setUpgradeLog([]);
        setUpgradeConnectionHint('');
        terminalNoticeRef.current = '';
        try {
          const response = await fetch('/api/system/version/upgrade', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ version: versionInfo.latest_version }),
          });
          const result = await response.json();
          if (!response.ok || (result.code !== 200 && result.code !== 202)) {
            throw new Error(result.message || '升级启动失败');
          }
          setUpgrading(true);
          message.success(result.message || '升级任务已启动');
          await fetchUpgradeStatus();
        } catch (requestError: any) {
          setUpgrading(false);
          setUpgradeStatus((current) => ({ ...current, state: 'failed', message: requestError?.message || '升级启动失败' }));
          message.error(requestError?.message || '升级启动失败');
          throw requestError;
        }
      },
    });
  };

  return (
    <div style={{ padding: '24px' }}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Title level={2}>关于系统</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchVersion}>
          检查更新
        </Button>
      </Space>

      <Divider>
        <Text strong>
          <InfoCircleOutlined /> 版本信息
        </Text>
      </Divider>
      <Card>
        <Space direction="vertical" size={12} style={{ width: '100%' }}>
          <div>
            <Text strong>当前版本：</Text> <Tag color="blue">{versionInfo.current_version}</Tag>
          </div>
          <div>
            <Text strong>发布日期：</Text> {versionInfo.release_date}
          </div>
          <div>
            <Text strong>构建提交：</Text> <Text code>{versionInfo.build_commit}</Text>
          </div>
          <div>
            <Text strong>开源仓库：</Text>{' '}
            <a href="https://github.com/CIITRS/SmartInspectPlatform" target="_blank" rel="noopener noreferrer">
              github.com/CIITRS/SmartInspectPlatform
            </a>
          </div>
          <Divider style={{ margin: '8px 0' }} />
          <Spin spinning={loading} tip="正在检查 GitHub Release...">
            <Space direction="vertical" size={12} style={{ width: '100%', minHeight: loading ? 72 : undefined }}>
              {error && <Alert type="warning" showIcon message={error} />}
              {!loading && !error && (
                <>
                  <div>
                    <Text strong>GitHub 最新版本：</Text>{' '}
                    {versionInfo.latest_version ? (
                      <Tag color={versionInfo.update_available ? 'orange' : 'green'}>{versionInfo.latest_version}</Tag>
                    ) : (
                      '尚未发布 Release'
                    )}
                  </div>
                  {versionInfo.latest_published_at && (
                    <div>
                      <Text strong>最新发布时间：</Text> {new Date(versionInfo.latest_published_at).toLocaleString()}
                    </div>
                  )}
                  {versionInfo.latest_notes && <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{versionInfo.latest_notes}</Paragraph>}
                  {versionInfo.update_available ? (
                    versionInfo.upgrade_supported ? (
                      <Button
                        type="primary"
                        icon={<CloudDownloadOutlined />}
                        onClick={() => (upgrading || versionInfo.upgrade_running ? setUpgradeModalOpen(true) : startUpgrade())}
                      >
                        {versionInfo.upgrade_running || upgrading ? '查看升级进度' : `升级到 ${versionInfo.latest_version}`}
                      </Button>
                    ) : (
                      <Alert type="info" showIcon message="发现新版本，但当前运行环境未启用自动升级，请按 README 的部署步骤手动升级。" />
                    )
                  ) : (
                    <Alert type="success" showIcon message="当前已是最新版本" />
                  )}
                </>
              )}
            </Space>
          </Spin>
        </Space>
      </Card>

      <Modal
        title={`系统升级${upgradeStatus.version ? ` · ${upgradeStatus.version}` : ''}`}
        open={upgradeModalOpen}
        onCancel={() => setUpgradeModalOpen(false)}
        width="min(920px, 94vw)"
        className="system-upgrade-progress-modal"
        centered
        maskClosable={!upgrading}
        footer={[
          upgradeStatus.state === 'failed' && (
            <Button key="retry" type="primary" onClick={startUpgrade}>重新尝试升级</Button>
          ),
          <Button key="close" onClick={() => setUpgradeModalOpen(false)}>
            {upgrading ? '后台继续，关闭窗口' : '关闭'}
          </Button>,
        ].filter(Boolean)}
      >
        <div aria-live="polite" aria-atomic="true">
          <Progress
            percent={upgradeStatus.progress || 0}
            status={upgradeStatus.state === 'failed' ? 'exception' : upgradeStatus.state === 'completed' ? 'success' : 'active'}
            strokeColor={upgradeStatus.state === 'completed' ? '#52c41a' : undefined}
          />
          {Boolean(upgradeStatus.download_total_bytes) && upgradeStatus.current_step === 2 && (
            <div className="system-upgrade-download" aria-label="发布包下载进度">
              <div className="system-upgrade-download-header">
                <Space size={8}>
                  {upgradeStatus.download_percent === 100 ? <CheckCircleOutlined /> : <CloudDownloadOutlined />}
                  <Text strong>{upgradeStatus.download_name || 'GitHub Release 发布包'}</Text>
                </Space>
                <Text type="secondary">
                  {formatBytes(upgradeStatus.download_bytes)} / {formatBytes(upgradeStatus.download_total_bytes)}
                </Text>
              </div>
              <Progress
                percent={upgradeStatus.download_percent || 0}
                status={upgradeStatus.download_percent === 100 ? 'success' : 'active'}
                size="small"
              />
              <Text type="secondary">
                {upgradeStatus.download_percent === 100
                  ? '下载完成，正在校验文件完整性'
                  : `下载速度 ${formatBytes(upgradeStatus.download_speed_bps)}/s`}
              </Text>
            </div>
          )}
          <Steps
            className="system-upgrade-steps"
            current={Math.max(0, (upgradeStatus.current_step || 1) - 1)}
            status={upgradeStatus.state === 'failed' ? 'error' : upgradeStatus.state === 'completed' ? 'finish' : 'process'}
            responsive
            items={upgradeSteps.map((title, index) => ({
              title,
              description: index + 1 === upgradeStatus.current_step ? upgradeStatus.message : undefined,
            }))}
          />
          {upgradeConnectionHint && <Alert type="warning" showIcon message={upgradeConnectionHint} />}
          {upgradeStatus.state === 'completed' && (
            <Alert type="success" showIcon icon={<CheckCircleOutlined />} message="升级已完成" description={upgradeStatus.message} />
          )}
          {upgradeStatus.state === 'failed' && (
            <Alert role="alert" type="error" showIcon message="升级未完成" description={`${upgradeStatus.message}。系统已尝试保留或恢复上一版本。`} />
          )}
          {upgradeStatus.state === 'running' && !upgradeConnectionHint && (
            <Alert type="info" showIcon message={upgradeStatus.message || '升级正在进行中'} description="服务重启时页面可能短暂断开，请不要重复点击升级。" />
          )}
          {upgradeLog.length > 0 && (
            <div className="system-upgrade-log" aria-label="最近升级日志">
              <Text strong>最近日志</Text>
              <div className="system-upgrade-log-list">
                {upgradeLog.slice(-10).map((line, index) => {
                  const entry = parseUpgradeLog(line);
                  const icon = entry.level === 'error'
                    ? <CloseCircleOutlined />
                    : entry.level === 'success'
                      ? <CheckCircleOutlined />
                      : entry.level === 'running'
                        ? <LoadingOutlined />
                        : <ClockCircleOutlined />;
                  return (
                    <div className={`system-upgrade-log-item is-${entry.level}`} key={`${line}-${index}`}>
                      <span className="system-upgrade-log-icon">{icon}</span>
                      <span className="system-upgrade-log-time">{entry.time || '--'}</span>
                      <span className="system-upgrade-log-content">
                        <Text strong>{entry.title}</Text>
                        {entry.detail && <Text type="secondary">{entry.detail}</Text>}
                      </span>
                    </div>
                  );
                })}
              </div>
            </div>
          )}
        </div>
      </Modal>

      <Divider style={{ marginTop: '24px' }}>
        <Text strong>
          <CopyrightOutlined /> 版权信息
        </Text>
      </Divider>
      <Card>
        <div style={{ marginBottom: '12px' }}>© {new Date().getFullYear()} 中创智科（上海）科技研究有限公司</div>
        <div style={{ marginBottom: '12px' }}>华微智检系统由华微智检医疗科技（哈尔滨）有限公司委托开发。</div>
        <div>本系统采用 AGPL-3.0 许可证；医疗检测结果不能替代专业诊断。</div>
      </Card>

      <Divider style={{ marginTop: '24px' }}>
        <Text strong>
          <CodeOutlined /> 主要开源组件
        </Text>
      </Divider>
      <Card>
        <List
          dataSource={openSourceComponents}
          renderItem={(component) => (
            <List.Item
              actions={[
                <a key={component.url} href={component.url} target="_blank" rel="noopener noreferrer">
                  访问官网
                </a>,
              ]}
            >
              <List.Item.Meta title={component.name} />
            </List.Item>
          )}
        />
      </Card>
    </div>
  );
};

export default About;
