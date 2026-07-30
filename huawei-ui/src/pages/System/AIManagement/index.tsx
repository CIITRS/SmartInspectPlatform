import React, { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, Form, Input, Button, App, Spin, Alert, Tooltip as AntTooltip, Tabs, Table, Select, Popconfirm, Tag } from 'antd';
import { 
  SettingOutlined, 
  KeyOutlined, 
  LinkOutlined, 
  AppstoreOutlined, 
  ProfileOutlined, 
  ThunderboltOutlined, 
  CalendarOutlined, 
  HistoryOutlined, 
  InfoCircleOutlined,
  SaveOutlined
} from '@ant-design/icons';
import { request } from '@umijs/max';
import { LineChart, Line, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer } from 'recharts';

interface AISettings {
  api_key: string;
  api_url: string;
  model: string;
  prompt: string;
  report_vision_model: string;
  report_text_model: string;
  report_prompt: string;
  configured: boolean;
}

interface AIUsageHistory {
  date: string;
  count: number;
}

interface AIUsageData {
  today_usage: number;
  month_usage: number;
  total_usage: number;
  history: AIUsageHistory[];
}

interface AIBlacklistItem {
  id: number;
  subject_type: 'patient' | 'employee';
  subject_code: string;
  subject_name: string;
  reason: string;
  created_at: string;
}

const AIManagement: React.FC = () => {
  const { message: appMessage } = App.useApp();
  const [form] = Form.useForm();
  const [blacklistForm] = Form.useForm();
  
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [loadingUsage, setLoadingUsage] = useState(false);
  const [saving, setSaving] = useState(false);
  const [blacklistLoading, setBlacklistLoading] = useState(false);
  const [blacklistSaving, setBlacklistSaving] = useState(false);
  
  const [settings, setSettings] = useState<AISettings | null>(null);
  const [usage, setUsage] = useState<AIUsageData>({
    today_usage: 0,
    month_usage: 0,
    total_usage: 0,
    history: []
  });
  const [blacklist, setBlacklist] = useState<AIBlacklistItem[]>([]);

  // 获取配置
  const fetchSettings = async () => {
    setLoadingConfig(true);
    try {
      const response = await request<{ code: number; success: boolean; data: AISettings }>('/api/system/ai-settings', {
        method: 'GET'
      });
      if (response && response.success) {
        setSettings(response.data);
        form.setFieldsValue({
          api_url: response.data.api_url,
          api_key: response.data.api_key,
          model: response.data.model,
          prompt: response.data.prompt,
          report_vision_model: response.data.report_vision_model,
          report_text_model: response.data.report_text_model,
          report_prompt: response.data.report_prompt
        });
      } else {
        appMessage.error('获取AI配置信息失败');
      }
    } catch (error) {
      console.error('Fetch settings error:', error);
      appMessage.error('获取AI配置信息失败');
    } finally {
      setLoadingConfig(false);
    }
  };

  // 获取用量统计
  const fetchUsage = async () => {
    setLoadingUsage(true);
    try {
      const response = await request<{ code: number; success: boolean; data: AIUsageData }>('/api/system/ai-usage', {
        method: 'GET'
      });
      if (response && response.success) {
        // 格式化日期格式以便图表更易读 (例如 2026-06-02 提取月份日期 06-02)
        const formattedHistory = (response.data.history || []).map((item) => {
          const dateParts = item.date.split('-');
          const displayDate = dateParts.length >= 3 ? `${dateParts[1]}-${dateParts[2]}` : item.date;
          return {
            ...item,
            displayDate,
            count: Number(item.count)
          };
        });

        setUsage({
          ...response.data,
          history: formattedHistory as any
        });
      } else {
        appMessage.error('获取用量统计失败');
      }
    } catch (error) {
      console.error('Fetch usage error:', error);
      appMessage.error('获取用量统计失败');
    } finally {
      setLoadingUsage(false);
    }
  };

  const fetchBlacklist = async () => {
    setBlacklistLoading(true);
    try {
      const response = await request<{ success: boolean; data: { list: AIBlacklistItem[] } }>('/api/system/ai-blacklist', {
        method: 'GET'
      });
      if (response && response.success) {
        setBlacklist(response.data?.list || []);
      } else {
        appMessage.error('获取AI黑名单失败');
      }
    } catch (error) {
      console.error('Fetch blacklist error:', error);
      appMessage.error('获取AI黑名单失败');
    } finally {
      setBlacklistLoading(false);
    }
  };

  useEffect(() => {
    fetchSettings();
    fetchUsage();
    fetchBlacklist();
  }, []);

  // 提交修改
  const handleSaveSettings = async (values: any) => {
    setSaving(true);
    try {
      const response = await request<{ code: number; success: boolean; message: string }>('/api/system/ai-settings', {
        method: 'PUT',
        data: {
          api_key: values.api_key,
          api_url: values.api_url,
          model: values.model,
          prompt: values.prompt,
          report_vision_model: values.report_vision_model,
          report_text_model: values.report_text_model,
          report_prompt: values.report_prompt
        }
      });

      if (response && response.success) {
        appMessage.success('AI配置保存成功且即时生效');
        fetchSettings(); // 重新加载以展示最新脱敏数据
      } else {
        appMessage.error(response.message || '保存配置失败');
      }
    } catch (error) {
      console.error('Save settings error:', error);
      appMessage.error('保存配置发生错误');
    } finally {
      setSaving(false);
    }
  };

  const handleAddBlacklist = async (values: any) => {
    setBlacklistSaving(true);
    try {
      const response = await request<{ success: boolean; message: string }>('/api/system/ai-blacklist', {
        method: 'POST',
        data: {
          subject_type: values.subject_type,
          subject_code: values.subject_code,
          reason: values.reason || ''
        }
      });
      if (response && response.success) {
        appMessage.success('已加入AI黑名单');
        blacklistForm.resetFields();
        fetchBlacklist();
      } else {
        appMessage.error(response.message || '加入AI黑名单失败');
      }
    } catch (error: any) {
      appMessage.error(error?.message || '加入AI黑名单失败');
    } finally {
      setBlacklistSaving(false);
    }
  };

  const handleRemoveBlacklist = async (subjectCode: string) => {
    try {
      const response = await request<{ success: boolean; message: string }>(`/api/system/ai-blacklist/${subjectCode}`, {
        method: 'DELETE'
      });
      if (response && response.success) {
        appMessage.success('已移出AI黑名单');
        fetchBlacklist();
      } else {
        appMessage.error(response.message || '移出AI黑名单失败');
      }
    } catch (error: any) {
      appMessage.error(error?.message || '移出AI黑名单失败');
    }
  };

  const blacklistColumns = [
    {
      title: '类型',
      dataIndex: 'subject_type',
      key: 'subject_type',
      render: (value: string) => value === 'patient' ? <Tag color="blue">患者</Tag> : <Tag color="green">员工</Tag>,
    },
    { title: '自编编号', dataIndex: 'subject_code', key: 'subject_code' },
    { title: '名称', dataIndex: 'subject_name', key: 'subject_name' },
    { title: '原因', dataIndex: 'reason', key: 'reason', render: (text: string) => text || '-' },
    {
      title: '加入时间',
      dataIndex: 'created_at',
      key: 'created_at',
      render: (text: string) => text ? text.slice(0, 10) : '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: AIBlacklistItem) => (
        <Popconfirm title="确认移出AI黑名单？" onConfirm={() => handleRemoveBlacklist(record.subject_code)}>
          <Button danger size="small">移出</Button>
        </Popconfirm>
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <div style={{ marginBottom: '20px' }}>
        <h2 style={{ fontSize: '20px', fontWeight: 600, color: '#1f2d3d', margin: 0 }}>AI 服务管理</h2>
        <p style={{ color: '#8c9aa8', margin: '4px 0 0 0', fontSize: '13px' }}>
          在此配置AI接口服务的对接参数，并监控小程序与系统内的AI Token用量消耗。
        </p>
      </div>

      <Tabs
        items={[
          {
            key: 'overview',
            label: '配置与用量',
            children: (
              <>
      <Row gutter={[20, 20]}>
        {/* 用量监控卡片统计 */}
        <Col xs={24} lg={8}>
          <Spin spinning={loadingUsage}>
            <Row gutter={[16, 16]}>
              <Col span={24}>
                <Card 
                  bordered={false}
                  style={{
                    background: 'linear-gradient(135deg, #1890ff 0%, #096dd9 100%)',
                    borderRadius: '12px',
                    color: '#fff',
                    boxShadow: '0 4px 12px rgba(24, 144, 255, 0.25)'
                  }}
                >
                  <Statistic 
                    title={<span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '14px' }}><ThunderboltOutlined /> 今日 Token 消耗量</span>}
                    value={usage.today_usage}
                    valueStyle={{ color: '#fff', fontSize: '28px', fontWeight: 'bold' }}
                    suffix={<span style={{ fontSize: '14px', color: 'rgba(255,255,255,0.8)' }}>Tokens</span>}
                  />
                  <div style={{ marginTop: '12px', fontSize: '12px', color: 'rgba(255,255,255,0.7)' }}>
                    实时统计截至今天 23:59:59 的总消耗量
                  </div>
                </Card>
              </Col>
              
              <Col span={24}>
                <Card 
                  bordered={false}
                  style={{
                    background: 'linear-gradient(135deg, #722ed1 0%, #531dab 100%)',
                    borderRadius: '12px',
                    color: '#fff',
                    boxShadow: '0 4px 12px rgba(114, 46, 209, 0.25)'
                  }}
                >
                  <Statistic 
                    title={<span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '14px' }}><CalendarOutlined /> 本月 Token 消耗量</span>}
                    value={usage.month_usage}
                    valueStyle={{ color: '#fff', fontSize: '28px', fontWeight: 'bold' }}
                    suffix={<span style={{ fontSize: '14px', color: 'rgba(255,255,255,0.8)' }}>Tokens</span>}
                  />
                  <div style={{ marginTop: '12px', fontSize: '12px', color: 'rgba(255,255,255,0.7)' }}>
                    当前自然月计费周期的累计消费
                  </div>
                </Card>
              </Col>

              <Col span={24}>
                <Card 
                  bordered={false}
                  style={{
                    background: 'linear-gradient(135deg, #13c2c2 0%, #08979c 100%)',
                    borderRadius: '12px',
                    color: '#fff',
                    boxShadow: '0 4px 12px rgba(19, 194, 194, 0.25)'
                  }}
                >
                  <Statistic 
                    title={<span style={{ color: 'rgba(255,255,255,0.8)', fontSize: '14px' }}><HistoryOutlined /> 累计 Token 消耗量</span>}
                    value={usage.total_usage}
                    valueStyle={{ color: '#fff', fontSize: '28px', fontWeight: 'bold' }}
                    suffix={<span style={{ fontSize: '14px', color: 'rgba(255,255,255,0.8)' }}>Tokens</span>}
                  />
                  <div style={{ marginTop: '12px', fontSize: '12px', color: 'rgba(255,255,255,0.7)' }}>
                    系统上线以来的全量消耗总和
                  </div>
                </Card>
              </Col>
            </Row>
          </Spin>
        </Col>

        {/* AI 配置面板 */}
        <Col xs={24} lg={16}>
          <Card 
            title={<span><SettingOutlined /> AI 核心对接参数</span>}
            bordered={false}
            style={{ borderRadius: '12px', boxShadow: '0 2px 8px rgba(0,0,0,0.05)' }}
            extra={
              settings?.configured ? (
                <Alert message="AI已配置接入" type="success" showIcon style={{ padding: '2px 8px', fontSize: '12px' }} />
              ) : (
                <Alert message="未配置API密钥" type="warning" showIcon style={{ padding: '2px 8px', fontSize: '12px' }} />
              )
            }
          >
            <Spin spinning={loadingConfig}>
              <Form
                form={form}
                layout="vertical"
                onFinish={handleSaveSettings}
              >
                <Row gutter={16}>
                  <Col span={24}>
                    <Form.Item
                      name="api_url"
                      label={<span>AI 服务接口 URL <AntTooltip title="AI平台对话端点。百度千帆V2多为 https://qianfan.baidubce.com/v2。如果为 /v1 或 /v2 结尾，系统会自动补充补全端点。"><InfoCircleOutlined style={{ color: '#999', marginLeft: 4 }} /></AntTooltip></span>}
                      rules={[{ required: true, message: '请输入AI接口的Base URL' }]}
                    >
                      <Input prefix={<LinkOutlined style={{ color: '#1890ff' }} />} placeholder="https://qianfan.baidubce.com/v2" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="report_vision_model"
                      label="图片报告视觉模型"
                      rules={[{ required: true, message: '请输入图片报告视觉模型' }]}
                    >
                      <Input prefix={<AppstoreOutlined />} placeholder="ernie-4.5-turbo-vl-32k" />
                    </Form.Item>
                  </Col>
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="report_text_model"
                      label="PDF 报告文本模型"
                      rules={[{ required: true, message: '请输入PDF报告文本模型' }]}
                    >
                      <Input prefix={<AppstoreOutlined />} placeholder="ernie-lite-pro-128k" />
                    </Form.Item>
                  </Col>
                  <Col span={24}>
                    <Form.Item
                      name="report_prompt"
                      label="上传报告分析提示词"
                      rules={[{ required: true, message: '请输入报告分析提示词' }]}
                    >
                      <Input.TextArea rows={8} placeholder="先识别报告类型，再客观总结，不提供诊断或治疗建议。" />
                    </Form.Item>
                  </Col>
                  
                  <Col xs={24} md={12}>
                    <Form.Item
                      name="api_key"
                      label={<span>API Key (密钥) <AntTooltip title="平台颁发的对接密钥。保存时可保持未变，系统展示为脱敏状态。"><InfoCircleOutlined style={{ color: '#999', marginLeft: 4 }} /></AntTooltip></span>}
                      rules={[{ required: true, message: '请输入API对接密钥' }]}
                    >
                      <Input.Password prefix={<KeyOutlined style={{ color: '#fa8c16' }} />} placeholder="请输入 API Key" />
                    </Form.Item>
                  </Col>

                  <Col xs={24} md={12}>
                    <Form.Item
                      name="model"
                      label={<span>模型名称 (Model) <AntTooltip title="调用的具体模型标识号，例如 ernie-lite-pro-128k 或 gpt-4o。"><InfoCircleOutlined style={{ color: '#999', marginLeft: 4 }} /></AntTooltip></span>}
                      rules={[{ required: true, message: '请输入对应的AI模型标识名称' }]}
                    >
                      <Input prefix={<AppstoreOutlined style={{ color: '#52c41a' }} />} placeholder="ernie-lite-pro-128k" />
                    </Form.Item>
                  </Col>

                  <Col span={24}>
                    <Form.Item
                      name="prompt"
                      label={<span>全局系统指令角色 Prompt (System Prompt) <AntTooltip title="设定AI对话的性格、知识背景与边界（如禁止提问除公司及ctDNA检验外的信息等）。"><InfoCircleOutlined style={{ color: '#999', marginLeft: 4 }} /></AntTooltip></span>}
                    >
                      <Input.TextArea 
                        rows={5} 
                        placeholder="请输入全局 AI System Prompt。系统会以此作为系统上下文指导AI响应问答。" 
                      />
                    </Form.Item>
                  </Col>

                  <Col span={24} style={{ textAlign: 'right', marginTop: '10px' }}>
                    <Button 
                      type="primary" 
                      htmlType="submit" 
                      loading={saving}
                      icon={<SaveOutlined />}
                      size="large"
                      style={{ borderRadius: '6px' }}
                    >
                      保存并即时应用配置
                    </Button>
                  </Col>
                </Row>
              </Form>
            </Spin>
          </Card>
        </Col>
      </Row>

      {/* 最近 7 日 Token 使用趋势线图 */}
      <Row style={{ marginTop: '24px' }}>
        <Col span={24}>
          <Card 
            title="最近 7 日 Token 消耗趋势统计"
            bordered={false}
            style={{ borderRadius: '12px', boxShadow: '0 2px 8px rgba(0,0,0,0.05)' }}
          >
            <Spin spinning={loadingUsage}>
              {usage.history && usage.history.length > 0 ? (
                <div style={{ width: '100%', height: 320 }}>
                  <ResponsiveContainer width="100%" height="100%">
                    <LineChart
                      data={usage.history}
                      margin={{ top: 20, right: 30, left: 10, bottom: 5 }}
                    >
                      <CartesianGrid strokeDasharray="3 3" vertical={false} stroke="#f0f0f0" />
                      <XAxis 
                        dataKey="displayDate" 
                        tickLine={false}
                        axisLine={{ stroke: '#e8e8e8' }}
                        tick={{ fill: '#8c9aa8', fontSize: 12 }}
                      />
                      <YAxis 
                        tickLine={false}
                        axisLine={{ stroke: '#e8e8e8' }}
                        tick={{ fill: '#8c9aa8', fontSize: 12 }}
                        tickFormatter={(value) => value.toLocaleString()}
                      />
                      <Tooltip 
                        contentStyle={{
                          background: '#fff',
                          border: 'none',
                          borderRadius: '8px',
                          boxShadow: '0 4px 12px rgba(0,0,0,0.1)'
                        }}
                        formatter={(value: any) => [`${Number(value).toLocaleString()} Tokens`, '消耗用量']}
                        labelFormatter={(label) => `日期: ${label}`}
                      />
                      <Line 
                        type="monotone" 
                        dataKey="count" 
                        stroke="#1890ff" 
                        strokeWidth={3}
                        activeDot={{ r: 8, strokeWidth: 0 }}
                        dot={{ r: 4, stroke: '#1890ff', strokeWidth: 2, fill: '#fff' }}
                      />
                    </LineChart>
                  </ResponsiveContainer>
                </div>
              ) : (
                <div style={{ textAlign: 'center', padding: '60px 0', color: '#999' }}>
                  暂无最近 7 日的使用记录数据
                </div>
              )}
            </Spin>
          </Card>
        </Col>
      </Row>
              </>
            ),
          },
          {
            key: 'blacklist',
            label: '黑名单',
            children: (
              <Card bordered={false} style={{ borderRadius: '8px', boxShadow: '0 2px 8px rgba(0,0,0,0.05)' }}>
                <Form form={blacklistForm} layout="inline" onFinish={handleAddBlacklist} style={{ marginBottom: 16 }}>
                  <Form.Item name="subject_type" rules={[{ required: true, message: '请选择类型' }]}>
                    <Select placeholder="类型" style={{ width: 120 }}>
                      <Select.Option value="patient">患者</Select.Option>
                      <Select.Option value="employee">员工</Select.Option>
                    </Select>
                  </Form.Item>
                  <Form.Item name="subject_code" rules={[{ required: true, message: '请输入自编编号' }]}>
                    <Input placeholder="患者编号或员工编号" style={{ width: 220 }} />
                  </Form.Item>
                  <Form.Item name="reason">
                    <Input placeholder="原因" style={{ width: 260 }} />
                  </Form.Item>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" loading={blacklistSaving}>加入黑名单</Button>
                  </Form.Item>
                </Form>
                <Table
                  rowKey="subject_code"
                  loading={blacklistLoading}
                  columns={blacklistColumns}
                  dataSource={blacklist}
                  pagination={{ pageSize: 10 }}
                />
              </Card>
            ),
          },
        ]}
      />
    </div>
  );
};

export default AIManagement;
