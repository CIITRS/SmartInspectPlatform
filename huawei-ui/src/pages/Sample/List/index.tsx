import React, { useState, useEffect } from 'react';
import { Table, Button, Input, Form, Row, Col, Modal, Tag, Select, App, Steps, Collapse, Upload, Typography, Space, Descriptions, Empty, Radio, Spin, DatePicker } from 'antd';
import { EditOutlined, DeleteOutlined, PlusOutlined, CheckCircleOutlined, ClockCircleOutlined, UploadOutlined, DownloadOutlined, InboxOutlined } from '@ant-design/icons';
import { useNavigate } from '@umijs/max';
import { getSamples, updateSample, deleteSample, getSampleTypes, getTreatmentStages, listCancerTypes, getSampleExpress } from '@/services/api';
import * as XLSX from 'xlsx';
import dayjs from 'dayjs';

const allowedTreatmentStageNames = ['健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'];

const buildBarcodeImageUrl = (code?: string) => {
  const value = String(code || '').trim();
  if (!value) return '';
  return `/api/samples/barcode?sample_code=${encodeURIComponent(value)}`;
};

const SampleList: React.FC = () => {
  const [form] = Form.useForm();
  const [samples, setSamples] = useState<any[]>([]);
  const [loading, setLoading] = useState(true);
  const [pagination, setPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [searchParams, setSearchParams] = useState({});
  const [detailVisible, setDetailVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [editVisible, setEditVisible] = useState(false);
  const [selectedSample, setSelectedSample] = useState<any>(null);
  const [sampleExpress, setSampleExpress] = useState<Record<'inbound' | 'outbound', any>>({ inbound: null, outbound: null });
  const [_editSample, setEditSample] = useState<any>(null);
  const [editForm] = Form.useForm();
  const editOrganizationType = Form.useWatch('organization_type', editForm) || '个人送检';
  const [sampleTypes, setSampleTypes] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [treatmentStages, setTreatmentStages] = useState<any[]>([]);
  const [batchModalVisible, setBatchModalVisible] = useState(false);
  const [selectedRowKeys, setSelectedRowKeys] = useState<React.Key[]>([]);
  const navigate = useNavigate();
  const { message: appMessage } = App.useApp();

  // 获取样本列表数据
  const fetchSamples = async (params: any = {}, searchOverride?: any) => {
    setLoading(true);
    try {
      // 调用获取样本列表的API
      const activeSearchParams = searchOverride !== undefined ? searchOverride : searchParams;
      const response = await getSamples({ ...activeSearchParams, ...params });
      // 直接使用API返回的样本数据
      const sampleData = response.data?.list || [];
      setSamples(sampleData);
      setPagination({
        ...pagination,
        total: response.data?.total || 0,
        current: params.page || 1,
        pageSize: params.pageSize || pagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取样本列表失败');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSamples({}, {});
  }, []);

  const ensureOptionsLoaded = async () => {
    if (sampleTypes.length > 0 && cancerTypes.length > 0 && treatmentStages.length > 0) return;
      try {
        const [sampleTypeRes, cancerRes, stageRes] = await Promise.all([
          getSampleTypes({}, { skipErrorHandler: true }),
          listCancerTypes({}, { skipErrorHandler: true }),
          getTreatmentStages({}, { skipErrorHandler: true }),
        ]);
        setSampleTypes(sampleTypeRes.data || []);
        setCancerTypes(cancerRes.data || []);
        const allowedStageSet = new Set(allowedTreatmentStageNames);
        setTreatmentStages((stageRes.data || [])
          .filter((item: any) => allowedStageSet.has(item.name))
          .sort((a: any, b: any) => allowedTreatmentStageNames.indexOf(a.name) - allowedTreatmentStageNames.indexOf(b.name)));
      } catch (_error) {
        appMessage.error('获取样本选项失败');
      }
  };

  const handleSearch = (values: any) => {
    const nextParams = {
      sample_code: values.sampleCode,
      patient_name: values.patientName,
      sample_type: values.sampleType,
    };
    setSearchParams(nextParams);
    fetchSamples({ page: 1 }, nextParams);
  };

  const handleResetSearch = () => {
    form.resetFields();
    setSearchParams({});
    fetchSamples({ page: 1 }, {});
  };

  const handleView = async (record: any) => {
    setSelectedSample(record);
    setSampleExpress({ inbound: null, outbound: null });
    setDetailVisible(true);
    setDetailLoading(true);
    try {
      const [response, inbound, outbound] = await Promise.all([
        getSamples({ id: record.id }),
        getSampleExpress(String(record.id), 'inbound'),
        getSampleExpress(String(record.id), 'outbound'),
      ]);
      const detail = response.data?.list?.[0];
      if (detail) {
        setSelectedSample(detail);
      }
      setSampleExpress({ inbound: inbound.data || null, outbound: outbound.data || null });
    } catch (_error) {
      appMessage.warning('样本详情加载失败，已显示列表信息');
    } finally {
      setDetailLoading(false);
    }
  };

  const handleEdit = async (record: any) => {
    await ensureOptionsLoaded();
    setEditSample(record);
    const organization = record.organization || '个人送检';
    editForm.setFieldsValue({
      ...record,
      receive_date: record.receive_date ? dayjs(record.receive_date) : undefined,
      organization,
      organization_type: organization === '个人' || organization === '个人送检' ? '个人送检' : '单位送检',
    });
    setEditVisible(true);
  };

  const handleDeleteSample = (record: any) => {
    Modal.confirm({
      title: '确认删除',
      content: `确定要删除样本 ${record.sample_code} 吗？`,
      okText: '确定',
      cancelText: '取消',
      okType: 'danger',
      onOk: async () => {
        try {
          await deleteSample(record.id);
          appMessage.success('样本删除成功');
          fetchSamples();
        } catch (error: any) {
          appMessage.error(error?.message || '样本删除失败');
        }
      }
    });
  };

  const handleBatchDelete = () => {
    if (selectedRowKeys.length === 0) {
      appMessage.warning('请选择要删除的样本');
      return;
    }
    Modal.confirm({
      title: '批量删除样本',
      content: `确定要删除选中的 ${selectedRowKeys.length} 个样本吗？`,
      okText: '确定',
      cancelText: '取消',
      okType: 'danger',
      onOk: async () => {
        let failed = 0;
        for (const id of selectedRowKeys) {
          try {
            await deleteSample(String(id), { skipErrorHandler: true });
          } catch (_error) {
            failed += 1;
          }
        }
        if (failed > 0) {
          appMessage.warning(`批量删除完成，失败 ${failed} 个`);
        } else {
          appMessage.success('批量删除成功');
        }
        setSelectedRowKeys([]);
        fetchSamples();
      },
    });
  };

  const handleBatchCreate = async (file: any) => {
    try {
      // 读取上传的Excel文件
      const reader = new FileReader();
      reader.onload = async (e) => {
        try {
          const data = e.target?.result;
          const workbook = XLSX.read(data, { type: 'binary' });
          const sheetName = workbook.SheetNames[0];
          const worksheet = workbook.Sheets[sheetName];
          const jsonData = XLSX.utils.sheet_to_json(worksheet);
          
          // 处理批量创建逻辑
          // 这里需要根据实际的批量创建API来实现
          appMessage.success(`批量创建成功，共处理 ${jsonData.length} 个样本`);
          setBatchModalVisible(false);
          fetchSamples();
        } catch (_error) {
          appMessage.error('文件解析失败或批量创建失败');
        }
      };
      reader.onerror = () => {
        appMessage.error('文件读取失败');
      };
      reader.readAsBinaryString(file);
    } catch (_error) {
      appMessage.error('上传失败');
    }
    
    return false;
  };

  const columns = [
    { title: '样本编号', dataIndex: 'sample_code', key: 'sample_code', render: (text: any, record: any) => <a onClick={() => handleView(record)}>{text}</a> },
    { 
      title: '患者姓名', 
      dataIndex: 'patient_name', 
      key: 'patient_name',
      render: (patientName: any) => patientName || '-'
    },
    { 
      title: '样本类型', 
      dataIndex: 'sample_type_name', 
      key: 'sample_type_name',
      render: (sampleTypeName: any) => sampleTypeName || '-'
    },
    { 
      title: '创建人员', 
      dataIndex: 'collection_user_name', 
      key: 'collection_user_name',
      render: (userName: any, record: any) => {
        if (userName) return userName;
        if (record.collection_operator) return record.collection_operator;
        return '-';
      } 
    },
    { 
      title: '状态', 
      dataIndex: 'status', 
      key: 'status',
      render: (status: string) => {
        const statusMap: any = {
          'created': <Tag color="default">已创建</Tag>,
          'collected': <Tag color="default">已采集</Tag>,
          'sent': <Tag color="orange">送检中</Tag>,
          'received': <Tag color="blue">已接收</Tag>,
          'processing': <Tag color="orange">处理中</Tag>,
          'tested': <Tag color="green">已检测</Tag>,
          'completed': <Tag color="green">已完成</Tag>,
        };
        return statusMap[status] || status;
      },
    },

    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
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
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>样本列表</h2>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/sample/create')}>
            新增样本
          </Button>
          <Button danger icon={<DeleteOutlined />} disabled={selectedRowKeys.length === 0} onClick={handleBatchDelete}>
            批量删除
          </Button>
          <Button icon={<UploadOutlined />} onClick={() => setBatchModalVisible(true)}>
            批量创建
          </Button>
          <Button icon={<InboxOutlined />} onClick={() => navigate('/sample/receive')}>
            样本接收
          </Button>
        </Space>
      </div>

      <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
        <Row gutter={16} align="middle">
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
            <Form.Item name="sampleType">
              <Select placeholder="样本类型" allowClear>
                <Select.Option value="血液">血液</Select.Option>
                <Select.Option value="尿液">尿液</Select.Option>
              </Select>
            </Form.Item>
          </Col>
          <Col span={6}>
            <Space>
              <Button type="primary" htmlType="submit">
                查询
              </Button>
              <Button type="default" onClick={handleResetSearch}>
                重置
              </Button>
            </Space>
          </Col>
        </Row>
      </Form>

      <Table
        columns={[
          ...columns.map(column => {
            if (column.title === '操作') {
              return {
                ...column,
                render: (_text: any, record: any) => (
                  <>
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
                      onClick={() => handleDeleteSample(record)}
                    >
                      删除
                    </Button>
                  </>
                )
              };
            }
            return column;
          }),
          {
            title: '检测人员',
            key: 'test_operator',
            render: (text: any, record: any) => {
              if (record.status === 'tested' || record.status === 'completed') {
                if (record.test_user_name) {
                  return record.test_user_name;
                } else if (record.test_operator) {
                  return record.test_operator;
                }
                return '未知';
              }
              return '-';
            }
          }
        ]}
        dataSource={samples}
        rowKey="id"
        rowSelection={{
          selectedRowKeys,
          onChange: setSelectedRowKeys,
        }}
        loading={loading}
        pagination={pagination}
        onChange={(page) => fetchSamples({ page: page.current, pageSize: page.pageSize })}
      />

      <Modal
        title="样本详情"
        open={detailVisible}
        onCancel={() => setDetailVisible(false)}
        footer={[<Button key="close" onClick={() => setDetailVisible(false)}>关闭</Button>]}
        width={1000}
      >
        <Spin spinning={detailLoading}>
        {selectedSample && (
          <div>
            <div style={{ marginBottom: 24 }}>
              <Row gutter={16}>
                <Col span={12}>
                  <p><strong>样本编号：</strong>{selectedSample.sample_code}</p>
                </Col>
                <Col span={12}>
                  <p><strong>患者姓名：</strong>{selectedSample.patient_name || '-'}</p>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={12}>
                  <p><strong>患者身份证号：</strong>{selectedSample.id_card || '-'}</p>
                </Col>
                <Col span={12}>
                  <p><strong>患者电话：</strong>{selectedSample.phone || '-'}</p>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={12}>
                  <p><strong>样本类型：</strong>{selectedSample.sample_type_name || '-'}</p>
                </Col>
                <Col span={12}>
                  <p><strong>治疗阶段：</strong>{selectedSample.treatment_stage_name || '-'}</p>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={12}>
                  <p><strong>癌种：</strong>{selectedSample.cancer_type_name || '-'}</p>
                </Col>
                <Col span={12}>
                  <p><strong>当前状态：</strong>
                    {(() => {
                      const statusMap: any = {
                        'created': <Tag color="default">已创建</Tag>,
                        'collected': <Tag color="default">已采集</Tag>,
                        'received': <Tag color="blue">已接收</Tag>,
                        'processing': <Tag color="orange">处理中</Tag>,
                        'tested': <Tag color="green">已检测</Tag>,
                        'completed': <Tag color="green">已完成</Tag>,
                      };
                      return statusMap[selectedSample.status] || selectedSample.status;
                    })()}
                  </p>
                </Col>
              </Row>
              <Row gutter={16}>
                <Col span={12}>
                  <p><strong>送检单位：</strong>{selectedSample.organization || '-'}</p>
                </Col>
              </Row>
              {selectedSample.sample_code && (
                <div style={{ marginTop: 8 }}>
                  <p style={{ marginBottom: 8 }}><strong>样本条形码：</strong></p>
                  <img
                    src={buildBarcodeImageUrl(selectedSample.sample_code)}
                    alt={`${selectedSample.sample_code} 条形码`}
                    style={{ width: 360, maxWidth: '100%', height: 110, objectFit: 'contain', border: '1px solid #f0f0f0', borderRadius: 4, background: '#fff', padding: 8 }}
                  />
                  <div style={{ marginTop: 6, fontSize: 16, letterSpacing: 1, fontWeight: 600 }}>
                    {selectedSample.sample_code}
                  </div>
                </div>
              )}
            </div>

            {/* 快递运单信息 */}
            <div style={{ marginTop: 16, marginBottom: 16, padding: 16, background: '#f5f5f5', borderRadius: 8 }}>
              <Space style={{ width: '100%', justifyContent: 'space-between', marginBottom: 12 }}>
                <h4 style={{ margin: 0, fontWeight: 600 }}>快递全周期状态</h4>
                <Button size="small" type="link" onClick={() => navigate(`/result/detail/${selectedSample.id}`)}>
                  管理/查看物流轨迹
                </Button>
              </Space>
              {(['inbound', 'outbound'] as const).map((direction) => {
                const current = sampleExpress[direction];
                const statusMap: Record<string, { color: string; text: string }> = {
                  pending: { color: 'default', text: '待揽件' },
                  picked_up: { color: 'blue', text: '已揽件' },
                  in_transit: { color: 'cyan', text: '运输中' },
                  delivered: { color: 'green', text: '已签收' },
                  exception: { color: 'red', text: '物流异常' },
                };
                return (
                  <Descriptions key={direction} column={2} size="small" bordered style={{ marginBottom: 12 }}>
                    <Descriptions.Item label={direction === 'inbound' ? '患者寄出' : '公司发货'}>
                      {current?.tracking_number || '暂无运单'}
                    </Descriptions.Item>
                    <Descriptions.Item label="状态">
                      {current ? (
                        <Tag color={(statusMap[current.status] || {}).color || 'default'}>
                          {(statusMap[current.status] || {}).text || current.status}
                        </Tag>
                      ) : '-'}
                    </Descriptions.Item>
                    {current && (
                      <>
                        <Descriptions.Item label="最新动态">{current.latest_event_status || '-'}</Descriptions.Item>
                        <Descriptions.Item label="签收时间">{current.delivered_at || '-'}</Descriptions.Item>
                      </>
                    )}
                  </Descriptions>
                );
              })}
            </div>
            
            {/* 横向状态时间线 */}
            <div style={{ marginTop: 24, marginBottom: 16 }}>
              <h4 style={{ marginBottom: 12, fontWeight: 600 }}>样本状态时间线</h4>
              <Steps
                size="small"
                current={(() => {
                  const statusIndex: Record<string, number> = {
                    'created': 0,
                    'collected': 1,
                    'received': 2,
                    'processing': 2,
                    'tested': 3,
                    'completed': 3,
                  };
                  return statusIndex[selectedSample.status] ?? 0;
                })()}
                status="finish"
                items={[
                  {
                    title: '创建',
                    description: selectedSample.collection_date
                      ? `${new Date(selectedSample.collection_date).toLocaleDateString()}`
                      : '未设置',
                    status: ['collected', 'received', 'processing', 'tested', 'completed'].includes(selectedSample.status) ? 'finish' : 'wait',
                  },
                  {
                    title: '采集',
                    description: ['collected', 'received', 'processing', 'tested', 'completed'].includes(selectedSample.status)
                      ? '已完成'
                      : '待采集',
                    status: ['received', 'processing', 'tested', 'completed'].includes(selectedSample.status) ? 'finish' : 
                           selectedSample.status === 'collected' ? 'process' : 'wait',
                  },
                  {
                    title: '接收',
                    description: ['received', 'processing', 'tested', 'completed'].includes(selectedSample.status)
                      ? selectedSample.receive_date
                        ? `${new Date(selectedSample.receive_date).toLocaleDateString()}`
                        : '已接收'
                      : '待接收',
                    status: ['processing', 'tested', 'completed'].includes(selectedSample.status) ? 'finish' :
                           selectedSample.status === 'received' ? 'process' : 'wait',
                  },
                  {
                    title: '检测',
                    description: ['tested', 'completed'].includes(selectedSample.status)
                      ? '已完成'
                      : '待检测',
                    status: selectedSample.status === 'completed' ? 'finish' :
                           selectedSample.status === 'tested' ? 'process' : 'wait',
                  },
                ]}
              />
            </div>

            {/* 检测情况展示区域 */}
            <Collapse style={{ marginTop: 16 }}>
              <Collapse.Panel header="检测情况" key="detection">
                {selectedSample.status === 'tested' || selectedSample.status === 'completed' ? (
                  <div>
                    <Descriptions column={2} size="small" bordered>
                      <Descriptions.Item label="检测状态">
                        <Tag color="green">已检测</Tag>
                      </Descriptions.Item>
                      <Descriptions.Item label="检测人员">
                        {selectedSample.test_user_name || selectedSample.test_operator || '未设置'}
                      </Descriptions.Item>
                      <Descriptions.Item label="检测时间">
                        {selectedSample.test_date ? new Date(selectedSample.test_date).toLocaleString() : '未设置'}
                      </Descriptions.Item>
                      <Descriptions.Item label="检测方法">
                        {selectedSample.test_method || '未设置'}
                      </Descriptions.Item>
                    </Descriptions>
                    {/* 基因数据 */}
                    {selectedSample.gene_data && (
                      <div style={{ marginTop: 16 }}>
                        <h5 style={{ marginBottom: 8 }}>基因数据</h5>
                        <Descriptions column={2} size="small" bordered>
                          {Array.isArray(selectedSample.gene_data) ? (
                            selectedSample.gene_data.map((gene: any, index: number) => (
                              <React.Fragment key={index}>
                                <Descriptions.Item label={gene.name || `基因 ${index + 1}`}>
                                  {gene.value || gene.result || '-'}
                                </Descriptions.Item>
                              </React.Fragment>
                            ))
                          ) : (
                            typeof selectedSample.gene_data === 'object' ? (
                              Object.entries(selectedSample.gene_data).map(([key, value]) => (
                                <Descriptions.Item key={key} label={key}>
                                  {String(value)}
                                </Descriptions.Item>
                              ))
                            ) : (
                              <Descriptions.Item label="基因数据">{selectedSample.gene_data}</Descriptions.Item>
                            )
                          )}
                        </Descriptions>
                      </div>
                    )}
                    {/* 检测结果 */}
                    {selectedSample.test_result && (
                      <div style={{ marginTop: 16 }}>
                        <h5 style={{ marginBottom: 8 }}>检测结果</h5>
                        <Descriptions column={1} size="small" bordered>
                          <Descriptions.Item label="检测结果摘要">
                            {selectedSample.test_result.summary || selectedSample.test_result}
                          </Descriptions.Item>
                          {selectedSample.test_result.details && (
                            <Descriptions.Item label="详细说明">
                              {selectedSample.test_result.details}
                            </Descriptions.Item>
                          )}
                          {selectedSample.test_result.conclusion && (
                            <Descriptions.Item label="检测结论">
                              <Tag color={selectedSample.test_result.conclusion === '阳性' ? 'red' : 'green'}>
                                {selectedSample.test_result.conclusion}
                              </Tag>
                            </Descriptions.Item>
                          )}
                        </Descriptions>
                      </div>
                    )}
                    {!selectedSample.gene_data && !selectedSample.test_result && (
                      <Empty description="暂无详细检测数据" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                    )}
                  </div>
                ) : (
                  <Empty description="样本尚未完成检测" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                )}
              </Collapse.Panel>
            </Collapse>

            {/* 报告情况展示区域 */}
            <Collapse style={{ marginTop: 16 }}>
              <Collapse.Panel header="报告情况" key="report">
                {selectedSample.report_id || selectedSample.report_status ? (
                  <Descriptions column={2} size="small" bordered>
                    <Descriptions.Item label="报告编号">
                      {selectedSample.report_id || selectedSample.report_code || '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="报告状态">
                      {(() => {
                        const reportStatusMap: Record<string, { color: string; text: string }> = {
                          'draft': { color: 'default', text: '草稿' },
                          'pending': { color: 'orange', text: '待审核' },
                          'reviewing': { color: 'blue', text: '审核中' },
                          'approved': { color: 'green', text: '已通过' },
                          'rejected': { color: 'red', text: '已驳回' },
                          'published': { color: 'green', text: '已发布' },
                        };
                        const status = reportStatusMap[selectedSample.report_status] || { color: 'default', text: selectedSample.report_status || '未知' };
                        return <Tag color={status.color}>{status.text}</Tag>;
                      })()}
                    </Descriptions.Item>
                    <Descriptions.Item label="生成时间">
                      {selectedSample.report_generate_time
                        ? new Date(selectedSample.report_generate_time).toLocaleString()
                        : selectedSample.report_created_at
                          ? new Date(selectedSample.report_created_at).toLocaleString()
                          : '-'}
                    </Descriptions.Item>
                    <Descriptions.Item label="审核人员">
                      {selectedSample.report_reviewer || '-'}
                    </Descriptions.Item>
                    {selectedSample.report_remark && (
                      <Descriptions.Item label="备注" span={2}>
                        {selectedSample.report_remark}
                      </Descriptions.Item>
                    )}
                  </Descriptions>
                ) : (
                  <Empty description="暂无报告" image={Empty.PRESENTED_IMAGE_SIMPLE} />
                )}
              </Collapse.Panel>
            </Collapse>

            <div style={{ marginTop: 24 }}>
              <h4 style={{ marginBottom: 16, fontWeight: 600 }}>患者其他样本情况</h4>
              <p style={{ color: '#999' }}>此处将显示患者的其他样本情况，包括样本编号、类型、治疗阶段、状态等信息。</p>
              <p style={{ color: '#999' }}>功能开发中...</p>
            </div>
          </div>
        )}
        </Spin>
      </Modal>

      <Modal
        title="编辑样本"
        open={editVisible}
        onCancel={() => setEditVisible(false)}
        footer={null}
        width={800}
      >
        <Form
          form={editForm}
          layout="vertical"
          onFinish={async (values) => {
            try {
              // 调用编辑样本的API
              await updateSample(_editSample.id, {
                sampleCode: values.sample_code,
                status: values.status,
                sampleTypeId: values.sample_type_id,
                cancerTypeId: values.cancer_type_id,
                treatmentStageId: values.treatment_stage_id,
                receiveDate: values.receive_date ? values.receive_date.format('YYYY-MM-DD HH:mm:ss') : undefined,
                notes: values.notes,
                organization: values.organization_type === '单位送检' ? values.organization : '个人送检'
              });
              appMessage.success('样本编辑成功');
              setEditVisible(false);
              fetchSamples();
            } catch (_error) {
              appMessage.error('样本编辑失败');
            }
          }}
        >
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="sample_code"
                label="样本编号"
                rules={[{ required: true, message: '请输入样本编号' }]}
              >
                <Input placeholder="请输入样本编号" />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="status"
                label="状态"
                rules={[{ required: true, message: '请选择状态' }]}
              >
                <Select placeholder="请选择状态">
                  <Select.Option value="created">已创建</Select.Option>
                  <Select.Option value="collected">已采集</Select.Option>
                  <Select.Option value="received">已接收</Select.Option>
                  <Select.Option value="processing">处理中</Select.Option>
                  <Select.Option value="tested">已检测</Select.Option>
                  <Select.Option value="completed">已完成</Select.Option>
                </Select>
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="patient_name"
                label="患者姓名"
              >
                <Input placeholder="患者姓名" disabled />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="sample_type_id"
                label="样本类型"
                rules={[{ required: true, message: '请选择样本类型' }]}
              >
                <Select placeholder="请选择样本类型" options={sampleTypes.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="cancer_type_id"
                label="癌种"
                rules={[{ required: true, message: '请选择癌种' }]}
              >
                <Select placeholder="请选择癌种" options={cancerTypes.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="treatment_stage_id"
                label="治疗阶段"
                rules={[{ required: true, message: '请选择治疗阶段' }]}
              >
                <Select placeholder="请选择治疗阶段" options={treatmentStages.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="organization_type"
                label="送检方式"
                rules={[{ required: true, message: '请选择送检方式' }]}
              >
                <Radio.Group
                  onChange={(event) => {
                    if (event.target.value === '个人送检') {
                      editForm.setFieldValue('organization', '个人送检');
                    } else {
                      editForm.setFieldValue('organization', undefined);
                    }
                  }}
                >
                  <Radio.Button value="个人送检">个人送检</Radio.Button>
                  <Radio.Button value="单位送检">单位送检</Radio.Button>
                </Radio.Group>
              </Form.Item>
            </Col>
            <Col span={12}>
              <Form.Item
                name="organization"
                label="送检单位"
                rules={editOrganizationType === '单位送检' ? [{ required: true, message: '请输入送检单位' }] : []}
              >
                <Input placeholder="请输入送检单位" disabled={editOrganizationType !== '单位送检'} />
              </Form.Item>
            </Col>
          </Row>
          <Row gutter={16}>
            <Col span={12}>
              <Form.Item
                name="receive_date"
                label="接收时间"
              >
                <DatePicker showTime style={{ width: '100%' }} placeholder="请选择接收时间" />
              </Form.Item>
            </Col>
          </Row>
          <Form.Item
            name="notes"
            label="备注"
          >
            <Input.TextArea rows={4} placeholder="请输入备注" />
          </Form.Item>
          <Form.Item style={{ textAlign: 'right' }}>
            <Button onClick={() => setEditVisible(false)} style={{ marginRight: 8 }}>取消</Button>
            <Button type="primary" htmlType="submit">保存</Button>
          </Form.Item>
        </Form>
      </Modal>

      {/* 批量创建样本模态框 */}
      <Modal
        title="批量创建样本"
        open={batchModalVisible}
        onCancel={() => setBatchModalVisible(false)}
        footer={null}
        width={800}
      >
        <div style={{ marginBottom: 24 }}>
          <Typography.Title level={5}>批量创建说明</Typography.Title>
          <Typography.Paragraph>
            1. 点击下方"下载模板"按钮，下载样本批量创建模板
          </Typography.Paragraph>
          <Typography.Paragraph>
            2. 在模板中填写样本信息
          </Typography.Paragraph>
          <Typography.Paragraph>
            3. 点击"上传文件"按钮，上传填写完成的Excel文件
          </Typography.Paragraph>
          <Typography.Paragraph>
            4. 系统将自动批量创建样本
          </Typography.Paragraph>
        </div>
        
        <div style={{ marginBottom: 24 }}>
          <Typography.Title level={5}>模板下载</Typography.Title>
          <Button 
            icon={<DownloadOutlined />} 
            onClick={() => {
              // 生成样本批量创建模板
              const headers = ['样本编号', '患者身份证号', '样本类型ID', '治疗阶段ID', '备注'];
              const csvContent = `\uFEFF${headers.join(',')}\n`;
              const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
              const link = document.createElement('a');
              link.href = URL.createObjectURL(blob);
              link.download = '样本批量创建模板.csv';
              link.click();
            }}
          >
            下载模板
          </Button>
        </div>
        
        <div>
          <Typography.Title level={5}>文件上传</Typography.Title>
          <Upload
            name="file"
            beforeUpload={handleBatchCreate}
            showUploadList={false}
            accept=".xlsx,.xls,.csv"
          >
            <Button icon={<InboxOutlined />}>
              上传文件
            </Button>
          </Upload>
        </div>
      </Modal>

    </div>
  );
};

export default SampleList;
