/**
 * @name umi 的路由配置
 * @description 只支持 path,component,routes,redirect,wrappers,name,icon 的配置
 * @param path  path 只支持两种占位符配置，第一种是动态参数 :id 的形式，第二种是 * 通配符，通配符只能出现路由字符串的最后。
 * @param component 配置 location 和 path 匹配后用于渲染的 React 组件路径。可以是绝对路径，也可以是相对路径，如果是相对路径，会从 src/pages 开始找起。
 * @param routes 配置子路由，通常在需要为多个路径增加 layout 组件时使用。
 * @param redirect 配置路由跳转
 * @param wrappers 配置路由组件的包装组件，通过包装组件可以为当前的路由组件组合进更多的功能。 比如，可以用于路由级别的权限校验
 * @param name 配置路由的标题，默认读取国际化文件 menu.ts 中 menu.xxxx 的值，如配置 name 为 login，则读取 menu.ts 中 menu.login 的取值作为标题
 * @param icon 配置路由的图标，取值参考 https://ant.design/components/icon-cn， 注意去除风格后缀和大小写，如想要配置图标为 <StepBackwardOutlined /> 则取值应为 stepBackward 或 StepBackward，如想要配置图标为 <UserOutlined /> 则取值应为 user 或者 User
 * @doc https://umijs.org/docs/guides/routes
 */
export default [
  {
    path: '/user',
    layout: false,
    routes: [
      {
        name: '登录',
        path: '/user/login',
        component: './user/login',
      },
    ],
  },
  {
    path: '/account',
    name: '个人中心',
    icon: 'user',
    hideInMenu: true,
    routes: [
      {
        path: '/account/center',
        name: '个人信息',
        component: './account/center',
      },
    ],
  },
  {
    path: '/',
    name: '首页',
    icon: 'HomeOutlined',
    access: 'canAccessDashboard',
    component: './Dashboard',
  },
  {
    path: '/patient',
    name: '患者中心',
    icon: 'user',
    access: 'canAccessPatient',
    routes: [
      {
        path: '/patient',
        redirect: '/patient/list',
      },
      {
        path: '/patient/list',
        component: './Patient/List',
      },
      {
        path: '/patient/create',
        component: './Patient/Create',
        hideInMenu: true,
      },
      {
        path: '/patient/perfect',
        component: './Patient/Perfect',
        hideInMenu: true,
      },
      {
        path: '/patient/edit/:patientCode',
        component: './Patient/Edit',
        hideInMenu: true,
      },
      {
        path: '/patient/complete/:patientCode',
        component: './Patient/Complete',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/sample',
    name: '样本中心',
    icon: 'experiment',
    access: 'canAccessSample',
    routes: [
      {
        path: '/sample',
        redirect: '/sample/list',
      },
      {
        path: '/sample/list',
        component: './Sample/List',
      },
      {
        path: '/sample/create',
        component: './Sample/Create',
        hideInMenu: true,
      },
      {
        path: '/sample/receive',
        component: './Sample/Receive',
        hideInMenu: true,
      },
      {
        path: '/sample/detail/:id',
        component: './Sample/Detail',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/result',
    name: '结果中心',
    icon: 'FileDoneOutlined',
    access: 'canAccessResult',
    routes: [
      {
        path: '/result',
        redirect: '/result/center',
      },
      {
        path: '/result/center',
        name: '结果中心',
        component: './Result/Center',
        hideInMenu: true,
      },
      {
        path: '/result/detail/:id',
        component: './Result/Detail',
        hideInMenu: true,
      },
      {
        path: '/result/batch/detail/:id',
        component: './Result/Batch/Detail',
        hideInMenu: true,
      },
      {
        path: '/result/sample-query',
        name: '样本查询',
        component: './Result/SampleQuery',
        hideInMenu: true,
      },
      {
        path: '/result/gene-match',
        name: '基因匹配',
        component: './Result/GeneMatch',
        hideInMenu: true,
      },
      {
        path: '/result/gene-match/:batchId',
        component: './Result/GeneMatch',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/report',
    name: '报告中心',
    icon: 'BookOutlined',
    access: 'canAccessReport',
    routes: [
      {
        path: '/report',
        component: './Report',
      },
      {
        path: '/report/create/:id',
        component: './Report/Create',
        hideInMenu: true,
      },
      {
        path: '/report/view/:id',
        component: './Report/View',
        hideInMenu: true,
      },
      {
        path: '/report/batch/:batchCode',
        component: './Report/Batch',
        hideInMenu: true,
      },
      {
        path: '/report/review',
        component: './Report/Review',
        hideInMenu: true,
      },
      {
        path: '/report/templates',
        component: './Report/Templates',
        hideInMenu: true,
      },
      {
        path: '/report/preview/:id',
        component: './Report/Preview',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/sales',
    name: '销售中心',
    icon: 'shopping',
    access: 'canAccessSales',
    routes: [
      {
        path: '/sales',
        redirect: '/sales/patient-packages',
      },
      {
        path: '/sales/patient-packages',
        name: '患者套餐管理',
        component: './Sales/Edit',
      },
      {
        path: '/sales/configure',
        name: '套餐配置',
        component: './Sales/Configure',
        access: 'canAdmin',
      },
      {
        path: '/sales/assignment',
        name: '销售分配',
        component: './Sales/Assignment',
        access: 'canAdmin',
      },
      {
        path: '/sales/statistics',
        name: '销售统计',
        component: './Sales/Statistics',
      },
    ],
  },
  {
    path: '/appointment',
    name: '物流中心',
    icon: 'car',
    access: 'canAccessAppointment',
    component: './Appointment/Manage',
  },
  {
    path: '/users',
    name: '用户管理',
    icon: 'user',
    access: 'canAccessUsers',
    routes: [
      {
        path: '/users',
        component: './System/User',
      },
    ],
  },
  {
    path: '/announcement',
    name: '公告管理',
    icon: 'notification',
    access: 'canAccessAnnouncement',
    routes: [
      {
        path: '/announcement',
        component: './System/Announcement',
      },
      {
        path: '/announcement/create',
        name: '新增公告',
        component: './System/Announcement/Create',
        hideInMenu: true,
      },
      {
        path: '/announcement/edit/:id',
        name: '编辑公告',
        component: './System/Announcement/Edit',
        hideInMenu: true,
      },
      {
        path: '/announcement/:id',
        name: '公告详情',
        component: './System/Announcement/Detail',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/announcements',
    name: '公告查看',
    icon: 'notification',
    hideInMenu: true,
    routes: [
      {
        path: '/announcements',
        component: './AnnouncementView',
      },
      {
        path: '/announcements/:id',
        name: '公告详情',
        component: './System/Announcement/Detail',
        hideInMenu: true,
      },
    ],
  },
  {
    path: '/system',
    name: '系统设置',
    icon: 'setting',
    access: 'canAccessSystem',
    routes: [
      {
        path: '/system/detectionType',
        name: '检测管理',
        component: './System/CancerType',
      },
      {
        path: '/system/threshold',
        name: '阈值设置',
        component: './System/Threshold',
      },
      {
        path: '/system/help-center',
        name: '帮助中心',
        component: './System/HelpCenter',
      },
      {
        path: '/system/report-settings',
        name: '报告设置',
        component: './System/ReportSettings',
      },
      {
        path: '/system/settings',
        name: '系统配置',
        component: './System/Settings',
      },
      {
        path: '/system/about',
        name: '关于',
        component: './System/About',
      },
    ],
  },
  {
    path: '/ai-management',
    name: 'AI管理',
    icon: 'robot',
    access: 'canAccessAIManagement',
    routes: [
      {
        path: '/ai-management',
        component: './System/AIManagement',
      },
    ],
  },
  {
    path: '*',
    layout: false,
    component: './404',
  },
];
