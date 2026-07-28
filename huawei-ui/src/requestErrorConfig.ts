import type { RequestOptions } from '@@/plugin-request/request';
import type { RequestConfig } from '@umijs/max';
import { history } from '@umijs/max';
import { message } from 'antd';

const AUTH_FAILURE_MESSAGE = '认证失败，账号已退出，请检查有无异地登录等情况';
const LOGIN_PATH = '/user/login';
let authFailureDeadline = 0;

const isUnauthorizedError = (error: any) => {
  const status = Number(error?.response?.status || error?.status);
  const code = Number(
    error?.info?.errorCode
    || error?.response?.data?.code
    || error?.response?.data?.errorCode
    || error?.data?.code,
  );
  return status === 401 || code === 401;
};

export const shouldSuppressSecondaryError = (error?: any) => (
  Boolean(error?.__rootErrorShown) || isUnauthorizedError(error) || Date.now() < authFailureDeadline
);

const markRootErrorShown = (error: any) => {
  if (error && typeof error === 'object') {
    error.__rootErrorShown = true;
  }
};

const handleAuthenticationFailure = () => {
  const now = Date.now();
  if (now < authFailureDeadline) return;
  authFailureDeadline = now + 10_000;
  localStorage.removeItem('token');
  message.destroy();
  message.error({
    key: 'authentication-failure',
    content: AUTH_FAILURE_MESSAGE,
    duration: 4,
  });
  if (history.location.pathname !== LOGIN_PATH) {
    history.replace(LOGIN_PATH);
  }
};

const getApiErrorMessage = (error: any, fallback = '请求失败') => {
  const responseData = error?.response?.data || error?.data || {};
  return responseData?.errorMessage || responseData?.message || error?.message || fallback;
};

// 错误处理方案： 错误类型
enum ErrorShowType {
  SILENT = 0,
  WARN_MESSAGE = 1,
  ERROR_MESSAGE = 2,
  NOTIFICATION = 3,
  REDIRECT = 9,
}
// 与后端约定的响应数据格式
interface ResponseStructure {
  success: boolean;
  data: any;
  errorCode?: number;
  errorMessage?: string;
  showType?: ErrorShowType;
}

/**
 * @name 错误处理
 * pro 自带的错误处理， 可以在这里做自己的改动
 * @doc https://umijs.org/docs/max/request#配置
 */
export const errorConfig: RequestConfig = {
  // 错误处理： umi@3 的错误处理方案。
  errorConfig: {
    // 错误抛出
    errorThrower: (res) => {
      const { success, data, errorCode, errorMessage, showType } =
        res as unknown as ResponseStructure;
      if (!success) {
        const error: any = new Error(errorMessage);
        error.name = 'BizError';
        error.info = { errorCode, errorMessage, showType, data };
        throw error; // 抛出自制的错误
      }
    },
    // 错误接收及处理
    errorHandler: (error: any, opts: any) => {
      if (opts?.skipErrorHandler) throw error;
      if (isUnauthorizedError(error)) {
        handleAuthenticationFailure();
        return;
      }
      if (shouldSuppressSecondaryError()) {
        return;
      }
      // 我们的 errorThrower 抛出的错误。
      if (error.name === 'BizError') {
        const errorInfo: ResponseStructure | undefined = error.info;
        if (errorInfo) {
          const { errorMessage } = errorInfo;
          switch (errorInfo.showType) {
            case ErrorShowType.SILENT:
              // do nothing
              break;
            case ErrorShowType.WARN_MESSAGE:
              markRootErrorShown(error);
              message.warning(errorMessage);
              break;
            case ErrorShowType.ERROR_MESSAGE:
              markRootErrorShown(error);
              message.error(errorMessage);
              break;
            case ErrorShowType.NOTIFICATION:
              markRootErrorShown(error);
              message.error(errorMessage);
              break;
            case ErrorShowType.REDIRECT:
              // TODO: redirect
              break;
            default:
              markRootErrorShown(error);
              message.error(errorMessage);
          }
        }
      } else if (error.response) {
        // Axios 的错误
        // 请求成功发出且服务器也响应了状态码，但状态代码超出了 2xx 的范围
        if (error.response.status === 400 || error.response.status === 422) {
          error.message = getApiErrorMessage(error);
        } else {
           markRootErrorShown(error);
           message.error(getApiErrorMessage(error));
         }
      } else if (error.request) {
        // 请求已经成功发起，但没有收到响应
        markRootErrorShown(error);
        message.error('无响应，请重试');
      } else {
        // 发送请求时出了点问题
        markRootErrorShown(error);
        message.error('请求错误，请重试');
      }
    },
  },

  // 请求拦截器
  requestInterceptors: [
    (config: RequestOptions) => {
      // 拦截请求配置，添加认证token
      const token = localStorage.getItem('token');
      if (token) {
        if (config.headers) {
          config.headers.Authorization = `Bearer ${token}`;
        } else {
          config.headers = {
            Authorization: `Bearer ${token}`
          };
        }
      }
      return config;
    },
  ],

  // 响应拦截器
  responseInterceptors: [
    (response) => {
      // 拦截响应数据，进行个性化处理
      // 移除重复的错误提示，由errorHandler统一处理
      return response;
    },
  ],
};
