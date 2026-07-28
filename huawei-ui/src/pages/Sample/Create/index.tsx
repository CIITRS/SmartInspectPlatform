import React, { useEffect, useRef, useState } from 'react';
import { App, Button, Card, Col, DatePicker, Form, Input, InputNumber, Radio, Row, Select, Space } from 'antd';
import { useNavigate, useSearchParams } from '@umijs/max';
import dayjs from 'dayjs';
import { allocateSamples, createSample, getSampleTypes, getTreatmentStages, listCancerTypes, listPatients } from '@/services/api';

const treatmentStageOrder = ['健康体检', '辅助诊断', '术前评估', '术后检测', '残留检测', '复发监测', '化疗前', '化疗后'];

const getTreatmentStageOrderIndex = (name?: string) => {
  const index = treatmentStageOrder.indexOf(String(name || '').trim());
  return index >= 0 ? index : treatmentStageOrder.length;
};

const SampleCreate: React.FC = () => {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const { message } = App.useApp();
  const [patients, setPatients] = useState<any[]>([]);
  const [patientLoading, setPatientLoading] = useState(false);
  const [cancerTypes, setCancerTypes] = useState<any[]>([]);
  const [sampleTypes, setSampleTypes] = useState<any[]>([]);
  const [treatmentStages, setTreatmentStages] = useState<any[]>([]);
  const [mode, setMode] = useState<'single' | 'batch'>('single');
  const patientSearchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const patientRequestSequenceRef = useRef(0);

  const loadPatients = async (keyword = '') => {
    const requestSequence = ++patientRequestSequenceRef.current;
    setPatientLoading(true);
    try {
      const response = await listPatients({
        page: 1,
        pageSize: 50,
        keyword: keyword.trim() || undefined,
      }, { skipErrorHandler: true });
      if (requestSequence !== patientRequestSequenceRef.current) return;

      const nextPatients = response.data?.list || [];
      setPatients((currentPatients) => {
        const selectedIds = [
          form.getFieldValue('patient_id'),
          ...(form.getFieldValue('patient_ids') || []),
        ].filter(Boolean);
        const selectedPatients = currentPatients.filter((patient) => selectedIds.includes(patient.id));
        const selectedPatientIds = new Set(selectedPatients.map((patient) => patient.id));
        return [...selectedPatients, ...nextPatients.filter((patient: any) => !selectedPatientIds.has(patient.id))];
      });
    } catch (_error) {
      if (requestSequence === patientRequestSequenceRef.current) {
        message.error('搜索患者失败');
      }
    } finally {
      if (requestSequence === patientRequestSequenceRef.current) {
        setPatientLoading(false);
      }
    }
  };

  const handlePatientSearch = (keyword: string) => {
    if (patientSearchTimerRef.current) {
      clearTimeout(patientSearchTimerRef.current);
    }
    patientSearchTimerRef.current = setTimeout(() => {
      loadPatients(keyword);
    }, 300);
  };

  useEffect(() => {
    const loadOptions = async () => {
      try {
        const [cancerRes, sampleTypeRes, stageRes] = await Promise.all([
          listCancerTypes({}, { skipErrorHandler: true }),
          getSampleTypes({}, { skipErrorHandler: true }),
          getTreatmentStages({}, { skipErrorHandler: true }),
        ]);
        setCancerTypes(cancerRes.data || []);
        setSampleTypes(sampleTypeRes.data || []);
        setTreatmentStages((stageRes.data || [])
          .filter((item: any) => Number(item.isActive ?? item.is_active ?? 1) === 1)
          .filter((item: any) => treatmentStageOrder.includes(String(item.name || '').trim()))
          .sort((a: any, b: any) => getTreatmentStageOrderIndex(a.name) - getTreatmentStageOrderIndex(b.name) || Number(a.id || 0) - Number(b.id || 0)));
      } catch (_error) {
        message.error('获取选项列表失败');
      }
    };
    loadPatients();
    loadOptions();
    form.setFieldsValue({
      mode: 'single',
      collection_date: dayjs(),
      report_type: 'normal',
      sample_code: searchParams.get('sampleCode') || undefined,
      cancer_type_id: searchParams.get('cancerTypeId') ? Number(searchParams.get('cancerTypeId')) : undefined,
      organization_type: '个人送检',
      organization: '个人送检',
    });

    return () => {
      if (patientSearchTimerRef.current) {
        clearTimeout(patientSearchTimerRef.current);
      }
      patientRequestSequenceRef.current += 1;
    };
  }, []);

  const organizationType = Form.useWatch('organization_type', form) || '个人送检';

  const handleSubmit = async (values: any) => {
    const patientIds = mode === 'single' ? [values.patient_id] : values.patient_ids;
    if (!patientIds || patientIds.length === 0) {
      message.error('请选择患者');
      return;
    }

    const payload = {
      patient_ids: patientIds,
      sample_type_id: values.sample_type_id,
      cancer_type_id: values.cancer_type_id,
      treatment_stage_id: values.treatment_stage_id,
      report_type: values.report_type || 'normal',
      start_sequence: values.start_sequence || 0,
      collection_date: values.collection_date ? values.collection_date.toISOString() : dayjs().toISOString(),
      organization: values.organization_type === '单位送检' ? values.organization || '' : '个人送检',
      notes: values.notes || '',
    };

    try {
      if (mode === 'single' && values.sample_code) {
        await createSample({
          patient_id: values.patient_id,
          sample_type_id: values.sample_type_id,
          cancer_type_id: values.cancer_type_id,
          treatment_stage_id: values.treatment_stage_id,
          report_type: values.report_type || 'normal',
          collection_date: payload.collection_date,
          sample_code: values.sample_code,
          organization: payload.organization,
          notes: values.notes || '',
        }, { skipErrorHandler: true });
      } else {
        await allocateSamples(payload, { skipErrorHandler: true });
      }

      message.success('新增样本成功');
      form.setFieldsValue({ sample_code: undefined, start_sequence: undefined });
    } catch (error: any) {
      message.error(error?.response?.data?.message || error?.data?.message || error?.message || '新增样本失败');
    }
  };

  return (
    <div>
      <Card title="新增样本">
        <Form form={form} layout="vertical" onFinish={handleSubmit}>
          <Form.Item name="mode" label="新增方式">
            <Radio.Group
              onChange={(event) => {
                setMode(event.target.value);
              }}
            >
              <Radio.Button value="single">逐个新增</Radio.Button>
              <Radio.Button value="batch">批量新增</Radio.Button>
            </Radio.Group>
          </Form.Item>

          {mode === 'single' ? (
            <Form.Item name="patient_id" label="患者" rules={[{ required: true, message: '请选择患者' }]}>
              <Select
                showSearch
                placeholder="选择自己的患者"
                filterOption={false}
                loading={patientLoading}
                onSearch={handlePatientSearch}
                notFoundContent={patientLoading ? '搜索中...' : '未找到匹配患者'}
                options={patients.map((patient) => ({
                  value: patient.id,
                  label: `${patient.name || '-'} ${patient.phone || ''} ${patient.idCard || ''} ${patient.patientCode || ''}`,
                }))}
              />
            </Form.Item>
          ) : (
            <Form.Item name="patient_ids" label="患者" rules={[{ required: true, message: '请选择患者' }]}>
              <Select
                mode="multiple"
                showSearch
                placeholder="选择自己的患者，顺序即分配顺序"
                filterOption={false}
                loading={patientLoading}
                onSearch={handlePatientSearch}
                notFoundContent={patientLoading ? '搜索中...' : '未找到匹配患者'}
                options={patients.map((patient) => ({
                  value: patient.id,
                  label: `${patient.name || '-'} ${patient.phone || ''} ${patient.idCard || ''} ${patient.patientCode || ''}`,
                }))}
              />
            </Form.Item>
          )}

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="sample_type_id" label="样本类型" rules={[{ required: true, message: '请选择样本类型' }]}>
                <Select placeholder="请选择样本类型" options={sampleTypes.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="cancer_type_id" label="检测癌种" rules={[{ required: true, message: '请选择检测癌种' }]}>
                <Select placeholder="请选择检测癌种" options={cancerTypes.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="report_type" label="报告类型" rules={[{ required: true, message: '请选择报告类型' }]}>
                <Select options={[
                  { value: 'normal', label: '高敏（MePlex高敏98CpG）' },
                  { value: 'high', label: '超敏（MePlex超敏180CpG）' },
                ]} />
              </Form.Item>
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={8}>
              <Form.Item name="treatment_stage_id" label="治疗阶段" rules={[{ required: true, message: '请选择治疗阶段' }]}>
                <Select placeholder="请选择治疗阶段" options={treatmentStages.map((item) => ({ value: item.id, label: item.name }))} />
              </Form.Item>
            </Col>
            <Col span={8}>
              <Form.Item name="collection_date" label="创建时间">
                <DatePicker showTime style={{ width: '100%' }} />
              </Form.Item>
            </Col>
            <Col span={8}>
              {mode === 'single' ? (
                <Form.Item name="sample_code" label="完整样本编号">
                  <Input placeholder="可扫码/手输；留空按工号自动生成" />
                </Form.Item>
              ) : (
                <Form.Item name="start_sequence" label="起始后4位序号">
                  <InputNumber min={1} max={9999} precision={0} style={{ width: '100%' }} placeholder="留空自动挨着分配" />
                </Form.Item>
              )}
            </Col>
          </Row>

          <Row gutter={16}>
            <Col span={12}>
              <Form.Item name="organization_type" label="送检方式" rules={[{ required: true, message: '请选择送检方式' }]}>
                <Radio.Group
                  onChange={(event) => {
                    if (event.target.value === '个人送检') {
                      form.setFieldValue('organization', '个人送检');
                    } else {
                      form.setFieldValue('organization', undefined);
                    }
                  }}
                >
                  <Radio.Button value="个人送检">个人送检</Radio.Button>
                  <Radio.Button value="单位送检">单位送检</Radio.Button>
                </Radio.Group>
              </Form.Item>
              {organizationType === '单位送检' ? (
                <Form.Item name="organization" label="送检单位" rules={[{ required: true, message: '请输入送检单位' }]}>
                  <Input placeholder="请输入送检单位" />
                </Form.Item>
              ) : (
                <Form.Item name="organization" label="送检单位">
                  <Input disabled />
                </Form.Item>
              )}
            </Col>
            <Col span={12}>
              <Form.Item name="notes" label="备注">
                <Input placeholder="请输入备注" />
              </Form.Item>
            </Col>
          </Row>

          <Space>
            <Button type="primary" htmlType="submit">提交新增</Button>
            <Button onClick={() => navigate('/sample/list')}>返回列表</Button>
          </Space>
        </Form>
      </Card>

    </div>
  );
};

export default SampleCreate;
