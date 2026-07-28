import type React from 'react';
import { useState, useEffect, useCallback } from 'react';
import { Form, Input, Select, DatePicker, Button, Card, App, Row, Col, Modal } from 'antd';
import { createPatient } from '@/services/api';
import { useNavigate, useModel } from '@umijs/max';
import dayjs from 'dayjs'; // 引入 dayjs

interface User {
  id: number;
  name: string;
  role_name: string;
  real_name?: string;
  username?: string;
  employee_id?: string;
  [key: string]: unknown;
}

interface PatientFormValues {
  name: string;
  gender: string;
  age: number;
  idDocumentType: string;
  idDocumentNo: string;
  idCard: string;
  phone: string;
  birthday?: dayjs.Dayjs;
  address?: string;
  salesPersonId: number;
  patientStatus?: number;
  cancerDiameter?: string;
  [key: string]: unknown;
}

interface CurrentUser {
  id: number;
  name: string;
  role_name: string;
  [key: string]: unknown;
}

const { Option } = Select;
const documentTypeOptions = ['居民身份证', '护照', '港澳居民来往内地通行证', '台湾居民来往大陆通行证', '自编号'];

const getSalesPersonCode = (user: any) =>
  String(user?.employee_id || '').trim();

const getRoleName = (user: any) => user?.role_name || user?.role?.name || '';

const getApiErrorMessage = (error: any, fallback: string) => (
  error?.response?.data?.message ||
  error?.response?.data?.errorMessage ||
  error?.data?.message ||
  error?.message ||
  fallback
);

const Create: React.FC = () => {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const { message: appMessage } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const [salesUsers, setSalesUsers] = useState<User[]>([]);
  const [salesLoading, setSalesLoading] = useState(false);
  const idDocumentType = Form.useWatch('idDocumentType', form) || '居民身份证';

  // 获取销售岗用户列表
  const fetchSalesUsers = useCallback(async () => {
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
        const sales = result.data.list.filter((user: User) =>
          (user.role_name === '管理员' || user.role_name === '销售') && getSalesPersonCode(user)
        );
        setSalesUsers(sales);

        // 如果当前用户是销售岗，默认设置为当前用户
        const currentUser = initialState?.currentUser;
        const roleName = getRoleName(currentUser);
        if (currentUser && (roleName === '管理员' || roleName === '销售')) {
          // 默认设置为当前用户
          form.setFieldsValue({ salesPerson: getSalesPersonCode(currentUser) });
        }
      }
    } catch (_error) {
      appMessage.error('获取销售列表失败');
    } finally {
      setSalesLoading(false);
    }
  }, [form, initialState, appMessage]);

  // 组件挂载时获取销售列表
  useEffect(() => {
    fetchSalesUsers();
  }, [fetchSalesUsers]);

  const handleSubmit = async (values: PatientFormValues) => {
    try {
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
        // 设置初始完成状态为待完善
        completionStatus: 'pending',
        // 添加患者状态字段，默认为患病
        patientStatus: values.patientStatus !== undefined ? values.patientStatus : 1,
        // 添加肿瘤直径字段
        cancerDiameter: values.cancerDiameter || ''
      };
      const response = await createPatient(formattedValues, { skipErrorHandler: true });
      appMessage.success(`患者创建成功，患者编号为：${response.data.patientCode}`);
      navigate('/patient/list');
    } catch (error: any) {
      if (error?.message === 'cancel') return;
      appMessage.error(getApiErrorMessage(error, '患者创建失败'));
    }
  };

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
  const handleIdCardChange = (e: React.ChangeEvent<HTMLInputElement>) => {
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

  return (
    <div>
      <Card title="新增患者">
        <Form
          form={form}
          layout="vertical"
          initialValues={{ idDocumentType: '居民身份证' }}
          onFinish={handleSubmit}
        >
          {/* 第一行：姓名、身份证件、联系电话 */}
          <Row gutter={16}>
            <Col xs={24} sm={12} md={6} lg={6} xl={6}>
              <Form.Item
                name="name"
                label="姓名"
                rules={[{ required: true, message: '请输入姓名' }]}
              >
                <Input placeholder="请输入姓名" />
              </Form.Item>
            </Col>
            <Col xs={24} sm={12} md={6} lg={6} xl={6}>
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
            <Col xs={24} sm={12} md={6} lg={6} xl={6}>
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
            <Col xs={24} sm={12} md={6} lg={6} xl={6}>
              <Form.Item
                name="phone"
                label="联系电话"
              >
                <Input placeholder="请输入联系电话" />
              </Form.Item>
            </Col>
          </Row>

          {/* 第二行：性别、出生日期、销售 */}
          <Row gutter={16}>
            <Col xs={24} sm={12} md={8} lg={8} xl={8}>
              <Form.Item
                name="gender"
                label="性别"
                rules={[{ required: true, message: '请选择性别' }]}
              >
                <Select placeholder="性别" disabled={idDocumentType === '居民身份证'}>
                  <Option value="男">男</Option>
                  <Option value="女">女</Option>
                </Select>
              </Form.Item>
            </Col>
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

          {/* 地址行：单独一行 */}
          <Form.Item
            name="address"
            label="地址"
          >
            <Input.TextArea placeholder="请输入地址" rows={3} />
          </Form.Item>

          {/* 医疗信息 */}
          <Card type="inner" title="医疗信息" style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                <Form.Item
                  name="diagnosis"
                  label="临床诊断"
                >
                  <Input.TextArea placeholder="请输入临床诊断" rows={3} />
                </Form.Item>
              </Col>
              <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                <Row gutter={16}>
                  <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                    <Form.Item
                      name="smokingStatus"
                      label="吸烟状态"
                    >
                      <Select placeholder="请选择吸烟状态">
                        <Option value="不吸烟">不吸烟</Option>
                        <Option value="10支以内/日">10支以内/日</Option>
                        <Option value="10-20支/日">10-20支/日</Option>
                        <Option value="20支以上/日">20支以上/日</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                  <Col xs={24} sm={12} md={12} lg={12} xl={12}>
                    <Form.Item
                      name="patientStatus"
                      label="患者状态"
                      rules={[{ required: true, message: '请选择患者状态' }]}
                    >
                      <Select placeholder="请选择患者状态">
                        <Option value={1}>患病</Option>
                        <Option value={0}>健康</Option>
                      </Select>
                    </Form.Item>
                  </Col>
                </Row>
                <Row gutter={16} style={{ marginTop: 16 }}>
                  <Col xs={24} sm={24} md={24} lg={24} xl={24}>
                    <Form.Item
                      name="cancerDiameter"
                      label="肿瘤直径"
                    >
                      <Input placeholder="请输入肿瘤直径，单位：cm" />
                    </Form.Item>
                  </Col>
                </Row>
              </Col>
            </Row>
          </Card>

          {/* 其他信息 - 可选 */}
          <Form.Item
            name="otherInfo"
            label="其他信息"
          >
            <Input.TextArea placeholder="请输入其他信息" rows={4} />
          </Form.Item>

          {/* 提交按钮行 */}
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

export default Create;
