import React, { useState, useEffect } from 'react';
import { Card, Form, Input, Select, Button, message, DatePicker } from 'antd';
import { createOrder, listPackages, listCancerTypes, checkIdCard } from '@/services/api';
import dayjs from 'dayjs';

const { Option } = Select;
const SalesPurchase: React.FC = () => {
  const [form] = Form.useForm();
  const [packages, setPackages] = useState<any[]>([]);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [loading, setLoading] = useState(false);
  const [patientInfo, setPatientInfo] = useState<any>(null);
  const [detectionDates, setDetectionDates] = useState<string[]>([]);

  // 加载套餐列表
  useEffect(() => {
    const loadPackages = async () => {
      try {
        const response = await listPackages();
        setPackages(response.data?.list || []);
      } catch (error) {
        message.error('加载套餐列表失败');
      }
    };

    const loadCancerTypes = async () => {
      try {
        const response = await listCancerTypes();
        setCancerTypes(response.data || []);
      } catch (error) {
        message.error('加载癌型列表失败');
      }
    };

    loadPackages();
    loadCancerTypes();
  }, []);

  // 处理身份证号输入，查询患者信息
  const handleIdCardChange = async (idCard: string) => {
    if (idCard.length === 18) {
      try {
        const response = await checkIdCard(idCard);
        if (response.data?.exists) {
          setPatientInfo(response.data.patient);
          form.setFieldsValue({
            surgeryDate: response.data.patient?.surgeryDate ? dayjs(response.data.patient.surgeryDate) : undefined,
            chemoStartDate: response.data.patient?.chemoStartDate ? dayjs(response.data.patient.chemoStartDate) : undefined,
          });
          message.success('患者信息已找到');
        } else {
          setPatientInfo(null);
          message.warning('患者不存在，请先添加患者信息');
        }
      } catch (error) {
        message.error('查询患者信息失败');
      }
    }
  };

  // 计算检测日期
  const calculateDetectionDates = (firstDate: string, packageId: string) => {
    const selectedPackage = packages.find(pkg => pkg.id.toString() === packageId);
    if (!selectedPackage || !firstDate) {
      setDetectionDates([]);
      return;
    }

    const dates: string[] = [];
    const startDate = dayjs(firstDate);
    const { detectionCount, intervalDays } = selectedPackage;

    for (let i = 0; i < detectionCount; i++) {
      const date = startDate.add(i * intervalDays, 'day');
      dates.push(date.format('YYYY-MM-DD'));
    }

    setDetectionDates(dates);
  };

  // 处理表单提交
  const handleSubmit = async (values: any) => {
    try {
      setLoading(true);
      const response = await createOrder({
        patientIdCard: values.patientIdCard,
        packageId: values.packageId,
        cancerTypeId: values.cancerTypeId,
        firstDetectionDate: values.firstDetectionDate.format('YYYY-MM-DD'),
        paymentMethod: values.paymentMethod,
        surgeryDate: values.surgeryDate ? values.surgeryDate.format('YYYY-MM-DD') : '',
        chemoStartDate: values.chemoStartDate ? values.chemoStartDate.format('YYYY-MM-DD') : '',
      });
      message.success('套餐购买成功');
      form.resetFields();
      setPatientInfo(null);
      setDetectionDates([]);
    } catch (error) {
      message.error('套餐购买失败，请稍后重试');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div style={{ padding: '20px' }}>
      <Card title="套餐购买">
        <Form
          form={form}
          layout="vertical"
          onFinish={handleSubmit}
        >
          <Form.Item
            label="患者身份证号"
            name="patientIdCard"
            rules={[
              { required: true, message: '请输入患者身份证号' },
              { pattern: /^[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[0-9Xx]$/, message: '请输入有效的身份证号' },
            ]}
          >
            <Input placeholder="请输入患者身份证号" onChange={(e) => handleIdCardChange(e.target.value)} />
          </Form.Item>

          {patientInfo && (
            <Card type="inner" title="患者信息" style={{ marginBottom: '20px' }}>
              <div>姓名: {patientInfo.name}</div>
              <div>性别: {patientInfo.gender === 'male' ? '男' : '女'}</div>
              <div>年龄: {patientInfo.age}</div>
            </Card>
          )}

          <Form.Item
            label="手术时间"
            name="surgeryDate"
            tooltip="录入后可用于后续检测日期评估"
          >
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="化疗开始时间"
            name="chemoStartDate"
            tooltip="录入后可用于后续检测日期评估"
          >
            <DatePicker style={{ width: '100%' }} />
          </Form.Item>

          <Form.Item
            label="选择套餐"
            name="packageId"
            rules={[{ required: true, message: '请选择套餐' }]}
          >
            <Select placeholder="请选择套餐" onChange={(value) => {
              const firstDate = form.getFieldValue('firstDetectionDate');
              if (firstDate) {
                calculateDetectionDates(firstDate.format('YYYY-MM-DD'), value);
              }
            }}>
              {packages.map(pkg => (
                <Option key={pkg.id} value={pkg.id}>
                  {pkg.name} - ¥{pkg.price} (检测{pkg.detectionCount}次，间隔{pkg.intervalDays}天)
                </Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label="选择癌型"
            name="cancerTypeId"
            rules={[{ required: true, message: '请选择癌型' }]}
          >
            <Select placeholder="请选择癌型">
              {cancerTypes.map(type => (
                <Option key={type.id} value={type.id}>{type.name}</Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label="第一次检查日"
            name="firstDetectionDate"
            rules={[{ required: true, message: '请选择第一次检查日' }]}
          >
            <DatePicker
              style={{ width: '100%' }}
              onChange={(date) => {
                if (date) {
                  const packageId = form.getFieldValue('packageId');
                  if (packageId) {
                    calculateDetectionDates(date.format('YYYY-MM-DD'), packageId);
                  }
                } else {
                  setDetectionDates([]);
                }
              }}
            />
          </Form.Item>

          {detectionDates.length > 0 && (
            <Card type="inner" title="检测计划" style={{ marginBottom: '20px' }}>
              {detectionDates.map((date, index) => (
                <div key={index} style={{ marginBottom: '8px' }}>
                  第{index + 1}次检测: {date}
                </div>
              ))}
            </Card>
          )}

          <Form.Item
            label="付款方式"
            name="paymentMethod"
            rules={[{ required: true, message: '请选择付款方式' }]}
          >
            <Select placeholder="请选择付款方式">
              <Option value="online">在线支付</Option>
              <Option value="offline">线下支付</Option>
            </Select>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={loading}>
              提交订单
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
};

export default SalesPurchase;
