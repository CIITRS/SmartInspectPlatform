import React, { useEffect, useState } from 'react';
import { App, Button, Card, Form, Input, Space, Spin } from 'antd';
import { DeleteOutlined, PlusOutlined, SaveOutlined } from '@ant-design/icons';

const { TextArea } = Input;

type HelpItem = {
  question: string;
  answer: string;
};

type HelpCategory = {
  name: string;
  items: HelpItem[];
};

type HelpPayload = {
  categories: HelpCategory[];
};

const emptyPayload: HelpPayload = { categories: [] };

const HelpCenter: React.FC = () => {
  const [form] = Form.useForm<HelpPayload>();
  const [loading, setLoading] = useState(true);
  const [saving, setSaving] = useState(false);
  const { message } = App.useApp();

  const fetchHelpCenter = async () => {
    setLoading(true);
    try {
      const response = await fetch('/api/system/help-center');
      const result = await response.json();
      if (result.code === 200) {
        form.setFieldsValue({
          categories: Array.isArray(result.data?.categories) ? result.data.categories : [],
        });
      } else {
        message.error(result.message || '获取帮助中心失败');
        form.setFieldsValue(emptyPayload);
      }
    } catch (error) {
      message.error('获取帮助中心失败');
      form.setFieldsValue(emptyPayload);
    } finally {
      setLoading(false);
    }
  };

  const handleSave = async (values: HelpPayload) => {
    setSaving(true);
    try {
      const payload = {
        categories: (values.categories || []).map((category) => ({
          name: category.name || '',
          items: (category.items || []).map((item) => ({
            question: item.question || '',
            answer: item.answer || '',
          })),
        })),
      };
      const response = await fetch('/api/system/help-center', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      const result = await response.json();
      if (result.code === 200) {
        message.success('帮助中心已保存');
        form.setFieldsValue(payload);
      } else {
        message.error(result.message || '保存帮助中心失败');
      }
    } catch (error) {
      message.error('保存帮助中心失败');
    } finally {
      setSaving(false);
    }
  };

  useEffect(() => {
    fetchHelpCenter();
  }, []);

  return (
    <Spin spinning={loading}>
      <Card
        title="帮助中心"
        extra={
          <Button type="primary" icon={<SaveOutlined />} loading={saving} onClick={() => form.submit()}>
            保存
          </Button>
        }
      >
        <Form form={form} layout="vertical" onFinish={handleSave} initialValues={emptyPayload}>
          <Form.List name="categories">
            {(categoryFields, { add: addCategory, remove: removeCategory }) => (
              <Space direction="vertical" size={16} style={{ width: '100%' }}>
                {categoryFields.map((categoryField) => (
                  <div
                    key={categoryField.key}
                    style={{ border: '1px solid #f0f0f0', borderRadius: 8, padding: 16 }}
                  >
                    <div style={{ display: 'flex', justifyContent: 'space-between', gap: 16, marginBottom: 16 }}>
                      <Form.Item
                        name={[categoryField.name, 'name']}
                        rules={[{ required: true, message: '请输入分类名称' }]}
                        style={{ marginBottom: 0, maxWidth: 360, flex: 1 }}
                      >
                        <Input placeholder="分类名称" />
                      </Form.Item>
                      <Button danger icon={<DeleteOutlined />} onClick={() => removeCategory(categoryField.name)}>
                        删除分类
                      </Button>
                    </div>
                    <Form.List name={[categoryField.name, 'items']}>
                      {(itemFields, { add: addItem, remove: removeItem }) => (
                        <Space direction="vertical" size={12} style={{ width: '100%' }}>
                          {itemFields.map((itemField) => (
                            <div
                              key={itemField.key}
                              style={{ border: '1px solid #f5f5f5', borderRadius: 6, padding: 16, background: '#fafafa' }}
                            >
                              <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 8 }}>
                                <Button danger type="text" icon={<DeleteOutlined />} onClick={() => removeItem(itemField.name)} />
                              </div>
                              <Form.Item
                                name={[itemField.name, 'question']}
                                label="问题"
                                rules={[{ required: true, message: '请输入问题' }]}
                              >
                                <Input placeholder="常见问题" />
                              </Form.Item>
                              <Form.Item
                                name={[itemField.name, 'answer']}
                                label="回答"
                                rules={[{ required: true, message: '请输入回答' }]}
                              >
                                <TextArea rows={4} placeholder="回答内容" />
                              </Form.Item>
                            </div>
                          ))}
                          <Button icon={<PlusOutlined />} onClick={() => addItem({ question: '', answer: '' })}>
                            新增问题
                          </Button>
                        </Space>
                      )}
                    </Form.List>
                  </div>
                ))}
                <Button type="dashed" icon={<PlusOutlined />} onClick={() => addCategory({ name: '', items: [] })} block>
                  新增分类
                </Button>
              </Space>
            )}
          </Form.List>
        </Form>
      </Card>
    </Spin>
  );
};

export default HelpCenter;
