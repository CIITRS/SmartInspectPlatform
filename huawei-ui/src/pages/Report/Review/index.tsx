import type React from 'react';
import { useState, useEffect, useCallback } from 'react';
import { Button, Card, Col, Descriptions, Form, Input, Modal, Row, Select, Space, Spin, Tag, Typography, App } from 'antd';
import { CloseOutlined, EyeOutlined } from '@ant-design/icons';
import { useNavigate, useModel } from '@umijs/max';

import { getPendingReviewReports, getReportPdfStatus, listUsers, reviewReport } from '@/services/api';

interface Report {
  id: number;
  patientName: string;
  sampleCode: string;
  reportData: string;
  reportType?: string;
  generatedTime?: string;
  generatedBy?: string;
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

interface ModalValues {
  report_id: number;
  is_approved: boolean;
  comments?: string;
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

const Review: React.FC = () => {
  const [form] = Form.useForm();
  const [reports, setReports] = useState<Report[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchParams, setSearchParams] = useState<SearchParams>({});
  const [_selectedReport, setSelectedReport] = useState<Report | null>(null);
  const [modalVisible, setModalVisible] = useState(false);
  const [modalForm] = Form.useForm();
  const [reviewingId, setReviewingId] = useState<number | null>(null);
  const [users, setUsers] = useState<any[]>([]);
  const [isLoadingUsers, setIsLoadingUsers] = useState(false);
  const { message: appMessage } = App.useApp();
  const navigate = useNavigate();
  const { initialState } = useModel('@@initialState');
  const currentUser = initialState?.currentUser;
  
  const handleViewReport = (record: Report) => {
    navigate(`/report/view/${encodeURIComponent(record.sampleCode)}`);
  };

  const fetchReports = useCallback(async (params: SearchParams = {}) => {
    setLoading(true);
    try {
      const currentPage = params.page || 1;
      const currentPageSize = params.pageSize || 100;
      
      const response = await getPendingReviewReports({
        ...searchParams,
        page: currentPage,
        page_size: currentPageSize,
      });
      if (response.data) {
        setReports(response.data.list || []);
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

  useEffect(() => {
    const fetchUsers = async () => {
      setIsLoadingUsers(true);
      try {
        const response = await listUsers();
        const data: any = response.data;
        setUsers(Array.isArray(data) ? data : (Array.isArray(data?.list) ? data.list : []));
      } catch (_error) {
        appMessage.error('获取审核人列表失败');
      } finally {
        setIsLoadingUsers(false);
      }
    };
    fetchUsers();
  }, [appMessage]);

  const handleSearch = (values: SearchParams) => {
    setSearchParams(values);
    fetchReports({ page: 1 });
  };

  const waitForPdf = async (reportId: number) => {
    for (let i = 0; i < 20; i += 1) {
      const response = await getReportPdfStatus(String(reportId));
      if (response.data?.pdfExists) {
        return true;
      }
      await new Promise((resolve) => setTimeout(resolve, 1000));
    }
    return false;
  };

  const handleApprove = async (record: Report) => {
    try {
      let reviewerId: number | undefined;
      if (mustSelectRealReviewer(currentUser)) {
        reviewerId = await new Promise<number | undefined>((resolve) => {
        let selectedReviewerId: number | undefined;
        Modal.confirm({
          title: '选择真实审核人',
          content: (
            <Select
              style={{ width: '100%', marginTop: 12 }}
              placeholder="请选择审核人"
              loading={isLoadingUsers}
              showSearch
              optionFilterProp="children"
              onChange={(value) => {
                selectedReviewerId = Number(value);
              }}
            >
              {users.filter(isValidReviewerUser).map((user) => (
                <Select.Option key={user.id} value={user.id}>
                  {user.real_name || user.name || user.username}（{user.employee_id || user.username || '-'}）
                </Select.Option>
              ))}
            </Select>
          ),
          okText: '确认审核',
          cancelText: '取消',
          onOk: () => {
            if (!selectedReviewerId) {
              appMessage.error('请选择审核人');
              return Promise.reject();
            }
            resolve(selectedReviewerId);
            return Promise.resolve();
          },
          onCancel: () => resolve(undefined),
        });
      });
        if (!reviewerId) return;
      }
      setReviewingId(record.id);
      await reviewReport(record.id.toString(), {
        status: 'reviewed',
        rejectedReason: '',
        remarks: '',
        reviewer_id: reviewerId,
      });
      appMessage.loading('审核已通过，正在生成PDF报告', 1);
      const ready = await waitForPdf(record.id);
      if (ready) {
        appMessage.success('PDF报告已生成');
        navigate(`/report/view/${encodeURIComponent(record.sampleCode)}`);
        return;
      }
      appMessage.warning('报告已通过，PDF仍在生成中');
      fetchReports();
    } catch (_error) {
      appMessage.error('报告审核失败');
    } finally {
      setReviewingId(null);
    }
  };

  const handleReject = (record: Report) => {
    setSelectedReport(record);
    modalForm.setFieldsValue({
      report_id: record.id,
      is_approved: false,
      comments: '',
    });
    setModalVisible(true);
  };

  const handleModalSubmit = async (values: ModalValues) => {
    try {
      await reviewReport(values.report_id.toString(), {
        status: 'rejected',
        rejectedReason: values.comments,
        remarks: values.comments,
      });
      appMessage.success('报告已驳回');
      setModalVisible(false);
      fetchReports();
    } catch (_error) {
      appMessage.error('报告审核失败');
    }
  };

  const parseReportData = (reportData: string) => {
    try {
      return JSON.parse(reportData || '{}');
    } catch {
      return {};
    }
  };

  const statusTag = (status: string) => {
    const statusMap: Record<string, React.ReactNode> = {
      pending: <Tag color="orange">待审核</Tag>,
      reviewed: <Tag color="green">已审核</Tag>,
      published: <Tag color="blue">已发布</Tag>,
    };
    return statusMap[status] || <Tag>{status || '-'}</Tag>;
  };

  return (
    <div>
      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={16}>
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
            <Form.Item name="reportType">
              <Select placeholder="报告类型" allowClear>
                <Select.Option value="standard">标准报告</Select.Option>
                <Select.Option value="detailed">详细报告</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item>
              <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                查询
              </Button>
              <Button type="default" onClick={() => form.resetFields()}>重置</Button>
            </Form.Item>
          </Col>
        </Row>
      </Form>

      <Spin spinning={loading}>
        <Space direction="vertical" size={20} style={{ width: '100%' }}>
          {reports.map((report, index) => {
            const data = parseReportData(report.reportData);
            const score = Number(data.calculationResult);
            return (
              <Card
                key={report.id}
                title={`报告 ${index + 1}：${report.sampleCode}`}
                extra={statusTag(report.status)}
                actions={[
                  <Button key="view" type="link" icon={<EyeOutlined />} onClick={() => handleViewReport(report)}>查看详情</Button>,
                  report.status === 'pending'
                    ? <Space key="review-actions">
                      <Button type="primary" loading={reviewingId === report.id} onClick={() => handleApprove(report)}>通过</Button>
                      <Button danger onClick={() => handleReject(report)}>驳回</Button>
                    </Space>
                    : <Button key="reviewed" disabled>已审核</Button>,
                ]}
              >
                <Descriptions bordered size="small" column={3}>
                  <Descriptions.Item label="样本编号">{report.sampleCode || '-'}</Descriptions.Item>
                  <Descriptions.Item label="患者姓名">{report.patientName || '-'}</Descriptions.Item>
                  <Descriptions.Item label="报告类型">{report.reportType || data.reportType || '-'}</Descriptions.Item>
                  <Descriptions.Item label="计算值">
                    {Number.isFinite(score) ? score.toFixed(1) : '-'}
                  </Descriptions.Item>
                  <Descriptions.Item label="治疗阶段">{data.treatmentStageName || '-'}</Descriptions.Item>
                  <Descriptions.Item label="生成人">{report.generatedBy || '-'}</Descriptions.Item>
                </Descriptions>
                <div style={{ marginTop: 16 }}>
                  <Typography.Text strong>信号值说明</Typography.Text>
                  <Typography.Paragraph style={{ marginTop: 8, whiteSpace: 'pre-wrap' }}>
                    {data.signalValueExplanation || '-'}
                  </Typography.Paragraph>
                </div>
                <div>
                  <Typography.Text strong>结果说明</Typography.Text>
                  <Typography.Paragraph style={{ marginTop: 8, whiteSpace: 'pre-wrap' }}>
                    {data.resultExplanation || '-'}
                  </Typography.Paragraph>
                </div>
              </Card>
            );
          })}
          {!loading && reports.length === 0 && <Card>暂无待审核报告</Card>}
        </Space>
      </Spin>

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
              <Select.Option value={false}>
                <CloseOutlined style={{ color: '#ff4d4f' }} /> 驳回
              </Select.Option>
            </Select>
          </Form.Item>

          <Form.Item
            name="comments"
            label="审核意见"
            rules={[{ required: true, message: '请输入审核意见' }]}
          >
            <Input.TextArea rows={4} placeholder="请输入驳回意见" />
          </Form.Item>

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
    </div>
  );
};

export default Review;
