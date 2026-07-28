import React, { useState, useEffect } from 'react';
import { Table, Button, Input, Form, Modal, Switch, App, Tag, Space, message, Transfer } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, SettingOutlined, ReloadOutlined } from '@ant-design/icons';
import { listPanels, createPanel, updatePanel, deletePanel, getPanelGenes, updatePanelGenes, listGenes, clearCache } from '@/services/api';

const { TextArea } = Input;

const Panel: React.FC = () => {
  const [panels, setPanels] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState({});
  const [modalVisible, setModalVisible] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [modalForm] = Form.useForm();
  const [geneModalVisible, setGeneModalVisible] = useState(false);
  const [selectedPanelId, setSelectedPanelId] = useState<string | null>(null);
  const [selectedPanelGenes, setSelectedPanelGenes] = useState<any[]>([]);
  const [allGenes, setAllGenes] = useState<any[]>([]);
  const [geneLoading, setGeneLoading] = useState(false);
  const [cacheRefreshing, setCacheRefreshing] = useState(false);
  const { message: appMessage } = App.useApp();

  const fetchPanels = async (params: any = {}) => {
    setLoading(true);
    try {
      const response = await listPanels({ ...searchParams, ...params });
      setPanels(response.data || []);
      setPagination({
        ...pagination,
        total: response.data?.length || 0,
        current: params.page || 1,
        pageSize: params.pageSize || pagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取Panel列表失败');
    } finally {
      setLoading(false);
    }
  };

  const fetchAllGenes = async () => {
    try {
      const response = await listGenes();
      const genes = response.data || [];
      setAllGenes(genes.map((gene: any) => ({
        key: gene.id,
        title: `${gene.geneSymbol} (${gene.name})`,
        description: gene.description || '',
      })));
    } catch (_error) {
      appMessage.error('获取基因列表失败');
    }
  };

  const handleRefreshCache = async () => {
    setCacheRefreshing(true);
    try {
      await clearCache('genes');
      appMessage.success('缓存刷新成功');
      // 刷新后重新加载基因数据
      fetchAllGenes();
    } catch (_error) {
      appMessage.error('缓存刷新失败');
    } finally {
      setCacheRefreshing(false);
    }
  };

  const fetchPanelGenes = async (panelId: string) => {
    setGeneLoading(true);
    try {
      const response = await getPanelGenes(panelId);
      setSelectedPanelGenes(response.data || []);
    } catch (_error) {
      appMessage.error('获取Panel基因列表失败');
    } finally {
      setGeneLoading(false);
    }
  };

  useEffect(() => {
    fetchPanels();
    fetchAllGenes();
  }, []);

  const handleCreate = () => {
    setEditingId(null);
    modalForm.resetFields();
    setModalVisible(true);
  };

  const handleEdit = (record: any) => {
    setEditingId(record.id);
    modalForm.setFieldsValue({
      panelName: record.panelName,
      panelCode: record.panelCode,
      description: record.description,
      isActive: record.isActive === 1,
    });
    setModalVisible(true);
  };

  const handleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该Panel吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deletePanel(id);
          appMessage.success('删除成功');
          fetchPanels();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleModalSubmit = async (values: any) => {
    try {
      const requestBody = {
        panelName: values.panelName,
        panelCode: values.panelCode,
        description: values.description,
        isActive: values.isActive ? 1 : 0,
      };

      if (editingId) {
        await updatePanel(editingId, requestBody);
        appMessage.success('更新成功');
      } else {
        await createPanel(requestBody);
        appMessage.success('创建成功');
      }
      setModalVisible(false);
      fetchPanels();
    } catch (_error) {
      appMessage.error('操作失败');
    }
  };

  const handleGeneSettings = async (record: any) => {
    setSelectedPanelId(record.id);
    // 重新从数据库获取基因列表，不使用缓存
    try {
      const response = await listGenes({ skipCache: true });
      const genes = response.data || [];
      setAllGenes(genes.map((gene: any) => ({
        key: gene.id,
        title: `${gene.geneSymbol} (${gene.name})`,
        description: gene.description || '',
      })));
    } catch (_error) {
      appMessage.error('刷新基因列表失败');
    }
    fetchPanelGenes(record.id);
    setGeneModalVisible(true);
  };

  const handleGeneTransferChange = (targetKeys: any[]) => {
    setSelectedPanelGenes(targetKeys.map((key: any) => {
      const gene = allGenes.find((g: any) => g.key === key);
      return {
        id: gene?.key,
        geneSymbol: gene?.title?.split(' ')[0] || '',
        name: gene?.title?.replace(/\s*\([^)]*\)/, '') || '',
      };
    }));
  };

  const handleSaveGenes = async () => {
    if (!selectedPanelId) return;
    try {
      const geneIds = selectedPanelGenes.map((gene: any) => gene.id);
      await updatePanelGenes(selectedPanelId, geneIds);
      appMessage.success('保存成功');
      setGeneModalVisible(false);
    } catch (_error) {
      appMessage.error('保存失败');
    }
  };

  const columns = [
    {
      title: 'Panel名称',
      dataIndex: 'panelName',
      key: 'panelName',
      render: (text: string, record: any) => (
        <a onClick={() => handleEdit(record)}>{text}</a>
      ),
    },
    {
      title: 'Panel编码',
      dataIndex: 'panelCode',
      key: 'panelCode',
    },
    {
      title: '描述',
      dataIndex: 'description',
      key: 'description',
      ellipsis: true,
    },
    {
      title: '状态',
      dataIndex: 'isActive',
      key: 'isActive',
      render: (status: number) => (
        <Tag color={status === 1 ? 'green' : 'red'}>
          {status === 1 ? '启用' : '禁用'}
        </Tag>
      ),
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: string) => text ? text.split('T')[0] : '-',
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <>
          <Button
            type="link"
            icon={<SettingOutlined />}
            onClick={() => handleGeneSettings(record)}
            style={{ marginRight: 8 }}
          >
            基因设置
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>
          新增Panel
        </Button>
        <Button
          type="default"
          icon={<ReloadOutlined />}
          onClick={handleRefreshCache}
          loading={cacheRefreshing}
          style={{ marginLeft: 8 }}
        >
          刷新基因缓存
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={panels}
        rowKey="id"
        loading={loading}
        pagination={pagination}
        onChange={(page) => fetchPanels({ page: page.current, pageSize: page.pageSize })}
      />

      <Modal
        title={editingId ? '编辑Panel' : '新增Panel'}
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
            name="panelName"
            label="Panel名称"
            rules={[{ required: true, message: '请输入Panel名称' }]}
          >
            <Input placeholder="请输入Panel名称" />
          </Form.Item>

          <Form.Item
            name="panelCode"
            label="Panel编码"
            rules={[{ required: true, message: '请输入Panel编码' }]}
          >
            <Input placeholder="请输入Panel编码" />
          </Form.Item>

          <Form.Item
            name="description"
            label="描述"
          >
            <TextArea rows={3} placeholder="请输入描述" />
          </Form.Item>

          <Form.Item
            name="isActive"
            label="状态"
            valuePropName="checked"
            getValueProps={(checked: boolean) => ({ checked })}
            initialValue={true}
          >
            <Switch checkedChildren="启用" unCheckedChildren="禁用" />
          </Form.Item>

          <Form.Item>
            <Space>
              <Button type="primary" htmlType="submit">
                保存
              </Button>
              <Button onClick={() => setModalVisible(false)}>
                取消
              </Button>
            </Space>
          </Form.Item>
        </Form>
      </Modal>

      <Modal
        title="Panel基因设置"
        open={geneModalVisible}
        onCancel={() => setGeneModalVisible(false)}
        onOk={handleSaveGenes}
        width={800}
        confirmLoading={geneLoading}
      >
        <div style={{ marginBottom: 16 }}>
          <p>选择该Panel包含的基因：</p>
        </div>
        <Transfer
          dataSource={allGenes}
          titles={['可选择基因', '已选基因']}
          targetKeys={selectedPanelGenes.map((g: any) => g.id)}
          onChange={handleGeneTransferChange}
          render={(item: any) => item.title || item.geneSymbol}
          listStyle={{
            width: 350,
            height: 400,
          }}
          showSearch
          filterOption={(inputValue: string, item: any) =>
            item.title.toLowerCase().indexOf(inputValue.toLowerCase()) !== -1 ||
            (item.description && item.description.toLowerCase().indexOf(inputValue.toLowerCase()) !== -1)
          }
        />
      </Modal>
    </div>
  );
};

export default Panel;
