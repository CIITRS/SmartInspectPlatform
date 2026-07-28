export interface Settings {
  navTheme: 'light' | 'dark';
  colorPrimary: string;
  layout: 'mix' | 'top' | 'side';
  contentWidth: 'Fluid' | 'Fixed';
  fixedHeader: boolean;
  fixSiderbar: boolean;
  colorWeak: boolean;
  title: string;
  pwa: boolean;
  logo: string;
  iconfontUrl: string;
}

const settings: Settings = {
  navTheme: 'light',
  colorPrimary: '#1890ff',
  layout: 'mix',
  contentWidth: 'Fluid',
  fixedHeader: false,
  fixSiderbar: true,
  colorWeak: false,
  title: '华微智检',
  pwa: true,
  logo: '/logo.svg',
  iconfontUrl: '',
};

export default settings;
