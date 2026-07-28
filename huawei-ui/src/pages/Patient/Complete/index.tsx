import type React from 'react';
import { useState, useEffect, useCallback } from 'react';
import { Form, Input, Button, Card, App, Row, Col, Select, Upload, message } from 'antd';
import type { UploadFile, UploadChangeParam } from 'antd/es/upload/interface';
import { useParams, useNavigate } from '@umijs/max';
import { getPatientDetail, updatePatient } from '@/services/api';
import dayjs from 'dayjs';

interface PatientFormValues {
  name: string;
  gender: string;
  idDocumentType: string;
  idDocumentNo: string;
  age: number;
  idCard: string;
  phone: string;
  address: string;
  medicalHistory: string;
  medicationHistory: string;
  familyHistory: string;
  otherInfo: string;
  salesPersonId: number | string;
  birthday?: dayjs.Dayjs | string;
  collectionDate?: dayjs.Dayjs | string;
  cancerDiameter?: string;
  smokingStatus?: string;
  medicalRecordNo?: string;
  cancerPathology?: string;
  prognosisInfo?: string;
  sampleType?: string;
  [key: string]: unknown;
}
import { UploadOutlined } from '@ant-design/icons';

const Complete: React.FC = () => {
  const [form] = Form.useForm();
  const { patientCode } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const { message: appMessage } = App.useApp();
  const [fileList, setFileList] = useState<UploadFile[]>([]);
  const [healthStatus, setHealthStatus] = useState<number>(1); // 默认值设为患病，使用数字值：0=健康，1=患病

  // 保存销售ID，用于提交时使用
  const [salesPersonId, setSalesPersonId] = useState<number | string>(0);

  const fetchPatientDetail = useCallback(async () => {
    if (!patientCode) {
      appMessage.error('患者编号不存在');
      setLoading(false);
      navigate('/patient/list');
      return;
    }
    try {
      const response = await getPatientDetail(patientCode);
      // 处理销售字段，确保显示姓名而不是对象
      const salesId = response.data.salesPerson?.code || response.data.salesPerson?.id || response.data.salesPerson;
      const salesName = response.data.salesPerson?.name || response.data.salesPerson;
      setSalesPersonId(salesId);
      
      // 使用patientStatus值来设置健康状态，保持数字值一致
      const status = response.data.patientStatus || 1;
      setHealthStatus(status);
      
      form.setFieldsValue({
        name: response.data.name,
        gender: response.data.gender,
        idDocumentType: response.data.idDocumentType || '居民身份证',
        idDocumentNo: response.data.idDocumentNo || response.data.idCard,
        idCard: response.data.idCard,
        phone: response.data.phone,
        birthday: response.data.birthday ? dayjs(response.data.birthday) : null,
        address: response.data.address,
        cancerDiameter: response.data.cancerDiameter,
        smokingStatus: response.data.smokingStatus,
        medicalRecordNo: response.data.medicalRecordNo,
        salesPerson: salesName,
        cancerPathology: response.data.cancerPathology,
        prognosisInfo: response.data.prognosisInfo,
        sampleType: response.data.sampleType,
        collectionDate: response.data.collectionDate ? dayjs(response.data.collectionDate) : null,
        otherInfo: response.data.otherInfo,
      });
      setLoading(false);
    } catch (_error) {
      appMessage.error('获取患者信息失败');
      setLoading(false);
    }
  }, [patientCode, appMessage, navigate, form, setLoading, setSalesPersonId, setHealthStatus]);

  useEffect(() => {
    fetchPatientDetail();
  }, [fetchPatientDetail]);

  // 文件上传前的校验
  const beforeUpload = (file: File) => {
    const isLt2M = file.size / 1024 / 1024 < 20;
    if (!isLt2M) {
      message.error('文件大小不能超过20MB!');
    }
    const isAllowedType = ['image/jpeg', 'image/png', 'application/pdf', 'application/msword', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document'].includes(file.type);
    if (!isAllowedType) {
      message.error('只允许上传JPG、PNG、PDF、DOC、DOCX格式的文件!');
    }
    return isLt2M && isAllowedType ? false : Upload.LIST_IGNORE;
  };

  // 文件上传进度
  const handleUploadChange = (info: UploadChangeParam<UploadFile>) => {
    let newFileList = [...info.fileList];
    newFileList = newFileList.slice(-5);
    newFileList = newFileList.map((file) => {
      if (file.response && typeof file.response === 'object' && 'url' in file.response) {
        file.url = file.response.url as string;
      }
      return file;
    });
    setFileList(newFileList as UploadFile[]);
  };

  // 文件上传
  const handleFileUpload = async () => {
    if (fileList.length === 0) {
      return Promise.resolve();
    }

    try {
      for (const file of fileList) {
        if (!file.originFileObj) continue;
        const formData = new FormData();
        formData.append('file', file.originFileObj);
        if (patientCode) {
          formData.append('patient_code', patientCode.toString());
        }

        const response = await fetch('/api/patients/upload', {
          method: 'POST',
          headers: {
            'Authorization': `Bearer ${localStorage.getItem('token')}`,
          },
          body: formData,
        });

        const result = await response.json();
        if (result.code !== 200) {
          message.error(`文件上传失败: ${result.message}`);
          throw new Error(result.message || '文件上传失败');
        }
      }
      message.success('文件上传成功');
      setFileList([]);
    } catch (error) {
      message.error(`文件上传失败: ${error}`);
      throw error;
    }
  };

  const handleSubmit = async (values: PatientFormValues) => {
    if (!patientCode) {
      appMessage.error('患者编号不存在');
      return;
    }
    try {
      // 构建简化的提交数据，只包含必要信息
      let formattedValues: Partial<PatientFormValues> & {
        completionStatus: number;
        patientStatus: number;
        salesPerson: number | string;
      } = {
        // 标记为已完成
        completionStatus: 1,
        // 添加患者状态字段：健康=0，患病=1
        patientStatus: healthStatus,
        // 使用销售代表ID而不是名称
        salesPerson: salesPersonId,
      };

      // 添加可选字段（只有当它们存在且健康状态为患病时）
      if (healthStatus === 1) {
        // 如果生日是 dayjs 对象，转换为字符串格式
        if (values.birthday) {
          formattedValues = {
            ...formattedValues,
            birthday: typeof values.birthday === 'string' ? values.birthday : values.birthday.format('YYYY-MM-DD'),
          };
        }
        if (values.collectionDate) {
          formattedValues = {
            ...formattedValues,
            collectionDate: typeof values.collectionDate === 'string' ? values.collectionDate : values.collectionDate.format('YYYY-MM-DD'),
          };
        }
        if (values.address) {
          formattedValues = {
            ...formattedValues,
            address: values.address,
          };
        }
        if (values.cancerDiameter) {
          formattedValues = {
            ...formattedValues,
            cancerDiameter: values.cancerDiameter,
          };
        }
        if (values.cancerPathology) {
          formattedValues = {
            ...formattedValues,
            cancerPathology: values.cancerPathology,
          };
        }
        if (values.prognosisInfo) {
          formattedValues = {
            ...formattedValues,
            prognosisInfo: values.prognosisInfo,
          };
        }
        if (values.otherInfo) {
          formattedValues = {
            ...formattedValues,
            otherInfo: values.otherInfo,
          };
        }
        if (values.smokingStatus) {
          formattedValues = {
            ...formattedValues,
            smokingStatus: values.smokingStatus,
          };
        }
        if (values.medicalRecordNo) {
          formattedValues = {
            ...formattedValues,
            medicalRecordNo: values.medicalRecordNo,
          };
        }
        if (values.sampleType) {
          formattedValues = {
            ...formattedValues,
            sampleType: values.sampleType,
          };
        }
        // 先上传文件
        await handleFileUpload();
      } else {
        // 健康状态下只上传必要信息
        if (values.birthday) {
          formattedValues = {
            ...formattedValues,
            birthday: typeof values.birthday === 'string' ? values.birthday : values.birthday.format('YYYY-MM-DD'),
          };
        }
        if (values.address) {
          formattedValues = {
            ...formattedValues,
            address: values.address,
          };
        }
      }

      await updatePatient(patientCode, formattedValues);
      appMessage.success('患者信息完善成功');
      // 提交后刷新页面，保持在完善信息页面
      window.location.reload();
    } catch (_error) {
      appMessage.error('患者信息完善失败');
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Card loading={loading} title="完善患者信息" />
      </div>
    );
  }

  return (
    <div>
      <Card title="完善患者信息">
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          {/* 基本信息 - 只读 */}
          <Card type="inner" title="基本信息" style={{ marginBottom: 16 }}>
            {/* 第一行：姓名、性别、身份证件 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="name"
                  label="姓名"
                >
                  <Input placeholder="请输入姓名" disabled />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="gender"
                  label="性别"
                >
                  <Select placeholder="性别" disabled>
                    <Select.Option value="男">男</Select.Option>
                    <Select.Option value="女">女</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="idDocumentType"
                  label="身份证件类型"
                >
                  <Input placeholder="身份证件类型" disabled />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="idDocumentNo"
                  label="身份证件号"
                >
                  <Input placeholder="身份证件号" disabled />
                </Form.Item>
              </Col>
            </Row>

            {/* 第二行：联系电话、销售 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                <Form.Item
                  name="phone"
                  label="联系电话"
                >
                  <Input placeholder="请输入联系电话" disabled />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                <Form.Item
                  name="salesPerson"
                  label="销售"
                >
                  <Input placeholder="请输入销售" disabled />
                </Form.Item>
              </Col>
            </Row>

            {/* 第三行：地址 */}
            <Form.Item
              name="address"
              label="地址"
            >
              <Input.TextArea placeholder="请输入地址" rows={3} disabled />
            </Form.Item>
            
            {/* 第四行：吸烟状态 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                <Form.Item
                  name="smokingStatus"
                  label="吸烟状态"
                >
                  <Select placeholder="请选择吸烟状态">
                    <Select.Option value="不吸烟">不吸烟</Select.Option>
                    <Select.Option value="10支以内/日">10支以内/日</Select.Option>
                    <Select.Option value="10-20支/日">10-20支/日</Select.Option>
                    <Select.Option value="20支以上/日">20支以上/日</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Card>

          {/* 健康状态选择 */}
          <Card type="inner" title="健康状态" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  label="健康状态"
                >
                  <Select
                    value={healthStatus}
                    onChange={(value: number) => setHealthStatus(value)}
                    style={{ width: '100%' }}
                  >
                    <Select.Option value={0}>健康</Select.Option>
                    <Select.Option value={1}>患病</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Card>

          {/* 病理与预后信息 - 仅在患病时显示 */}
          {healthStatus === 1 && (
            <Card type="inner" title="病理与预后信息" style={{ marginBottom: 16 }}>
              <Row gutter={16}>
                <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                  <Form.Item
                    name="cancerDiameter"
                    label="癌直径（mm）"
                  >
                    <Input placeholder="请输入癌直径" />
                  </Form.Item>
                </Col>
              </Row>
              
              <Row gutter={16}>
                <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                  <Form.Item
                    name="cancerPathology"
                    label="癌症病理信息"
                  >
                    <Input.TextArea placeholder="请输入癌症病理信息" rows={4} />
                  </Form.Item>
                </Col>
              </Row>
              
              <Row gutter={16}>
                <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                  <Form.Item
                    name="prognosisInfo"
                    label="预后信息"
                  >
                    <Input.TextArea placeholder="请输入预后信息" rows={4} />
                  </Form.Item>
                </Col>
              </Row>
              
              <Row gutter={16}>
                <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                  <Form.Item
                    name="otherInfo"
                    label="其他信息"
                  >
                    <Input.TextArea placeholder="请输入其他信息" rows={3} />
                  </Form.Item>
                </Col>
              </Row>
            </Card>
          )}

          {/* 文件上传 - 仅在患病时显示 */}
          {healthStatus === 1 && (
            <Card type="inner" title="报告文件上传" style={{ marginBottom: 16 }}>
              <Form.Item label="上传报告图片（支持多个，单个文件不超过20MB）">
                <Upload
                  name="file"
                  fileList={fileList}
                  beforeUpload={beforeUpload}
                  onChange={handleUploadChange}
                  multiple
                  accept=".jpg,.jpeg,.png,.pdf,.doc,.docx"
                >
                  <Button icon={<UploadOutlined />}>点击上传</Button>
                </Upload>
                <div style={{ marginTop: 8, fontSize: '12px', color: '#666' }}>
                  支持上传JPG、PNG、PDF、DOC、DOCX格式文件，单个文件不超过20MB，最多上传5个文件
                </div>
              </Form.Item>
            </Card>
          )}

          <Form.Item>
            <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
              提交
            </Button>
            <Button onClick={() => navigate('/patient/list')}>
              取消
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default Complete;
