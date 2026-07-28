import React from 'react';

const Footer: React.FC = () => {
  return (
    <div style={{ textAlign: 'center', padding: '24px 0', background: 'none' }}>
      <div style={{ marginBottom: '16px', fontSize: '15px', fontWeight: 500, color: 'rgba(0, 0, 0, 0.85)' }}>
        © 中创智科（上海）科技研究有限公司
      </div>
      <div style={{ fontSize: '15px', color: 'rgba(0, 0, 0, 0.45)', lineHeight: '1.8' }}>
        <a href="https://beian.miit.gov.cn/" target="_blank" rel="noopener noreferrer" style={{ color: 'rgba(0, 0, 0, 0.45)', textDecoration: 'none', margin: '0 8px' }}>
          沪ICP备2024104322号-1
        </a>
        <span style={{ margin: '0 8px', color: 'rgba(0, 0, 0, 0.25)' }}>|</span>
        <a href="http://www.beian.gov.cn/portal/registerSystemInfo" target="_blank" rel="noopener noreferrer" style={{ color: 'rgba(0, 0, 0, 0.45)', textDecoration: 'none', margin: '0 8px' }}>
          沪公网安备31011502402616号
        </a>
        <span style={{ margin: '0 8px', color: 'rgba(0, 0, 0, 0.25)' }}>|</span>
        <a href="https://dxzhgl.miit.gov.cn/dxxzsp/xkz/xkzgl/resource/qiyesearch.jsp?num=B1.B2-20252621&type=xuke" target="_blank" rel="noopener noreferrer" style={{ color: 'rgba(0, 0, 0, 0.45)', textDecoration: 'none', margin: '0 8px' }}>
          增值电信许可证：B1.B2-20252621
        </a>
      </div>
    </div>
  );
};

export default Footer;
