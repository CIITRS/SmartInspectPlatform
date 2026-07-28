/* eslint-disable */
import { request } from '@umijs/max';

// 定义本地类型
interface CurrentUser {
  name: string;
  avatar: string;
  userid: string;
  email: string;
  signature: string;
  title: string;
  group: string;
  tags: {
    key: string;
    label: string;
  }[];
  notifyCount: number;
  unreadCount: number;
  country: string;
  geographic: {
    province: {
      label: string;
      key: string;
    };
    city: {
      label: string;
      key: string;
    };
  };
  address: string;
  phone: string;
}

interface LoginParams {
  username: string;
  password: string;
  autoLogin?: boolean;
  type?: string;
  mobile?: string;
  captcha?: string;
}

interface LoginResult {
  status?: string;
  type?: string;
  currentAuthority?: string;
  token?: string;
}

interface NoticeIconList {
  data: {
    list: {
      id: string;
      avatar: string;
      title: string;
      datetime: string;
      type: string;
      read: boolean;
      description: string;
      extra: string;
      status: string;
    }[];
  };
}

interface RuleList {
  data: {
    key: number;
    disabled: boolean;
    href: string;
    avatar: string;
    name: string;
    owner: string;
    desc: string;
    callNo: number;
    status: string;
    updatedAt: string;
    createdAt: string;
    progress: number;
  }[];
  total: number;
  success: boolean;
  pageSize: number;
  current: number;
}

interface RuleListItem {
  key: number;
  disabled: boolean;
  href: string;
  avatar: string;
  name: string;
  owner: string;
  desc: string;
  callNo: number;
  status: string;
  updatedAt: string;
  createdAt: string;
  progress: number;
}

/** 获取当前的用户 GET /api/auth/me */
export async function currentUser(options?: { [key: string]: any }) {
  return request<{
    data: CurrentUser;
  }>('/api/auth/me', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 退出登录接口 POST /api/auth/logout */
export async function outLogin(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/auth/logout', {
    method: 'POST',
    ...(options || {}),
  });
}

export async function switchAdminUser(userId: number, options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/auth/switch-user', {
    method: 'POST',
    data: { user_id: userId },
    ...(options || {}),
  });
}

/** 登录接口 POST /api/auth/login */
export async function login(body: LoginParams, options?: { [key: string]: any }) {
  return request<LoginResult>('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

/** 后台发送短信验证码 POST /api/auth/sms/send */
export async function sendAdminSmsCode(phone: string, options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/auth/sms/send', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      phone,
      purpose: 'admin_login',
      client: 'admin',
    },
    ...(options || {}),
  });
}

/** 后台短信验证码登录 POST /api/auth/sms/login */
export async function adminSmsLogin(body: { phone: string; code: string }, options?: { [key: string]: any }) {
  return request<LoginResult>('/api/auth/sms/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      phone: body.phone,
      code: body.code,
      purpose: 'admin_login',
      client: 'admin',
    },
    ...(options || {}),
  });
}

/** 发送后台绑定手机号验证码 POST /api/auth/sms/send */
export async function sendAdminBindPhoneCode(body: { phone: string; bindToken: string }, options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/auth/sms/send', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      phone: body.phone,
      bind_token: body.bindToken,
      purpose: 'admin_bind_phone',
      client: 'admin',
    },
    ...(options || {}),
  });
}

/** 后台绑定手机号并登录 POST /api/auth/bind-phone */
export async function bindAdminPhone(body: { phone: string; code: string; bindToken: string }, options?: { [key: string]: any }) {
  return request<LoginResult>('/api/auth/bind-phone', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      phone: body.phone,
      code: body.code,
      bind_token: body.bindToken,
      client: 'admin',
    },
    ...(options || {}),
  });
}

/** 此处后端没有提供注释 GET /api/notices */
export async function getNotices(options?: { [key: string]: any }) {
  return request<NoticeIconList>('/api/notices', {
    method: 'GET',
    ...(options || {}),
  });
}

/** 获取规则列表 GET /api/rule */
export async function rule(
  params: {
    // query
    /** 当前的页码 */
    current?: number;
    /** 页面的容量 */
    pageSize?: number;
  },
  options?: { [key: string]: any },
) {
  return request<RuleList>('/api/rule', {
    method: 'GET',
    params: {
      ...params,
    },
    ...(options || {}),
  });
}

/** 更新规则 PUT /api/rule */
export async function updateRule(options?: { [key: string]: any }) {
  return request<RuleListItem>('/api/rule', {
    method: 'POST',
    data: {
      method: 'update',
      ...(options || {}),
    },
  });
}

/** 新建规则 POST /api/rule */
export async function addRule(options?: { [key: string]: any }) {
  return request<RuleListItem>('/api/rule', {
    method: 'POST',
    data: {
      method: 'post',
      ...(options || {}),
    },
  });
}

/** 删除规则 DELETE /api/rule */
export async function removeRule(options?: { [key: string]: any }) {
  return request<Record<string, any>>('/api/rule', {
    method: 'POST',
    data: {
      method: 'delete',
      ...(options || {}),
    },
  });
}
