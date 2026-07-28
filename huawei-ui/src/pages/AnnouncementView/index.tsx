import type React from 'react';
import { useState, useEffect } from 'react';
import { Card, List, Typography, Button, message } from 'antd';
import { ArrowLeftOutlined, BellOutlined } from '@ant-design/icons';
import { request, useNavigate } from 'umi';

const { Title, Paragraph } = Typography;

interface Announcement {
  id: number;
  title: string;
  content: string;
  user_id: number;
  created_at: string;
  updated_at: string;
}

const AnnouncementView: React.FC = () => {
  const navigate = useNavigate();
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(true);

  const fetchAnnouncements = async () => {
    setLoading(true);
    try {
      const response = await request('/api/announcements', {
        method: 'GET'
      });
      
      if (response && response.data) {
        setAnnouncements(response.data);
      }
    } catch (error) {
      console.error('Error fetching announcements:', error);
      message.error('获取公告列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAnnouncements();
  }, []);

  const handleBack = () => {
    navigate('/');
  };

  const handleAnnouncementClick = (announcement: Announcement) => {
    navigate(`/announcements/${announcement.id}`);
  };

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span><BellOutlined /> 系统公告</span>
            <Button
              type="default"
              icon={<ArrowLeftOutlined />}
              onClick={handleBack}
            >
              返回
            </Button>
          </div>
        }
      >
        {loading ? (
          <p style={{ textAlign: 'center', color: '#999', padding: '20px 0' }}>
            加载中...
          </p>
        ) : announcements.length > 0 ? (
          <List
            dataSource={announcements}
            renderItem={item => (
              <List.Item
                key={item.id}
                style={{ borderBottom: '1px solid #f0f0f0', padding: '16px 0' }}
              >
                <List.Item.Meta
                  title={
                    <a onClick={() => handleAnnouncementClick(item)} style={{ fontSize: '16px', fontWeight: 'bold' }}>
                      {item.title}
                    </a>
                  }
                  description={
                    <div>
                      <p style={{ marginBottom: '8px' }}>
                        {item.content.length > 100 ? item.content.substring(0, 100) + '...' : item.content}
                      </p>
                      <p style={{ color: '#999', fontSize: '12px' }}>
                        发布时间：{new Date(item.created_at).toLocaleString()}
                      </p>
                    </div>
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
    </div>
  );
};

export default AnnouncementView;