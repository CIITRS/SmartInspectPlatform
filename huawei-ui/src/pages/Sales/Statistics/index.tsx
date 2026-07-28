import React, { useState, useEffect } from 'react';
import {
  Card,
  Table,
  message,
  DatePicker,
  Select,
  Button,
  Space,
  Row,
  Col,
  Statistic,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { getSalesStatistics, getUserInfo } from '@/services/api';
import dayjs from 'dayjs';

const { Option } = Select;
const { RangePicker } = DatePicker;

const StatisticsPage: React.FC = () => {
  const [statistics, setStatistics] = useState<any[]>([]);
  const [userStatistics, setUserStatistics] = useState<any>(null);
  const [allStatistics, setAllStatistics] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [userInfo, setUserInfo] = useState<any>(null);
  const [dateRange, setDateRange] = useState<[dayjs.Dayjs, dayjs.Dayjs] | null>(null);

  // 获取用户信息
  useEffect(() => {
    fetchUserInfo();
  }, []);

  // 获取销售统计
  useEffect(() => {
    if (userInfo) {
      fetchSalesStatistics();
    }
  }, [userInfo, dateRange]);

  const fetchUserInfo = async () => {
    try {
      const response = await getUserInfo();
      if (response.data) {
        setUserInfo(response.data);
      } else {
        message.error('获取用户信息失败');
      }
    } catch (error) {
      message.error('网络错误');
    }
  };

  const fetchSalesStatistics = async () => {
    try {
      setLoading(true);
      const params: any = {};
      if (dateRange) {
        params.start_date = dateRange[0].format('YYYY-MM-DD');
        params.end_date = dateRange[1].format('YYYY-MM-DD');
      }
      const response = await getSalesStatistics(params);
      if (response.data) {
        setStatistics(response.data.details || []);
        setUserStatistics(response.data.user_statistics || null);
        setAllStatistics(response.data.all_statistics || null);
      } else {
        message.error('获取销售统计失败');
      }
    } catch (error) {
      message.error('网络错误');
    } finally {
      setLoading(false);
    }
  };

  // 处理日期范围变化
  const handleDateRangeChange = (dates: any, dateStrings: [string, string]) => {
    setDateRange(dates);
  };

  // 处理刷新
  const handleRefresh = () => {
    fetchSalesStatistics();
  };

  // 销售统计列
  const columns = [
    {
      title: '销售人',
      dataIndex: 'salesperson_name',
      key: 'salesperson_name',
    },
    {
      title: '套餐数',
      dataIndex: 'package_count',
      key: 'package_count',
    },
    {
      title: '总金额',
      dataIndex: 'total_amount',
      key: 'total_amount',
      render: (text: number) => `¥${text.toFixed(2)}`,
    },
    {
      title: '销售日期',
      dataIndex: 'date',
      key: 'date',
    },
  ];

  return (
    <div className="sales-statistics-page">
      <Card title="销售统计" className="mb-4">
        <Row gutter={16} className="mb-4">
          <Col span={12}>
            <Space>
              <RangePicker
                onChange={handleDateRangeChange}
                placeholder={['开始日期', '结束日期']}
              />
              <Button
                type="primary"
                icon={<ReloadOutlined />}
                onClick={handleRefresh}
                loading={loading}
              >
                刷新
              </Button>
            </Space>
          </Col>
        </Row>

        {/* 个人销售统计 */}
        {userStatistics && (
          <Card title="个人销售统计" className="mb-4">
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="销售套餐数"
                  value={userStatistics.package_count}
                  prefix="📦"
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="销售总金额"
                  value={userStatistics.total_amount}
                  prefix="¥"
                  precision={2}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="销售订单数"
                  value={userStatistics.order_count}
                  prefix="📋"
                />
              </Col>
            </Row>
          </Card>
        )}

        {/* 管理员看到的总统计 */}
        {allStatistics && userInfo?.role === 'admin' && (
          <Card title="总销售统计" className="mb-4">
            <Row gutter={16}>
              <Col span={8}>
                <Statistic
                  title="总销售套餐数"
                  value={allStatistics.total_package_count}
                  prefix="📦"
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="总销售金额"
                  value={allStatistics.total_amount}
                  prefix="¥"
                  precision={2}
                />
              </Col>
              <Col span={8}>
                <Statistic
                  title="总销售订单数"
                  value={allStatistics.total_order_count}
                  prefix="📋"
                />
              </Col>
            </Row>
          </Card>
        )}

        {/* 销售详情 */}
        <Card title={userInfo?.role === 'admin' ? '所有销售详情' : '个人销售详情'}>
          <Table
            columns={columns}
            dataSource={statistics}
            rowKey="id"
            loading={loading}
            pagination={{ pageSize: 10 }}
          />
        </Card>
      </Card>
    </div>
  );
};

export default StatisticsPage;