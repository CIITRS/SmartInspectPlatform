
import React, { useState, useEffect } from 'react';
import { Table, Button, Form, Input, Row, Col, message, Modal, Upload, Tag, Select, DatePicker, Space, Typography } from 'antd';
import { SearchOutlined, SwapOutlined, CheckCircleOutlined, ExclamationCircleOutlined, InboxOutlined, ClockCircleOutlined } from '@ant-design/icons';
import { listBatches, getBatchDetail, submitBatch, batchMultiUpload, listUsers, createSample, batchSetSampleReceiveDate } from '@/services/api';
import { useNavigate, useModel } from '@umijs/max';
import dayjs from 'dayjs';
import { shouldSuppressSecondaryError } from '@/requestErrorConfig';

const { Dragger } = Upload;
const { Text } = Typography;

interface SearchParams {
  patientName?: string;
  batchCode?: string;
  sampleKeyword?: string;
  startDate?: string;
  endDate?: string;
}

const hasTesterRole = (user: any) => {
  const roleText = Array.isArray(user?.role_names) && user.role_names.length > 0
    ? user.role_names.join('、')
    : String(user?.role_name || user?.role?.name || '');
  const accountText = [
    user?.username,
    user?.real_name,
    user?.name,
    roleText,
  ].filter(Boolean).join(' ');
  if (/系统管理员|超级管理员|实验室|实验室账号|admin|administrator/i.test(accountText)) {
    return false;
  }
  return /检测|检验|管理/.test(roleText);
};

const Center: React.FC = () => {
  const [form] = Form.useForm();
  const [batches, setBatches] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState<SearchParams>({});
  const [missingSamples, setMissingSamples] = useState<string[]>([]);
  const [missingSamplesModalVisible, setMissingSamplesModalVisible] = useState(false);
  const [createSampleModalVisible, setCreateSampleModalVisible] = useState(false);
  const [newSampleForm] = Form.useForm();
  const [submitting, setSubmitting] = useState(false);
  const [multiUploadModalVisible, setMultiUploadModalVisible] = useState(false);
  const [multiUploadFileList, setMultiUploadFileList] = useState<any[]>([]);
  const [multiUploading, setMultiUploading] = useState(false);
  const [testers, setTesters] = useState<any[]>([]);
  const [selectedUploader, setSelectedUploader] = useState<number | undefined>(undefined);
  const [selectedTester, setSelectedTester] = useState<number | undefined>(undefined);
  // 接收时间校验对话框相关状态
  const [receiveDateModalVisible, setReceiveDateModalVisible] = useState(false);
  const [samplesMissingReceiveDate, setSamplesMissingReceiveDate] = useState<any[]>([]);
  const [selectedReceiveDate, setSelectedReceiveDate] = useState<dayjs.Dayjs | null>(null);
  const [pendingUploadData, setPendingUploadData] = useState<FormData | null>(null);
  const [updatingReceiveDate, setUpdatingReceiveDate] = useState(false);
  const navigate = useNavigate();
  const { initialState } = useModel('@@initialState');

  const currentUserId = Number((initialState?.currentUser as any)?.id) || undefined;
  const currentUserName = (initialState?.currentUser as any)?.real_name
    || (initialState?.currentUser as any)?.name
    || (initialState?.currentUser as any)?.username
    || '';

  const fetchBatches = async (params: any = {}, searchOverride?: SearchParams) => {
    setLoading(true);
    try {
      let response;
      const activeSearchParams = searchOverride !== undefined ? searchOverride : searchParams;
      response = await listBatches({
        ...activeSearchParams,
        ...params
      });
      setBatches(response.data.list);
      setPagination({
        ...pagination,
        total: response.data.total,
        current: params?.page || 1,
        pageSize: params?.pageSize || pagination.pageSize,
      });
    } catch (error) {
      if (!shouldSuppressSecondaryError(error)) {
        message.error('获取批次列表失败');
      }
    } finally {
      setLoading(false);
    }
  };

  const fetchTesters = async () => {
    try {
      const response = await listUsers();
      const responseData = response.data as any;
      const userList = responseData?.list || responseData || [];
      setTesters(userList.filter((user: any) => Number(user?.status ?? 1) === 1));
    } catch (error) {
      if (!shouldSuppressSecondaryError(error)) {
        message.error('获取检测人员列表失败');
      }
    }
  };

  const handleMultiUpload = async () => {
    if (multiUploadFileList.length === 0) {
      message.error('请选择要上传的文件');
      return;
    }
    const uploaderId = selectedUploader || currentUserId;
    if (!uploaderId) {
      message.error('无法获取当前上传人，请重新登录后再试');
      return;
    }
    if (!selectedTester) {
      message.error('请选择检测人员');
      return;
    }

    setMultiUploading(true);
    
    const formData = new FormData();
    formData.append('uploaderId', uploaderId.toString());
    formData.append('testerId', selectedTester.toString());
    multiUploadFileList.forEach((file) => {
      formData.append('files', file.originFileObj);
    });

    try {
      const response = await batchMultiUpload(formData);
      message.success(response.message);
      setMultiUploadModalVisible(false);
      setMultiUploadFileList([]);
      setSelectedUploader(currentUserId);
      setSelectedTester(undefined);
      
      if (response.data?.batchCode) {
        navigate(`/result/batch/detail/${response.data.batchCode}`);
      } else if (response.data?.batchId) {
        navigate(`/result/batch/detail/${response.data.batchId}`);
      } else {
        fetchBatches();
      }
    } catch (error: any) {
      // 处理 422 状态码 - 样本缺少接收时间
      if (error.response?.status === 422 || error.code === 422) {
        const errorData = error.response?.data || error.data;
        if (errorData?.data?.samplesMissingReceiveDate) {
          // 保存待上传的数据，以便后续继续上传
          setPendingUploadData(formData);
          setSamplesMissingReceiveDate(errorData.data.samplesMissingReceiveDate);
          setReceiveDateModalVisible(true);
          setMultiUploadModalVisible(false);
        } else {
          message.error(error.message || '样本缺少接收时间');
        }
      } else {
        message.error(error.message || '上传失败');
      }
    } finally {
      setMultiUploading(false);
    }
  };

  const openMultiUploadModal = () => {
    fetchTesters();
    setSelectedUploader(currentUserId);
    setSelectedTester(undefined);
    setMultiUploadModalVisible(true);
  };

  // 处理设置接收时间并继续上传
  const handleSetReceiveDateAndContinue = async (useDetectionTime?: 'start' | 'end') => {
    if (!selectedReceiveDate && !useDetectionTime) {
      message.error('请选择接收时间');
      return;
    }

    setUpdatingReceiveDate(true);
    try {
      const sampleCodes = samplesMissingReceiveDate.map((s: any) => s.sampleCode || s.sample_code);
      
      let receiveDateStr: string;
      if (useDetectionTime === 'start') {
        // 使用检测开始时间
        const detectionStartTime = samplesMissingReceiveDate[0]?.detectionStartTime || samplesMissingReceiveDate[0]?.detection_start_time;
        if (!detectionStartTime) {
          message.error('无法获取检测开始时间');
          return;
        }
        receiveDateStr = detectionStartTime;
      } else if (useDetectionTime === 'end') {
        // 使用检测结束时间
        const detectionEndTime = samplesMissingReceiveDate[0]?.detectionEndTime || samplesMissingReceiveDate[0]?.detection_end_time;
        if (!detectionEndTime) {
          message.error('无法获取检测结束时间');
          return;
        }
        receiveDateStr = detectionEndTime;
      } else {
        // 使用用户选择的日期
        receiveDateStr = selectedReceiveDate!.format('YYYY-MM-DD HH:mm:ss');
      }

      // 批量设置样本接收时间
      await batchSetSampleReceiveDate({
        sampleCodes,
        receiveDate: receiveDateStr,
      });

      message.success('接收时间设置成功');
      setReceiveDateModalVisible(false);
      setSelectedReceiveDate(null);
      setSamplesMissingReceiveDate([]);

      // 继续上传流程
      if (pendingUploadData) {
        setMultiUploading(true);
        try {
          const response = await batchMultiUpload(pendingUploadData);
          message.success(response.message);
          setMultiUploadFileList([]);
          setSelectedUploader(currentUserId);
          setSelectedTester(undefined);
          setPendingUploadData(null);
          
          if (response.data?.batchCode) {
            navigate(`/result/batch/detail/${response.data.batchCode}`);
          } else if (response.data?.batchId) {
            navigate(`/result/batch/detail/${response.data.batchId}`);
          } else {
            fetchBatches();
          }
        } catch (error: any) {
          // 再次检查是否还有其他样本缺少接收时间
          if (error.response?.status === 422 || error.code === 422) {
            const errorData = error.response?.data || error.data;
            if (errorData?.data?.samplesMissingReceiveDate) {
              setSamplesMissingReceiveDate(errorData.data.samplesMissingReceiveDate);
              setReceiveDateModalVisible(true);
            } else {
              message.error(error.message || '样本缺少接收时间');
            }
          } else {
            message.error(error.message || '上传失败');
          }
        } finally {
          setMultiUploading(false);
        }
      }
    } catch (error: any) {
      message.error(error.message || '设置接收时间失败');
    } finally {
      setUpdatingReceiveDate(false);
    }
  };

  // 取消接收时间设置
  const handleCancelReceiveDate = () => {
    setReceiveDateModalVisible(false);
    setSelectedReceiveDate(null);
    setSamplesMissingReceiveDate([]);
    setPendingUploadData(null);
    setMultiUploadFileList([]);
  };

  useEffect(() => {
    fetchBatches();
  }, [searchParams]);

  const handleSearch = (values: any) => {
    const nextParams: SearchParams = {
      patientName: values.patientName,
      batchCode: values.batchCode,
      sampleKeyword: values.sampleKeyword,
    };
    if (values.detectTimeRange && values.detectTimeRange.length === 2) {
      nextParams.startDate = values.detectTimeRange[0].format('YYYY-MM-DD');
      nextParams.endDate = values.detectTimeRange[1].format('YYYY-MM-DD');
    }
    setSearchParams(nextParams);
  };

  const handleResetSearch = () => {
    form.resetFields();
    setSearchParams({});
  };

  const handleTableChange = (paginationConfig: any) => {
    fetchBatches({
      page: paginationConfig.current,
      pageSize: paginationConfig.pageSize
    });
  };

  const handleCreateSample = async (values: any) => {
    try {
      await createSample(values);
      message.success('样本创建成功');
      setCreateSampleModalVisible(false);
      newSampleForm.resetFields();
      fetchBatches();
    } catch (error: any) {
      message.error(error.message || '样本创建失败');
    }
  };

  const handleSubmitBatch = async (batchId: number) => {
    setSubmitting(true);
    try {
      const batchDetailResponse = await getBatchDetail(batchId.toString());
      const batchData = batchDetailResponse.data.batch;
      const missingSamplesData = batchDetailResponse.data.missingSamples || [];
      const resultsData = batchDetailResponse.data.results || [];
      const medianData = batchDetailResponse.data.medianData || [];
      const countData = batchDetailResponse.data.countData || [];
      
      const hasMissingSamples = missingSamplesData.filter((sample: string) => {
        const trimmedSample = sample.trim();
        return trimmedSample !== '无样本' && trimmedSample !== '';
      }).length > 0;
      
      if (hasMissingSamples) {
        message.error('无法提交，存在系统中不存在的样本');
        return;
      }
      
      const sampleMap = new Map();
      
      medianData.forEach((data: any) => {
        const sampleCode = data.Sample || data.sample_code;
        if (sampleCode) {
          if (!sampleMap.has(sampleCode)) {
            sampleMap.set(sampleCode, { sampleCode, median: data });
          } else {
            const sample = sampleMap.get(sampleCode);
            sample.median = data;
          }
        }
      });
      
      countData.forEach((data: any) => {
        const sampleCode = data.Sample || data.sample_code;
        if (sampleCode) {
          if (!sampleMap.has(sampleCode)) {
            sampleMap.set(sampleCode, { sampleCode, count: data });
          } else {
            const sample = sampleMap.get(sampleCode);
            sample.count = data;
          }
        }
      });
      
      let geneColumns: string[] = [];
      if (medianData.length > 0 && medianData[0]) {
        geneColumns = Object.keys(medianData[0]).filter(key => 
          key !== 'Sample' && key !== 'sample_code' && key !== 'location' && key !== 'Location' && key !== 'Total Events'
        );
      }
      
      const beadCountWarnings: string[] = [];
      sampleMap.forEach((sample: any) => {
        if (sample.count) {
          geneColumns.forEach((gene) => {
            const count = sample.count[gene];
            if (typeof count === 'number' && count < 10) {
              beadCountWarnings.push(`${sample.sampleCode}样本${gene}基因磁珠数过少`);
            }
          });
        }
      });
      
      const hasHighExpression = Array.from(sampleMap.values()).some((sample: any) => {
        if (sample.median) {
          return geneColumns.some(gene => {
            const value = sample.median[gene];
            return typeof value === 'number' && value > 100;
          });
        }
        return false;
      });
      
      const hasValidationIssues = beadCountWarnings.length > 0 || hasHighExpression;
      
      if (hasValidationIssues) {
        Modal.confirm({
          title: '确认提交',
          content: '该批次存在验证问题（表达值异常或磁珠计数过少），确定要提交吗？',
          okText: '确定',
          cancelText: '取消',
          onOk: async () => {
            try {
              const response = await submitBatch(batchId.toString());
              message.success(response.message || '提交成功');
              fetchBatches();
            } catch (error: any) {
              message.error(error.message || '提交失败');
            } finally {
              setSubmitting(false);
            }
          },
          onCancel: () => {
            setSubmitting(false);
          }
        });
      } else {
        const response = await submitBatch(batchId.toString());
        message.success(response.message || '提交成功');
        fetchBatches();
      }
    } catch (error: any) {
      message.error(error.message || '提交失败');
    } finally {
      setSubmitting(false);
    }
  };

  const columns = [
    {
      title: '批次编号',
      dataIndex: 'batchCode',
      key: 'batchCode',
      render: (text: string, record: any) => <a onClick={() => navigate(`/result/batch/detail/${record.batchCode || record.id}`)}>{text}</a>
    },
    {
      title: '样本数量',
      dataIndex: 'sampleCount',
      key: 'sampleCount'
    },
    {
      title: '上传人',
      dataIndex: 'uploaderName',
      key: 'uploaderName'
    },
    {
      title: '检测人',
      dataIndex: 'testerName',
      key: 'testerName'
    },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => {
        const statusMap = {
          'pending': { text: '待处理', color: 'orange', icon: <ExclamationCircleOutlined /> },
          'verified': { text: '已检验', color: 'green', icon: <CheckCircleOutlined /> },
          'submitted': { text: '已提交', color: 'blue', icon: <CheckCircleOutlined /> },
          'completed': { text: '已完成', color: 'green', icon: <CheckCircleOutlined /> },
          'forced_completed': { text: '强制完成', color: 'purple', icon: <CheckCircleOutlined /> },
          'import_blocked': { text: '缺失样本', color: 'red', icon: <ExclamationCircleOutlined /> },
          'withdrawn': { text: '批次撤回', color: 'default', icon: <ExclamationCircleOutlined /> }
        };
        const statusInfo = statusMap[status as keyof typeof statusMap] || { text: status, color: 'default', icon: null };
        return (
          <Tag color={statusInfo.color} icon={statusInfo.icon}>
            {statusInfo.text}
          </Tag>
        );
      }
    },
    {
      title: '上传时间',
      dataIndex: 'createdAt',
      key: 'createdAt'
    },
    {
      title: '操作',
      key: 'action',
      render: (_text: any, record: any) => (
        <div>
          <Button
            type="link"
            onClick={() => navigate(`/result/batch/detail/${record.batchCode || record.id}`)}
            style={{ marginRight: 8 }}
          >
            查看详情
          </Button>
          {record.status === 'pending' && (
            <Button
              type="primary"
              onClick={() => handleSubmitBatch(record.id)}
              loading={submitting}
            >
              提交
            </Button>
          )}
        </div>
      )
    }
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>结果中心</h2>
        <div>
          <Button 
            type="primary" 
            icon={<SwapOutlined />}
            onClick={() => navigate('/result/sample-query')}
            style={{ marginRight: 8 }}
          >
            按样本查询
          </Button>
          <Button 
            type="primary" 
            icon={<InboxOutlined />}
            onClick={openMultiUploadModal}
          >
            上传文件
          </Button>
        </div>
      </div>

      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={[16, 12]}>
          <Col span={6}>
            <Form.Item name="patientName">
              <Input placeholder="患者姓名" prefix={<SearchOutlined />} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="batchCode">
              <Input placeholder="批次编号" prefix={<SearchOutlined />} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="sampleKeyword">
              <Input placeholder="样本编号/样本条码" prefix={<SearchOutlined />} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item name="detectTimeRange">
              <DatePicker.RangePicker placeholder={['检测开始', '检测结束']} style={{ width: '100%' }} />
            </Form.Item>
          </Col>
          <Col span={6}>
            <Form.Item>
              <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                搜索
              </Button>
              <Button type="default" onClick={handleResetSearch}>
                重置
              </Button>
            </Form.Item>
          </Col>
        </Row>
      </Form>

      <Table
        columns={columns}
        dataSource={batches}
        rowKey="id"
        loading={loading}
        pagination={{
          ...pagination,
          showSizeChanger: true,
          showQuickJumper: true,
          showTotal: (total) => `共 ${total} 条记录`
        }}
        onChange={handleTableChange}
      />

      {/* 创建样本模态框 */}
      <Modal
        title="创建新样本"
        open={createSampleModalVisible}
        onCancel={() => setCreateSampleModalVisible(false)}
        footer={null}
      >
        <Form
          form={newSampleForm}
          layout="vertical"
          onFinish={handleCreateSample}
        >
          <Form.Item
            name="sampleCode"
            label="样本编号"
            rules={[{ required: true, message: '请输入样本编号' }]}
          >
            <Input placeholder="请输入样本编号" />
          </Form.Item>
          <Form.Item
            name="patientId"
            label="患者ID"
            rules={[{ required: true, message: '请输入患者ID' }]}
          >
            <Input placeholder="请输入患者ID" type="number" />
          </Form.Item>
          <Form.Item
            name="collectionTime"
            label="采集时间"
            rules={[{ required: true, message: '请输入采集时间' }]}
          >
            <Input placeholder="请输入采集时间，格式：YYYY-MM-DD HH:MM:SS" />
          </Form.Item>
          <Form.Item
            name="status"
            label="状态"
            initialValue="received"
          >
            <Input placeholder="请输入状态" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
              提交
            </Button>
            <Button onClick={() => setCreateSampleModalVisible(false)}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Modal>

      {/* 缺失样本模态框 */}
      <Modal
        title="缺失样本"
        open={missingSamplesModalVisible}
        onCancel={() => setMissingSamplesModalVisible(false)}
        footer={null}
      >
        <div style={{ marginBottom: 20 }}>
          <p>以下样本在系统中不存在：</p>
          <div style={{ margin: '16px 0' }}>
            {missingSamples.map((sample, index) => (
              <Tag key={index} color="orange" style={{ margin: '2px' }}>
                {sample}
              </Tag>
            ))}
          </div>
          <Button
            type="primary"
            style={{ marginTop: 16 }}
            onClick={() => setMissingSamplesModalVisible(false)}
          >
            进入查看详情
          </Button>
        </div>
      </Modal>

      {/* 多文件上传模态框 */}
      <Modal
        title="上传文件"
        open={multiUploadModalVisible}
        onCancel={() => {
          setMultiUploadModalVisible(false);
          setMultiUploadFileList([]);
          setSelectedUploader(currentUserId);
          setSelectedTester(undefined);
        }}
        footer={[
          <Button key="cancel" onClick={() => {
            setMultiUploadModalVisible(false);
            setMultiUploadFileList([]);
            setSelectedUploader(currentUserId);
            setSelectedTester(undefined);
          }}>
            取消
          </Button>,
          <Button key="submit" type="primary" onClick={handleMultiUpload} loading={multiUploading}>
            上传
          </Button>
        ]}
        width={600}
      >
        <div style={{ marginBottom: 20 }}>
          <p>请选择多个CSV文件上传，系统将自动识别Panel。</p>
        </div>

        <Form layout="vertical" style={{ marginBottom: 20 }}>
          <Form.Item label="上传人" required>
            <Input value={currentUserName || '当前用户'} disabled />
            <Text type="secondary" style={{ fontSize: 12 }}>
              上传人默认使用当前登录用户，无需选择。
            </Text>
          </Form.Item>
          <Form.Item label="选择检测人员" required>
            <Select
              showSearch
              optionFilterProp="label"
              placeholder="请选择检测人员"
              value={selectedTester}
              onChange={setSelectedTester}
              style={{ width: '100%' }}
            >
              {testers.filter(hasTesterRole).map((tester) => {
                const displayName = tester.real_name || tester.name || tester.username;
                return (
                  <Select.Option key={tester.id} value={tester.id} label={displayName}>
                    {displayName}
                  </Select.Option>
                );
              })}
            </Select>
          </Form.Item>
        </Form>

        <Dragger
          fileList={multiUploadFileList}
          onChange={({ fileList }) => setMultiUploadFileList(fileList)}
          beforeUpload={() => false}
          multiple
          accept=".csv"
          disabled={multiUploading}
        >
          <p className="ant-upload-drag-icon">
            <InboxOutlined />
          </p>
          <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
          <p className="ant-upload-hint">支持批量上传多个CSV文件，系统将自动识别Panel</p>
        </Dragger>
      </Modal>

      {/* 接收时间校验对话框 */}
      <Modal
        title={
          <span>
            <ClockCircleOutlined style={{ marginRight: 8, color: '#faad14' }} />
            样本缺少接收时间
          </span>
        }
        open={receiveDateModalVisible}
        onCancel={handleCancelReceiveDate}
        footer={[
          <Button key="cancel" onClick={handleCancelReceiveDate}>
            取消上传
          </Button>,
          <Button 
            key="useStart" 
            onClick={() => handleSetReceiveDateAndContinue('start')}
            loading={updatingReceiveDate}
            disabled={!samplesMissingReceiveDate[0]?.detectionStartTime && !samplesMissingReceiveDate[0]?.detection_start_time}
          >
            使用检测开始时间
          </Button>,
          <Button 
            key="useEnd" 
            onClick={() => handleSetReceiveDateAndContinue('end')}
            loading={updatingReceiveDate}
            disabled={!samplesMissingReceiveDate[0]?.detectionEndTime && !samplesMissingReceiveDate[0]?.detection_end_time}
          >
            使用检测结束时间
          </Button>,
          <Button 
            key="submit" 
            type="primary" 
            onClick={() => handleSetReceiveDateAndContinue()}
            loading={updatingReceiveDate}
            disabled={!selectedReceiveDate}
          >
            确认并继续上传
          </Button>
        ]}
        width={700}
      >
        <div style={{ marginBottom: 20 }}>
          <p style={{ color: '#666' }}>以下样本缺少接收时间，请设置接收时间后继续上传：</p>
          <div style={{ margin: '16px 0', maxHeight: 200, overflowY: 'auto', border: '1px solid #d9d9d9', borderRadius: 4, padding: 8 }}>
            {samplesMissingReceiveDate.map((sample: any, index: number) => (
              <Tag key={index} color="orange" style={{ margin: '4px' }}>
                {sample.sampleCode || sample.sample_code || sample}
              </Tag>
            ))}
          </div>
        </div>

        <Form layout="vertical">
          <Form.Item label="选择接收时间" required>
            <DatePicker
              showTime
              format="YYYY-MM-DD HH:mm:ss"
              value={selectedReceiveDate}
              onChange={(date) => setSelectedReceiveDate(date)}
              placeholder="请选择接收时间"
              style={{ width: '100%' }}
              disabled={updatingReceiveDate}
            />
          </Form.Item>
          <Form.Item label="快捷操作">
            <Space>
              <Button
                icon={<ClockCircleOutlined />}
                onClick={() => handleSetReceiveDateAndContinue('start')}
                loading={updatingReceiveDate}
                disabled={!samplesMissingReceiveDate[0]?.detectionStartTime && !samplesMissingReceiveDate[0]?.detection_start_time}
              >
                同检测开始时间
              </Button>
              <Button
                icon={<ClockCircleOutlined />}
                onClick={() => handleSetReceiveDateAndContinue('end')}
                loading={updatingReceiveDate}
                disabled={!samplesMissingReceiveDate[0]?.detectionEndTime && !samplesMissingReceiveDate[0]?.detection_end_time}
              >
                同检测结束时间
              </Button>
            </Space>
            {!samplesMissingReceiveDate[0]?.detectionStartTime && !samplesMissingReceiveDate[0]?.detection_start_time && 
             !samplesMissingReceiveDate[0]?.detectionEndTime && !samplesMissingReceiveDate[0]?.detection_end_time && (
              <span style={{ marginLeft: 8, color: '#999', fontSize: 12 }}>（检测时间信息不可用）</span>
            )}
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default Center;
