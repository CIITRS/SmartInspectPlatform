import type { Settings as LayoutSettings } from '@ant-design/pro-components';
import { SettingDrawer, RouteContext } from '@ant-design/pro-components';
import type { RequestConfig, RunTimeLayoutConfig } from '@umijs/max';
import { history } from '@umijs/max';
import React, { useState, useEffect, useRef } from 'react';
import {
  AvatarDropdown,
  AvatarName,
  Footer,
} from '@/components';
import { currentUser as queryCurrentUser, switchAdminUser } from '@/services/ant-design-pro/api';
import defaultSettings from '../config/defaultSettings';
import { errorConfig } from './requestErrorConfig';
import { Dropdown, Tag } from 'antd';
import { SwapOutlined, TeamOutlined } from '@ant-design/icons';
import { flushSync } from 'react-dom';
import Tabs from 'antd/lib/tabs';
import routes from '../config/routes';

const TabPane = Tabs.TabPane;

const isDev = process.env.NODE_ENV === 'development';
const loginPath = '/user/login';
const CLOSE_TAB_EVENT = 'hw-close-tab';

const CurrentIdentityButton: React.FC<{
  user?: any;
  refreshUser?: () => Promise<AppCurrentUser | undefined>;
  setInitialState?: any;
}> = ({ user, refreshUser, setInitialState }) => {
  const roleNames = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names
    : user?.role?.name
      ? [user.role.name]
      : [];
  const switchIdentities = Array.isArray(user?.switch_identities) ? user.switch_identities : [];
  const otherIdentities = switchIdentities.filter((item: any) => !item.current);
  if (roleNames.length === 0) {
    return null;
  }
  const handleMenuClick = async ({ key }: { key: string }) => {
    if (!key.startsWith('switch:')) return;
    const userId = Number(key.replace('switch:', ''));
    if (!userId) return;
    await switchAdminUser(userId);
    const nextUser = await refreshUser?.();
    if (setInitialState && nextUser) {
      flushSync(() => {
        (setInitialState as any)((state: any) => ({ ...state, currentUser: nextUser }));
      });
    }
    history.replace('/');
    window.location.reload();
  };
  const menuItems = [
    {
      key: 'roles',
      type: 'group' as const,
      label: '当前角色',
      children: roleNames.map((name: string) => ({
        key: `role:${name}`,
        disabled: true,
        label: name,
      })),
    },
    ...(otherIdentities.length > 0
      ? [
        { type: 'divider' as const },
        {
          key: 'identities',
          type: 'group' as const,
          label: '同手机号其他身份',
          children: otherIdentities.map((item: any) => ({
            key: `switch:${item.user_id}`,
            icon: <SwapOutlined />,
            label: `${item.role_name || '账号'}：${item.real_name || item.username}`,
          })),
        },
      ]
      : []),
  ];
  return (
    <Dropdown menu={{ items: menuItems, onClick: handleMenuClick }} trigger={['hover', 'click']}>
      <span
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          gap: 6,
          height: 28,
          padding: '0 8px',
          borderRadius: 4,
          background: '#f0f5ff',
          color: '#1d39c4',
          fontSize: 13,
          cursor: 'pointer',
          whiteSpace: 'nowrap',
        }}
      >
        <TeamOutlined />
        {roleNames.map((name: string) => (
          <Tag key={name} color="blue" style={{ marginInlineEnd: 0 }}>
            {name}
          </Tag>
        ))}
        {otherIdentities.length > 0 && (
          <span style={{ color: '#595959' }}>同手机号 {otherIdentities.length}</span>
        )}
      </span>
    </Dropdown>
  );
};

// 定义用户类型
interface AppCurrentUser {
  id: number;
  username: string;
  real_name: string;
  employee_id?: string;
  phone: string;
  avatar?: string;
  role: {
    id: number;
    name: string;
    description: string;
  };
  roles?: Array<{
    id: number;
    name: string;
    description: string;
  }>;
  role_ids?: number[];
  role_names?: string[];
  switch_identities?: Array<{
    user_id: number;
    username: string;
    real_name: string;
    employee_id?: string;
    role_name: string;
    current: boolean;
  }>;
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

export async function getInitialState(): Promise<{
  settings?: Partial<LayoutSettings>;
  currentUser?: AppCurrentUser;
  loading?: boolean;
  fetchUserInfo?: () => Promise<AppCurrentUser | undefined>;
}> {
  const fetchUserInfo = async () => {
    try {
      const msg = await queryCurrentUser({
        skipErrorHandler: true,
      });
      return msg.data as unknown as AppCurrentUser;
    } catch (_error) {
      history.push(loginPath);
    }
    return undefined;
  };
  const { location } = history;
  if (
    ![loginPath, '/user/register', '/user/register-result'].includes(
      location.pathname,
    )
  ) {
    const currentUser = await fetchUserInfo();
    return {
      fetchUserInfo,
      currentUser,
      settings: defaultSettings as Partial<LayoutSettings>,
    };
  }
  return {
    fetchUserInfo,
    settings: defaultSettings as Partial<LayoutSettings>,
  };
}

// ProLayout 支持的api https://procomponents.ant.design/components/layout
export const layout: RunTimeLayoutConfig = ({
  initialState,
  setInitialState,
}) => {
  const matchPath = (routePath: string | undefined, pathname: string) => {
    if (!routePath) return false;
    const routeParts = routePath.split('/').filter(Boolean);
    const pathParts = pathname.split('/').filter(Boolean);
    if (routeParts.length !== pathParts.length) return false;
    return routeParts.every((part, index) => part.startsWith(':') || part === pathParts[index]);
  };

  const getPathParam = (pathname: string) => {
    const parts = pathname.split('/').filter(Boolean);
    return parts[parts.length - 1] || '';
  };

  const findRoute = (routeList: any[], path: string, parentRoute?: any): any => {
    for (const route of routeList) {
      if (route.path === path || matchPath(route.path, path)) {
        if (!route.name && parentRoute?.name) {
          return { ...route, name: parentRoute.name };
        }
        return route;
      }
      if (route.routes) {
        const found: any = findRoute(route.routes, path, route);
        if (found) return found;
      }
    }
    return null;
  };

  const getTabTitle = (pathname: string) => {
    if (pathname.startsWith('/report/batch/')) {
      return `报告生成-${getPathParam(pathname)}`;
    }
    if (pathname.startsWith('/result/batch/detail/')) {
      return `批次详情-${getPathParam(pathname)}`;
    }
    const route = findRoute(routes, pathname);
    return route?.name || pathname;
  };

  // 页签状态
  const [activeTab, setActiveTab] = useState(() => {
    const { pathname } = history.location;
    return pathname === loginPath ? '/' : pathname;
  });
  const [tabItems, setTabItems] = useState(() => {
    // 每次刷新都从首页开始，清除之前保存的标签页
    // 只保留首页标签
    const homeTab = {
      key: '/',
      pathname: '/',
      title: '首页',
      closable: false
    };
    
    // 检查当前路径是否是首页
    const { pathname } = history.location;
    
    // 如果不是首页且不是登录页，添加当前页标签
    if (pathname !== '/' && pathname !== loginPath) {
      const title = getTabTitle(pathname);
      
      const currentTab = {
        key: pathname,
        pathname,
        title,
        closable: true
      };
      
      return [homeTab, currentTab];
    }
    
    return [homeTab];
  });
  
  // 使用 ref 来持久化存储每个页签的内容，避免每次渲染时重置
  const tabContentsRef = useRef<Record<string, React.ReactNode>>({});
  
  // 切换 Tab
  const switchTab = (newActiveTab: string) => {
    setActiveTab(newActiveTab);
    history.push(newActiveTab);
  };
  
  // 移除 Tab
  const removeTab = (tabKey: string | React.MouseEvent | React.KeyboardEvent, action?: 'add' | 'remove') => {
    if (typeof tabKey !== 'string') return;
    // 首页不可关闭
    if (tabKey === '/') {
      return;
    }

    let newActiveTab = activeTab;
    let lastIndex = -1;

    tabItems.forEach((item: any, i: number) => {
      if (item.key === tabKey) {
        lastIndex = i - 1;
      }
    });

    const newPanes = tabItems.filter((item: any) => item.key !== tabKey);

    if (newPanes.length && newActiveTab === tabKey) {
      if (lastIndex >= 0) {
        newActiveTab = newPanes[lastIndex].key;
      } else {
        newActiveTab = newPanes[0].key;
      }
    }

    setTabItems(newPanes);
    delete tabContentsRef.current[tabKey];
    switchTab(newActiveTab);
  };
  
  // 检查是否是重定向路由（递归检查所有路由）
  const isRedirectRoute = (routes: any[], pathname: string) => {
    for (const route of routes) {
      if (route.redirect === pathname) {
        return true;
      }
      if (route.routes) {
        if (isRedirectRoute(route.routes, pathname)) {
          return true;
        }
      }
    }
    return false;
  };
  
  // 监听路由变化，添加新页签并激活
  useEffect(() => {
    const { location } = history;
    const { pathname } = location;
    
    // 跳过登录页
    if (pathname === loginPath) {
      return;
    }
    
    // 检查是否已存在相同路径的页签
    const existingTab = tabItems.find((item: any) => item.pathname === pathname);
    if (existingTab) {
      // 如果页签已存在，直接激活
      setActiveTab(pathname);
      const nextTitle = getTabTitle(pathname);
      if (existingTab.title !== nextTitle) {
        setTabItems(prevTabs => prevTabs.map((tab: any) => (
          tab.pathname === pathname ? { ...tab, title: nextTitle } : tab
        )));
      }
    } else {
      const title = getTabTitle(pathname);
      
      // 如果是重定向路由，不创建标签
      if (isRedirectRoute(routes, pathname)) {
        return;
      }
      
      // 添加新页签
      const newTab = {
        key: pathname,
        pathname,
        title,
        closable: pathname !== '/' 
      };
      
      setTabItems(prevTabs => {
        // 确保首页始终在第一位
        if (pathname === '/') {
          const filteredTabs = prevTabs.filter((tab: any) => tab.pathname !== '/');
          return [newTab, ...filteredTabs];
        }
        return [...prevTabs, newTab];
      });
      
      // 激活新标签
      setActiveTab(pathname);
    }
  }, [history.location.pathname, tabItems, routes]);

  useEffect(() => {
    const handleCloseTab = (event: Event) => {
      const pathname = (event as CustomEvent<{ pathname?: string }>).detail?.pathname || history.location.pathname;
      removeTab(pathname);
    };
    window.addEventListener(CLOSE_TAB_EVENT, handleCloseTab);
    return () => window.removeEventListener(CLOSE_TAB_EVENT, handleCloseTab);
  }, [tabItems, activeTab]);
  
  // 监听页签数据的变动，触发更新
  useEffect(() => {
    localStorage.setItem('tabPages', JSON.stringify(tabItems));
  }, [tabItems]);
  
  // 参考掘金文章实现 childrenRender
  const childrenRender = (children: React.ReactNode) => {
    const { location } = history;
    const { pathname } = location;
    
    // 保存当前页面的引用到 ref 中
    tabContentsRef.current[pathname] = children;
    
    return (
      <>
        <Tabs
          type="editable-card"
          hideAdd
          onChange={switchTab}
          activeKey={activeTab}
          onEdit={removeTab}
          style={{ marginBottom: 2 }}
          destroyInactiveTabPane={false}
          tabBarStyle={{
            padding: '0',
            margin: '0',
            fontSize: '12px',
            lineHeight: '28px',
            minHeight: '28px'
          }}
        >
          {tabItems.map((tabItem: any) => (
            <TabPane
              tab={tabItem.title}
              key={tabItem.key}
              closable={tabItem.closable}
              forceRender={true}
            >
              {/* 从 ref 中获取对应路径的内容 */}
              {tabContentsRef.current[tabItem.pathname]}
            </TabPane>
          ))}
        </Tabs>
        {isDev && (
          <SettingDrawer
            disableUrlParams
            enableDarkTheme
            settings={initialState?.settings}
            onSettingChange={(settings) => {
              setInitialState((preInitialState: any) => ({
                ...preInitialState,
                settings,
              }));
            }}
          />
        )}
      </>
    );
  };

  return {
    contentWidth: 'Fluid',
    actionsRender: () => [
    ],
    avatarProps: {
      render: () => {
        return (
          <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
            <AvatarDropdown menu>
              <div style={{ display: 'flex', alignItems: 'center', cursor: 'pointer', padding: '0 8px', borderRadius: '4px' }}>
                {initialState?.currentUser?.avatar && (
                  <img
                    src={initialState.currentUser.avatar}
                    alt={initialState.currentUser.real_name}
                    style={{ width: 24, height: 24, borderRadius: '50%', marginRight: 8 }}
                  />
                )}
                <AvatarName />
              </div>
            </AvatarDropdown>
            <CurrentIdentityButton
              user={initialState?.currentUser}
              refreshUser={initialState?.fetchUserInfo}
              setInitialState={setInitialState}
            />
          </div>
        );
      },
    },
    waterMarkProps: {
      content: initialState?.currentUser?.real_name,
    },
    footerRender: () => <Footer />,
    onPageChange: () => {
      const { location } = history;
      if (!initialState?.currentUser && location.pathname !== loginPath) {
        history.push(loginPath);
      }
    },
    bgLayoutImgList: [
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/D2LWSqNny4sAAAAAAAAAAAAAFl94AQBr',
        left: 85,
        bottom: 100,
        height: '303px',
      },
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/C2TWRpJpiC0AAAAAAAAAAAAAFl94AQBr',
        bottom: -68,
        right: -45,
        height: '303px',
      },
      {
        src: 'https://mdn.alipayobjects.com/yuyan_qk0oxh/afts/img/F6vSTbj8KpYAAAAAAAAAAAAAFl94AQBr',
        bottom: 0,
        left: 0,
        width: '331px',
      },
    ],
    menuHeaderRender: undefined,
    childrenRender: childrenRender,
    ...initialState?.settings,
  };
};



export const request: RequestConfig = {
  ...errorConfig,
};
