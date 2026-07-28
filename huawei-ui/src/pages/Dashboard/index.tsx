import type React from 'react';
import { useState, useEffect } from 'react';
import { Card, Row, Col, Statistic, List, Tag, Button, message, Radio, Avatar, Divider, Modal } from 'antd';
import { useNavigate } from 'umi';
import { UserOutlined, FileTextOutlined, FilePdfOutlined, EditOutlined, BellOutlined } from '@ant-design/icons';
import { request } from 'umi';

interface TodoItem {
  id: number;
  title: string;
  type: string;
  patientId?: number;
  sampleId?: number;
  reportId?: number;
  createdAt: string;
  status?: string;
  priority?: string;
}

interface Announcement {
  id: number;
  title: string;
  content: string;
  user_id: number;
  created_at: string;
  updated_at: string;
}

interface Role {
  id: number;
  name: string;
  description: string;
}

interface UserInfo {
  id: number;
  username: string;
  name: string;
  role: Role;
  department?: string;
  email?: string;
  phone?: string;
}

const Dashboard: React.FC = () => {
  const navigate = useNavigate();
  const [todos, setTodos] = useState<TodoItem[]>([]);
  const [loading, setLoading] = useState(false);
  const [stats, setStats] = useState({
    patients: 0,
    samples: 0,
    results: 0,
    reports: 0,
    todos: {
      pendingPatients: 0,
      pendingReports: 0,
      pendingReviews: 0
    }
  });
  const [timeRange, setTimeRange] = useState('all');
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [userInfo, setUserInfo] = useState<UserInfo | null>(null);
  const [isTodoModalVisible, setIsTodoModalVisible] = useState(false);
  const [selectedTodo, setSelectedTodo] = useState<TodoItem | null>(null);

  const fetchStats = async () => {
    setLoading(true);
    try {
      const response = await request('/api/dashboard/stats', {
        method: 'GET',
        params: { timeRange }
      });
      
      // 检查响应数据结构
      if (response) {
        let statsData = null;
        
        // 检查响应是否直接是数据对象（没有data属性）
        if (response.patients !== undefined) {
          statsData = response;
        } else if (response.data && response.data.patients !== undefined) {
          statsData = response.data;
        } else {
          message.error('获取统计数据失败：响应数据格式错误');
          setLoading(false);
          return;
        }
        
        if (statsData) {
          setStats(statsData);
        }
      } else {
        message.error('获取统计数据失败：无响应数据');
      }
    } catch (error) {
      console.error('Error:', error);
      message.error('获取统计数据失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchTodos = async () => {
    try {
      const response = await request('/api/dashboard/stats', {
        method: 'GET',
        params: { timeRange: 'all' }
      });
      
      // 检查响应数据结构
      if (response) {
        // 构建待办任务列表
        const todoList = [];
        let statsData = null;
        
        // 检查响应是否直接是数据对象（没有data属性）
        if (response.patients !== undefined) {
          statsData = response;
        } else if (response.data && response.data.patients !== undefined) {
          statsData = response.data;
        }
        
        if (statsData && statsData.todos) {
          const now = new Date().toISOString();
          // 添加高优先级任务：待完善诊断
          if (statsData.todos.pendingPatients > 0) {
            todoList.push({
              id: 1,
              title: `${statsData.todos.pendingPatients}个待完善诊断`,
              type: 'patient',
              status: 'pending',
              priority: 'high',
              createdAt: now
            });
          }
          
          // 添加中优先级任务：检测报告待生成
          if (statsData.todos.pendingReports > 0) {
            todoList.push({
              id: 2,
              title: `${statsData.todos.pendingReports}个检测报告待生成`,
              type: 'report',
              status: 'pending',
              priority: 'medium',
              createdAt: now
            });
          }
          
          // 添加中优先级任务：检测报告待审核
          if (statsData.todos.pendingReviews > 0) {
            todoList.push({
              id: 3,
              title: `${statsData.todos.pendingReviews}个检测报告待审核`,
              type: 'review',
              status: 'pending',
              priority: 'medium',
              createdAt: now
            });
          }
          
          // 按优先级排序：高优先级在前
          todoList.sort((a, b) => {
            const priorityOrder = { 'high': 0, 'medium': 1, 'low': 2 };
            return priorityOrder[a.priority as keyof typeof priorityOrder] - priorityOrder[b.priority as keyof typeof priorityOrder];
          });
          
          setTodos(todoList);
        }
      }
    } catch (error) {
      console.error('Error fetching todos:', error);
    }
  };

  const fetchAnnouncements = async () => {
    try {
      const response = await request('/api/announcements', {
        method: 'GET'
      });
      
      if (response && response.data) {
        setAnnouncements(response.data);
      }
    } catch (error) {
      console.error('Error fetching announcements:', error);
    }
  };

  const fetchUserInfo = async () => {
    try {
      const response = await request('/api/auth/me', {
        method: 'GET'
      });
      
      if (response && response.data) {
        // 转换后端返回的数据结构，将real_name映射为name
        const userData = {
          ...response.data,
          name: response.data.real_name || response.data.username
        };
        setUserInfo(userData);
      }
    } catch (error) {
      console.error('Error fetching user info:', error);
    }
  };

  useEffect(() => {
    fetchStats();
    fetchAnnouncements();
    fetchUserInfo();
  }, [timeRange]);

  useEffect(() => {
    fetchTodos();
  }, []);

  const [pendingPatients, setPendingPatients] = useState<any[]>([]);
  const [loadingPatients, setLoadingPatients] = useState(false);

  const fetchPendingPatients = async () => {
    setLoadingPatients(true);
    try {
      const response = await request('/api/patients', {
        method: 'GET',
        params: { status: 'pending' }
      });
      
      if (response && response.data && response.data.list) {
        setPendingPatients(response.data.list);
      }
    } catch (error) {
      console.error('Error fetching pending patients:', error);
    } finally {
      setLoadingPatients(false);
    }
  };

  const handleTodoClick = (todo: TodoItem) => {
    setSelectedTodo(todo);
    if (todo.type === 'patient') {
      fetchPendingPatients();
    }
    setIsTodoModalVisible(true);
  };

  const handleTodoModalClose = () => {
    setIsTodoModalVisible(false);
    setSelectedTodo(null);
  };

  const handleTodoAction = (todo: TodoItem) => {
    if (todo.type === 'patient') {
      window.location.href = `/patient/perfect`;
    } else if (todo.type === 'report') {
      window.location.href = `/result/list`;
    } else if (todo.type === 'review') {
      window.location.href = `/report/list`;
    }
  };

  const handleTimeRangeChange = (e: any) => {
    setTimeRange(e.target.value);
  };

  const handleAnnouncementClick = (announcement: Announcement) => {
    navigate(`/announcement/${announcement.id}`);
  };

  const priorityColorMap = {
    high: 'red',
    medium: 'orange',
    low: 'blue'
  };

  return (
    <div style={{ padding: '24px' }}>
      <Row gutter={[16, 16]}>
        {/* 统计信息区域 */}
        <Col span={18}>
          <Card>
            {/* 时间筛选功能 */}
            <Row gutter={[16, 16]} style={{ marginBottom: '24px' }}>
              <Col span={24}>
                <div style={{ display: 'flex', justifyContent: 'flex-end', alignItems: 'center' }}>
                  <span style={{ marginRight: '16px' }}>时间范围：</span>
                  <Radio.Group value={timeRange} onChange={handleTimeRangeChange}>
                    <Radio.Button value="all">所有</Radio.Button>
                    <Radio.Button value="week">本周</Radio.Button>
                    <Radio.Button value="day">本日</Radio.Button>
                  </Radio.Group>
                </div>
              </Col>
            </Row>
            
            <Row gutter={[16, 16]}>
              <Col span={6}>
                <Statistic
                  title="患者总数"
                  value={stats.patients}
                  prefix={<UserOutlined />}
                  styles={{ content: { color: '#3f8600' } }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="样本总数"
                  value={stats.samples}
                  prefix={<FileTextOutlined />}
                  styles={{ content: { color: '#13c2c2' } }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="结果总数"
                  value={stats.results}
                  prefix={<FileTextOutlined />}
                  styles={{ content: { color: '#52c41a' } }}
                />
              </Col>
              <Col span={6}>
                <Statistic
                  title="报告总数"
                  value={stats.reports}
                  prefix={<FilePdfOutlined />}
                  styles={{ content: { color: '#722ed1' } }}
                />
              </Col>
            </Row>
          </Card>



          {/* 待办任务 */}
          <Row gutter={[16, 16]} style={{ marginTop: '24px' }}>
            <Col span={24}>
              <Card title="待办任务">
                {todos.length > 0 ? (
                  <List
                    dataSource={todos}
                    renderItem={item => (
                      <List.Item
                        actions={[
                          <Button
                            key={item.id}
                            type="link"
                            size="small"
                            icon={<EditOutlined />}
                            onClick={() => handleTodoAction(item)}
                          >
                            去处理
                          </Button>
                        ]}
                      >
                        <List.Item.Meta
                          title={
                            <a 
                              onClick={() => handleTodoClick(item)}
                              style={{ color: '#000', cursor: 'pointer' }}
                            >
                              {item.title}
                            </a>
                          }
                        />
                      </List.Item>
                    )}
                  />
                ) : (
                  <p style={{ textAlign: 'center', color: '#999', padding: '20px 0' }}>
                    暂无待办任务
                  </p>
                )}
              </Card>
            </Col>
          </Row>
        </Col>

        {/* 个人信息展示模块 */}
        <Col span={6}>
          <Card title="个人信息" style={{ marginBottom: '16px' }}>
            {userInfo ? (
              <div style={{ textAlign: 'center' }}>
                <div>
                  <p>{userInfo.name || userInfo.username}【{userInfo.username}】</p>
                  <p>{userInfo.role.name}</p>
                  {userInfo.department && <p><strong>部门：</strong>{userInfo.department}</p>}
                  {userInfo.email && <p><strong>邮箱：</strong>{userInfo.email}</p>}
                  {userInfo.phone && <p><strong>电话：</strong>{userInfo.phone}</p>}
                </div>
              </div>
            ) : (
              <p style={{ textAlign: 'center', color: '#999', padding: '20px 0' }}>
                加载用户信息中...
              </p>
            )}
          </Card>
          
          {/* 系统公告模块 */}
          <Card 
            title={<><BellOutlined /> 系统公告</>}
            extra={
              <Button 
                type="link" 
                onClick={() => navigate('/announcements')}
              >
                查看更多
              </Button>
            }
          >
            {announcements.length > 0 ? (
              <List
                dataSource={announcements}
                renderItem={item => (
                  <List.Item>
                    <List.Item.Meta
                      title={
                        <a onClick={() => handleAnnouncementClick(item)}>{item.title}</a>
                      }
                      description={
                        <p style={{ color: '#999', fontSize: '12px' }}>
                          发布时间：{new Date(item.created_at).toLocaleString()}
                        </p>
                      }
                    />
                  </List.Item>
                )}
              />
            ) : (
              <p style={{ textAlign: 'center', color: '#999', padding: '20px 0' }}>
                暂无系统公告
              </p>
            )}
          </Card>
        </Col>
      </Row>
      


      {/* 待办任务详情模态框 */}
      <Modal
        title={selectedTodo?.type === 'patient' ? '待完善诊断名单' : '待办任务详情'}
        open={isTodoModalVisible}
        onCancel={handleTodoModalClose}
        footer={[
          <Button key="cancel" onClick={handleTodoModalClose}>
            取消
          </Button>,
          <Button key="ok" type="primary" onClick={() => handleTodoAction(selectedTodo!)}>
            去处理
          </Button>
        ]}
        width={800}
      >
        {selectedTodo && (
          <div>
            {selectedTodo.type === 'patient' ? (
              <div>
                <h3 style={{ marginBottom: '16px' }}>{selectedTodo.title}</h3>
                <div style={{ marginBottom: '16px' }}>
                  <p><strong>类型：</strong>待完善诊断</p>
                  <p><strong>状态：</strong>待处理</p>
                </div>
                <div>
                  <h4 style={{ marginBottom: '12px' }}>待完善诊断名单：</h4>
                  {loadingPatients ? (
                    <p style={{ textAlign: 'center', color: '#999' }}>加载中...</p>
                  ) : pendingPatients.length > 0 ? (
                    <List
                      dataSource={pendingPatients}
                      renderItem={patient => (
                        <List.Item>
                          <List.Item.Meta
                            title={patient.name}
                            description={
                              <div>
                                <p>性别：{patient.gender}</p>
                                <p>年龄：{patient.age}</p>
                                <p>联系电话：{patient.phone}</p>
                              </div>
                            }
                          />
                        </List.Item>
                      )}
                    />
                  ) : (
                    <p style={{ textAlign: 'center', color: '#999' }}>暂无待完善诊断</p>
                  )}
                </div>
              </div>
            ) : (
              <div>
                <h3 style={{ marginBottom: '16px' }}>{selectedTodo.title}</h3>
                <p><strong>类型：</strong>
                  {selectedTodo.type === 'report' ? '检测报告待生成' : '检测报告待审核'}
                </p>
                <p><strong>状态：</strong>待处理</p>
                <p style={{ color: '#999', fontSize: '12px', marginTop: '16px' }}>
                  点击"去处理"按钮跳转到相应页面进行处理
                </p>
              </div>
            )}
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Dashboard;
