import type React from 'react';
import { useState, useEffect } from 'react';
import { Card, Form, Input, Button, Switch, message, Space, Spin } from 'antd';
import { useParams, history, request } from 'umi';
import WangEditor from '@/components/WangEditor';

interface AnnouncementForm {
  title: string;
  content: string;
  publisher: string;
  is_pinned: boolean;
}

interface Announcement {
  id: number;
  title: string;
  content: string;
  is_published: boolean;
  user_id: number;
  publisher?: string;
  is_pinned?: boolean;
  created_at: string;
  updated_at: string;
}

const EditAnnouncement: React.FC = () => {
  const [form] = Form.useForm();
  const { id } = useParams<{ id: string }>();
  const [submitting, setSubmitting] = useState(false);
  const [loading, setLoading] = useState(false);
  const [content, setContent] = useState('');

  // 获取公告详情
  const fetchAnnouncementDetail = async () => {
    if (!id) {
      message.error('公告ID不存在');
      history.push('/announcement');
      return;
    }

    setLoading(true);
    try {
      const response = await request(`/api/announcements/${id}`, {
        method: 'GET',
      });

      if (response && response.data) {
        const announcement: Announcement = response.data;
        form.setFieldsValue({
          title: announcement.title,
          publisher: announcement.publisher || '管理员',
          is_pinned: announcement.is_pinned || false,
        });
        setContent(announcement.content || '');
      } else {
        message.error('获取公告详情失败');
        history.push('/announcement');
      }
    } catch (error) {
      console.error('Error fetching announcement:', error);
      message.error('获取公告详情失败');
      history.push('/announcement');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchAnnouncementDetail();
  }, [id]);

  // 提交表单
  const handleSubmit = async (values: AnnouncementForm, isPublished: boolean) => {
    if (!id) {
      message.error('公告ID不存在');
      return;
    }

    setSubmitting(true);
    try {
      const response = await request(`/api/announcements/${id}`, {
        method: 'PUT',
        data: {
          title: values.title,
          content: content,
          is_published: isPublished,
          publisher: values.publisher || '管理员',
          is_pinned: values.is_pinned ?? false,
        },
      });

      if (response && response.success) {
        message.success('更新公告成功');
        history.push('/announcement');
      } else {
        message.error(response?.message || '更新公告失败');
      }
    } catch (error) {
      console.error('Error updating announcement:', error);
      message.error('更新公告失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    history.push('/announcement');
  };

  return (
    <div style={{ padding: '24px' }}>
      <Card title="编辑公告">
        <Spin spinning={loading} tip="加载中...">
          <Form
            form={form}
            layout="vertical"
            autoComplete="off"
          >
            <Form.Item
              name="title"
              label="公告标题"
              rules={[
                { required: true, message: '请输入公告标题' },
                { max: 200, message: '标题长度不能超过200个字符' },
              ]}
            >
              <Input placeholder="请输入公告标题" maxLength={200} showCount />
            </Form.Item>

            <Form.Item
              name="publisher"
              label="发布人"
              rules={[{ required: true, message: '请输入发布人' }]}
            >
              <Input placeholder="请输入发布人" />
            </Form.Item>

            <Form.Item
              name="is_pinned"
              label="置顶状态"
              valuePropName="checked"
              initialValue={false}
            >
              <Switch
                checkedChildren="已置顶"
                unCheckedChildren="未置顶"
              />
            </Form.Item>

            <Form.Item
              label="公告内容"
              required
              validateStatus={!content ? 'error' : ''}
              help={!content ? '请输入公告内容' : ''}
            >
              <WangEditor
                value={content}
                onChange={setContent}
                placeholder="请输入公告内容..."
                height={400}
              />
            </Form.Item>

            <Form.Item>
              <Space>
                <Button
                  type="primary"
                  htmlType="button"
                  loading={submitting}
                  onClick={() => form.validateFields().then(values => handleSubmit(values, true))}
                >
                  保存并发布
                </Button>
                <Button
                  htmlType="button"
                  loading={submitting}
                  onClick={() => form.validateFields().then(values => handleSubmit(values, false))}
                >
                  保存草稿
                </Button>
                <Button onClick={handleCancel}>取消</Button>
              </Space>
            </Form.Item>
          </Form>
        </Spin>
      </Card>
    </div>
  );
};

export default EditAnnouncement;
