import React, { useEffect, useState } from 'react';
import { Alert, App, Button, Card, Divider, List, Modal, Space, Spin, Tag, Typography } from 'antd';
import { CopyrightOutlined, CodeOutlined, InfoCircleOutlined, ReloadOutlined, CloudDownloadOutlined } from '@ant-design/icons';

const { Title, Text, Paragraph } = Typography;

const About: React.FC = () => {
  const { message } = App.useApp();
  const [loading, setLoading] = useState(true);
  const [upgrading, setUpgrading] = useState(false);
  const [error, setError] = useState('');
  const [versionInfo, setVersionInfo] = useState<any>(null);

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
      setVersionInfo(result.data || null);
      if (!response.ok || result.code !== 200) {
        throw new Error(result.message || 'GitHub 版本读取失败');
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

  const startUpgrade = () => {
    if (!versionInfo?.latest_version) return;
    Modal.confirm({
      title: `升级到 ${versionInfo.latest_version}`,
      content: '升级过程会构建新版本、备份当前服务并自动重启。请确认当前没有正在进行的批次导入或报告生成任务。',
      okText: '开始升级',
      cancelText: '取消',
      onOk: async () => {
        setUpgrading(true);
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
          message.success(result.message || '升级任务已启动');
          await fetchVersion();
        } catch (requestError: any) {
          message.error(requestError?.message || '升级启动失败');
          throw requestError;
        } finally {
          setUpgrading(false);
        }
      },
    });
  };

  return (
    <div style={{ padding: '24px' }}>
      <Space style={{ width: '100%', justifyContent: 'space-between' }}>
        <Title level={2}>关于系统</Title>
        <Button icon={<ReloadOutlined />} loading={loading} onClick={fetchVersion}>检查更新</Button>
      </Space>

      <Divider><Text strong><InfoCircleOutlined /> 版本信息</Text></Divider>
      <Spin spinning={loading}>
        {error && <Alert type="warning" showIcon message={error} style={{ marginBottom: 16 }} />}
        <Card>
          <Space direction="vertical" size={12} style={{ width: '100%' }}>
            <div><Text strong>当前版本：</Text> <Tag color="blue">{versionInfo?.current_version || 'v0.1'}</Tag></div>
            <div><Text strong>发布日期：</Text> {versionInfo?.release_date || '2026-07-29'}</div>
            <div><Text strong>构建提交：</Text> <Text code>{versionInfo?.build_commit || 'development'}</Text></div>
            <div>
              <Text strong>开源仓库：</Text>{' '}
              <a href="https://github.com/CIITRS/SmartInspectPlatform" target="_blank" rel="noopener noreferrer">
                github.com/CIITRS/SmartInspectPlatform
              </a>
            </div>
            <Divider style={{ margin: '8px 0' }} />
            <div>
              <Text strong>GitHub 最新版本：</Text>{' '}
              {versionInfo?.latest_version ? <Tag color={versionInfo?.update_available ? 'orange' : 'green'}>{versionInfo.latest_version}</Tag> : '尚未发布 Release'}
            </div>
            {versionInfo?.latest_published_at && <div><Text strong>最新发布时间：</Text> {new Date(versionInfo.latest_published_at).toLocaleString()}</div>}
            {versionInfo?.latest_notes && <Paragraph style={{ whiteSpace: 'pre-wrap' }}>{versionInfo.latest_notes}</Paragraph>}
            {versionInfo?.update_available ? (
              versionInfo?.upgrade_supported ? (
                <Button type="primary" icon={<CloudDownloadOutlined />} loading={upgrading || versionInfo?.upgrade_running} onClick={startUpgrade}>
                  {versionInfo?.upgrade_running ? '正在升级' : `升级到 ${versionInfo.latest_version}`}
                </Button>
              ) : (
                <Alert type="info" showIcon message="发现新版本，但当前运行环境未启用自动升级，请按 README 的部署步骤手动升级。" />
              )
            ) : (
              <Alert type="success" showIcon message="当前已是最新版本" />
            )}
          </Space>
        </Card>
      </Spin>

      <Divider style={{ marginTop: '24px' }}><Text strong><CopyrightOutlined /> 版权信息</Text></Divider>
      <Card>
        <div style={{ marginBottom: '12px' }}>© {new Date().getFullYear()} 中创智科（上海）科技研究有限公司</div>
        <div style={{ marginBottom: '12px' }}>华微智检系统由华微智检医疗科技（哈尔滨）有限公司委托开发。</div>
        <div>本系统采用 AGPL-3.0 许可证；医疗检测结果不能替代专业诊断。</div>
      </Card>

      <Divider style={{ marginTop: '24px' }}><Text strong><CodeOutlined /> 主要开源组件</Text></Divider>
      <Card>
        <List
          dataSource={openSourceComponents}
          renderItem={(component) => (
            <List.Item actions={[<a key={component.url} href={component.url} target="_blank" rel="noopener noreferrer">访问官网</a>]}>
              <List.Item.Meta title={component.name} />
            </List.Item>
          )}
        />
      </Card>
    </div>
  );
};

export default About;
