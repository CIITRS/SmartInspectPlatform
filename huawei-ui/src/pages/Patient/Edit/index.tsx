import React, { useState, useEffect } from 'react';
import { Form, Input, Select, Button, Card, Spin, DatePicker, App, Upload, message, Row, Col, Modal, Alert, Descriptions, Empty, Skeleton, Space, Tag, Typography } from 'antd';
import { useParams, useNavigate, useModel } from '@umijs/max';
import { getPatientDetail, updatePatient } from '@/services/api';
import dayjs from 'dayjs'; // 引入 dayjs
import { RobotOutlined, SyncOutlined, UploadOutlined } from '@ant-design/icons';
import '@/components/PatientReportPreview/index.less';

const { Dragger } = Upload;
const { Option } = Select;
const documentTypeOptions = ['居民身份证', '护照', '港澳居民来往内地通行证', '台湾居民来往大陆通行证', '自编号'];

const getSalesPersonCode = (user: any) =>
  String(user?.employee_id || '').trim();

const getRoleName = (user: any) => user?.role_name || user?.role?.name || '';

const reportFileName = (fileUrl: string, index: number) => {
  const cleanUrl = String(fileUrl || '').split('?')[0];
  const encodedName = cleanUrl.split('/').pop();
  if (!encodedName) return `患者报告${index + 1}`;
  try {
    return decodeURIComponent(encodedName);
  } catch (_error) {
    return encodedName;
  }
};

const Edit: React.FC = () => {
  const [form] = Form.useForm();
  const { patientCode } = useParams();
  const navigate = useNavigate();
  const [loading, setLoading] = useState(true);
  const { message: appMessage } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const [fileList, setFileList] = useState<any[]>([]);
  const [salesUsers, setSalesUsers] = useState<any[]>([]);
  const [salesLoading, setSalesLoading] = useState(false);
  const [completionStatus, setCompletionStatus] = useState(0);
  const [patientStatus, setPatientStatus] = useState<number>(1); // 默认值设为患病
  const [reportPreview, setReportPreview] = useState<{
    open: boolean;
    loading: boolean;
    name: string;
    url: string;
    kind: 'image' | 'pdf' | 'other';
    localObjectUrl?: boolean;
    fileUrl?: string;
    analysisLoading?: boolean;
    analysis?: any;
  }>({ open: false, loading: false, name: '', url: '', kind: 'other' });
  const idDocumentType = Form.useWatch('idDocumentType', form) || '居民身份证';

  // 获取销售岗用户列表
  const fetchSalesUsers = async () => {
    setSalesLoading(true);
    try {
      const response = await fetch('/api/system/users', {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        // 过滤出销售岗用户（角色为管理员或销售）
        const sales = result.data.list.filter((user: any) =>
          (user.role_name === '管理员' || user.role_name === '销售') && getSalesPersonCode(user)
        );
        setSalesUsers(sales);
      }
    } catch (_error) {
      appMessage.error('获取销售列表失败');
    } finally {
      setSalesLoading(false);
    }
  };

  useEffect(() => {
    fetchPatientDetail();
    fetchSalesUsers();
  }, [patientCode]);

  // 表单初始化，确保birthday字段初始值为undefined
  useEffect(() => {
    // 页面加载时不重置，只在身份证号变更时处理
  }, []);

  // 身份证号校验函数（包含18位校验码验证）
  const validateIdCard = (idCard: string): { isValid: boolean; message: string } => {
    if (!idCard) {
      return { isValid: false, message: '请输入身份证号' };
    }
    
    // 基本格式验证
    const idCardRegex = /(^\d{15}$)|(^\d{17}([0-9]|X)$)/i;
    if (!idCardRegex.test(idCard)) {
      return { isValid: false, message: '身份证号格式不正确' };
    }
    
    // 18位身份证校验码验证
    if (idCard.length === 18) {
      const weights = [7, 9, 10, 5, 8, 4, 2, 1, 6, 3, 7, 9, 10, 5, 8, 4, 2];
      const checkCodes = ['1', '0', 'X', '9', '8', '7', '6', '5', '4', '3', '2'];
      
      let sum = 0;
      for (let i = 0; i < 17; i++) {
        sum += parseInt(idCard[i], 10) * weights[i];
      }
      
      const checkCode = checkCodes[sum % 11];
      if (checkCode !== idCard[17].toUpperCase()) {
        return { isValid: false, message: '身份证号校验码错误，可能存在输入错位' };
      }
    }
    
    // 验证出生日期
    try {
      let year = 0;
      let month = 0;
      let day = 0;
      
      if (idCard.length === 18) {
        year = parseInt(idCard.substring(6, 10), 10);
        month = parseInt(idCard.substring(10, 12), 10);
        day = parseInt(idCard.substring(12, 14), 10);
      } else if (idCard.length === 15) {
        year = parseInt(`19${idCard.substring(6, 8)}`, 10);
        month = parseInt(idCard.substring(8, 10), 10);
        day = parseInt(idCard.substring(10, 12), 10);
      }
      
      const birthday = dayjs(`${year}-${month}-${day}`);
      if (!birthday.isValid()) {
        return { isValid: false, message: '身份证号中包含无效的出生日期' };
      }
      
      // 检查出生日期是否在合理范围内
      const currentYear = dayjs().year();
      if (year < 1900 || year > currentYear) {
        return { isValid: false, message: '出生日期超出合理范围' };
      }
    } catch (_error) {
      return { isValid: false, message: '身份证号中包含无效的出生日期' };
    }
    
    return { isValid: true, message: '身份证号有效' };
  };

  // 自动解析身份证号，填充性别和出生日期
  const handleIdCardChange = (e: any) => {
    if (form.getFieldValue('idDocumentType') !== '居民身份证') return;
    const idCard = e.target.value;
    
    // 只有当身份证号完全输入完成（15位或18位）时，才进行解析
    if (!idCard || (idCard.length !== 15 && idCard.length !== 18)) {
      return;
    }
    
    // 先进行完整校验
    const validationResult = validateIdCard(idCard);
    if (!validationResult.isValid) {
      return;
    }
    
    try {
      let year = 0;
      let month = 0;
      let day = 0;
      let genderCode = 0;
      
      if (idCard.length === 18) {
        // 18位身份证解析
        year = parseInt(idCard.substring(6, 10), 10);
        month = parseInt(idCard.substring(10, 12), 10) - 1; // 月份从0开始
        day = parseInt(idCard.substring(12, 14), 10);
        genderCode = parseInt(idCard.substring(16, 17), 10);
      } else if (idCard.length === 15) {
        // 15位身份证解析
        year = parseInt(`19${idCard.substring(6, 8)}`, 10);
        month = parseInt(idCard.substring(8, 10), 10) - 1; // 月份从0开始
        day = parseInt(idCard.substring(10, 12), 10);
        genderCode = parseInt(idCard.substring(14, 15), 10);
      }
      
      // 使用 dayjs 创建日期对象
      const birthday = dayjs(new Date(year, month, day));
      
      // 确定性别
      const gender = genderCode % 2 === 1 ? '男' : '女';
      
      // 使用 dayjs 对象填充表单
      form.setFieldsValue({ gender, birthday });
    } catch (_error) {
      // 静默失败，不影响用户体验
      console.log('身份证号解析失败，但已被优雅处理');
    }
  };

  const fetchPatientDetail = async () => {
    if (!patientCode) {
      appMessage.error('患者编号不存在');
      setLoading(false);
      navigate('/patient/list');
      return;
    }
    try {
      const response = await getPatientDetail(patientCode);
      const patientData = response.data;
      
      // 检查当前用户是否有权限编辑该患者
      const currentUser = initialState?.currentUser as any;
      const salesPersonId = patientData.salesPerson?.code || patientData.salesPerson?.id || patientData.salesPerson;
      if (currentUser && getRoleName(currentUser) === '销售' && salesPersonId !== getSalesPersonCode(currentUser)) {
        appMessage.error('您只能编辑自己负责的患者');
        setLoading(false);
        navigate('/patient/list');
        return;
      }
      
      // 保存completionStatus值
      setCompletionStatus(patientData.completionStatus || 0);
      
      // 保存patientStatus值
      const status = patientData.patientStatus || 1;
      setPatientStatus(status);
      
      form.setFieldsValue({
        name: patientData.name,
        gender: patientData.gender,
        idDocumentType: patientData.idDocumentType || '居民身份证',
        idDocumentNo: patientData.idDocumentNo || patientData.idCard,
        idCard: patientData.idCard,
        phone: patientData.phone,
        birthday: patientData.birthday ? dayjs(patientData.birthday) : null,
        address: patientData.address,
        cancerDiameter: patientData.cancerDiameter,
        smokingStatus: patientData.smokingStatus,
        medicalRecordNo: patientData.medicalRecordNo,
        salesPerson: salesPersonId,
        cancerPathology: patientData.cancerPathology,
        prognosisInfo: patientData.prognosisInfo,
        sampleType: patientData.sampleType,
        collectionDate: patientData.collectionDate ? dayjs(patientData.collectionDate) : null,
        otherInfo: patientData.otherInfo,
        patientStatus: status,
      });
      const existingReportFiles = String(patientData.reportFiles || patientData.report_files || '')
        .split(',')
        .map((file: string) => file.trim())
        .filter(Boolean);
      setFileList(existingReportFiles.map((fileUrl: string, index: number) => ({
        uid: `existing-report-${index}-${fileUrl}`,
        name: reportFileName(fileUrl, index),
        status: 'done',
        url: fileUrl,
        isExistingReport: true,
      })));
      setLoading(false);
    } catch (_error) {
      appMessage.error('获取患者信息失败');
      setLoading(false);
    }
  };

  // 文件上传前的校验
  const beforeUpload = (file: any) => {
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
  const handleUploadChange = (info: any) => {
    let newFileList = [...info.fileList];
    newFileList = newFileList.map((file) => {
      if (file.response) {
        // Component will show file.url as link
        file.url = file.response.url;
      }
      return file;
    });
    setFileList(newFileList);
  };

  const handleRemoveReport = (file: any) => {
    if (!file.isExistingReport) {
      setFileList((current) => current.filter((item) => item.uid !== file.uid));
      return false;
    }
    Modal.confirm({
      title: '删除患者报告',
      content: `确定删除“${file.name}”吗？该操作只删除这一份报告。`,
      okText: '删除',
      okButtonProps: { danger: true },
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/patients/${encodeURIComponent(String(patientCode))}/report-files`, {
            method: 'DELETE',
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`,
              'Content-Type': 'application/json',
            },
            body: JSON.stringify({ file_url: file.url }),
          });
          const result = await response.json();
          if (!response.ok || result.code !== 200) {
            throw new Error(result.message || '报告删除失败');
          }
          setFileList((current) => current.filter((item) => item.uid !== file.uid));
          if (result.data?.cleanup_warning) {
            appMessage.warning(result.data.cleanup_warning);
          } else {
            appMessage.success('报告删除成功');
          }
        } catch (error: any) {
          appMessage.error(error?.message || '报告删除失败');
          throw error;
        }
      },
    });
    return false;
  };

  const reportPreviewKind = (file: any): 'image' | 'pdf' | 'other' => {
    const type = String(file?.type || file?.originFileObj?.type || '').toLowerCase();
    const name = String(file?.name || file?.url || '').split('?')[0].toLowerCase();
    if (type.startsWith('image/') || /\.(jpg|jpeg|png|gif|webp|bmp)$/.test(name)) return 'image';
    if (type === 'application/pdf' || name.endsWith('.pdf')) return 'pdf';
    return 'other';
  };

  const handlePreviewReport = async (file: any) => {
    const kind = reportPreviewKind(file);
    if (file.originFileObj && !file.isExistingReport) {
      const localURL = URL.createObjectURL(file.originFileObj);
      setReportPreview({
        open: true, loading: false, name: file.name, url: localURL, kind, localObjectUrl: true,
      });
      return;
    }
    if (!file.url || !patientCode) {
      appMessage.error('报告文件地址不存在');
      return;
    }
    setReportPreview({ open: true, loading: true, name: file.name, url: '', kind, fileUrl: file.url, analysisLoading: true });
    void loadExistingReportAnalysis(file.url);
    try {
      const response = await fetch(
        `/api/patients/${encodeURIComponent(String(patientCode))}/report-files/preview?file_url=${encodeURIComponent(file.url)}`,
        {
          headers: { 'Authorization': `Bearer ${localStorage.getItem('token')}` },
        },
      );
      const result = await response.json();
      if (!response.ok || result.code !== 200 || !result.data?.preview_url) {
        throw new Error(result.message || '报告预览失败');
      }
      setReportPreview((current) => ({
        ...current,
        open: true,
        loading: false,
        name: file.name,
        url: result.data.preview_url,
        kind,
        fileUrl: file.url,
      }));
    } catch (error: any) {
      setReportPreview((current) => ({ ...current, open: false, loading: false }));
      appMessage.error(error?.message || '报告预览失败');
    }
  };

  const loadExistingReportAnalysis = async (fileUrl: string, force = false) => {
    if (!patientCode || !fileUrl) return;
    setReportPreview((current) => ({ ...current, analysisLoading: true, analysis: force ? undefined : current.analysis }));
    const endpoint = `/api/patients/${encodeURIComponent(String(patientCode))}/report-files/analysis`;
    const headers = { 'Authorization': `Bearer ${localStorage.getItem('token')}` };
    try {
      if (!force) {
        const response = await fetch(`${endpoint}?file_url=${encodeURIComponent(fileUrl)}`, { headers });
        const result = await response.json();
        if (!response.ok || result.code !== 200) throw new Error(result.message || '读取AI分析失败');
        if (result.data?.status === 'completed') {
          setReportPreview((current) => ({ ...current, analysisLoading: false, analysis: result.data }));
          return;
        }
      }
      const response = await fetch(`${endpoint}${force ? '?force=1' : ''}`, {
        method: 'POST',
        headers: { ...headers, 'Content-Type': 'application/json' },
        body: JSON.stringify({ file_url: fileUrl }),
      });
      const result = await response.json();
      if (!response.ok || result.code !== 200) throw new Error(result.message || '报告分析失败');
      setReportPreview((current) => ({ ...current, analysisLoading: false, analysis: result.data }));
    } catch (error: any) {
      setReportPreview((current) => ({
        ...current,
        analysisLoading: false,
        analysis: { status: 'failed', error_message: error?.message || '报告分析失败，请稍后重试' },
      }));
    }
  };

  const closeReportPreview = () => {
    if (reportPreview.localObjectUrl && reportPreview.url) {
      URL.revokeObjectURL(reportPreview.url);
    }
    setReportPreview({ open: false, loading: false, name: '', url: '', kind: 'other' });
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
        if (!response.ok || result.code !== 200) {
          throw new Error(result?.data?.error || result.message || '文件上传失败');
        }
      }
      message.success('文件上传成功');
      setFileList([]);
    } catch (error: any) {
      message.error(`文件上传失败: ${error?.message || '未知错误'}`);
      throw error;
    }
  };

  const handleSubmit = async (values: any) => {
    if (!patientCode) {
      appMessage.error('患者编号不存在');
      return;
    }
    try {
      // 先上传文件
      await handleFileUpload();
      if (!values.phone) {
        await new Promise<void>((resolve, reject) => {
          Modal.confirm({
            title: '未填写手机号',
            content: '未填写手机号，仅用于存量用户无信息，请及时提醒患者完善身份信息和手机号。',
            okText: '继续提交',
            cancelText: '返回填写',
            onOk: () => resolve(),
            onCancel: () => reject(new Error('cancel')),
          });
        });
      }
      
      // 如果生日是 dayjs 对象，转换为字符串格式
      const formattedValues = {
        ...values,
        idDocumentType: values.idDocumentType || '居民身份证',
        idDocumentNo: values.idDocumentNo || values.idCard,
        idCard: (values.idDocumentType || '居民身份证') === '居民身份证' ? (values.idDocumentNo || values.idCard) : '',
        birthday: values.birthday ? values.birthday.format('YYYY-MM-DD') : undefined,
        collectionDate: values.collectionDate ? values.collectionDate.format('YYYY-MM-DD') : undefined,
        completionStatus: completionStatus,
        patientStatus: values.patientStatus !== undefined ? values.patientStatus : 1,
      };
      await updatePatient(patientCode, formattedValues);
      appMessage.success('患者信息更新成功');
      navigate('/patient/list');
    } catch (_error) {
      appMessage.error('患者信息更新失败');
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: 'center', padding: '50px' }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <div>
      <Card title="编辑患者">
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          {/* 基本信息 */}
          <Card type="inner" title="基本信息" style={{ marginBottom: 16 }}>
            {/* 第一行：姓名、性别、身份证件 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="name"
                  label="姓名"
                  rules={[{ required: true, message: '请输入姓名' }]}
                >
                  <Input placeholder="请输入姓名" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="gender"
                  label="性别"
                  rules={[{ required: true, message: '请选择性别' }]}
                >
                    <Select placeholder="性别" disabled={idDocumentType === '居民身份证'}>
                    <Select.Option value="男">男</Select.Option>
                    <Select.Option value="女">女</Select.Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="idDocumentType"
                  label="身份证件类型"
                  rules={[{ required: true, message: '请选择身份证件类型' }]}
                >
                  <Select
                    onChange={(value) => {
                      form.setFieldsValue({ idDocumentNo: '', idCard: '' });
                      if (value === '自编号') {
                        Modal.info({
                          title: '提示',
                          content: '此选项仅用于存量用户，请及时提醒患者完善身份信息和手机号。',
                        });
                      }
                    }}
                  >
                    {documentTypeOptions.map(item => <Option key={item} value={item}>{item}</Option>)}
                  </Select>
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="idDocumentNo"
                  label="身份证件号"
                  rules={[
                    { required: true, message: '请输入身份证件号' },
                    {
                      validator: (_, value) => {
                        if (!value) return Promise.resolve();
                        if (form.getFieldValue('idDocumentType') !== '居民身份证') return Promise.resolve();
                        const validationResult = validateIdCard(value);
                        if (!validationResult.isValid) {
                          return Promise.reject(new Error(validationResult.message));
                        }
                        return Promise.resolve();
                      }
                    }
                  ]}
                >
                  <Input placeholder="请输入身份证件号" onChange={handleIdCardChange} />
                </Form.Item>
              </Col>
            </Row>

            {/* 第二行：出生日期、联系电话、销售 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="birthday"
                  label="出生日期"
                  rules={[{ required: true, message: '请选择出生日期' }]}
                >
                  <DatePicker placeholder="出生日期" style={{ width: '100%' }} disabled={idDocumentType === '居民身份证'} format="YYYY-MM-DD" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                name="phone"
                label="联系电话"
              >
                  <Input placeholder="请输入联系电话" />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="salesPerson"
                  label="销售"
                  rules={[{ required: true, message: '请选择销售' }]}
                >
                  <Select
                    placeholder="请选择销售"
                    showSearch
                    filterOption={(input, option) => {
                      const optionLabel = typeof option?.children === 'string' ? option.children : '';
                      return optionLabel.toLowerCase().includes(input.toLowerCase());
                    }}
                    loading={salesLoading}
                    style={{ width: '100%' }}
                    disabled={getRoleName(initialState?.currentUser) === '销售'}
                  >
                    {salesUsers.map((user) => (
                      <Option key={getSalesPersonCode(user)} value={getSalesPersonCode(user)}>
                        {user.real_name || user.username} {user.employee_id ? `(${user.employee_id})` : ''}
                      </Option>
                    ))}
                  </Select>
                </Form.Item>
              </Col>
            </Row>

            {/* 第三行：地址 */}
            <Form.Item
              name="address"
              label="地址"
            >
              <Input.TextArea placeholder="请输入地址" rows={3} />
            </Form.Item>

            {/* 第四行：患者状态、吸烟状态 */}
            <Row gutter={16}>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="patientStatus"
                  label="患者状态"
                  rules={[{ required: true, message: '请选择患者状态' }]}
                >
                  <Select 
                    placeholder="请选择患者状态"
                    onChange={(value) => setPatientStatus(value)}
                  >
                    <Option value={1}>患病</Option>
                    <Option value={0}>健康</Option>
                  </Select>
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={8} lg={8} xl={8}>
                <Form.Item
                  name="smokingStatus"
                  label="吸烟状态"
                >
                  <Select placeholder="请选择吸烟状态" allowClear>
                    <Option value="不吸烟">不吸烟</Option>
                    <Option value="10支以内/日">10支以内/日</Option>
                    <Option value="10-20支/日">10-20支/日</Option>
                    <Option value="20支以上/日">20支以上/日</Option>
                  </Select>
                </Form.Item>
              </Col>
            </Row>
          </Card>



          {/* 病理与预后信息 - 仅在患病时显示 */}
          {patientStatus === 1 && (
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
          {patientStatus === 1 && (
            <Card type="inner" title="报告文件上传" style={{ marginBottom: 16 }}>
              <Form.Item label={`患者报告（已有及待上传共 ${fileList.length} 个，每个文件计为一个报告）`}>
                <Dragger
                  name="file"
                  fileList={fileList}
                  beforeUpload={beforeUpload}
                  onChange={handleUploadChange}
                  onRemove={handleRemoveReport}
                  onPreview={handlePreviewReport}
                  multiple
                >
                  <p className="ant-upload-drag-icon">
                    <UploadOutlined />
                  </p>
                  <p className="ant-upload-text">点击或拖拽文件到此区域上传</p>
                  <p className="ant-upload-hint">
                    支持上传JPG、PNG、PDF、DOC、DOCX格式文件，单个文件不超过20MB
                  </p>
                </Dragger>
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
      <Modal
        title={reportPreview.name || '报告预览'}
        open={reportPreview.open}
        onCancel={closeReportPreview}
        footer={null}
        width="min(1500px, 97vw)"
        className="patient-report-modal"
        destroyOnClose
      >
        <div className="patient-report-layout">
          <section className="patient-report-original" aria-label="报告原图">
            <div className="patient-report-original-head"><span className="patient-report-panel-title">报告原件</span></div>
            {reportPreview.loading && <Skeleton.Image active className="patient-report-skeleton-image" />}
            {!reportPreview.loading && reportPreview.kind === 'image' && reportPreview.url && (
              <div className="patient-report-media">
                <img src={reportPreview.url} alt={`${reportPreview.name} 原图`} />
              </div>
            )}
            {!reportPreview.loading && reportPreview.kind === 'pdf' && reportPreview.url && (
              <iframe src={reportPreview.url} title={`${reportPreview.name} 原文`} className="patient-report-pdf" />
            )}
            {!reportPreview.loading && reportPreview.kind === 'other' && reportPreview.url && (
              <Empty description="当前浏览器不支持直接预览此文件格式">
                <Button type="primary" onClick={() => window.open(reportPreview.url, '_blank', 'noopener,noreferrer')}>打开文件</Button>
              </Empty>
            )}
          </section>
          <section className="patient-report-analysis" aria-label="AI分析">
            <div className="patient-report-analysis-head">
              <Space>
                <RobotOutlined />
                <span className="patient-report-panel-title">AI 报告分析</span>
                {reportPreview.analysis?.status === 'completed' && <Tag color="success">已完成</Tag>}
              </Space>
              {reportPreview.fileUrl && (
                <Button
                  type="link"
                  icon={<SyncOutlined spin={reportPreview.analysisLoading} />}
                  disabled={reportPreview.analysisLoading}
                  onClick={() => loadExistingReportAnalysis(reportPreview.fileUrl!, true)}
                >
                  重新分析
                </Button>
              )}
            </div>
            {reportPreview.localObjectUrl ? (
              <Alert type="info" showIcon message="请先保存患者信息，上传完成后系统会生成AI分析。" />
            ) : reportPreview.analysisLoading ? (
              <div className="patient-report-analysis-loading" aria-live="polite">
                <Skeleton active paragraph={{ rows: 8 }} />
                <Typography.Text type="secondary">AI 正在识别报告类型并整理内容，请稍候…</Typography.Text>
              </div>
            ) : reportPreview.analysis?.status === 'completed' ? (
              <div className="patient-report-analysis-result">
                <Descriptions bordered size="small" column={1} className="patient-report-fields">
                  <Descriptions.Item label="报告类型">{reportPreview.analysis.report_type || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="医院">{reportPreview.analysis.hospital || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="检查时间">{reportPreview.analysis.examination_time || '未识别'}</Descriptions.Item>
                  <Descriptions.Item label="检查项目">{reportPreview.analysis.examination_item || '未识别'}</Descriptions.Item>
                </Descriptions>
                <Typography.Title level={5}>内容摘要</Typography.Title>
                <Typography.Paragraph className="patient-report-analysis-text">{reportPreview.analysis.content}</Typography.Paragraph>
                <div className="patient-report-analysis-meta">
                  {reportPreview.analysis.model && <span>模型：{reportPreview.analysis.model}</span>}
                  {reportPreview.analysis.analyzed_at && <span>分析时间：{reportPreview.analysis.analyzed_at}</span>}
                </div>
              </div>
            ) : reportPreview.analysis?.status === 'failed' ? (
              <Alert
                type="error"
                showIcon
                message="AI 分析未完成"
                description={reportPreview.analysis.error_message}
                action={<Button onClick={() => reportPreview.fileUrl && loadExistingReportAnalysis(reportPreview.fileUrl, true)}>重试</Button>}
              />
            ) : <Empty description="等待AI分析" />}
          </section>
        </div>
      </Modal>
    </div>
  );
};

export default Edit;
