import React, { createContext, useState, useContext, useEffect, ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { login, getCurrentUser as getMe } from '../services/api';

interface User {
  id: number;
  username: string;
  real_name: string;
  phone: string;
  role: {
    id: number;
    name: string;
    description: string;
  };
  status: number;
  department: string;
  permissions: Array<{
    id: string;
    title: string;
    key: string;
    checked: boolean;
    children: Array<{
      id: string;
      title: string;
      key: string;
      checked: boolean;
      children: unknown[];
    }>;
  }>;
  last_login_time: string | null;
}

interface AuthContextType {
  user: User | null;
  loading: boolean;
  isAuthenticated: boolean;
  login: (username: string, password: string) => Promise<void>;
  logout: () => void;
  refreshUser: () => Promise<void>;
}

const AuthContext = createContext<AuthContextType | undefined>(undefined);

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error('useAuth must be used within an AuthProvider');
  }
  return context;
};

interface AuthProviderProps {
  children: ReactNode;
}

export const AuthProvider: React.FC<AuthProviderProps> = ({ children }) => {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  const isAuthenticated = !!user;

  // 检查用户是否已登录
  useEffect(() => {
    const checkAuth = async () => {
      try {
        const response = await getMe();
        if (response.data && response.data.token) {
          setUser(response.data.user);
        } else {
          setUser(null);
        }
      } catch (error) {
        setUser(null);
      } finally {
        setLoading(false);
      }
    };

    checkAuth();
  }, []);

  // 登录
  const handleLogin = async (username: string, password: string) => {
    try {
      const timestamp = Date.now().toString();
      const response = await login({
        username,
        password,
        timestamp,
        autoLogin: false
      });

      if (response.token) {
        // 登录成功后获取用户信息
        const userResponse = await getMe();
        if (userResponse.data && userResponse.data.token) {
          setUser(userResponse.data.user);
          navigate('/dashboard');
        }
      }
    } catch (error) {
      console.error('登录失败:', error);
      throw error;
    }
  };

  // 退出登录
  const handleLogout = async () => {
    try {
      // 清除本地状态
      setUser(null);
      // 跳转到登录页
      navigate('/user/login');
    } catch (error) {
      console.error('退出登录失败:', error);
    }
  };

  // 刷新用户信息
  const refreshUser = async () => {
    try {
      const response = await getMe();
      if (response.data && response.data.token) {
        setUser(response.data.user);
      } else {
        setUser(null);
        navigate('/user/login');
      }
    } catch (error) {
      setUser(null);
      navigate('/user/login');
    }
  };

  const value = {
    user,
    loading,
    isAuthenticated,
    login: handleLogin,
    logout: handleLogout,
    refreshUser
  };

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};
