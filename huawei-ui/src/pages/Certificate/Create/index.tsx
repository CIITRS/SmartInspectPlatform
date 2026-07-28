import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Row, Col, Select, DatePicker, Upload, Space, message, Tabs } from 'antd';
import { UploadOutlined, EyeOutlined, SearchOutlined, RestOutlined, DownloadOutlined, PrinterOutlined } from '@ant-design/icons';
import dayjs from 'dayjs';
import type { UploadFile, UploadProps } from 'antd';
import type { RcFile } from 'antd/es/upload';

const { Option } = Select;
const { TextArea } = Input;
const { RangePicker } = DatePicker;

interface CertificateForm {
  certificateNumber: string;
  patientName: string;
  idCard: string;
  phone: string;
  certificateType: string;
  issueDate: dayjs.Dayjs;
  expiryDate: dayjs.Dayjs;
  issuer: string;
  description: string;
}

const CertificateCreate: React.FC = () => {
  const [form] = Form.useForm<CertificateForm>();
  const [loading, setLoading] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewImage, setPreviewImage] = useState('');
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [browserTab, setBrowserTab] = useState('preview');

  const handlePreview = async (file: UploadFile) => {
    if (!file.url && !file.preview) {
      file.preview = await new Promise((resolve) => {
        const reader = new FileReader();
        reader.readAsDataURL(file.originFileObj as RcFile);
        reader.onload = () => resolve(reader.result as string | undefined);
      });
    }
    setPreviewImage(file.url || (file.preview as string));
    setPreviewVisible(true);
  };

  const handleChange: UploadProps['onChange'] = ({ fileList: newFileList }) => {
    setFileList(newFileList);
  };

  const handleSubmit = async (values: CertificateForm) => {
    try {
      setLoading(true);
      // 模拟提交
      await new Promise(resolve => setTimeout(resolve, 1000));
      message.success('凭证创建成功！');
      form.resetFields();
      setFileList([]);
    } catch (error) {
      message.error('凭证创建失败，请重试！');
    } finally {
      setLoading(false);
    }
  };

  const uploadProps: UploadProps = {
    name: 'file',
    action: 'https://run.mocky.io/v3/435e224c-44fb-4773-9faf-380c5e6a2188',
    headers: {
      authorization: 'authorization-text',
    },
    onPreview: handlePreview,
    onChange: handleChange,
    multiple: true,
  };

  return (
    <div style={{ padding: '20px' }}>
      <Row gutter={16}>
        {/* 左侧凭证录入表单 */}
        <Col xs={24} lg={12}>
          <Card title="凭证录入" style={{ height: '100%' }}>
            <Form
              form={form}
              layout="vertical"
              onFinish={handleSubmit}
              initialValues={{
                issueDate: dayjs(),
                expiryDate: dayjs().add(1, 'year'),
              }}
            >
              <Form.Item
                name="certificateNumber"
                label="凭证编号"
                rules={[{ required: true, message: '请输入凭证编号' }]}
              >
                <Input placeholder="请输入凭证编号" />
              </Form.Item>

              <Form.Item
                name="patientName"
                label="患者姓名"
                rules={[{ required: true, message: '请输入患者姓名' }]}
              >
                <Input placeholder="请输入患者姓名" />
              </Form.Item>

              <Form.Item
                name="idCard"
                label="身份证号"
                rules={[{ required: true, message: '请输入身份证号' }]}
              >
                <Input placeholder="请输入身份证号" />
              </Form.Item>

              <Form.Item
                name="phone"
                label="联系电话"
                rules={[{ required: true, message: '请输入联系电话' }]}
              >
                <Input placeholder="请输入联系电话" />
              </Form.Item>

              <Form.Item
                name="certificateType"
                label="凭证类型"
                rules={[{ required: true, message: '请选择凭证类型' }]}
              >
                <Select placeholder="请选择凭证类型">
                  <Option value="invoice">发票</Option>
                  <Option value="receipt">收据</Option>
                  <Option value="certificate">证明</Option>
                  <Option value="other">其他</Option>
                </Select>
              </Form.Item>

              <Row gutter={16}>
                <Col xs={24} md={12}>
                  <Form.Item
                    name="issueDate"
                    label="签发日期"
                    rules={[{ required: true, message: '请选择签发日期' }]}
                  >
                    <DatePicker style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
                <Col xs={24} md={12}>
                  <Form.Item
                    name="expiryDate"
                    label="有效期至"
                    rules={[{ required: true, message: '请选择有效期至' }]}
                  >
                    <DatePicker style={{ width: '100%' }} />
                  </Form.Item>
                </Col>
              </Row>

              <Form.Item
                name="issuer"
                label="签发人"
                rules={[{ required: true, message: '请输入签发人' }]}
              >
                <Input placeholder="请输入签发人" />
              </Form.Item>

              <Form.Item
                name="description"
                label="凭证描述"
              >
                <TextArea rows={4} placeholder="请输入凭证描述" />
              </Form.Item>

              <Form.Item label="附件上传">
                <Upload {...uploadProps} fileList={fileList}>
                  <Button icon={<UploadOutlined />}>点击上传</Button>
                </Upload>
              </Form.Item>

              <Form.Item>
                <Space>
                  <Button type="primary" htmlType="submit" loading={loading}>
                    提交
                  </Button>
                  <Button htmlType="button" onClick={() => form.resetFields()}>
                    重置
                  </Button>
                </Space>
              </Form.Item>
            </Form>
          </Card>
        </Col>

        {/* 右侧浏览器样式预览面板 */}
        <Col xs={24} lg={12}>
          <Card title="凭证预览" style={{ height: '100%' }}>
            {/* 浏览器导航栏 */}
            <div style={{ 
              background: '#f0f0f0', 
              padding: '8px 16px', 
              borderRadius: '4px 4px 0 0',
              borderBottom: '1px solid #e8e8e8',
              display: 'flex',
              alignItems: 'center',
              gap: '8px'
            }}>
              <div style={{ display: 'flex', gap: '4px' }}>
                <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#ff5f56' }}></div>
                <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#ffbd2e' }}></div>
                <div style={{ width: '12px', height: '12px', borderRadius: '50%', background: '#27ca3f' }}></div>
              </div>
              
              <div style={{ flex: 1, margin: '0 16px' }}>
                <div style={{ 
                  background: '#fff', 
                  borderRadius: '16px', 
                  padding: '4px 12px', 
                  display: 'flex', 
                  alignItems: 'center',
                  gap: '8px'
                }}>
                  <SearchOutlined style={{ fontSize: '14px', color: '#999' }} />
                  <span style={{ fontSize: '14px', color: '#666' }}>凭证预览</span>
                </div>
              </div>
              
              <Space size="small">
                <Button 
                  size="small" 
                  icon={<RestOutlined />} 
                  onClick={() => message.info('刷新预览')}
                />
                <Button 
                  size="small" 
                  icon={<EyeOutlined />} 
                  onClick={() => message.info('切换视图')}
                />
                <Button 
                  size="small" 
                  icon={<DownloadOutlined />} 
                  onClick={() => message.info('下载凭证')}
                />
                <Button 
                  size="small" 
                  icon={<PrinterOutlined />} 
                  onClick={() => message.info('打印凭证')}
                />
              </Space>
            </div>

            {/* 浏览器标签页 */}
            <Tabs 
              activeKey={browserTab} 
              onChange={setBrowserTab}
              style={{ borderBottom: '1px solid #e8e8e8' }}
            >
              <Tabs.TabPane tab="凭证预览" key="preview">
                <div style={{ padding: '20px' }}>
                  <Card type="inner" title="凭证信息" style={{ marginBottom: '20px' }}>
                    <div style={{ fontSize: '14px', lineHeight: '24px' }}>
                      <p><strong>凭证编号：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('certificateNumber') || '未填写'}</span></p>
                      <p><strong>患者姓名：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('patientName') || '未填写'}</span></p>
                      <p><strong>身份证号：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('idCard') || '未填写'}</span></p>
                      <p><strong>联系电话：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('phone') || '未填写'}</span></p>
                      <p><strong>凭证类型：</strong><span style={{ marginLeft: '8px' }}>
                        {form.getFieldValue('certificateType') ? 
                          form.getFieldValue('certificateType') === 'invoice' ? '发票' :
                          form.getFieldValue('certificateType') === 'receipt' ? '收据' :
                          form.getFieldValue('certificateType') === 'certificate' ? '证明' : '其他'
                          : '未选择'
                        }
                      </span></p>
                      <p><strong>签发日期：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('issueDate')?.format('YYYY-MM-DD') || '未选择'}</span></p>
                      <p><strong>有效期至：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('expiryDate')?.format('YYYY-MM-DD') || '未选择'}</span></p>
                      <p><strong>签发人：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('issuer') || '未填写'}</span></p>
                      <p><strong>凭证描述：</strong><span style={{ marginLeft: '8px' }}>{form.getFieldValue('description') || '未填写'}</span></p>
                    </div>
                  </Card>

                  {fileList.length > 0 && (
                    <Card type="inner" title="上传附件" style={{ marginBottom: '20px' }}>
                      <div style={{ display: 'flex', flexDirection: 'column', gap: '8px' }}>
                        {fileList.map((file) => (
                          <div key={file.uid} style={{ 
                            display: 'flex', 
                            alignItems: 'center', 
                            gap: '8px',
                            padding: '8px',
                            background: '#fafafa',
                            borderRadius: '4px'
                          }}>
                            <UploadOutlined />
                            <span style={{ flex: 1 }}>{file.name}</span>
                            <span style={{ fontSize: '12px', color: '#999' }}>{file.size ? `${(file.size / 1024).toFixed(2)} KB` : ''}</span>
                            <Button size="small" icon={<DownloadOutlined />} onClick={() => message.info('下载附件')} />
                          </div>
                        ))}
                      </div>
                    </Card>
                  )}

                  <div style={{ textAlign: 'center', padding: '40px 0', color: '#999' }}>
                    <EyeOutlined style={{ fontSize: '48px', marginBottom: '16px' }} />
                    <p>填写左侧表单以预览凭证信息</p>
                  </div>
                </div>
              </Tabs.TabPane>
              <Tabs.TabPane tab="历史记录" key="history">
                <div style={{ padding: '20px', textAlign: 'center', color: '#999' }}>
                  <SearchOutlined style={{ fontSize: '48px', marginBottom: '16px' }} />
                  <p>暂无历史记录</p>
                </div>
              </Tabs.TabPane>
              <Tabs.TabPane tab="相关凭证" key="related">
                <div style={{ padding: '20px', textAlign: 'center', color: '#999' }}>
                  <SearchOutlined style={{ fontSize: '48px', marginBottom: '16px' }} />
                  <p>暂无相关凭证</p>
                </div>
              </Tabs.TabPane>
            </Tabs>
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default CertificateCreate;