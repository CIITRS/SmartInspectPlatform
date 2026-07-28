import type React from 'react';
import { useState, useEffect } from 'react';
import { Card, Table, Button, message, Space, Popconfirm, Modal, Typography, Divider, Tag } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, UserOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { request, history } from 'umi';
import styles from './Detail.less';

const { Title } = Typography;

interface Announcement {
  id: number;
  title: string;
  content: string;
  user_id: number;
  user_name?: string;
  publisher?: string;
  is_pinned?: boolean;
  created_at: string;
  updated_at: string;
}

const AnnouncementManagement: React.FC = () => {
  const [announcements, setAnnouncements] = useState<Announcement[]>([]);
  const [loading, setLoading] = useState(false);
  const [previewVisible, setPreviewVisible] = useState(false);
  const [previewAnnouncement, setPreviewAnnouncement] = useState<Announcement | null>(null);


  const fetchAnnouncements = async () => {
    setLoading(true);
    try {
      const response = await request('/api/announcements', {
        method: 'GET'
      });
      
      if (response && response.data) {
        const sortedData = [...response.data].sort((a, b) => {
          if (a.is_pinned && !b.is_pinned) return -1;
          if (!a.is_pinned && b.is_pinned) return 1;
          return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
        });
        setAnnouncements(sortedData);
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

  const handleAddAnnouncement = () => {
    history.push('/announcement/create');
  };

  const handleEditAnnouncement = (announcement: Announcement) => {
    history.push(`/announcement/edit/${announcement.id}`);
  };

  const handleDeleteAnnouncement = async (id: number) => {
    try {
      const response = await request(`/api/announcements/${id}`, {
        method: 'DELETE'
      });

      if (response && response.success) {
        message.success('删除公告成功');
        fetchAnnouncements();
      } else {
        message.error('删除公告失败');
      }
    } catch (error) {
      console.error('Error deleting announcement:', error);
      message.error('删除公告失败');
    }
  };

  const handlePreview = (announcement: Announcement) => {
    setPreviewAnnouncement(announcement);
    setPreviewVisible(true);
  };

  const handleClosePreview = () => {
    setPreviewVisible(false);
    setPreviewAnnouncement(null);
  };



  const columns = [
    {
      title: '置顶',
      dataIndex: 'is_pinned',
      key: 'is_pinned',
      width: 80,
      render: (isPinned: boolean) => isPinned ? <Tag color="red">置顶</Tag> : null,
    },
    {
      title: '标题',
      dataIndex: 'title',
      key: 'title',
      render: (text: string, record: Announcement) => (
        <Button
          type="link"
          onClick={() => handlePreview(record)}
          style={{ padding: 0, textAlign: 'left', whiteSpace: 'normal', height: 'auto' }}
        >
          {text}
        </Button>
      ),
    },
    {
      title: '发布人',
      dataIndex: 'publisher',
      key: 'publisher',
      width: 120,
      render: (_: string, record: Announcement) => record.publisher || record.user_name || '管理员',
    },
    {
      title: '发布时间',
      dataIndex: 'created_at',
      key: 'created_at',
      width: 180,
      render: (text: string) => new Date(text).toLocaleString(),
    },
    {
      title: '操作',
      key: 'action',
      width: 150,
      render: (_: any, record: Announcement) => (
        <Space size="middle">
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEditAnnouncement(record)}
          >
            编辑
          </Button>
          <Popconfirm
            title="确定要删除这条公告吗？"
            onConfirm={() => handleDeleteAnnouncement(record.id)}
            okText="确定"
            cancelText="取消"
          >
            <Button
              type="link"
              danger
              icon={<DeleteOutlined />}
            >
              删除
            </Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <div style={{ padding: '24px' }}>
      <Card
        title="公告管理"
        extra={
          <Button
            type="primary"
            icon={<PlusOutlined />}
            onClick={handleAddAnnouncement}
          >
            新增公告
          </Button>
        }
      >
        <Table
          columns={columns}
          dataSource={announcements}
          rowKey="id"
          loading={loading}
          pagination={{ pageSize: 10 }}
        />
      </Card>

      <Modal
        title="公告预览"
        open={previewVisible}
        onCancel={handleClosePreview}
        footer={null}
        width={800}
        centered
      >
        {previewAnnouncement && (
          <div>
            <div className={styles.announcementHeader}>
              <Title level={3} className={styles.title}>
                {previewAnnouncement.title}
              </Title>
              <Space size={24} className={styles.meta}>
                <span className={styles.metaItem}>
                  <UserOutlined />
                  <span>发布人：{previewAnnouncement.publisher || previewAnnouncement.user_name || '管理员'}</span>
                </span>
                <span className={styles.metaItem}>
                  <ClockCircleOutlined />
                  <span>发布时间：{new Date(previewAnnouncement.created_at).toLocaleString()}</span>
                </span>
              </Space>
            </div>
            <Divider />
            <div
              className={styles.richContent}
              dangerouslySetInnerHTML={{ __html: previewAnnouncement.content }}
            />
          </div>
        )}
      </Modal>
    </div>
  );
};

export default AnnouncementManagement;
