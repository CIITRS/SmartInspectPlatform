import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Card, Tabs, App, Form, Input, Row, Col } from 'antd';
import { useNavigate, useModel } from '@umijs/max';
import { EditOutlined, EyeOutlined } from '@ant-design/icons';
import { listPatients, getPatientById, getSamplesByPatientId } from '@/services/api';
import dayjs from 'dayjs';
import PatientReportList from '@/components/PatientReportPreview/ReportList';

const { TabPane } = Tabs;

const getSalesPersonCode = (user: any) =>
  String(user?.employee_id || '').trim();

const getRoleName = (user: any) => user?.role_name || user?.role?.name || '';

const Perfect: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const [pendingPatients, setPendingPatients] = useState<any[]>([]);
  const [completedPatients, setCompletedPatients] = useState<any[]>([]);
  const [currentPatient, setCurrentPatient] = useState<any>(null);
  const [detailModalVisible, setDetailModalVisible] = useState(false);
  const [detailLoading, setDetailLoading] = useState(false);
  const [samples, setSamples] = useState<any[]>([]);
  const [samplesLoading, setSamplesLoading] = useState(false);
  const [searchParams, setSearchParams] = useState({});

  const navigate = useNavigate();
  const { message: appMessage } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const [currentUser, setCurrentUser] = useState<any>(null);

  useEffect(() => {
    if (initialState?.currentUser) {
      setCurrentUser(initialState.currentUser);
    }
  }, [initialState]);

  useEffect(() => {
    fetchPatients();
  }, [currentUser]);

  // 处理搜索
  const handleSearch = (values: any) => {
    setSearchParams(values);
    fetchPatients();
  };

  // 获取患者列表
  const fetchPatients = async () => {
    setLoading(true);
    try {
      // 构建查询参数
      const commonParams: any = {
        is_active: 1,
        ...searchParams
      };

      // 销售角色只查看自己负责的患者
      if (currentUser && getRoleName(currentUser) === '销售') {
        commonParams.sales_person = getSalesPersonCode(currentUser);
      }

      // 获取未完善患者列表
      const pendingResponse = await listPatients({
        ...commonParams,
        completion_status: 'pending'
      });
      const pendingList = pendingResponse.data?.list || [];
      setPendingPatients(pendingList.map((patient: any) => ({
        ...patient,
        completionStatus: patient.completionStatus || 'pending'
      })));

      // 获取已完善患者列表
      const completedResponse = await listPatients({
        ...commonParams,
        completion_status: 'completed'
      });
      const completedList = completedResponse.data?.list || [];
      setCompletedPatients(completedList.map((patient: any) => ({
        ...patient,
        completionStatus: patient.completionStatus || 'completed'
      })));
    } catch (_error) {
      appMessage.error('获取患者列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 查看患者详情
  const handleViewDetail = async (id: string) => {
    setDetailLoading(true);
    try {
      const response = await getPatientById(id);
      setCurrentPatient(response.data);
      setDetailModalVisible(true);
      // 获取患者样本
      await fetchPatientSamples(id);
    } catch (_error) {
      appMessage.error('获取患者详情失败');
    } finally {
      setDetailLoading(false);
    }
  };

  // 获取患者样本
  const fetchPatientSamples = async (patientId: string) => {
    setSamplesLoading(true);
    try {
      // 使用新的API获取样本列表
      const response = await getSamplesByPatientId(patientId);
      setSamples(response.data?.list || []);
    } catch (_error) {
      // 忽略错误，显示空列表
      setSamples([]);
    } finally {
      setSamplesLoading(false);
    }
  };

  // 完善/修改患者信息
  const handleEdit = (id: string, status: string) => {
    if (status === 'pending') {
      // 未完善的患者跳转到完善页面
      navigate(`/patient/complete/${id}`);
    } else {
      // 已完善的患者跳转到编辑页面
      navigate(`/patient/edit/${id}`);
    }
  };

  // 表格列配置
  const columns = [
    {
      title: '患者编号',
      dataIndex: 'patientCode',
      key: 'patientCode'
    },
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
      render: (_: any, record: any) => (
        <a onClick={() => handleViewDetail(record.id)} style={{ cursor: 'pointer', color: '#1890ff' }}>
          {record.name}
        </a>
      )
    },
    {
      title: '性别',
      dataIndex: 'gender',
      key: 'gender'
    },
    {
      title: '年龄',
      dataIndex: 'age',
      key: 'age'
    },
    {
      title: '身份证件类型',
      dataIndex: 'idDocumentType',
      key: 'idDocumentType',
      render: (text: any) => text || '-'
    },
    {
      title: '身份证件号',
      dataIndex: 'idDocumentNo',
      key: 'idDocumentNo',
      render: (_: any, record: any) => record.idDocumentNo || record.idCard || '-'
    },
    {
      title: '联系电话',
      dataIndex: 'phone',
      key: 'phone'
    },
    {
      title: '创建时间',
      dataIndex: 'createdAt',
      key: 'createdAt',
      render: (text: any) => dayjs(text).format('YYYY-MM-DD')
    },
    {
      title: '操作',
      key: 'action',
      render: (_: any, record: any) => (
        <>
          <Button
            type="link"
            size="small"
            icon={<EyeOutlined />}
            onClick={() => handleViewDetail(record.id)}
            style={{ marginRight: 8 }}
          >
            查看
          </Button>
          <Button
            type="link"
            size="small"
            icon={<EditOutlined />}
            onClick={() => handleEdit(record.id, record.completionStatus)}
          >
            {record.completionStatus === 'pending' ? '完善' : '修改'}
          </Button>
        </>
      ),
    },
  ];

  return (
    <div>
      <Card>
        <Tabs defaultActiveKey="pending">
          <TabPane tab="完善信息" key="pending">
            <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name="name">
                    <Input placeholder="姓名" />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="idDocumentNo">
                    <Input placeholder="身份证件号" />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                      查询
                    </Button>
                    <Button onClick={() => form.resetFields()}>重置</Button>
                  </Form.Item>
                </Col>
              </Row>
            </Form>
            <Table
              columns={columns}
              dataSource={pendingPatients}
              loading={loading}
              rowKey="id"
              pagination={{ pageSize: 10 }}
            />
          </TabPane>
          <TabPane tab="修改信息" key="completed">
            <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
              <Row gutter={16}>
                <Col span={8}>
                  <Form.Item name="name">
                    <Input placeholder="姓名" />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item name="idDocumentNo">
                    <Input placeholder="身份证件号" />
                  </Form.Item>
                </Col>
                <Col span={8}>
                  <Form.Item>
                    <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                      查询
                    </Button>
                    <Button onClick={() => form.resetFields()}>重置</Button>
                  </Form.Item>
                </Col>
              </Row>
            </Form>
            <Table
              columns={columns}
              dataSource={completedPatients}
              loading={loading}
              rowKey="id"
              pagination={{ pageSize: 10 }}
            />
          </TabPane>
        </Tabs>
      </Card>

      {/* 患者详情模态框 */}
      <Modal
        title="患者详情"
        open={detailModalVisible}
        onCancel={() => setDetailModalVisible(false)}
        footer={[<Button key="close" onClick={() => setDetailModalVisible(false)}>关闭</Button>]}
        width={900}
      >
        {currentPatient && (
          <div>
            <Card loading={detailLoading} title="基本信息">
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
                <div><strong>患者编号：</strong>{currentPatient.patientCode}</div>
                <div><strong>姓名：</strong>{currentPatient.name}</div>
                <div><strong>性别：</strong>{currentPatient.gender}</div>
                <div><strong>年龄：</strong>{currentPatient.age}</div>
                <div><strong>身份证件类型：</strong>{currentPatient.idDocumentType || '-'}</div>
                <div><strong>身份证件号：</strong>{currentPatient.idDocumentNo || currentPatient.idCard || '-'}</div>
                <div><strong>联系电话：</strong>{currentPatient.phone}</div>
                <div><strong>出生日期：</strong>{currentPatient.birthday ? dayjs(currentPatient.birthday).format('YYYY-MM-DD') : '-'}</div>
                <div><strong>地址：</strong>{currentPatient.address || '-'}</div>
                <div><strong>癌直径：</strong>{currentPatient.cancerDiameter || '-'}</div>
                <div><strong>吸烟状态：</strong>{currentPatient.smokingStatus || '-'}</div>
                <div><strong>销售：</strong>{currentPatient.salesPerson?.name || '-'}</div>
                <div><strong>创建时间：</strong>{dayjs(currentPatient.createdAt).format('YYYY-MM-DD HH:mm:ss')}</div>
                <div style={{ gridColumn: '1 / span 2' }}><strong>备注：</strong>{currentPatient.medicalRecordNo || '-'}</div>
              </div>
            </Card>

            {/* 病理与预后信息 */}
            <Card title="病理与预后信息" style={{ marginTop: 16 }}>
              <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: '16px' }}>
                <div><strong>癌症病理信息：</strong>{currentPatient.cancerPathology || '-'}</div>
                <div><strong>预后信息：</strong>{currentPatient.prognosisInfo || '-'}</div>
                <div style={{ gridColumn: '1 / span 2' }}><strong>其他信息：</strong>{currentPatient.otherInfo || '-'}</div>
                {(currentPatient as any).followUps?.map((item: any) => (
                  <div key={item.id} style={{ gridColumn: '1 / span 2', paddingTop: 12, borderTop: '1px solid #f0f0f0' }}>
                    <div><strong>检测时间：</strong>{item.detection_time || item.created_at || '-'}</div>
                    <div><strong>检测信息：</strong>{item.diagnosis_info || '-'}</div>
                    <div><strong>结果说明：</strong>{item.report_notes || '-'}</div>
                    <div><strong>报告文件：</strong>{Array.isArray(item.images) && item.images.length ? item.images.join('，') : '-'}</div>
                  </div>
                ))}
              </div>
            </Card>

            {/* 检测样本列表 */}
            <Card
              title="检测样本"
              loading={samplesLoading}
              style={{ marginTop: 16 }}
            >
              {samples.length > 0 ? (
                <Table
                  columns={[
                    {
                      title: '样本编号',
                      dataIndex: 'sample_code',
                      key: 'sample_code',
                    },
                    {
                      title: '样本类型',
                      dataIndex: 'sampleTypeName',
                      key: 'sampleTypeName',
                    },
                    {
                      title: '采集日期',
                      dataIndex: 'collection_date',
                      key: 'collection_date',
                      render: (text: any) => text ? dayjs(text).format('YYYY-MM-DD') : '-',
                    },
                    {
                      title: '接收日期',
                      dataIndex: 'receive_date',
                      key: 'receive_date',
                      render: (text: any) => text ? dayjs(text).format('YYYY-MM-DD') : '-',
                    },
                    {
                      title: '样本状态',
                      dataIndex: 'status',
                      key: 'status',
                      render: (text: any) => {
                        const statusMap: { [key: string]: { text: string; color: string } } = {
                          'collected': { text: '已采集', color: '#faad14' },
                          'received': { text: '已接收', color: '#1890ff' },
                          'processing': { text: '处理中', color: '#1890ff' },
                          'tested': { text: '已检测', color: '#52c41a' },
                          'completed': { text: '已完成', color: '#52c41a' },
                        };
                        const status = statusMap[text] || { text: text || '未知', color: '#d9d9d9' };
                        return <span style={{ color: status.color }}>{status.text}</span>;
                      },
                    },
                  ]}
                  dataSource={samples}
                  rowKey="id"
                  pagination={false}
                />
              ) : (
                <div style={{ textAlign: 'center', padding: '20px', color: '#999' }}>
                  暂无检测样本
                </div>
              )}
            </Card>

            {/* 报告文件 */}
            <Card title="报告文件" style={{ marginTop: 16 }}>
              <PatientReportList patientCode={currentPatient.patientCode} reportFiles={currentPatient.reportFiles} />
            </Card>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default Perfect;
