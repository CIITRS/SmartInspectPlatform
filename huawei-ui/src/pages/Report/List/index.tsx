import type React from 'react';
import { useState, useEffect, useCallback } from 'react';
import { Table, Button, Input, Form, Row, Col, Tag, Select, Modal, App } from 'antd';
import { EyeOutlined, DownloadOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';
import { useNavigate } from '@umijs/max';
import { listReports, downloadReport, updateReportStatus, deleteReport } from '@/services/api';

interface Report {
  id: number;
  patientName: string;
  sampleCode: string;
  isChildReport?: boolean;
  reportRole?: string;
  reportData: string;
  status: string;
  createdAt: string;
  updatedAt: string;
}

interface SearchParams {
  patientName?: string;
  sampleCode?: string;
  status?: string;
  page?: number;
  pageSize?: number;
}

interface StatusValues {
  status: string;
}

const List: React.FC = () => {
  const [form] = Form.useForm();
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current:1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState<SearchParams>({});
  const { message: appMessage } = App.useApp();
  const navigate = useNavigate();
  
  // 状态更新相关
  const [statusModalVisible, setStatusModalVisible] = useState(false);
  const [selectedReportForStatus, setSelectedReportForStatus] = useState<Report | null>(null);
  const [statusForm] = Form.useForm();

  const fetchReports = useCallback(async (params: SearchParams = {}, searchOverride?: SearchParams) => {
    setLoading(true);
    try {
      const currentPage = params.page || 1;
      const currentPageSize = params.pageSize || 10;
      const activeSearchParams = searchOverride !== undefined ? searchOverride : searchParams;
      
      const response = await listReports({
        ...activeSearchParams,
        page: currentPage,
        page_size: currentPageSize,
      });
      if (response.data) {
        setReports(response.data.list || []);
        setPagination({
          current: currentPage,
          pageSize: currentPageSize,
          total: response.data.total || 0,
        });
      } else {
        appMessage.error('获取报告列表失败');
      }
    } catch (_error) {
      appMessage.error('获取报告列表失败');
    } finally {
      setLoading(false);
    }
  }, [searchParams, appMessage]);

  useEffect(() => {
    fetchReports();
  }, [searchParams, fetchReports]);

  const handleSearch = (values: SearchParams) => {
    setSearchParams(values);
  };

  const handleResetSearch = () => {
    form.resetFields();
    setSearchParams({});
  };

  const handleView = (record: Report) => {
    navigate(`/report/view/${encodeURIComponent(record.sampleCode)}`);
  };

  const handleDownload = async (record: Report) => {
    try {
      const response = await downloadReport(record.id.toString());
      if (response.data) {
        appMessage.success('报告下载成功');
      } else {
        appMessage.error('报告文件不存在');
      }
    } catch (_error) {
      appMessage.error('下载失败');
    }
  };

  const handleStatusEdit = (record: Report) => {
    setSelectedReportForStatus(record);
    statusForm.setFieldsValue({ status: record.status });
    setStatusModalVisible(true);
  };

  const handleStatusUpdate = async (values: StatusValues) => {
    try {
      if (!selectedReportForStatus?.id) {
        appMessage.error('请选择要更新的报告');
        return;
      }
      await updateReportStatus(selectedReportForStatus.id.toString(), {
        status: values.status
      });
      // 只要请求成功，就认为状态更新成功
      appMessage.success('状态更新成功');
      setStatusModalVisible(false);
      fetchReports(); // 刷新列表
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  const handleDelete = async (record: Report) => {
    try {
      const response = await deleteReport(record.id.toString());
      if (response.data) {
        appMessage.success('报告删除成功');
        fetchReports(); // 刷新列表
      } else {
        appMessage.error('报告删除失败');
      }
    } catch (_error) {
      appMessage.error('删除失败');
    }
  };

  const columns = [
    {
      title: '样本编号',
      dataIndex: 'sampleCode',
      key: 'sampleCode',
      render: (text: string, record: Report) => (
        <>
          <span>{text || '-'}</span>
          {(record.isChildReport || record.reportRole === 'child') && <Tag color="purple" style={{ marginLeft: 6 }}>子报告</Tag>}
        </>
      ),
    },
    { title: '患者姓名', dataIndex: 'patientName', key: 'patientName' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap: Record<string, React.ReactNode> = {
          'draft': <Tag color="default">草稿</Tag>,
          'generating': <Tag color="processing">生成中</Tag>,
          'pending': <Tag color="orange">待审核</Tag>,
          'reviewed': <Tag color="green">已审核</Tag>,
          'published': <Tag color="blue">已发布</Tag>,
          'rejected': <Tag color="error">已拒绝</Tag>,
        };
        return statusMap[status] || status;
      },
    },
    { title: '生成人', dataIndex: 'generatedBy', key: 'generatedBy' },
    { title: '审核人', dataIndex: 'reviewedBy', key: 'reviewedBy' },
    {
      title: '操作',
      key: 'action',
      render: (_text: unknown, record: Report) => (
        <>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleView(record)}
            style={{ marginRight: 8 }}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleStatusEdit(record)}
            style={{ marginRight: 8 }}
          >
            编辑状态
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
            style={{ marginRight: 8 }}
          >
            删除
          </Button>
          {record.status === 'reviewed' || record.status === 'published' ? (
            <Button
              type="link"
              icon={<DownloadOutlined />}
              onClick={() => handleDownload(record)}
            >
              下载
            </Button>
          ) : null}
        </>
      ),
    },
  ];

  return (
    <div>
      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16, textAlign: 'center' }}>
        <Row gutter={16} justify="center">
          <Col span={6}>
            <Form.Item name="sampleCode">
              <Input placeholder="样本编号" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="patientName">
              <Input placeholder="患者姓名" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="status">
              <Select placeholder="报告状态" allowClear>
                <Select.Option value="draft">草稿</Select.Option>
                <Select.Option value="pending">待审核</Select.Option>
                <Select.Option value="generated">已生成</Select.Option>
                <Select.Option value="reviewed">已审核</Select.Option>
                <Select.Option value="published">已发布</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item>
              <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                查询
              </Button>
              <Button type="default" onClick={handleResetSearch}>重置</Button>
            </Form.Item>
          </Col>
        </Row>
      </Form>

      <Table
        columns={columns.map(col => ({
          ...col,
          align: 'center' as const
        }))}
        dataSource={reports}
        rowKey="id"
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          pageSizeOptions: ['10', '20', '50'],
          showTotal: (total) => `共 ${total} 个报告`,
          showQuickJumper: true,
          align: 'center' as const
        }}
        onChange={(page, _filters, _sorter) => {
          fetchReports({ 
            page: page.current, 
            pageSize: page.pageSize 
          });
        }}
        rowHoverable
        size="middle"
        style={{ textAlign: 'center' }}
      />

      {/* 状态更新模态框 */}
      <Modal
        title="编辑报告状态"
        open={statusModalVisible}
        onCancel={() => setStatusModalVisible(false)}
        footer={null}
      >
        <Form
          form={statusForm}
          layout="vertical"
          onFinish={handleStatusUpdate}
        >
          <Form.Item
            name="status"
            label="报告状态"
            rules={[{ required: true, message: '请选择报告状态' }]}
          >
            <Select placeholder="选择报告状态">
              <Select.Option value="draft">草稿</Select.Option>
              <Select.Option value="generating">生成中</Select.Option>
              <Select.Option value="pending">待审核</Select.Option>
              <Select.Option value="reviewed">已审核</Select.Option>
              <Select.Option value="published">已发布</Select.Option>
              <Select.Option value="rejected">已拒绝</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
              确定
            </Button>
            <Button onClick={() => setStatusModalVisible(false)}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default List;
