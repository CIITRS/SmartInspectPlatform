import type React from 'react';
import { useState, useEffect } from 'react';
import { Card, Button, message, Typography, Divider, Space } from 'antd';
import { ArrowLeftOutlined, UserOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { request, useParams, useNavigate } from 'umi';
import styles from './Detail.less';

const { Title, Paragraph } = Typography;

interface Announcement {
  id: number;
  title: string;
  content: string;
  user_id: number;
  user_name?: string;
  publisher?: string;
  created_at: string;
  updated_at: string;
}

const AnnouncementDetail: React.FC = () => {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [announcement, setAnnouncement] = useState<Announcement | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchAnnouncement = async () => {
    setLoading(true);
    try {
      const response = await request(`/api/announcements/${id}`, {
        method: 'GET'
      });

      if (response && response.data) {
        setAnnouncement(response.data);
      }
    } catch (error) {
      console.error('Error fetching announcement:', error);
      message.error('获取公告详情失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAnnouncement();
  }, [id]);

  const handleBack = () => {
    navigate(-1);
  };

  if (loading) {
    return <div style={{ padding: '24px' }}>加载中...</div>;
  }

  if (!announcement) {
    return <div style={{ padding: '24px' }}>公告不存在</div>;
  }

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title={
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span>公告详情</span>
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
        <div className={styles.announcementHeader}>
          <Title level={2} className={styles.title}>
            {announcement.title}
          </Title>
          <Space size={24} className={styles.meta}>
            <span className={styles.metaItem}>
              <UserOutlined />
              <span>发布人：{announcement.publisher || announcement.user_name || '管理员'}</span>
            </span>
            <span className={styles.metaItem}>
              <ClockCircleOutlined />
              <span>发布时间：{new Date(announcement.created_at).toLocaleString()}</span>
            </span>
          </Space>
        </div>
        <Divider />
        <div
          className={styles.richContent}
          dangerouslySetInnerHTML={{ __html: announcement.content }}
        />
      </Card>
    </div>
  );
};

export default AnnouncementDetail;
