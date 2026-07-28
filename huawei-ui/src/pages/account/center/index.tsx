import React, { useState, useEffect } from 'react';
import { Form, Input, Button, Card, Descriptions, App, Modal, message } from 'antd';
import { UserOutlined, PhoneOutlined, LockOutlined } from '@ant-design/icons';
import { getCurrentUser, changePassword, updateUserInfo, updateUsername } from '@/services/api';

const { Password } = Input;

const PersonalInfo: React.FC = () => {
  const [form] = Form.useForm();
  const [passwordForm] = Form.useForm();
  const [usernameForm] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [passwordLoading, setPasswordLoading] = useState(false);
  const [usernameLoading, setUsernameLoading] = useState(false);
  const [currentUser, setCurrentUser] = useState<any>(null);
  const [usernameModalVisible, setUsernameModalVisible] = useState(false);
  const { message: appMessage } = App.useApp();

  useEffect(() => {
    fetchCurrentUser();
  }, []);

  const fetchCurrentUser = async () => {
    try {
      const response = await getCurrentUser();
      setCurrentUser(response.data);
      form.setFieldsValue({
        username: response.data.username,
        realName: response.data.real_name || '',
        phone: response.data.phone || '',
        email: response.data.email || '',
      });
    } catch (_error) {
      appMessage.error('获取用户信息失败');
    }
  };

  const handleSubmit = async (values: any) => {
    setLoading(true);
    try {
      // 将realName转换为real_name（后端使用下划线命名）
      const updateData = {
        real_name: values.realName,
        phone: values.phone,
        email: values.email,
      };
      await updateUserInfo(updateData);
      appMessage.success('个人信息更新成功');
      // 更新成功后刷新用户信息
      fetchCurrentUser();
    } catch (_error) {
      appMessage.error('个人信息更新失败');
    } finally {
      setLoading(false);
    }
  };

  const handleChangePassword = async (values: any) => {
    setPasswordLoading(true);
    try {
      // 只传递必要的字段给后端，confirmPassword已经在前端验证过了
      const passwordData = {
        old_password: values.oldPassword,
        new_password: values.newPassword,
      };
      await changePassword(passwordData);
      appMessage.success('密码修改成功');
      passwordForm.resetFields();
    } catch (error: any) {
      appMessage.error(error.message || '密码修改失败');
    } finally {
      setPasswordLoading(false);
    }
  };

  const handleUpdateUsername = async (values: any) => {
    setUsernameLoading(true);
    try {
      await updateUsername({ username: values.username });
      appMessage.success('用户名修改成功');
      setUsernameModalVisible(false);
      usernameForm.resetFields();
      // 刷新用户信息
      fetchCurrentUser();
    } catch (error: any) {
      appMessage.error(error.message || '用户名修改失败');
    } finally {
      setUsernameLoading(false);
    }
  };

  return (
    <div style={{ padding: '24px' }}>
      <Card title="个人信息" style={{ marginBottom: 24 }}>
        <Descriptions column={1} size="middle">
          <Descriptions.Item label="用户名">
            {typeof currentUser?.username === 'string' ? (
              <span>
                {currentUser.username}
                <Button type="link" size="small" style={{ marginLeft: 8 }} onClick={() => setUsernameModalVisible(true)}>修改</Button>
              </span>
            ) : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="真实姓名">{typeof currentUser?.real_name === 'string' ? currentUser.real_name : '-'}</Descriptions.Item>
          <Descriptions.Item label="部门">{typeof currentUser?.department === 'string' ? currentUser.department : '-'}</Descriptions.Item>
          <Descriptions.Item label="角色">
            {typeof currentUser?.role === 'object' && currentUser.role !== null ? 
              (currentUser.role.name === 'super_admin' ? '超级管理员' : 
               currentUser.role.name === 'admin' ? '销售岗' : currentUser.role.name) : '-'
            }
          </Descriptions.Item>
          <Descriptions.Item label="状态">
            {typeof currentUser?.status === 'number' ? (currentUser.status === 1 ? '启用' : '禁用') : '-'}
          </Descriptions.Item>
          <Descriptions.Item label="最后登录时间">
            {typeof currentUser?.last_login_time === 'string' ? (
              new Date(currentUser.last_login_time).toLocaleString('zh-CN', {
                year: 'numeric',
                month: '2-digit',
                day: '2-digit',
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit'
              })
            ) : '-'}
          </Descriptions.Item>
        </Descriptions>
      </Card>

      <Card title="修改个人信息" style={{ marginBottom: 24 }}>
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
          style={{ maxWidth: 600 }}
        >
          <Form.Item
            name="realName"
            label="真实姓名"
            rules={[{ required: true, message: '请输入真实姓名' }]}
          >
            <Input prefix={<UserOutlined />} placeholder="请输入真实姓名" />
          </Form.Item>

          <Form.Item
            name="phone"
            label="联系电话"
            rules={[{ required: true, message: '请输入联系电话' }]}
          >
            <Input prefix={<PhoneOutlined />} placeholder="请输入联系电话" />
          </Form.Item>

          <Form.Item
            name="email"
            label="邮箱"
            rules={[{ type: 'email', message: '请输入正确的邮箱地址' }]}
          >
            <Input placeholder="请输入邮箱" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              保存修改
            </Button>
          </Form.Item>
        </Form>
      </Card>

      <Card title="修改密码">
        <Form
          form={passwordForm}
          layout="vertical"
          onFinish={handleChangePassword}
          style={{ maxWidth: 600 }}
        >
          <Form.Item
            name="oldPassword"
            label="旧密码"
            rules={[{ required: true, message: '请输入旧密码' }]}
          >
            <Password prefix={<LockOutlined />} placeholder="请输入旧密码" />
          </Form.Item>

          <Form.Item
            name="newPassword"
            label="新密码"
            rules={[
              { required: true, message: '请输入新密码' },
              { min: 6, message: '密码长度不能少于6位' },
            ]}
          >
            <Password prefix={<LockOutlined />} placeholder="请输入新密码" />
          </Form.Item>

          <Form.Item
            name="confirmPassword"
            label="确认新密码"
            dependencies={['newPassword']}
            rules={[
              { required: true, message: '请确认新密码' },
              ({ getFieldValue }) => ({
                validator(_, value) {
                  if (!value || getFieldValue('newPassword') === value) {
                    return Promise.resolve();
                  }
                  return Promise.reject(new Error('两次输入的密码不一致'));
                },
              }),
            ]}
          >
            <Password prefix={<LockOutlined />} placeholder="请确认新密码" />
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={passwordLoading}>
              修改密码
            </Button>
          </Form.Item>
        </Form>
      </Card>

      {/* 用户名修改模态框 */}
      <Modal
        title="修改用户名"
        open={usernameModalVisible}
        onCancel={() => setUsernameModalVisible(false)}
        footer={null}
      >
        <Form
          form={usernameForm}
          layout="vertical"
          onFinish={handleUpdateUsername}
          style={{ maxWidth: 400 }}
        >
          <Form.Item
            name="username"
            label="新用户名"
            rules={[
              { required: true, message: '请输入新用户名' },
              { min: 3, message: '用户名长度不能少于3位' },
              { max: 20, message: '用户名长度不能超过20位' },
            ]}
          >
            <Input prefix={<UserOutlined />} placeholder="请输入新用户名" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" loading={usernameLoading} style={{ marginRight: 8 }}>
              确认修改
            </Button>
            <Button onClick={() => setUsernameModalVisible(false)}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default PersonalInfo;
