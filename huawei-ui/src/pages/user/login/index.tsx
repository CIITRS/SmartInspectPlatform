import { LockOutlined, MobileOutlined, UserOutlined } from '@ant-design/icons';
import { LoginForm, ProFormCaptcha, ProFormCheckbox, ProFormText } from '@ant-design/pro-components';
import { Helmet, useModel } from '@umijs/max';
import { Alert, App, Form, Input, Modal, Tabs } from 'antd';
import { createStyles } from 'antd-style';
import React, { useState, useEffect } from 'react';
import { flushSync } from 'react-dom';
import CryptoJS from 'crypto-js';
import JSEncrypt from 'jsencrypt';
import { Footer } from '@/components';
import { adminSmsLogin, bindAdminPhone, login, sendAdminBindPhoneCode, sendAdminSmsCode } from '@/services/ant-design-pro/api';
import Settings from '../../../../config/defaultSettings';

// 定义登录相关类型
interface LoginResult {
  status?: string;
  type?: string;
  currentAuthority?: string;
  token?: string;
}

interface LoginParams {
  username: string;
  password: string;
  autoLogin?: boolean;
  type?: string;
  mobile?: string;
  captcha?: string;
}

const useStyles = createStyles(({ token: _ }) => {
  return {
    container: {
      display: 'flex',
      flexDirection: 'column',
      height: '100vh',
      overflow: 'auto',
      backgroundImage:
        "url('https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/V-_oS6r-i7wAAAAAAAAAAAAAFl94AQBr')",
      backgroundSize: '100% 100%',
    },
    // 调整LoginForm子标题样式为h3大小
    '@global': {
      '.ant-pro-login-form-sub-title': {
        fontSize: '20px', // h3大小
        fontWeight: '500',
      },
    },
  };
});

const LoginMessage: React.FC<{
  content: string;
}> = ({ content }) => {
  return (
    <Alert
      style={{
        marginBottom: 24,
      }}
      message={content}
      type="error"
      showIcon
    />
  );
};

const Login: React.FC = () => {
  const [userLoginState, setUserLoginState] = useState<LoginResult>({});
  const [type, setType] = useState<string>('account');
  const [_publicKey, setPublicKey] = useState<string>('');
  const [bindModalOpen, setBindModalOpen] = useState(false);
  const [bindToken, setBindToken] = useState('');
  const [loginForm] = Form.useForm();
  const [bindForm] = Form.useForm();
  const { initialState, setInitialState } = useModel('@@initialState');
  const { styles } = useStyles();
  const { message } = App.useApp();

  // 获取公钥
  const fetchPublicKey = async () => {
    try {
      const response = await fetch('/api/auth/publicKey');
      const result = await response.json();
      if (result.code === 200) {
        setPublicKey(result.data.publicKey);
      }
    } catch (error) {
      console.error('获取公钥失败:', error);
    }
  };

  // 组件挂载时获取公钥
  useEffect(() => {
    fetchPublicKey();
  }, []);

  const fetchUserInfo = async () => {
    const userInfo = await initialState?.fetchUserInfo?.();
    if (userInfo) {
      flushSync(() => {
        setInitialState((s) => ({
          ...s,
          currentUser: userInfo,
        }));
      });
    }
  };

  // MD5加密函数
  const md5Hash = (text: string): string => {
    return CryptoJS.MD5(text).toString();
  };

  // SHA-256加密函数
  const sha256Hash = (text: string): string => {
    return CryptoJS.SHA256(text).toString();
  };

  // 加密函数：使用时间戳和密码生成加密密码
	const encryptPassword = (password: string, timestamp: string): string => {
		// 1. 计算密码的MD5
		const passwordMd5 = md5Hash(password);
		// 2. 将时间戳和密码MD5拼接
		const combined = passwordMd5 + timestamp;
		// 3. 计算拼接后的SHA-256作为最终加密密码
		const encryptedPassword = sha256Hash(combined);
		return encryptedPassword;
	};

  // RSA加密函数
  const rsaEncrypt = (password: string, publicKey: string): string => {
    try {
      const encrypt = new JSEncrypt();
      encrypt.setPublicKey(publicKey);
      const encrypted = encrypt.encrypt(password);
      return encrypted || password;
    } catch (error) {
      console.error('RSA加密失败:', error);
      // 加密失败时返回原始密码
      return password;
    }
  };

  const handleSubmit = async (values: LoginParams) => {
    try {
      if (type === 'mobile') {
        const msg = await adminSmsLogin(
          {
            phone: values.mobile || '',
            code: values.captcha || '',
          },
          { skipErrorHandler: true },
        );
        if ((msg as any).success || (msg as any).code === 200) {
          message.success('登录成功！');
          await fetchUserInfo();
          const urlParams = new URL(window.location.href).searchParams;
          window.location.href = urlParams.get('redirect') || '/';
          return;
        }
        setUserLoginState({
          ...(msg as any),
          status: 'error',
          type: 'mobile',
        });
        return;
      }

      // 获取当前时间戳
      const timestamp = Date.now().toString();
      
      let password = values.password || '';
      let isRsaEncrypted = false;
      
      // 如果获取到公钥，使用RSA加密密码
      if (_publicKey) {
        password = rsaEncrypt(password, _publicKey);
        isRsaEncrypted = true;
      } else {
        // 否则使用时间戳加密
        password = encryptPassword(password, timestamp);
      }
      
      const loginParams = {
        ...values,
        type,
        password: password,
        autoLogin: values.autoLogin || false
      };
      
      // 添加 timestamp 和 isRsaEncrypted 属性
      (loginParams as any).timestamp = timestamp;
      (loginParams as any).isRsaEncrypted = isRsaEncrypted;
      
      const msg = await login(loginParams, { skipErrorHandler: true });
      if ((msg as any).success || (msg as any).code === 200) {
        if ((msg as any).data?.need_bind_phone) {
          setBindToken((msg as any).data.bind_token || '');
          setBindModalOpen(true);
          message.info('首次密码登录需要绑定手机号');
          return;
        }
        // 存储token到localStorage
        const token = (msg as any).token || (msg as any).data?.token;
        if (token) {
          localStorage.setItem('token', token);
        }
        message.success('登录成功！');
        await fetchUserInfo();
        const urlParams = new URL(window.location.href).searchParams;
        window.location.href = urlParams.get('redirect') || '/';
        return;
      }
      console.log(msg);
      // 如果后端返回非200，尝试显示后端返回的错误信息
      const typedMsg = msg as any;
      if (typedMsg.status === 'error' && typedMsg.message) {
         message.error(typedMsg.message);
      }
      setUserLoginState(msg);
    } catch (error: any) {
      console.log(error);
      const errorMsg = error.response?.data?.message || error.data?.message || '登录失败，请重试！';
      message.error(errorMsg);
      setUserLoginState({
        status: 'error',
        type,
        ...(error.response?.data || error.data || {}),
      });
    }
  };
  const { status, type: loginType } = userLoginState;

  return (
    <div className={styles.container}>
      <Helmet>
        <title>
          登录页
          {Settings.title && ` - ${Settings.title}`}
        </title>
      </Helmet>
      <div
        style={{
          flex: '1',
          padding: '32px 0',
        }}
      >
        <LoginForm
          form={loginForm}
          contentStyle={{
            minWidth: 280,
            maxWidth: '75vw',
          }}
          logo={<img alt="logo" src="/logo.svg" />}
          title="华微智检"
          subTitle="检测报告一体化管理系统"
          initialValues={{
            autoLogin: true,
          }}
          onFinish={async (values) => {
            await handleSubmit(values as LoginParams);
          }}
        >
          <Tabs
            activeKey={type}
            onChange={setType}
            centered
            items={[
              {
                key: 'account',
                label: '账户密码登录',
              },
              {
                key: 'mobile',
                label: '手机号登录',
              },
            ]}
          />

          {status === 'error' && loginType === 'account' && (
            <LoginMessage content={(userLoginState as any).message || "账户或密码错误"} />
          )}
          {type === 'account' && (
            <>
              <ProFormText
                name="username"
                fieldProps={{
                  size: 'large',
                  prefix: <UserOutlined />,
                }}
                placeholder="用户名"
                rules={[
                  {
                    required: true,
                    message: '请输入用户名！',
                  },
                ]}
              />
              <ProFormText.Password
                name="password"
                fieldProps={{
                  size: 'large',
                  prefix: <LockOutlined />,
                }}
                placeholder="密码"
                rules={[
                  {
                    required: true,
                    message: '请输入密码！',
                  },
                ]}
              />
            </>
          )}

          {status === 'error' && loginType === 'mobile' && (
            <LoginMessage content={(userLoginState as any).message || "验证码错误"} />
          )}
          {type === 'mobile' && (
            <>
              <ProFormText
                fieldProps={{
                  size: 'large',
                  prefix: <MobileOutlined />,
                }}
                name="mobile"
                placeholder="手机号"
                rules={[
                  {
                    required: true,
                    message: '请输入手机号！',
                  },
                  {
                    pattern: /^1\d{10}$/,
                    message: '手机号格式错误！',
                  },
                ]}
              />
              <ProFormCaptcha
                fieldProps={{
                  size: 'large',
                  prefix: <LockOutlined />,
                }}
                captchaProps={{
                  size: 'large',
                }}
                placeholder="请输入验证码"
                captchaTextRender={(timing, count) => {
                  if (timing) {
                    return `${count} 秒后重新获取`;
                  }
                  return '获取验证码';
                }}
                name="captcha"
                rules={[
                  {
                    required: true,
                    message: '请输入验证码！',
                  },
                ]}
                onGetCaptcha={async () => {
                  const phone = String(loginForm.getFieldValue('mobile') || '').trim();
                  if (!/^1\d{10}$/.test(phone)) {
                    message.error('请先输入正确手机号');
                    throw new Error('invalid phone');
                  }
                  await sendAdminSmsCode(phone, { skipErrorHandler: true });
                  message.success('验证码已发送');
                }}
              />
            </>
          )}
          <div
            style={{
              marginBottom: 24,
            }}
          >
            <ProFormCheckbox noStyle name="autoLogin">
              自动登录
            </ProFormCheckbox>
            <a
              style={{
                float: 'right',
              }}
            >
              忘记密码
            </a>
          </div>
        </LoginForm>
      </div>
      <Modal
        title="绑定手机号"
        open={bindModalOpen}
        onCancel={() => setBindModalOpen(false)}
        onOk={() => bindForm.submit()}
        okText="绑定并登录"
        cancelText="取消"
      >
        <Form
          form={bindForm}
          layout="vertical"
          onFinish={async (values) => {
            try {
              const result = await bindAdminPhone(
                {
                  phone: values.phone,
                  code: values.code,
                  bindToken,
                },
                { skipErrorHandler: true },
              );
              if ((result as any).success || (result as any).code === 200) {
                message.success('绑定成功，正在登录');
                await fetchUserInfo();
                const urlParams = new URL(window.location.href).searchParams;
                window.location.href = urlParams.get('redirect') || '/';
              }
            } catch (error: any) {
              message.error(error.response?.data?.message || error.data?.message || '绑定失败');
            }
          }}
        >
          <Form.Item
            name="phone"
            label="手机号"
            rules={[
              { required: true, message: '请输入手机号' },
              { pattern: /^1\d{10}$/, message: '手机号格式错误' },
            ]}
          >
            <Input placeholder="请输入要绑定的手机号" />
          </Form.Item>
          <Form.Item
            name="code"
            label="验证码"
            rules={[{ required: true, message: '请输入验证码' }]}
          >
            <Input.Search
              placeholder="请输入验证码"
              enterButton="发送验证码"
              onSearch={async () => {
                const phone = bindForm.getFieldValue('phone');
                if (!/^1\d{10}$/.test(phone || '')) {
                  message.error('请先输入正确手机号');
                  return;
                }
                await sendAdminBindPhoneCode({ phone, bindToken }, { skipErrorHandler: true });
                message.success('验证码已发送');
              }}
            />
          </Form.Item>
        </Form>
      </Modal>
      <Footer />
    </div>
  );
};

export default Login;
