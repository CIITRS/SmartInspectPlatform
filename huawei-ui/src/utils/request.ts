import axios from 'axios';

// 扩展 axios 配置类型
declare module 'axios' {
  interface AxiosRequestConfig {
    resolveResponse?: <T>(response: import('axios').AxiosResponse<T>) => T;
  }
}

const request = axios.create({
  baseURL: process.env.VITE_API_BASE_URL || '',
  timeout: 10000,
  headers: {
    'Content-Type': 'application/json',
  },
});

// 请求拦截器
request.interceptors.request.use(
  (config) => {
    // 可以在这里添加 token 等认证信息
    const token = localStorage.getItem('token');
    if (token) {
      config.headers.Authorization = `Bearer ${token}`;
    }
    return config;
  },
  (error) => {
    return Promise.reject(error);
  }
);

// 响应拦截器
request.interceptors.response.use(
  (response) => {
    // 检查是否有自定义响应处理函数
    if (response.config?.resolveResponse) {
      return response.config.resolveResponse(response);
    }
    // 默认返回 response.data
    return response.data;
  },
  (error) => {
    // 可以在这里统一处理错误
    console.error('请求错误:', error);
    return Promise.reject(error);
  }
);

export default request;

