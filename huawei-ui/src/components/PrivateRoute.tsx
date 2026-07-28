import React from 'react';
import { Navigate, Outlet } from 'react-router-dom';
import { useAuth } from '../contexts/AuthContext';

interface PrivateRouteProps {
  children?: React.ReactNode;
}

export const PrivateRoute: React.FC<PrivateRouteProps> = ({ children }) => {
  const { isAuthenticated, loading } = useAuth();

  if (loading) {
    // 可以返回一个加载组件
    return <div>加载中...</div>;
  }

  if (!isAuthenticated) {
    return <Navigate to="/user/login" replace />;
  }

  return children ? children : <Outlet />;
};
