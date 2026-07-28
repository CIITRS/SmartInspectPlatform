import React, { useState, useEffect } from 'react';
import { Table, Button, Input, Form, Row, Col, Card, Tabs, App, InputNumber, Select, Switch, Statistic, Progress, Tag, Space, Alert, Typography } from 'antd';
import { SaveOutlined, ReloadOutlined, CloudServerOutlined, MessageOutlined, FileTextOutlined, FolderOpenOutlined, FileOutlined } from '@ant-design/icons';

const { Option } = Select;
const { TextArea } = Input;
const { Text } = Typography;

const formatBytes = (value: number) => {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const unitIndex = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${(value / (1024 ** unitIndex)).toFixed(unitIndex === 0 ? 0 : 2)} ${units[unitIndex]}`;
};

const SMS_SWITCH_KEYS = new Set([
  'SMS_ADMIN_LOGIN_ENABLED', 'SMS_MINIAPP_LOGIN_ENABLED', 'SMS_ADMIN_BIND_PHONE_ENABLED',
  'SMS_MINIAPP_BIND_PHONE_ENABLED', 'SMS_INVITE_REGISTER_ENABLED', 'SMS_REPORT_READY_ENABLED',
]);
const SETTINGS_SWITCH_KEYS = new Set([...SMS_SWITCH_KEYS, 'QINIU_ENABLED']);

const smsSwitchItems = [
  ['SMS_ADMIN_LOGIN_ENABLED', '管理后台登录', '后台员工登录验证码'],
  ['SMS_MINIAPP_LOGIN_ENABLED', '小程序登录', '患者及员工小程序登录验证码'],
  ['SMS_ADMIN_BIND_PHONE_ENABLED', '后台绑定手机', '后台账号绑定或更换手机号'],
  ['SMS_MINIAPP_BIND_PHONE_ENABLED', '小程序绑定手机', '小程序账号绑定手机号'],
  ['SMS_INVITE_REGISTER_ENABLED', '邀请注册', '受邀人员注册验证码'],
  ['SMS_REPORT_READY_ENABLED', '报告出具通知', '报告完成后的患者通知'],
];

const Settings: React.FC = () => {
  const [settings, setSettings] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const [smsPackages, setSmsPackages] = useState<any[]>([]);
  const [packagesLoading, setPackagesLoading] = useState(false);
  const [packagesError, setPackagesError] = useState('');
  const [smsTemplates, setSmsTemplates] = useState<any[]>([]);
  const [templatesLoading, setTemplatesLoading] = useState(false);
  const [templatesError, setTemplatesError] = useState('');
  const [smsLogs, setSmsLogs] = useState<any[]>([]);
  const [logsLoading, setLogsLoading] = useState(false);
  const [logTotal, setLogTotal] = useState(0);
  const [logPage, setLogPage] = useState(1);
  const [logFilters, setLogFilters] = useState({ purpose: '', status: '', mobile: '', start_date: '', end_date: '' });
  const [storageOverview, setStorageOverview] = useState<any>(null);
  const [storageLoading, setStorageLoading] = useState(false);
  const [storageError, setStorageError] = useState('');
  const [storagePrefix, setStoragePrefix] = useState('');
  const [form] = Form.useForm();
  const { message } = App.useApp();

  // 加载系统配置
  const fetchSettings = async () => {
    setLoading(true);
    try {
      const response = await fetch('/api/system/settings', {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        setSettings(result.data || []);
        // 构建表单数据
        const formData: any = {};
        result.data.forEach((setting: any) => {
          formData[setting.key_name] = SETTINGS_SWITCH_KEYS.has(setting.key_name)
            ? !['0', 'false', 'off', 'disabled'].includes(String(setting.key_value).toLowerCase())
            : setting.key_value;
        });
        form.setFieldsValue(formData);
      } else {
        message.error('获取系统配置失败');
      }
    } catch (error) {
      message.error('获取系统配置失败');
    } finally {
      setLoading(false);
    }
  };

  // 保存系统配置
  const handleSave = async (values: any) => {
    setSaving(true);
    try {
      // 构建请求数据
      const settingsData = Object.entries(values).map(([key, value]) => {
        const setting = settings.find(s => s.key_name === key);
        return {
          key_name: key,
          key_value: SETTINGS_SWITCH_KEYS.has(key) ? (value ? '1' : '0') : String(value ?? ''),
          is_encrypted: setting ? setting.is_encrypted : 0,
        };
      });

      const response = await fetch('/api/system/settings', {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ settings: settingsData }),
      });
      const result = await response.json();
      if (result.code === 200) {
        message.success('保存系统配置成功');
        await fetchSettings();
        await fetchStorageOverview(storagePrefix);
        fetchSmsPackages();
        fetchSmsTemplates();
      } else {
        message.error('保存系统配置失败');
      }
    } catch (error) {
      message.error('保存系统配置失败');
    } finally {
      setSaving(false);
    }
  };

  const fetchStorageOverview = async (prefix = '') => {
    setStorageLoading(true);
    setStorageError('');
    try {
      const params = new URLSearchParams({ prefix, limit: '200' });
      const response = await fetch(`/api/system/storage/qiniu/overview?${params.toString()}`);
      const result = await response.json();
      if (!response.ok || result.code !== 200) {
        throw new Error(result.message || '七牛云信息读取失败');
      }
      setStorageOverview(result.data || null);
      setStoragePrefix(prefix);
    } catch (error: any) {
      setStorageOverview(null);
      setStorageError(error?.message || '七牛云信息读取失败');
    } finally {
      setStorageLoading(false);
    }
  };

  const fetchSmsPackages = async () => {
    setPackagesLoading(true);
    setPackagesError('');
    try {
      const response = await fetch('/api/system/sms/packages?page=1&page_size=100');
      const result = await response.json();
      if (result.code === 200) setSmsPackages(result.data?.list || []);
      else setPackagesError(result.message || '量包查询失败');
    } catch {
      setPackagesError('量包查询失败，请检查网络和百度短信配置');
    } finally {
      setPackagesLoading(false);
    }
  };

  const fetchSmsTemplates = async () => {
    setTemplatesLoading(true);
    setTemplatesError('');
    try {
      const response = await fetch('/api/system/sms/templates');
      const result = await response.json();
      if (result.code === 200) setSmsTemplates(result.data?.list || []);
      else setTemplatesError(result.message || '模板查询失败');
    } catch {
      setTemplatesError('模板查询失败，请检查网络和百度短信配置');
    } finally {
      setTemplatesLoading(false);
    }
  };

  const fetchSmsLogs = async (page = 1, filters = logFilters) => {
    setLogsLoading(true);
    try {
      const params = new URLSearchParams({ page: String(page), page_size: '10' });
      Object.entries(filters).forEach(([key, value]) => value && params.set(key, value));
      const response = await fetch(`/api/system/sms/logs?${params.toString()}`);
      const result = await response.json();
      if (result.code === 200) {
        setSmsLogs(result.data?.list || []);
        setLogTotal(result.data?.total || 0);
        setLogPage(page);
      }
    } finally {
      setLogsLoading(false);
    }
  };

  // 初始化加载配置
  useEffect(() => {
    fetchSettings();
    fetchSmsPackages();
    fetchSmsTemplates();
    fetchSmsLogs();
    fetchStorageOverview();
  }, []);

  // 系统基础设置表单
  const BasicSettingsForm = () => (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Card title="数据库配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="DB_HOST" label="数据库主机">
              <Input placeholder="请输入数据库主机地址" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="DB_PORT" label="数据库端口">
              <Input placeholder="请输入数据库端口" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="DB_NAME" label="数据库名称">
              <Input placeholder="请输入数据库名称" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="DB_USER" label="数据库用户名">
              <Input placeholder="请输入数据库用户名" />
            </Form.Item>
          </Col>
          <Col span={8}>
            <Form.Item name="DB_PASSWORD" label="数据库密码">
              <Input.Password placeholder="请输入数据库密码" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Card title="服务器配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={8}>
            <Form.Item name="SERVER_PORT" label="服务器端口">
              <Input placeholder="请输入服务器端口" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
  );

  // 短信通知配置表单
  const SmsSettingsForm = () => {
    const totalCapacity = smsPackages.reduce((sum, item) => sum + Number(item.capacity || 0), 0);
    const remainingCapacity = smsPackages.reduce((sum, item) => sum + Number(item.remainingCapacity || 0), 0);
    const usedPercent = totalCapacity > 0 ? Math.round((1 - remainingCapacity / totalCapacity) * 100) : 0;
    const selectableTemplates = smsTemplates.filter(item => ['READY', 'APPROVED'].includes(String(item.status).toUpperCase()));
    return (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Card
        bordered={false}
        style={{ marginBottom: 24, color: '#fff', background: 'linear-gradient(120deg, #1677ff 0%, #36cfc9 100%)' }}
        styles={{ body: { padding: 24 } }}
      >
        <Row gutter={[24, 16]} align="middle">
          <Col xs={24} md={8}><Statistic title={<span style={{ color: 'rgba(255,255,255,.8)' }}>量包总额度</span>} value={totalCapacity} prefix={<CloudServerOutlined />} valueStyle={{ color: '#fff' }} /></Col>
          <Col xs={24} md={8}><Statistic title={<span style={{ color: 'rgba(255,255,255,.8)' }}>剩余短信</span>} value={remainingCapacity} prefix={<MessageOutlined />} valueStyle={{ color: '#fff' }} /></Col>
          <Col xs={24} md={8}>
            <Text style={{ color: 'rgba(255,255,255,.8)' }}>量包使用率</Text>
            <Progress percent={usedPercent} strokeColor="#fff" trailColor="rgba(255,255,255,.25)" style={{ marginTop: 10 }} />
          </Col>
        </Row>
      </Card>

      <Card title={<Space><CloudServerOutlined />短信量包</Space>} extra={<Button icon={<ReloadOutlined />} loading={packagesLoading} onClick={fetchSmsPackages}>刷新</Button>} style={{ marginBottom: 24 }}>
        {packagesError && <Alert type="warning" showIcon message={packagesError} style={{ marginBottom: 16 }} />}
        <Table rowKey="packageId" size="small" loading={packagesLoading} pagination={false} dataSource={smsPackages} columns={[
          { title: '量包名称', dataIndex: 'name' },
          { title: '适用范围', dataIndex: 'countryType', render: (v: string) => v === 'domestic' ? '国内' : v === 'international' ? '国际' : v },
          { title: '总量', dataIndex: 'capacity', align: 'right' },
          { title: '剩余', dataIndex: 'remainingCapacity', align: 'right' },
          { title: '状态', dataIndex: 'packageStatus', render: (v: string) => <Tag color={v === 'RUNNING' ? 'success' : 'default'}>{v === 'RUNNING' ? '使用中' : v}</Tag> },
          { title: '到期时间', dataIndex: 'expireDate' },
        ]} />
      </Card>

      <Card title="短信发放开关" extra={<Text type="secondary">关闭后对应业务不会向百度提交短信</Text>} style={{ marginBottom: 24 }}>
        <Row gutter={[16, 16]}>
          {smsSwitchItems.map(([key, title, description]) => (
            <Col xs={24} md={12} lg={8} key={key}>
              <div style={{ display: 'flex', justifyContent: 'space-between', gap: 12, padding: 16, border: '1px solid #f0f0f0', borderRadius: 10, background: '#fafafa' }}>
                <div><div style={{ fontWeight: 600 }}>{title}</div><Text type="secondary" style={{ fontSize: 12 }}>{description}</Text></div>
                <Form.Item name={key} valuePropName="checked" noStyle><Switch checkedChildren="启用" unCheckedChildren="关闭" /></Form.Item>
              </div>
            </Col>
          ))}
        </Row>
      </Card>

      <Card title="百度智能云连接配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_ACCESS_KEY" label="Access">
              <Input placeholder="请输入百度智能云 Access" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_SECRET_KEY" label="Secret">
              <Input.Password placeholder="请输入百度智能云 Secret" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_ENDPOINT" label="服务域名">
              <Input placeholder="https://sms.bj.baidubce.com" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_SIGNATURE_ID" label="签名ID">
              <Input placeholder="sms-sign-..." />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_SIGNATURE_CONTENT" label="签名内容">
              <Input placeholder="华微智检" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_CERTIFICATE_ID" label="资质ID">
              <Input placeholder="sms-cert-..." />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_USER_ID" label="百度智能云用户 ID" tooltip="查询短信量包必填，不是 Access Key">
              <Input placeholder="请输入百度智能云用户 ID" />
            </Form.Item>
          </Col>
        </Row>

      </Card>

      <Card title={<Space><FileTextOutlined />短信模板</Space>} extra={<Button icon={<ReloadOutlined />} loading={templatesLoading} onClick={fetchSmsTemplates}>读取模板</Button>} style={{ marginBottom: 24 }}>
        {templatesError && <Alert type="warning" showIcon message={templatesError} style={{ marginBottom: 16 }} />}
        <Form.Item name="SMS_BAIDU_TEMPLATE_IDS" label="候选模板 ID" tooltip="百度接口只支持按模板 ID 查询；多个 ID 用逗号分隔，保存后点击读取模板">
          <TextArea rows={2} placeholder="sms-tmpl-xxx, sms-tmpl-yyy" />
        </Form.Item>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_LOGIN_TEMPLATE_ID" label="验证码模板">
              <Select showSearch optionFilterProp="label" placeholder="请选择已审核模板" options={selectableTemplates.map(item => ({ value: item.templateId, label: `${item.name || item.templateId}（${item.templateId}）` }))} />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="SMS_BAIDU_REPORT_TEMPLATE_ID" label="报告出具通知模板">
              <Select showSearch optionFilterProp="label" placeholder="请选择已审核模板" options={selectableTemplates.map(item => ({ value: item.templateId, label: `${item.name || item.templateId}（${item.templateId}）` }))} />
            </Form.Item>
          </Col>
        </Row>
        <Table rowKey="templateId" size="small" loading={templatesLoading} pagination={false} dataSource={smsTemplates} columns={[
          { title: '模板', dataIndex: 'name', render: (v: string, row: any) => <div><div>{v || '未读取'}</div><Text type="secondary">{row.templateId}</Text></div> },
          { title: '内容', dataIndex: 'content', ellipsis: true },
          { title: '类型', dataIndex: 'smsType' },
          { title: '状态', dataIndex: 'status', render: (v: string) => <Tag color={['READY', 'APPROVED'].includes(String(v).toUpperCase()) ? 'success' : v === 'QUERY_FAILED' ? 'error' : 'warning'}>{v}</Tag> },
          { title: '审核说明', dataIndex: 'review', ellipsis: true },
        ]} />
      </Card>

      <Card title="短信发放日志" style={{ marginBottom: 24 }}>
        <Space wrap style={{ marginBottom: 16 }}>
          <Input placeholder="手机号" allowClear value={logFilters.mobile} onChange={e => setLogFilters({ ...logFilters, mobile: e.target.value })} style={{ width: 150 }} />
          <Select allowClear placeholder="短信功能" value={logFilters.purpose || undefined} onChange={v => setLogFilters({ ...logFilters, purpose: v || '' })} style={{ width: 190 }} options={smsSwitchItems.map(([key, title]) => ({ value: key.replace('SMS_', '').replace('_ENABLED', '').toLowerCase(), label: title }))} />
          <Select allowClear placeholder="发送状态" value={logFilters.status || undefined} onChange={v => setLogFilters({ ...logFilters, status: v || '' })} style={{ width: 130 }} options={[{ value: 'success', label: '发送成功' }, { value: 'failed', label: '发送失败' }, { value: 'skipped', label: '已关闭' }]} />
          <Input type="date" value={logFilters.start_date} onChange={e => setLogFilters({ ...logFilters, start_date: e.target.value })} style={{ width: 150 }} />
          <Input type="date" value={logFilters.end_date} onChange={e => setLogFilters({ ...logFilters, end_date: e.target.value })} style={{ width: 150 }} />
          <Button type="primary" onClick={() => fetchSmsLogs(1, logFilters)}>查询</Button>
        </Space>
        <Table rowKey="id" size="small" loading={logsLoading} dataSource={smsLogs} pagination={{ current: logPage, pageSize: 10, total: logTotal, showTotal: total => `共 ${total} 条`, onChange: page => fetchSmsLogs(page, logFilters) }} columns={[
          { title: '发送时间', dataIndex: 'created_at', width: 170 },
          { title: '功能', dataIndex: 'purpose_name', width: 180 },
          { title: '手机号', dataIndex: 'mobile', width: 130 },
          { title: '模板 ID', dataIndex: 'template_id', ellipsis: true },
          { title: '状态', dataIndex: 'status', width: 100, render: (v: string) => <Tag color={v === 'success' ? 'success' : v === 'failed' ? 'error' : v === 'skipped' ? 'default' : 'processing'}>{v === 'success' ? '成功' : v === 'failed' ? '失败' : v === 'skipped' ? '已关闭' : '发送中'}</Tag> },
          { title: '结果', dataIndex: 'provider_message', ellipsis: true, render: (v: string, row: any) => v || row.provider_code || '-' },
        ]} />
      </Card>

      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
    );
  };

  // 邮箱通知配置表单
  const EmailSettingsForm = () => (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Card title="邮箱服务器配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="EMAIL_HOST" label="邮箱服务器">
              <Input placeholder="请输入邮箱服务器地址" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="EMAIL_PORT" label="邮箱端口">
              <Input placeholder="请输入邮箱服务器端口" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="EMAIL_USERNAME" label="邮箱用户名">
              <Input placeholder="请输入邮箱用户名" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="EMAIL_PASSWORD" label="邮箱密码">
              <Input.Password placeholder="请输入邮箱密码" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="EMAIL_FROM" label="发件人邮箱">
              <Input placeholder="请输入发件人邮箱地址" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Card title="邮件模板配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={24}>
            <Form.Item name="EMAIL_TEMPLATE_VERIFICATION" label="验证码邮件模板">
              <TextArea rows={4} placeholder="请输入验证码邮件模板，{{code}}会被替换为实际验证码" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={24}>
            <Form.Item name="EMAIL_TEMPLATE_NOTIFICATION" label="通知邮件模板">
              <TextArea rows={4} placeholder="请输入通知邮件模板，{{content}}会被替换为实际通知内容" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
  );

  const WechatSettingsForm = () => (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Card title="微信小程序配置" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="WECHAT_APP_ID" label="AppID">
              <Input placeholder="请输入微信小程序 AppID" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="WECHAT_APP_SECRET" label="AppSecret">
              <Input.Password placeholder="请输入微信小程序 AppSecret" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="WECHAT_QRCODE_ENV_VERSION" label="小程序码环境">
              <Select placeholder="请选择环境">
                <Option value="release">正式版 release</Option>
                <Option value="trial">体验版 trial</Option>
                <Option value="develop">开发版 develop</Option>
              </Select>
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Card title="证书与密钥路径" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="WECHAT_CERT_PATH" label="开放平台证书">
              <Input placeholder="cart/开放平台证书.cer" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="WECHAT_AES_KEY_PATH" label="对称密钥">
              <Input placeholder="cart/对称密钥.txt" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="WECHAT_RSA_PRIVATE_KEY_PATH" label="RSA 私钥">
              <Input placeholder="cart/RSA PRIVATE KEY.txt" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="WECHAT_RSA_PUBLIC_KEY_PATH" label="RSA 公钥">
              <Input placeholder="cart/PUBLIC KEY.txt" />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
  );

  const MiniappContentForm = () => (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Card title="帮助中心" style={{ marginBottom: 24 }}>
        <Row gutter={16}>
          <Col span={24}>
            <Form.Item
              name="MINIAPP_HELP_CENTER_JSON"
              label="帮助中心分类 JSON"
              tooltip="格式：{ categories: [{ name, items: [{ question, answer }] }] }"
            >
              <TextArea
                rows={14}
                placeholder='{"categories":[{"name":"报告查看","items":[{"question":"什么时候可以查看报告？","answer":"审核通过后可查看。"}]}]}'
              />
            </Form.Item>
          </Col>
        </Row>
      </Card>

      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存配置
        </Button>
      </Form.Item>
    </Form>
  );

  const StorageSettingsForm = () => (
    <Form form={form} layout="vertical" onFinish={handleSave}>
      <Alert
        type="info"
        showIcon
        message="AccessKey/SecretKey 仅由服务端用于生成上传凭证，SecretKey 不会下发给文件上传客户端。"
        style={{ marginBottom: 24 }}
      />
      <Card title="七牛云对象存储" style={{ marginBottom: 24 }}>
        <Form.Item name="QINIU_ENABLED" label="启用七牛云" valuePropName="checked">
          <Switch checkedChildren="启用" unCheckedChildren="关闭" />
        </Form.Item>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="QINIU_ACCESS_KEY" label="AccessKey" rules={[{ required: true, message: '请输入 AccessKey' }]}>
              <Input autoComplete="off" placeholder="请输入七牛云 AccessKey" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="QINIU_SECRET_KEY" label="SecretKey" rules={[{ required: true, message: '请输入 SecretKey' }]}>
              <Input.Password autoComplete="new-password" placeholder="请输入七牛云 SecretKey" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="QINIU_BUCKET" label="空间名称" rules={[{ required: true, message: '请输入空间名称' }]}>
              <Input placeholder="bucket01-bgpt-huaweibio-com-cn" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="QINIU_DOMAIN" label="访问域名" rules={[{ required: true, message: '请输入访问域名' }]}>
              <Input placeholder="https://bucket01.huaweibio.com.cn" />
            </Form.Item>
          </Col>
        </Row>
        <Row gutter={16}>
          <Col span={12}>
            <Form.Item name="QINIU_UPLOAD_URL" label="上传地址" tooltip="若空间区域不匹配，请按七牛控制台显示的区域上传域名修改">
              <Input placeholder="https://upload.qiniup.com" />
            </Form.Item>
          </Col>
          <Col span={12}>
            <Form.Item name="QINIU_TOKEN_TTL_SECONDS" label="上传凭证有效期（秒）">
              <InputNumber min={60} max={86400} precision={0} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
        </Row>
      </Card>
      <Card
        title="空间用量与文件结构"
        style={{ marginBottom: 24 }}
        extra={<Button icon={<ReloadOutlined />} loading={storageLoading} onClick={() => fetchStorageOverview(storagePrefix)}>刷新</Button>}
      >
        {storageError ? (
          <Alert type="warning" showIcon message={storageError} description="请保存正确的七牛云配置后重试。" />
        ) : (
          <>
            <Row gutter={16} style={{ marginBottom: 20 }}>
              <Col span={8}><Statistic title="空间" value={storageOverview?.bucket || '-'} /></Col>
              <Col span={8}><Statistic title="文件数量" value={storageOverview?.total_files || 0} suffix="个" /></Col>
              <Col span={8}><Statistic title="已用容量" value={formatBytes(Number(storageOverview?.total_bytes || 0))} /></Col>
            </Row>
            <Space style={{ marginBottom: 12 }}>
              <Button
                size="small"
                disabled={!storagePrefix}
                onClick={() => {
                  const segments = storagePrefix.replace(/\/$/, '').split('/').filter(Boolean);
                  segments.pop();
                  fetchStorageOverview(segments.length ? `${segments.join('/')}/` : '');
                }}
              >
                返回上级
              </Button>
              <Text code>/{storagePrefix}</Text>
              {storageOverview?.usage_truncated ? <Tag color="warning">用量统计已达扫描上限</Tag> : <Tag color="success">连接正常</Tag>}
            </Space>
            <Table
              size="small"
              rowKey={(record: any) => record.rowKey}
              loading={storageLoading}
              pagination={false}
              dataSource={[
                ...(storageOverview?.common_prefixes || []).map((prefix: string) => ({
                  rowKey: `dir-${prefix}`, kind: 'directory', key: prefix, name: prefix.slice(storagePrefix.length).replace(/\/$/, ''),
                })),
                ...(storageOverview?.items || []).map((item: any) => ({
                  ...item, rowKey: `file-${item.key}`, kind: 'file', name: item.key.slice(storagePrefix.length),
                })),
              ]}
              columns={[
                {
                  title: '名称', dataIndex: 'name',
                  render: (name: string, record: any) => record.kind === 'directory'
                    ? <Button type="link" icon={<FolderOpenOutlined />} onClick={() => fetchStorageOverview(record.key)}>{name}</Button>
                    : <a href={record.url} target="_blank" rel="noreferrer"><FileOutlined /> {name}</a>,
                },
                { title: '类型', dataIndex: 'mimeType', width: 180, render: (value: string, record: any) => record.kind === 'directory' ? '目录' : (value || '-') },
                { title: '大小', dataIndex: 'fsize', width: 120, render: (value: number, record: any) => record.kind === 'directory' ? '-' : formatBytes(Number(value || 0)) },
                { title: '上传时间', dataIndex: 'uploaded_at', width: 180, render: (value: string) => value || '-' },
              ]}
            />
          </>
        )}
      </Card>
      <Form.Item>
        <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={saving}>
          保存七牛云配置
        </Button>
      </Form.Item>
    </Form>
  );

  // 定义标签页
  const tabsItems = [
    {
      key: 'basic',
      label: '系统基础设置',
      children: <BasicSettingsForm />,
    },
    {
      key: 'sms',
      label: '短信通知配置',
      children: <SmsSettingsForm />,
    },
    {
      key: 'email',
      label: '邮箱通知配置',
      children: <EmailSettingsForm />,
    },
    {
      key: 'wechat',
      label: '微信小程序/证书',
      children: <WechatSettingsForm />,
    },
    {
      key: 'miniapp-content',
      label: '小程序内容',
      children: <MiniappContentForm />,
    },
    {
      key: 'storage',
      label: '文件存储',
      children: <StorageSettingsForm />,
    },
  ];

  return (
    <div>
      <Tabs defaultActiveKey="basic" items={tabsItems} />
    </div>
  );
};

export default Settings;
