import React, { useState, useEffect, useCallback } from 'react';
import { Tabs, Table, Button, Input, Form, Row, Col, Tag, Select, Modal, App, DatePicker, Space } from 'antd';
import { EyeOutlined, DownloadOutlined, EditOutlined, DeleteOutlined, CheckOutlined, CloseOutlined } from '@ant-design/icons';
import { useNavigate, useModel } from '@umijs/max';
import { getPendingReviewReports, reviewReport, listReports, updateReportStatus, deleteReport, batchDownloadReports, listUsers, listCancerTypes } from '@/services/api';
import { formatReportProject } from '@/utils/reportProject';

const { TabPane } = Tabs;
const { RangePicker } = DatePicker;

interface Report {
  id: number;
  patientName: string;
  sampleCode: string;
  isChildReport?: boolean;
  reportRole?: string;
  patientId?: string;
  batchCode?: string;
  reportData: string;
  status: string;
  reportType?: string;
  generatedTime?: string;
  createdAt: string;
  updatedAt: string;
  generatedBy?: string;
  reviewedBy?: string;
  reportDate?: string;
  salesPerson?: string;
  salesName?: string;
  cancerTypeId?: number;
  cancerTypeName?: string;
}

interface SearchParams {
  patientName?: string;
  sampleCode?: string;
  reportType?: string;
  status?: string;
  startDate?: string;
  endDate?: string;
  salesPerson?: string;
  cancerTypeId?: number;
  page?: number;
  pageSize?: number;
}

interface ModalValues {
  report_id: number;
  is_approved: boolean;
  reviewer_id?: number;
}

interface StatusValues {
  status: string;
}

const isValidReviewerUser = (user: any) => {
  const employeeId = String(user?.employee_id || user?.employeeId || user?.username || '').trim().toLowerCase();
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || '');
  return employeeId !== 'admin' && /管理员|管理|IT/.test(roleText);
};

const mustSelectRealReviewer = (user: any) => {
  const employeeId = String(user?.employee_id || user?.employeeId || '').trim().toLowerCase();
  const username = String(user?.username || '').trim().toLowerCase();
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || '');
  return employeeId === 'admin' || username === 'admin' || /实验室/.test(roleText);
};

const isSalesUser = (user: any) => {
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || user?.position || '');
  return /销售|客户经理|sale|sales/i.test(roleText);
};

const getSalesPersonCode = (user: any) => String(user?.employee_id || user?.employeeId || user?.username || '').trim();

const getReportTypeLabel = (type?: string) => {
  const labels: Record<string, string> = {
    normal: '高敏',
    high: '超敏',
    screening: '早筛',
  };
  return labels[String(type || '').toLowerCase()] || type || '-';
};

const formatReportDate = (date?: string) => {
  if (!date) return '-';
  const parsed = new Date(date);
  if (Number.isNaN(parsed.getTime())) return String(date).slice(0, 10);
  return `${parsed.getFullYear()}-${String(parsed.getMonth() + 1).padStart(2, '0')}-${String(parsed.getDate()).padStart(2, '0')}`;
};

const ReportCenter: React.FC = () => {
  const [activeTab, setActiveTab] = useState('review');
  const [form] = Form.useForm();
  const [reviewReports, setReviewReports] = useState<Report[]>([]);
  const [completedReports, setCompletedReports] = useState<Report[]>([]);
  const [reviewLoading, setReviewLoading] = useState(true);
  const [listLoading, setListLoading] = useState(true);
  const [reviewPagination, setReviewPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [listPagination, setListPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState<SearchParams>({});
  const [selectedCompletedRowKeys, setSelectedCompletedRowKeys] = useState<React.Key[]>([]);
  const [batchDownloadVersion, setBatchDownloadVersion] = useState<'concise' | 'full'>('concise');
  const [batchDownloading, setBatchDownloading] = useState(false);
  const { message: appMessage } = App.useApp();
  const navigate = useNavigate();
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  const requiresReviewerSelection = mustSelectRealReviewer(currentUser);
  const [users, setUsers] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [isLoadingUsers, setIsLoadingUsers] = useState(false);
  
  // 审核相关
  const [selectedReport, setSelectedReport] = useState<Report | null>(null);
  const [modalVisible, setModalVisible] = useState(false);
  const [modalForm] = Form.useForm();
  
  // 状态更新相关
  const [statusModalVisible, setStatusModalVisible] = useState(false);
  const [selectedReportForStatus, setSelectedReportForStatus] = useState<Report | null>(null);
  const [statusForm] = Form.useForm();

  const renderSampleCode = (text: string, record: Report) => (
    <Space size={6}>
      <span>{text || '-'}</span>
      {(record.isChildReport || record.reportRole === 'child') && <Tag color="purple">子报告</Tag>}
    </Space>
  );

  useEffect(() => {
    const fetchFilterOptions = async () => {
      setIsLoadingUsers(true);
      try {
        const [userResponse, cancerTypeResponse] = await Promise.all([
          listUsers(),
          listCancerTypes(),
        ]);
        const userData: any = userResponse.data;
        setUsers(Array.isArray(userData) ? userData : (Array.isArray(userData?.list) ? userData.list : []));
        setCancerTypes(Array.isArray(cancerTypeResponse.data) ? cancerTypeResponse.data : []);
      } catch (_error) {
        appMessage.error('获取报告筛选选项失败');
      } finally {
        setIsLoadingUsers(false);
      }
    };
    fetchFilterOptions();
  }, [appMessage]);

  // 获取待审核报告
  const fetchReviewReports = useCallback(async (params: SearchParams = {}) => {
    setReviewLoading(true);
    try {
      const currentPage = params.page || 1;
      const currentPageSize = params.pageSize || 10;
      
      const response = await getPendingReviewReports({
        ...searchParams,
        page: currentPage,
        page_size: currentPageSize,
      });
      if (response.data) {
        setReviewReports(response.data.list || []);
        setReviewPagination({
          current: currentPage,
          pageSize: currentPageSize,
          total: response.data.total || 0,
        });
      } else {
        appMessage.error('获取待审核报告列表失败');
      }
    } catch (_error) {
      appMessage.error('获取待审核报告列表失败');
    } finally {
      setReviewLoading(false);
    }
  }, [searchParams, appMessage]);

  // 获取已完成报告
  const fetchListReports = useCallback(async (params: SearchParams = {}) => {
    setListLoading(true);
    try {
      const currentPage = params.page || 1;
      const currentPageSize = params.pageSize || 10;
      
      const response = await listReports({
        ...searchParams,
        page: currentPage,
        page_size: currentPageSize,
      });
      if (response.data) {
        const list = response.data.list || [];
        setCompletedReports(list);
        setSelectedCompletedRowKeys(prev => prev.filter(key => list.some((item: Report) => item.id === Number(key))));
        setListPagination({
          current: currentPage,
          pageSize: currentPageSize,
          total: response.data.total || 0,
        });
      } else {
        appMessage.error('获取已完成报告列表失败');
      }
    } catch (_error) {
      appMessage.error('获取已完成报告列表失败');
    } finally {
      setListLoading(false);
    }
  }, [searchParams, appMessage]);

  useEffect(() => {
    if (activeTab === 'review') {
      fetchReviewReports();
    } else if (activeTab === 'list') {
      fetchListReports();
    }
  }, [activeTab, searchParams, fetchReviewReports, fetchListReports]);

  const handleSearch = (values: any) => {
    const newParams: SearchParams = {
      patientName: values.patientName,
      sampleCode: values.sampleCode,
      reportType: values.reportType,
      status: activeTab === 'list' ? values.status : undefined,
      salesPerson: values.salesPerson,
      cancerTypeId: values.cancerTypeId,
    };
    
    if (values.dateRange && values.dateRange.length === 2) {
      newParams.startDate = values.dateRange[0].format('YYYY-MM-DD');
      newParams.endDate = values.dateRange[1].format('YYYY-MM-DD');
    }
    
    setSearchParams(newParams);
  };

  const handleResetSearch = () => {
    form.resetFields();
    setSearchParams({});
  };

  // 待审核报告操作
  const handleViewReport = (record: Report) => {
    navigate(`/report/view/${encodeURIComponent(record.sampleCode)}`);
  };

  const handleReview = (record: Report) => {
    setSelectedReport(record);
    modalForm.setFieldsValue({
      report_id: record.id,
      is_approved: true,
      reviewer_id: undefined,
    });
    setModalVisible(true);
  };

  const handleModalSubmit = async (values: ModalValues) => {
    try {
      const status = values.is_approved ? 'reviewed' : 'rejected';
      await reviewReport(values.report_id.toString(), {
        status: status,
        rejectedReason: '',
        remarks: '',
        reviewer_id: values.is_approved ? values.reviewer_id : undefined,
      });
      
      if (values.is_approved) {
        appMessage.loading('正在生成PDF报告', 2);
        setTimeout(() => {
          appMessage.success('报告审核成功');
        }, 2000);
      } else {
        appMessage.success('报告审核成功');
      }
      setModalVisible(false);
      fetchReviewReports();
    } catch (_error) {
      appMessage.error('报告审核失败');
    }
  };

  // 已完成报告操作
  const handleDownloadInstruction = () => {
    const a = document.createElement('a');
    a.href = '/Template/ReportInstruction.pdf';
    a.download = 'ReportInstruction.pdf';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  const downloadByUrl = (url: string, fileName?: string) => {
    const a = document.createElement('a');
    a.href = url;
    if (fileName) {
      a.download = fileName;
    }
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  const handleDownload = async (record: Report, version: 'concise' | 'full') => {
    try {
      const response = await fetch(`/api/reports/${record.id}/pdf/download?version=${version}`, {
        method: 'GET',
      });
      if (!response.ok) {
        throw new Error('下载失败');
      }
      const blob = await response.blob();
      const url = window.URL.createObjectURL(blob);
      downloadByUrl(url, `报告_${record.sampleCode}_${record.patientName || 'unknown'}_${version === 'concise' ? '简洁版' : '完整版'}.pdf`);
      window.URL.revokeObjectURL(url);
      appMessage.success('报告下载成功');
    } catch (_error) {
      appMessage.error('下载失败');
    }
  };

  const handleBatchDownload = async () => {
    const ids = selectedCompletedRowKeys.map(key => Number(key)).filter(id => !Number.isNaN(id));
    if (ids.length === 0) {
      appMessage.warning('请先选择要下载的已审核报告');
      return;
    }
    setBatchDownloading(true);
    try {
      const response = await batchDownloadReports({ ids, version: batchDownloadVersion });
      if (response.data?.downloadUrl) {
        downloadByUrl(response.data.downloadUrl, response.data.fileName);
        appMessage.success(`已生成 ${response.data.count || ids.length} 个报告的批量下载`);
      } else {
        appMessage.error('生成批量下载失败');
      }
    } catch (_error) {
      appMessage.error('批量下载失败');
    } finally {
      setBatchDownloading(false);
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
      appMessage.success('状态更新成功');
      setStatusModalVisible(false);
      fetchListReports();
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  const handleDelete = async (record: Report) => {
    try {
      const response = await deleteReport(record.id.toString());
      if (response.data) {
        appMessage.success('报告删除成功');
        fetchListReports();
      } else {
        appMessage.error('报告删除失败');
      }
    } catch (_error) {
      appMessage.error('删除失败');
    }
  };

  // 待审核报告列
  const reviewColumns = [
    { title: '样本编号', dataIndex: 'sampleCode', key: 'sampleCode', width: 160, ellipsis: true, render: renderSampleCode },
    { title: '患者姓名', dataIndex: 'patientName', key: 'patientName', width: 110, ellipsis: true },
    { title: '销售', dataIndex: 'salesName', key: 'salesName', width: 110, ellipsis: true, render: (value: string) => value || '-' },
    {
      title: '报告日期',
      dataIndex: 'reportDate',
      key: 'reportDate',
      width: 115,
      render: formatReportDate,
    },
    { title: '检测类型', dataIndex: 'reportType', key: 'reportType', width: 100, render: getReportTypeLabel },
    { title: '项目', dataIndex: 'cancerTypeName', key: 'project', width: 230, ellipsis: true, render: (value: string, record: Report) => formatReportProject(value, record.reportType) || '-' },
    {
      title: '状态', 
      dataIndex: 'status', 
      key: 'status',
      render: (status: string) => {
        const statusMap: Record<string, React.ReactNode> = {
          'pending': <Tag color="orange">待审核</Tag>,
          'reviewed': <Tag color="green">已审核</Tag>,
          'published': <Tag color="blue">已发布</Tag>,
        };
        return statusMap[status] || status;
      },
    },
    { title: '生成人', dataIndex: 'generatedBy', key: 'generatedBy', width: 100, ellipsis: true },
    {
      title: '操作', 
      key: 'action', 
      width: 180,
      fixed: 'right' as const,
      render: (_text: unknown, record: Report) => (
        <div>
          <Button 
            type="link" 
            icon={<EyeOutlined />}
            onClick={() => handleViewReport(record)}
            style={{ marginRight: 8 }}
          >
            查看详情
          </Button>
          {record.status === 'pending' ? (
            <Button 
              type="primary" 
              onClick={() => handleReview(record)}
            >
              审核
            </Button>
          ) : (
            <Button disabled>
              已审核
            </Button>
          )}
        </div>
      ),
    },
  ];

  // 已完成报告列
  const listColumns = [
    { title: '样本编号', dataIndex: 'sampleCode', key: 'sampleCode', width: 160, ellipsis: true, render: renderSampleCode },
    { title: '患者姓名', dataIndex: 'patientName', key: 'patientName', width: 110, ellipsis: true },
    { title: '销售', dataIndex: 'salesName', key: 'salesName', width: 110, ellipsis: true, render: (value: string) => value || '-' },
    { title: '报告日期', dataIndex: 'reportDate', key: 'reportDate', width: 115, render: formatReportDate },
    { title: '检测类型', dataIndex: 'reportType', key: 'reportType', width: 100, render: getReportTypeLabel },
    { title: '项目', dataIndex: 'cancerTypeName', key: 'project', width: 230, ellipsis: true, render: (value: string, record: Report) => formatReportProject(value, record.reportType) || '-' },
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
    { title: '生成人', dataIndex: 'generatedBy', key: 'generatedBy', width: 100, ellipsis: true },
    { title: '审核人', dataIndex: 'reviewedBy', key: 'reviewedBy', width: 100, ellipsis: true },
    {
      title: '操作',
      key: 'action',
      width: 430,
      fixed: 'right' as const,
      render: (_text: unknown, record: Report) => (
        <Space wrap>
          <Button
            type="link"
            icon={<EyeOutlined />}
            onClick={() => handleViewReport(record)}
          >
            查看
          </Button>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleStatusEdit(record)}
          >
            编辑状态
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDelete(record)}
          >
            删除
          </Button>
          {record.status === 'reviewed' || record.status === 'published' ? (
            <>
              <Button type="link" icon={<DownloadOutlined />} onClick={() => handleDownload(record, 'concise')}>
                简洁报告
              </Button>
              <Button type="link" onClick={handleDownloadInstruction}>
                说明书
              </Button>
              <Button type="link" icon={<DownloadOutlined />} onClick={() => handleDownload(record, 'full')}>
                完整报告
              </Button>
            </>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'flex-end', marginBottom: 12 }}>
        <Button type="primary" onClick={() => navigate('/report/templates')}>
          报告模板
        </Button>
      </div>
      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={[16, 12]} style={{ width: '100%' }}>
          <Col span={6}>
            <Form.Item name="patientName">
              <Input placeholder="患者姓名" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="sampleCode">
              <Input placeholder="样本编号" />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="dateRange">
              <RangePicker placeholder={['开始日期', '结束日期']} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="salesPerson">
              <Select placeholder="销售" allowClear showSearch optionFilterProp="children" loading={isLoadingUsers}>
                {users.filter(isSalesUser).map((user) => (
                  <Select.Option key={user.id || getSalesPersonCode(user)} value={getSalesPersonCode(user)}>
                    {user.real_name || user.name || user.username}
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="reportType">
              <Select placeholder="检测类型" allowClear>
                <Select.Option value="normal">高敏</Select.Option>
                <Select.Option value="high">超敏</Select.Option>
                <Select.Option value="screening">早筛</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="cancerTypeId">
              <Select placeholder="项目" allowClear showSearch optionFilterProp="children">
                {cancerTypes.map((item: any) => (
                  <Select.Option key={item.id} value={item.id}>{item.name}</Select.Option>
                ))}
              </Select>
            </Form.Item>
          </Col>
          {activeTab === 'list' && (
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
          )}
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

      <Tabs activeKey={activeTab} onChange={setActiveTab}>
        <TabPane tab="待审核" key="review">
          <Table
            columns={reviewColumns}
            dataSource={reviewReports}
            rowKey="id"
            loading={reviewLoading}
            pagination={reviewPagination}
            onChange={(page) => fetchReviewReports({ page: page.current, pageSize: page.pageSize })}
            scroll={{ x: 1350 }}
          />
        </TabPane>
        <TabPane tab="已完成" key="list">
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 12 }}>
            <Space>
              <Select
                value={batchDownloadVersion}
                onChange={setBatchDownloadVersion}
                style={{ width: 140 }}
              >
                <Select.Option value="concise">简洁版报告</Select.Option>
                <Select.Option value="full">完整版报告</Select.Option>
              </Select>
              <Button
                type="primary"
                icon={<DownloadOutlined />}
                loading={batchDownloading}
                disabled={selectedCompletedRowKeys.length === 0}
                onClick={handleBatchDownload}
              >
                批量下载报告
              </Button>
            </Space>
            <span style={{ color: '#666' }}>已选择 {selectedCompletedRowKeys.length} 个报告</span>
          </div>
          <Table
            columns={listColumns.map(col => ({
              ...col,
              align: 'center' as const
            }))}
            dataSource={completedReports}
            rowKey="id"
            rowSelection={{
              selectedRowKeys: selectedCompletedRowKeys,
              onChange: (keys) => setSelectedCompletedRowKeys(keys),
              getCheckboxProps: (record: Report) => ({
                disabled: record.status !== 'reviewed' && record.status !== 'published',
              }),
            }}
            loading={listLoading}
            pagination={{
              ...listPagination,
              showSizeChanger: true,
              pageSizeOptions: ['10', '20', '50'],
              showTotal: (total) => `共 ${total} 个报告`,
              showQuickJumper: true,
              align: 'center' as const
            }}
            onChange={(page) => fetchListReports({ page: page.current, pageSize: page.pageSize })}
            rowHoverable
            size="middle"
            scroll={{ x: 1750 }}
          />
        </TabPane>
      </Tabs>

      {/* 审核模态框 */}
      <Modal
        title="报告审核"
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
            name="report_id"
            label="报告ID"
            hidden
          >
            <Input />
          </Form.Item>

          <Form.Item
            name="is_approved"
            label="审核结果"
            rules={[{ required: true, message: '请选择审核结果' }]}
          >
            <Select placeholder="请选择审核结果">
              <Select.Option value={true}>
                <CheckOutlined style={{ color: '#52c41a' }} /> 通过
              </Select.Option>
              <Select.Option value={false}>
                <CloseOutlined style={{ color: '#ff4d4f' }} /> 驳回
              </Select.Option>
            </Select>
          </Form.Item>

          {requiresReviewerSelection && (
            <Form.Item
              name="reviewer_id"
              label="真实审核人"
              dependencies={['is_approved']}
              rules={[
                ({ getFieldValue }) => ({
                  validator: (_, value) => {
                    if (getFieldValue('is_approved') === true && !value) {
                      return Promise.reject(new Error('请选择真实审核人'));
                    }
                    return Promise.resolve();
                  },
                }),
              ]}
            >
              <Select
                placeholder="请选择真实审核人"
                loading={isLoadingUsers}
                showSearch
                optionFilterProp="children"
                allowClear
              >
                {users.filter(isValidReviewerUser).map((user) => (
                  <Select.Option key={user.id} value={user.id}>
                    {user.real_name || user.name || user.username}（{user.employee_id || user.username || '-'}）
                  </Select.Option>
                ))}
              </Select>
            </Form.Item>
          )}

          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
              提交审核
            </Button>
            <Button onClick={() => setModalVisible(false)}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Modal>

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

export default ReportCenter;
