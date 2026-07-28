import React from 'react';
import { Tabs } from 'antd';
import { useModel } from '@umijs/max';
import SalesPurchase from '../Purchase';
import SalesConfigure from '../Configure';
import SalesEdit from '../Edit';
import SalesStatistics from '../Statistics';
import SalesAssignment from '../Assignment';

const SalesCenter: React.FC = () => {
  const { initialState } = useModel('@@initialState');
  const currentUser: any = initialState?.currentUser || {};
  const roleName = currentUser.role_name || currentUser.role?.name || '';
  const canAssignSales = roleName.includes('管理员') || roleName.includes('IT');
  const items = [
    {
      key: 'purchase',
      label: '套餐购买',
      children: <SalesPurchase />,
    },
    {
      key: 'configure',
      label: '套餐配置',
      children: <SalesConfigure />,
    },
    {
      key: 'plans',
      label: '检测计划',
      children: <SalesEdit />,
    },
    ...(canAssignSales
      ? [
          {
            key: 'assignment',
            label: '销售分配',
            children: <SalesAssignment />,
          },
        ]
      : []),
    {
      key: 'statistics',
      label: '销售统计',
      children: <SalesStatistics />,
    },
  ];

  return (
    <div>
      <h2 style={{ marginBottom: 16 }}>销售中心</h2>
      <Tabs defaultActiveKey="purchase" items={items} />
    </div>
  );
};

export default SalesCenter;
