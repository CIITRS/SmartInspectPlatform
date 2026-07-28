import type React from 'react';
import { useState } from 'react';
import { Card, Form, Input, Button, message, Space, Switch } from 'antd';
import { history, request } from 'umi';
import WangEditor from '@/components/WangEditor';

interface AnnouncementForm {
  title: string;
  content: string;
  publisher: string;
  is_pinned: boolean;
}

const CreateAnnouncement: React.FC = () => {
  const [form] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [content, setContent] = useState('');

  const handleSubmit = async (values: AnnouncementForm, isPublished: boolean) => {
    setSubmitting(true);
    try {
      const response = await request('/api/announcements', {
        method: 'POST',
        data: {
          title: values.title,
          content: content,
          is_published: isPublished,
          publisher: values.publisher || '管理员',
          is_pinned: values.is_pinned ?? false,
        },
      });

      if (response && response.success) {
        message.success('创建公告成功');
        history.push('/announcement');
      } else {
        message.error(response?.message || '创建公告失败');
      }
    } catch (error) {
      console.error('Error creating announcement:', error);
      message.error('创建公告失败');
    } finally {
      setSubmitting(false);
    }
  };

  const handleCancel = () => {
    history.push('/announcement');
  };

  return (
    <div style={{ padding: '24px' }}>
      <Card title="新增公告">
        <Form
          form={form}
          layout="vertical"
          autoComplete="off"
          initialValues={{ publisher: '管理员', is_pinned: false }}
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
                loading={submitting}
                onClick={() => form.validateFields().then(values => handleSubmit(values, true))}
              >
                保存并发布
              </Button>
              <Button
                loading={submitting}
                onClick={() => form.validateFields().then(values => handleSubmit(values, false))}
              >
                保存草稿
              </Button>
              <Button onClick={handleCancel}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default CreateAnnouncement;
