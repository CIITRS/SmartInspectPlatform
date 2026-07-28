import React, { useState, useEffect } from 'react';
import { Button, Input, Form, Modal, Select, App, Tree, Space, Spin } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';

const { Option } = Select;

const Department: React.FC = () => {
  const [modalForm] = Form.useForm();
  const [departments, setDepartments] = useState<any[]>([]);
  const [departmentTree, setDepartmentTree] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const { message: appMessage } = App.useApp();

  // 获取部门列表
  const fetchDepartments = async () => {
    setLoading(true);
    try {
      const response = await fetch('/api/system/departments', {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        // 后端直接返回的是部门数组，所以使用result.data而不是result.data.list
        setDepartments(result.data || []);
      } else {
        appMessage.error('获取部门列表失败');
      }
    } catch (_error) {
      appMessage.error('获取部门列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 获取部门树形结构
  const fetchDepartmentTree = async () => {
    setLoading(true);
    try {
      const response = await fetch('/api/system/departments/tree', {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        setDepartmentTree(result.data || []);
      } else {
        appMessage.error('获取部门树形结构失败');
      }
    } catch (_error) {
      appMessage.error('获取部门树形结构失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDepartments();
    fetchDepartmentTree();
  }, []);

  const handleCreate = () => {
    setEditingId(null);
    modalForm.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: any) => {
    setEditingId(record.id);
    modalForm.setFieldsValue(record);
    setModalVisible(true);
  };

  const handleDelete = (id: number) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该部门吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/system/departments/${id}`, {
            method: 'DELETE',
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`,
            },
          });
          const result = await response.json();
          if (result.code === 200) {
            appMessage.success('部门删除成功');
            fetchDepartments();
            fetchDepartmentTree();
          } else {
            appMessage.error(result.message || '部门删除失败');
          }
        } catch (_error) {
          appMessage.error('部门删除失败');
        }
      },
    });
  };

  const handleModalSubmit = async (values: any) => {
    try {
      const url = editingId ? `/api/system/departments/${editingId}` : '/api/system/departments';
      const method = editingId ? 'PUT' : 'POST';
      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: JSON.stringify(values),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success(editingId ? '部门更新成功' : '部门创建成功');
        setModalVisible(false);
        fetchDepartments();
        fetchDepartmentTree();
      } else {
        appMessage.error(result.message || (editingId ? '部门更新失败' : '部门创建失败'));
      }
    } catch (_error) {
      appMessage.error(editingId ? '部门更新失败' : '部门创建失败');
    }
  };

  // 转换部门树数据，添加key字段
  const transformDepartmentTree = (data: any[]): any[] => {
    return data.map(item => ({
      ...item,
      title: (
        <Space>
          {item.name}
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => handleEdit(item)}>编辑</Button>
          <Button type="link" danger size="small" icon={<DeleteOutlined />} onClick={() => handleDelete(item.id)}>删除</Button>
        </Space>
      ),
      key: item.id,
      children: item.children ? transformDepartmentTree(item.children) : []
    }));
  };

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>部门管理</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新增部门
        </Button>
      </div>

      <div style={{ marginBottom: 16, padding: 16, background: '#f0f2f5', borderRadius: 4 }}>
        <h3>部门结构</h3>
        <Spin spinning={loading}>
          <Tree 
            treeData={transformDepartmentTree(departmentTree)} 
            defaultExpandAll
          />
        </Spin>
      </div>

      {/* 部门编辑/创建模态框 */}
      <Modal
        title={editingId ? '编辑部门' : '新增部门'}
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
      >
        <Form
          form={modalForm}
          layout="vertical"
          onFinish={handleModalSubmit}
        >
          <Form.Item
            name="name"
            label="部门名称"
            rules={[{ required: true, message: '请输入部门名称' }]}
          >
            <Input placeholder="请输入部门名称" />
          </Form.Item>

          <Form.Item
            name="parent_id"
            label="父部门"
          >
            <Select placeholder="选择父部门">
              <Option value={null}>无父部门</Option>
              {departments.map((dept) => (
                <Option key={dept.id} value={dept.id}>
                  {dept.name}
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            name="description"
            label="部门描述"
          >
            <Input.TextArea placeholder="请输入部门描述" rows={4} />
          </Form.Item>

          <Form.Item
            name="status"
            label="状态"
            initialValue={1}
            rules={[{ required: true, message: '请选择状态' }]}
          >
            <Select placeholder="请选择状态">
              <Option value={1}>启用</Option>
              <Option value={0}>禁用</Option>
            </Select>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
              保存
            </Button>
            <Button onClick={() => setModalVisible(false)}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Department;