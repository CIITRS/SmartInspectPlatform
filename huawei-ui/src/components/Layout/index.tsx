import React, { useState, useEffect } from 'react';
import { ProLayout } from '@ant-design/pro-components';
import { Link, useLocation, useNavigate } from 'react-router-dom';
import { HomeOutlined, UserOutlined, ExperimentOutlined, FileDoneOutlined, BookOutlined, SettingOutlined } from '@ant-design/icons';
import settings from '../../settings';
import { AvatarDropdown } from '../RightContent/AvatarDropdown';
import { useAuth } from '../../contexts/AuthContext';
import routes from '../../../config/routes';

interface LayoutProps {
  children: React.ReactNode;
}

const Layout: React.FC<LayoutProps> = ({ children }) => {
  const location = useLocation();
  const navigate = useNavigate();
  const { user, logout } = useAuth();
  
  // 标签页状态
  const [tabs, setTabs] = useState<Array<{
    key: string;
    tab: string;
  }>>([]);
  
  // 初始化标签页
  useEffect(() => {
    const currentPath = location.pathname;
    const currentRoute = findRouteByPath(routes, currentPath);
    
    if (currentRoute && currentRoute.name) {
      setTabs(prevTabs => {
        const existingTab = prevTabs.find(tab => tab.key === currentPath);
        if (existingTab) {
          return prevTabs;
        }
        return [...prevTabs, { key: currentPath, tab: currentRoute.name }];
      });
    }
  }, [location.pathname]);
  
  // 根据路径查找路由
  const findRouteByPath = (routes: any[], path: string): any => {
    for (const route of routes) {
      if (route.path === path) {
        return route;
      }
      if (route.routes) {
        const found: any = findRouteByPath(route.routes, path);
        if (found) {
          return found;
        }
      }
    }
    return null;
  };
  
  // 处理标签页变化
  const handleTabsChange = (key: string) => {
    navigate(key);
  };
  
  // 处理标签页关闭
  const handleTabsClose = (key: string) => {
    setTabs(prevTabs => {
      const newTabs = prevTabs.filter(tab => tab.key !== key);
      // 如果关闭的是当前标签页，导航到第一个标签页
      if (key === location.pathname && newTabs.length > 0) {
        const firstTab = newTabs[0];
        if (firstTab) {
          navigate(firstTab.key);
        }
      }
      return newTabs;
    });
  };

  // 图标映射
  const iconMap: Record<string, React.ElementType> = {
    'HomeOutlined': HomeOutlined,
    'user': UserOutlined,
    'experiment': ExperimentOutlined,
    'FileDoneOutlined': FileDoneOutlined,
    'BookOutlined': BookOutlined,
    'setting': SettingOutlined,
  };

  // 根据用户权限和路由配置生成菜单
  const generateMenuItems = () => {
    if (!user || !user.permissions) {
      return [];
    }

    // 获取用户权限ID列表
    const userPermissionIds = user.permissions
      .filter(permission => permission.checked)
      .map(permission => permission.id);

    // 递归生成菜单项
    const generateMenuFromRoutes = (routesConfig: any[]) => {
      return routesConfig
        .filter(route => {
          // 过滤掉隐藏在菜单中的路由
          if (route.hideInMenu) {
            return false;
          }
          // 过滤掉登录页面等不需要在菜单中显示的路由
          if (route.layout === false) {
            return false;
          }
          // 过滤掉没有名称的路由
          if (!route.name) {
            return false;
          }
          // 检查权限
          if (route.access) {
            // 简单的权限检查，实际项目中可能需要更复杂的逻辑
            return userPermissionIds.includes(route.access.replace('canAccess', '').toLowerCase());
          }
          return true;
        })
        .map(route => {
          const menuItem: any = {
            key: route.path,
            label: route.name,
          };

          // 添加图标
          if (route.icon) {
            const IconComponent = iconMap[route.icon] || HomeOutlined;
            menuItem.icon = IconComponent;
          }

          // 处理子路由
          if (route.routes && route.routes.length > 0) {
            const childMenuItems = generateMenuFromRoutes(route.routes);
            if (childMenuItems.length > 0) {
              menuItem.children = childMenuItems;
            }
          }

          return menuItem;
        })
        .filter(Boolean);
    };

    // 生成菜单项
    const menuItems = generateMenuFromRoutes(routes);

    return menuItems;
  };

  return (
    <ProLayout
      logo={settings.logo}
      title={settings.title}
      menuItemRender={(menuItemProps, defaultDom) => {
        return menuItemProps.key ? <Link to={menuItemProps.key}>{defaultDom}</Link> : defaultDom;
      }}
      layout={settings.layout}
      navTheme={settings.navTheme === 'dark' ? 'realDark' : settings.navTheme}
      contentWidth={settings.contentWidth}
      fixedHeader={settings.fixedHeader}
      fixSiderbar={settings.fixSiderbar}
      colorPrimary={settings.colorPrimary}
      rightContentRender={() => (
        <div style={{ display: 'flex', alignItems: 'center' }}>
          <span style={{ marginRight: 8 }}>{user?.real_name || '用户'}</span>
          <AvatarDropdown menu onLogout={logout} />
        </div>
      )}
      route={{
        path: location.pathname,
      }}
    >
      {children}
    </ProLayout>
  );
};

export default Layout;
