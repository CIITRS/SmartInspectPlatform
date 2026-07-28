import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, Card, App, Space, DatePicker, Checkbox, Upload, Select, Typography, Tag } from 'antd';
const { Option } = Select;
const { Title, Paragraph } = Typography;
import { useNavigate, useModel } from '@umijs/max';
import { PlusOutlined, UploadOutlined, DeleteOutlined, EditOutlined, DownloadOutlined, ReloadOutlined, SearchOutlined, FileExcelOutlined, DeleteFilled, UndoOutlined, EditTwoTone, InboxOutlined } from '@ant-design/icons';
import { listPatients, deletePatient, getPatientById, getSamplesByPatientId, listSalesAssignmentPatients, assignSalesToPatient } from '@/services/api';
import dayjs from 'dayjs';

const getSalesPersonCode = (user: any) =>
  String(user?.employee_id || '').trim();

const getRoleName = (user: any) => user?.role_name || user?.role?.name || '';
const hasRole = (user: any, role: string) => {
  const names = Array.isArray(user?.role_names) ? user.role_names : [getRoleName(user)];
  return names.some((name: string) => String(name || '').includes(role));
};

const patientSourceText: Record<string, string> = {
  miniapp_self: '自主注册',
  sales_invite: '销售邀请',
};

const renderPatientSource = (source?: string) => {
  const value = String(source || '').trim();
  if (!value) {
    return '-';
  }
  return <Tag color="blue">{patientSourceText[value] || value}</Tag>;
};

const List: React.FC = () => {
  const [form] = Form.useForm();
  const [loading, setLoading] = React.useState(false);
  const [patients, setPatients] = React.useState<any[]>([]);
  const [pagination, setPagination] = React.useState({
    current: 1,
    pageSize: 10,
    total: 0,
    showSizeChanger: true,
  });
  const [searchParams, setSearchParams] = React.useState({});
  const [currentPatient, setCurrentPatient] = React.useState<any>(null);
  const [detailModalVisible, setDetailModalVisible] = React.useState(false);
  const [detailLoading, setDetailLoading] = React.useState(false);
  const [samples, setSamples] = React.useState<any[]>([]);
  const [samplesLoading, setSamplesLoading] = React.useState(false);
  const [uploadModalVisible, setUploadModalVisible] = React.useState(false);
  const [exportModalVisible, setExportModalVisible] = React.useState(false);
  const [exportForm] = Form.useForm();
  const [exportFields, setExportFields] = React.useState<string[]>([
    'patientCode', 'name', 'gender', 'age', 'idDocumentType', 'idDocumentNo', 'phone', 'patientSource', 'treatmentStage', 'createdAt'
  ]);
  const [recycleModalVisible, setRecycleModalVisible] = React.useState(false);
  const [recyclePatients, setRecyclePatients] = React.useState<any[]>([]);
  const [recycleLoading, setRecycleLoading] = React.useState(false);
  const [recycleSearch, setRecycleSearch] = React.useState('');
  const [recyclePagination, setRecyclePagination] = React.useState({
    current: 1,
    pageSize: 10,
    total: 0,
    showSizeChanger: true,
  });
  const [pathologyExpanded, setPathologyExpanded] = React.useState(false);
  const [filesExpanded, setFilesExpanded] = React.useState(false);
  const [advancedSearchVisible, setAdvancedSearchVisible] = React.useState(false);
  const [salesUsers, setSalesUsers] = React.useState<any[]>([]);
  const [assignVisible, setAssignVisible] = React.useState(false);
  const [assignPatients, setAssignPatients] = React.useState<any[]>([]);
  const [assignLoading, setAssignLoading] = React.useState(false);
  const [assignKeyword, setAssignKeyword] = React.useState('');
  const [assignSelectedRowKeys, setAssignSelectedRowKeys] = React.useState<React.Key[]>([]);
  const [assignForm] = Form.useForm();
  
  const navigate = useNavigate();
  const { message: appMessage } = App.useApp();
  const { initialState } = useModel('@@initialState');
  const [currentUser, setCurrentUser] = useState<any>(null);

  useEffect(() => {
    if (initialState?.currentUser) {
      setCurrentUser(initialState.currentUser);
    }
  }, [initialState]);

  // 获取销售用户列表
  const fetchSalesUsers = async () => {
    try {
      const response = await fetch('/api/system/users', {
        method: 'GET',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
      });
      const result = await response.json();
      if (result.code === 200) {
        const sales = result.data.list.filter((user: any) => hasRole(user, '销售') && getSalesPersonCode(user));
        setSalesUsers(sales);
      }
    } catch (_error) {
      console.error('获取销售列表失败');
    }
  };

  // 组件挂载时获取销售用户列表
  useEffect(() => {
    fetchSalesUsers();
  }, []);

  // 获取活跃患者列表
  const fetchPatients = async (params: any = {}, searchParamsOverride?: any) => {
    setLoading(true);
    try {
      // 根据角色构建查询参数
      const queryParams = {
        ...(searchParamsOverride || searchParams),
        ...params,
        is_active: 1
      };

      // 销售角色只查看自己负责且未完成的患者
      if (currentUser && getRoleName(currentUser) === '销售') {
        queryParams.sales_person = getSalesPersonCode(currentUser);
        queryParams.completion_status = 'pending';
      }

      const response = await listPatients(queryParams);
      const patientList = response.data?.list || [];

      // 处理患者列表，添加默认完成状态
      const processedPatients = patientList.map((patient: any) => ({
        ...patient,
        completionStatus: patient.completionStatus || 'pending'
      }));

      setPatients(processedPatients);
      setPagination({
        ...pagination,
        total: response.data?.total || 0,
        current: params.page || 1,
        pageSize: params.pageSize || pagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取患者列表失败');
    } finally {
      setLoading(false);
    }
  };

  // 获取回收站患者列表
  const fetchRecyclePatients = async (params: any = {}) => {
    setRecycleLoading(true);
    try {
      const response = await listPatients({ 
        ...params, 
        is_active: 0, 
        keyword: recycleSearch 
      });
      setRecyclePatients(response.data?.list || []);
      setRecyclePagination({
        ...recyclePagination,
        total: response.data?.total || 0,
        current: params.page || 1,
        pageSize: params.pageSize || recyclePagination.pageSize,
      });
    } catch (_error) {
      appMessage.error('获取回收站患者列表失败');
    } finally {
      setRecycleLoading(false);
    }
  };

  React.useEffect(() => {
    fetchPatients();
  }, [searchParams]);

  // 当回收站模态框打开时自动加载数据
  React.useEffect(() => {
    if (recycleModalVisible) {
      fetchRecyclePatients({ page: 1 });
    }
  }, [recycleModalVisible]);

  const handleSearch = (values: any) => {
    // 转换字段名称，将驼峰命名转换为蛇形命名，以匹配后端API期望的格式
    const convertedValues = {
      name: values.name,
      id_document_no: values.idDocumentNo || values.idCard,
      completion_status: values.completionStatus,
      sales_person: values.salesPerson,
      phone: values.phone,
      patient_code: values.patientCode
    };
    setSearchParams(convertedValues);
    fetchPatients({ page: 1 }, convertedValues);
  };

  const handleResetSearch = () => {
    form.resetFields();
    setSearchParams({});
    fetchPatients({ page: 1 }, {});
  };

  const handleCreate = () => {
    navigate('/patient/create');
  };

  const handleEdit = (patientCode: string) => {
    navigate(`/patient/edit/${patientCode}`);
  };

  // 完善患者信息
  const handleComplete = (patientCode: string) => {
    navigate(`/patient/complete/${patientCode}`);
  };

  // 获取患者样本
  const fetchPatientSamples = async (patientCode: string) => {
    setSamplesLoading(true);
    try {
      // 使用新的API获取样本列表
      const response = await getSamplesByPatientId(patientCode);
      setSamples(response.data?.list || []);
    } catch (_error) {
      // 忽略错误，显示空列表
      setSamples([]);
    } finally {
      setSamplesLoading(false);
    }
  };

  const handleViewDetail = async (patientCode: string) => {
    setDetailLoading(true);
    try {
      const response = await getPatientById(patientCode);
      setCurrentPatient(response.data);
      setDetailModalVisible(true);
      // 获取患者样本
      await fetchPatientSamples(patientCode);
    } catch (_error) {
      appMessage.error('获取患者详情失败');
    } finally {
      setDetailLoading(false);
    }
  };

  const fetchAssignPatients = async (params: any = {}) => {
    setAssignLoading(true);
    try {
      const response = await listSalesAssignmentPatients({ pageSize: 100, keyword: assignKeyword, ...params });
      setAssignPatients(response.data?.list || []);
    } catch (_error) {
      appMessage.error('获取待分配患者失败');
    } finally {
      setAssignLoading(false);
    }
  };

  const openAssignModal = () => {
    setAssignVisible(true);
    setAssignSelectedRowKeys([]);
    fetchAssignPatients({ page: 1 });
  };

  const handleAssignSales = async () => {
    const values = await assignForm.validateFields();
    if (assignSelectedRowKeys.length === 0) {
      appMessage.warning('请选择患者');
      return;
    }
    try {
      await assignSalesToPatient({
        patient_ids: assignSelectedRowKeys.map((id) => Number(id)),
        sales_person: values.sales_person,
      });
      appMessage.success('分配销售成功');
      setAssignVisible(false);
      setAssignSelectedRowKeys([]);
      fetchPatients();
    } catch (_error) {
      appMessage.error('分配销售失败');
    }
  };

  const _handleDelete = (patientCode: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该患者信息吗？删除后可在回收站中恢复。',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await deletePatient(patientCode);
          appMessage.success('删除成功');
          fetchPatients();
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleDownloadTemplate = () => {
    // 只包含患者基本信息，标注是否必填
    const headers = ['患者编号*', '姓名*', '性别*', '身份证件类型*', '身份证件号*', '电话', '地址', '销售'];
    const csvContent = `\uFEFF${headers.join(',')}\n`;
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const link = document.createElement('a');
    link.href = URL.createObjectURL(blob);
    link.download = '患者信息导入模板.csv';
    link.click();
  };

  const handleUpload = async (file: any) => {
    const formData = new FormData();
    formData.append('file', file);

    try {
      const response = await fetch('/api/patients/import', {
        method: 'POST',
        headers: {
          'Authorization': `Bearer ${localStorage.getItem('token')}`,
        },
        body: formData,
      });
      
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success(`成功导入 ${result.data.count} 条患者信息`);
        setUploadModalVisible(false);
        fetchPatients();
      } else {
        appMessage.error(result.message || '导入失败');
      }
    } catch (_error) {
      appMessage.error('导入失败');
    }
    
    return false;
  };

  // 恢复患者
  const handleRestore = (patientCode: string) => {
    Modal.confirm({
      title: '确认恢复',
      content: '确定要恢复该患者信息吗？',
      okText: '确定',
      cancelText: '取消',
      onOk: async () => {
        try {
          await fetch(`/api/patients/${patientCode}/restore`, {
            method: 'PUT',
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`,
              'Content-Type': 'application/json',
            },
          });
          appMessage.success('恢复成功');
          fetchRecyclePatients();
          fetchPatients();
        } catch (_error) {
          appMessage.error('恢复失败');
        }
      },
    });
  };

  // 彻底删除患者
  const handleForceDelete = (patientCode: string) => {
    Modal.confirm({
      title: '确认彻底删除',
      content: '确定要彻底删除该患者信息吗？此操作不可恢复！',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          await fetch(`/api/patients/${patientCode}/force-delete`, {
            method: 'DELETE',
            headers: {
              'Authorization': `Bearer ${localStorage.getItem('token')}`,
            },
          });
          appMessage.success('彻底删除成功');
          fetchRecyclePatients();
        } catch (_error) {
          appMessage.error('彻底删除失败');
        }
      },
    });
  };

  // 处理回收站搜索
  const handleRecycleSearch = (e: any) => {
    setRecycleSearch(e.target.value);
  };

  const handleRecycleSearchSubmit = () => {
    fetchRecyclePatients({ page: 1 });
  };

  // 导出患者数据
  const handleExport = async () => {
    try {
      const exportParams = exportForm.getFieldsValue();
      const { startTime, endTime, ...otherParams } = exportParams;
      
      // 构建查询参数
      const params = {
        ...otherParams,
        is_active: 1,
        page: 1,
        pageSize: 1000, // 导出时获取所有数据
      };
      
      // 添加时间范围筛选
      if (startTime) {
        params.startTime = startTime.format('YYYY-MM-DD');
      }
      if (endTime) {
        params.endTime = endTime.format('YYYY-MM-DD');
      }
      
      // 获取患者数据
      const response = await listPatients(params);
      const patients = response.data?.list || [];
      
      // 构建CSV内容
      const fieldMap: { [key: string]: string } = {
        patientCode: '患者编号',
        name: '姓名',
        gender: '性别',
        age: '年龄',
        idDocumentType: '身份证件类型',
        idDocumentNo: '身份证件号',
        idCard: '身份证件号',
        phone: '联系电话',
        address: '地址',
        smokingStatus: '吸烟状态',
        cancerDiameter: '癌直径',
        patientSource: '来源',
        createdAt: '创建时间'
      };
      
      // 构建CSV标题行
      const headers = exportFields.map(field => fieldMap[field]);
      const csvContent = `${headers.join(',')}\n`;
      
      // 构建CSV数据行
      const csvData = patients.map(patient => {
        return exportFields.map(field => {
          let value = patient[field];
          if (field === 'createdAt') {
            value = dayjs(value).format('YYYY-MM-DD');
          } else if (field === 'patientSource') {
            value = patientSourceText[value] || value;
          }
          return `"${value || ''}"`;
        }).join(',');
      }).join('\n');
      
      // 生成并下载CSV文件
      const fullCsvContent = csvContent + csvData;
      const blob = new Blob([fullCsvContent], { type: 'text/csv;charset=utf-8;' });
      const link = document.createElement('a');
      link.href = URL.createObjectURL(blob);
      link.download = `患者信息_${dayjs().format('YYYYMMDDHHmmss')}.csv`;
      link.click();
      
      appMessage.success('导出成功');
      setExportModalVisible(false);
    } catch (_error) {
      appMessage.error('导出失败');
    }
  };

  // 处理导出字段选择
  const handleFieldChange = (checkedValues: string[]) => {
    setExportFields(checkedValues);
  };

  const columns = [
    { title: '患者编号', dataIndex: 'patientCode', key: 'patientCode', width: 150 },
    {
      title: '姓名',
      dataIndex: 'name',
      key: 'name',
      width: 120,
      render: (_: any, record: any) => (
        <a onClick={() => handleViewDetail(record.patientCode)} style={{ cursor: 'pointer', color: '#1890ff' }}>
          {record.name}
        </a>
      )
    },
    { title: '性别', dataIndex: 'gender', key: 'gender', width: 90 },
    { title: '年龄', dataIndex: 'age', key: 'age', width: 90 },
    { title: '身份证件类型', dataIndex: 'idDocumentType', key: 'idDocumentType', width: 210 },
    { title: '身份证件号', dataIndex: 'idDocumentNo', key: 'idDocumentNo', width: 220, render: (_: any, record: any) => record.idDocumentNo || record.idCard || '-' },
    { title: '联系电话', dataIndex: 'phone', key: 'phone', width: 150 },
    {
      title: '信息完善',
      dataIndex: 'completionStatus',
      key: 'completionStatus',
      width: 120,
      render: (status: any, record: any) => {
        // 直接使用数字作为索引，不转换为字符串
        const statusMap: { [key: number]: { text: string; color: string } } = {
          0: { text: '未完善', color: '#faad14' },
          1: { text: '已完善', color: '#52c41a' },
        };
        // 确保status是数字类型
        const statusNum = typeof status === 'number' ? status : parseInt(status, 10) || 0;
        const statusInfo = statusMap[statusNum] || { text: '未知', color: '#d9d9d9' };
        
        // 如果是未完善状态，添加点击事件跳转到完善页面
        if (statusNum === 0) {
          return (
            <a 
              onClick={() => handleComplete(record.patientCode)} 
              style={{ 
                color: statusInfo.color, 
                cursor: 'pointer',
                textDecoration: 'none'
              }}
            >
              {statusInfo.text}
            </a>
          );
        }
        
        return <span style={{ color: statusInfo.color }}>{statusInfo.text}</span>;
      }
    },
    {
      title: '销售',
      dataIndex: 'salesPerson',
      key: 'salesPerson',
      width: 150,
      render: (salesPerson: any) => salesPerson?.name || '-'
    },
    {
      title: '操作',
      key: 'action',
      width: 220,
      fixed: 'right' as const,
      render: (_: any, record: any) => {
        const actions = [];

        // 销售角色只能完善患者信息
        if (currentUser && getRoleName(currentUser) === '销售') {
          actions.push(
            <Button
              type="link"
              size="small"
              icon={<EditTwoTone />}
              onClick={() => handleComplete(record.patientCode)}
              style={{ marginRight: 8 }}
            >
              完善
            </Button>
          );
        } else {
          // 其他角色可以编辑和删除
          actions.push(
            <Button
              type="link"
              size="small"
              icon={<EditOutlined />}
              onClick={() => handleEdit(record.patientCode)}
              style={{ marginRight: 8 }}
            >
              编辑
            </Button>
          );
        }

        actions.push(
          <Button
            type="link"
            danger
            size="small"
            icon={<DeleteOutlined />}
            onClick={() => _handleDelete(record.patientCode)}
          >
            删除
          </Button>
        );

        return <>{actions}</>;
      },
    },
  ];

  // 回收站表格列
  const recycleColumns = [
    { title: '患者编号', dataIndex: 'patientCode', key: 'patientCode' },
    { title: '姓名', dataIndex: 'name', key: 'name' },
    { title: '身份证件类型', dataIndex: 'idDocumentType', key: 'idDocumentType' },
    { title: '身份证件号', dataIndex: 'idDocumentNo', key: 'idDocumentNo', render: (_: any, record: any) => record.idDocumentNo || record.idCard || '-' },
    { 
      title: '创建时间', 
      dataIndex: 'createdAt', 
      key: 'createdAt',
      render: (text: any) => dayjs(text).format('YYYY-MM-DD HH:mm'),
      sorter: (a: any, b: any) => dayjs(a.createdAt).valueOf() - dayjs(b.createdAt).valueOf(),
      sortOrder: 'descend' as const
    },
    { 
      title: '操作', 
      key: 'action', 
      render: (_: any, record: any) => (
        <Space size="middle">
          <Button type="default" size="small" icon={<UndoOutlined />} onClick={() => handleRestore(record.patientCode)}>恢复</Button>
          <Button danger size="small" icon={<DeleteFilled />} onClick={() => handleForceDelete(record.patientCode)}>彻底删除</Button>
        </Space>
      ),
    },
  ];

  return (
    <div>
      <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
        <h2>患者中心</h2>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={handleCreate}>新增患者</Button>
          <Button icon={<InboxOutlined />} onClick={openAssignModal}>分配销售</Button>
          <Button icon={<UploadOutlined />} onClick={() => setUploadModalVisible(true)}>导入患者</Button>
          <Button icon={<FileExcelOutlined />} onClick={() => setExportModalVisible(true)}>导出患者</Button>
          <Button icon={<SearchOutlined />} onClick={() => setRecycleModalVisible(true)}>回收站</Button>
          <Button icon={<ReloadOutlined />} onClick={() => fetchPatients()}>刷新</Button>
        </Space>
      </div>

        <Form form={form} layout="inline" style={{ marginBottom: 16 }}>
          <Form.Item name="name" label="姓名" style={{ marginBottom: 0 }}>
            <Input placeholder="请输入姓名" style={{ width: 150 }} />
          </Form.Item>
          <Form.Item name="idDocumentNo" label="身份证件号" style={{ marginBottom: 0 }}>
            <Input placeholder="请输入身份证件号" style={{ width: 200 }} />
          </Form.Item>
          <Form.Item name="completionStatus" label="信息完善状态" style={{ marginBottom: 0 }}>
            <Select placeholder="请选择状态" style={{ width: 150 }}>
              <Option value={0}>未完善</Option>
              <Option value={1}>已完善</Option>
            </Select>
          </Form.Item>
          <Form.Item style={{ marginBottom: 0 }}>
            <Button type="default" onClick={() => setAdvancedSearchVisible(!advancedSearchVisible)}>
              {advancedSearchVisible ? '收起高级搜索' : '高级搜索'}
            </Button>
          </Form.Item>
        </Form>

        {/* 高级搜索选项 */}
        {advancedSearchVisible && (
          <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16, padding: '16px', backgroundColor: '#f5f5f5', borderRadius: '8px' }}>
            <Form.Item name="salesPerson" label="销售" style={{ marginBottom: 0 }}>
              <Select placeholder="请选择销售" style={{ width: 150 }}>
                {salesUsers.map((user) => (
                  <Option key={getSalesPersonCode(user)} value={getSalesPersonCode(user)}>
                    {user.real_name || user.username} {user.employee_id ? `(${user.employee_id})` : ''}
                  </Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item name="phone" label="联系电话" style={{ marginBottom: 0 }}>
              <Input placeholder="请输入联系电话" style={{ width: 150 }} />
            </Form.Item>
            <Form.Item name="patientCode" label="患者编号" style={{ marginBottom: 0 }}>
              <Input placeholder="请输入患者编号" style={{ width: 150 }} />
            </Form.Item>
            <Form.Item style={{ marginBottom: 0 }}>
              <Space>
                <Button type="primary" htmlType="submit">搜索</Button>
                <Button onClick={handleResetSearch}>重置</Button>
              </Space>
            </Form.Item>
          </Form>
        )}

        {/* 基础搜索按钮 */}
        {!advancedSearchVisible && (
          <Form form={form} layout="inline" onFinish={handleSearch} style={{ marginBottom: 16 }}>
            <Form.Item style={{ marginBottom: 0 }}>
              <Space>
                <Button type="primary" htmlType="submit">搜索</Button>
                <Button onClick={handleResetSearch}>重置</Button>
              </Space>
            </Form.Item>
          </Form>
        )}

        <Table
          columns={columns}
          dataSource={patients}
          loading={loading}
          rowKey="id"
          scroll={{ x: 1500 }}
          pagination={pagination}
          onChange={(tablePagination) => {
            setPagination({
              current: tablePagination.current || 1,
              pageSize: tablePagination.pageSize || 10,
              total: tablePagination.total || 0,
              showSizeChanger: true,
            });
            fetchPatients({ page: tablePagination.current || 1, pageSize: tablePagination.pageSize || 10 });
          }}
        />

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
                <div><strong>来源：</strong>{renderPatientSource(currentPatient.patientSource || currentPatient.patient_source)}</div>
                <div><strong>出生日期：</strong>{currentPatient.birthday ? dayjs(currentPatient.birthday).format('YYYY-MM-DD') : '-'}</div>
                <div><strong>地址：</strong>{currentPatient.address || '-'}</div>
                <div><strong>吸烟状态：</strong>{currentPatient.smokingStatus || '-'}</div>
                <div><strong>销售：</strong>{currentPatient.salesPerson?.name || '-'}</div>
                <div><strong>患者状态：</strong>{currentPatient.patientStatus === 0 ? '健康' : '患病'}</div>
                <div><strong>创建时间：</strong>{dayjs(currentPatient.createdAt).format('YYYY-MM-DD HH:mm:ss')}</div>
                <div style={{ gridColumn: '1 / span 2' }}><strong>备注：</strong>{currentPatient.medicalRecordNo || '-'}</div>
              </div>
            </Card>

            {/* 根据患者状态显示不同内容 */}
            {currentPatient.patientStatus === 0 ? (
              <Card loading={detailLoading} title="健康患者体检" style={{ marginTop: 16 }}>
                <div style={{ padding: '20px', textAlign: 'center' }}>
                  <p style={{ fontSize: '16px', color: '#52c41a' }}>该患者为健康状态，无需病理与预后信息。</p>
                  <p style={{ marginTop: '10px' }}>建议定期进行健康体检，保持良好的生活习惯。</p>
                </div>
              </Card>
            ) : (
              <>
                {/* 病理与预后信息 - 可折叠 */}
                <Card 
                  loading={detailLoading} 
                  title="病理与预后信息" 
                  style={{ marginTop: 16 }}
                  extra={<Button type="link" onClick={() => setPathologyExpanded(!pathologyExpanded)}>{pathologyExpanded ? '收起' : '展开'}</Button>}
                >
                  {pathologyExpanded && (
                    <div style={{ display: 'grid', gridTemplateColumns: 'repeat(1, 1fr)', gap: '16px' }}>
                      <div><strong>癌直径：</strong>{currentPatient.cancerDiameter || '-'}</div>
                      <div><strong>癌症病理信息：</strong>{currentPatient.cancerPathology || '-'}</div>
                      <div><strong>预后信息：</strong>{currentPatient.prognosisInfo || '-'}</div>
                      <div><strong>其他信息：</strong>{currentPatient.otherInfo || '-'}</div>
                      {(currentPatient as any).followUps?.map((item: any) => (
                        <div key={item.id} style={{ paddingTop: 12, borderTop: '1px solid #f0f0f0' }}>
                          <div><strong>检测时间：</strong>{item.detection_time || item.created_at || '-'}</div>
                          <div><strong>检测信息：</strong>{item.diagnosis_info || '-'}</div>
                          <div><strong>结果说明：</strong>{item.report_notes || '-'}</div>
                          <div><strong>报告文件：</strong>{Array.isArray(item.images) && item.images.length ? item.images.join('，') : '-'}</div>
                        </div>
                      ))}
                    </div>
                  )}
                </Card>

                {/* 报告文件 */}
                <Card 
                  loading={detailLoading} 
                  title="报告文件" 
                  style={{ marginTop: 16 }}
                  extra={<Button type="link" onClick={() => setFilesExpanded(!filesExpanded)}>{filesExpanded ? '收起' : '展开'}</Button>}
                >
                  {filesExpanded && (
                    <div>
                      {currentPatient.reportFiles ? (
                        <div>
                          <strong>上传的文件：</strong>
                          <ul>
                            {currentPatient.reportFiles.split(',').map((file: string) => (
                              <li key={file}>
                                <a href={file} target="_blank" rel="noopener noreferrer">
                                  {file.split('/').pop()}
                                </a>
                              </li>
                            ))}
                          </ul>
                        </div>
                      ) : (
                        <div>暂无上传的文件</div>
                      )}
                    </div>
                  )}
                </Card>
              </>
            )}

            {/* 检测样本列表 */}
            <Card 
              title="检测样本" 
              loading={samplesLoading} 
              style={{ marginTop: 16 }} 
              extra={<Button type="link" onClick={() => fetchPatientSamples(currentPatient.patientCode)}>刷新</Button>}
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
                      dataIndex: 'sample_type_name',
                      key: 'sample_type_name',
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
                        // 样本状态映射
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
          </div>
        )}
      </Modal>

      <Modal
        title="分配销售"
        open={assignVisible}
        onCancel={() => setAssignVisible(false)}
        onOk={handleAssignSales}
        okText="确认分配"
        cancelText="取消"
        width={900}
      >
        <Space style={{ marginBottom: 16 }}>
          <Input
            placeholder="搜索姓名、电话、证件号、患者编号"
            value={assignKeyword}
            onChange={(event) => setAssignKeyword(event.target.value)}
            style={{ width: 280 }}
          />
          <Button type="primary" onClick={() => fetchAssignPatients({ page: 1 })}>搜索</Button>
          <Button onClick={() => { setAssignKeyword(''); fetchAssignPatients({ page: 1, keyword: '' }); }}>重置</Button>
        </Space>
        <Form form={assignForm} layout="vertical">
          <Form.Item name="sales_person" label="销售" rules={[{ required: true, message: '请选择销售' }]}>
            <Select placeholder="请选择销售">
              {salesUsers.map((user) => (
                <Option key={getSalesPersonCode(user)} value={getSalesPersonCode(user)}>
                  {user.real_name || user.username} {user.employee_id ? `(${user.employee_id})` : ''}
                </Option>
              ))}
            </Select>
          </Form.Item>
        </Form>
        <Table
          rowKey="id"
          size="small"
          loading={assignLoading}
          dataSource={assignPatients}
          rowSelection={{
            selectedRowKeys: assignSelectedRowKeys,
            onChange: setAssignSelectedRowKeys,
          }}
          columns={[
            { title: '患者编号', dataIndex: 'patientCode', key: 'patientCode' },
            { title: '姓名', dataIndex: 'name', key: 'name' },
            { title: '性别', dataIndex: 'gender', key: 'gender', width: 80 },
            { title: '身份证件号', dataIndex: 'idCard', key: 'idCard', render: (text: any) => text || '-' },
            { title: '联系电话', dataIndex: 'phone', key: 'phone', render: (text: any) => text || '-' },
            { title: '来源', dataIndex: 'patientSource', key: 'patientSource', render: renderPatientSource },
            { title: '创建时间', dataIndex: 'createdAt', key: 'createdAt', render: (text: any) => text ? dayjs(text).format('YYYY-MM-DD HH:mm') : '-' },
          ]}
          pagination={false}
        />
      </Modal>

      {/* 导入模态框 */}
      <Modal
        title="导入患者"
        open={uploadModalVisible}
        onCancel={() => setUploadModalVisible(false)}
        footer={null}
        width={800}
      >
        <div style={{ marginBottom: 24 }}>
          <Typography.Title level={5}>导入说明</Typography.Title>
          <Typography.Paragraph>
            1. 点击下方"下载模板"按钮，下载患者导入模板
          </Typography.Paragraph>
          <Typography.Paragraph>
            2. 在模板中填写患者信息
          </Typography.Paragraph>
          <Typography.Paragraph>
            3. 点击"上传文件"按钮，上传填写完成的Excel文件
          </Typography.Paragraph>
          <Typography.Paragraph>
            4. 系统将自动导入患者信息
          </Typography.Paragraph>
        </div>
        
        <div style={{ marginBottom: 24 }}>
          <Typography.Title level={5}>模板下载</Typography.Title>
          <Button 
            icon={<DownloadOutlined />} 
            onClick={handleDownloadTemplate}
          >
            下载模板
          </Button>
        </div>
        
        <div>
          <Typography.Title level={5}>文件上传</Typography.Title>
          <Upload
            name="file"
            beforeUpload={handleUpload}
            showUploadList={false}
            accept=".xlsx,.xls,.csv"
          >
            <Button icon={<InboxOutlined />}>
              上传文件
            </Button>
          </Upload>
        </div>
      </Modal>

      {/* 导出模态框 */}
      <Modal
        title="导出患者"
        open={exportModalVisible}
        onCancel={() => setExportModalVisible(false)}
        footer={[
          <Button key="close" onClick={() => setExportModalVisible(false)}>取消</Button>, 
          <Button key="export" type="primary" onClick={handleExport}>导出</Button>
        ]}
        width={800}
      >
        <Form form={exportForm} layout="vertical">
          <Form.Item name="name" label="姓名">
            <Input placeholder="请输入姓名" />
          </Form.Item>
          <Form.Item name="patientCode" label="患者编号">
            <Input placeholder="请输入患者编号" />
          </Form.Item>
          <Form.Item name="idDocumentNo" label="身份证件号">
            <Input placeholder="请输入身份证件号" />
          </Form.Item>
          <Form.Item name="phone" label="联系电话">
            <Input placeholder="请输入联系电话" />
          </Form.Item>
          <Form.Item label="新增时间范围" required={false}>
            <Space>
              <Form.Item name="startTime" noStyle>
                <DatePicker placeholder="开始日期" style={{ width: 200 }} />
              </Form.Item>
              <span>至</span>
              <Form.Item name="endTime" noStyle>
                <DatePicker placeholder="结束日期" style={{ width: 200 }} />
              </Form.Item>
            </Space>
          </Form.Item>
          <Form.Item label="选择导出字段" required={false}>
            <Checkbox.Group 
              value={exportFields} 
              onChange={handleFieldChange}
              style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: '10px' }}
            >
              <Checkbox value="patientCode">患者编号</Checkbox>
              <Checkbox value="name">姓名</Checkbox>
              <Checkbox value="gender">性别</Checkbox>
              <Checkbox value="age">年龄</Checkbox>
              <Checkbox value="idDocumentType">身份证件类型</Checkbox>
              <Checkbox value="idDocumentNo">身份证件号</Checkbox>
              <Checkbox value="phone">联系电话</Checkbox>
              <Checkbox value="patientSource">来源</Checkbox>
              <Checkbox value="address">地址</Checkbox>
              <Checkbox value="treatmentStage">治疗阶段</Checkbox>
              <Checkbox value="cancerTypeName">癌型</Checkbox>
              <Checkbox value="smokingStatus">吸烟状态</Checkbox>
              <Checkbox value="cancerDiameter">癌直径</Checkbox>
              <Checkbox value="createdAt">创建时间</Checkbox>
            </Checkbox.Group>
          </Form.Item>
        </Form>
      </Modal>

      {/* 回收站模态框 */}
      <Modal
        title="患者回收站"
        open={recycleModalVisible}
        onCancel={() => setRecycleModalVisible(false)}
        footer={null}
        width={1000}
      >
        <div style={{ marginBottom: 16, display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
          <div>
            <Input
              placeholder="请输入患者编号、姓名或身份证件号"
              value={recycleSearch}
              onChange={handleRecycleSearch}
              onPressEnter={handleRecycleSearchSubmit}
              style={{ width: 300, marginRight: 8 }}
            />
            <Button type="primary" onClick={handleRecycleSearchSubmit} icon={<SearchOutlined />}>搜索</Button>
          </div>
        </div>
        <Table
          columns={recycleColumns}
          dataSource={recyclePatients}
          loading={recycleLoading}
          rowKey="id"
          pagination={{
            ...recyclePagination,
            onChange: (current, pageSize) => {
              setRecyclePagination({
                current: current || 1,
                pageSize: pageSize || 10,
                total: recyclePagination.total,
                showSizeChanger: true,
              });
              fetchRecyclePatients({ page: current || 1, pageSize: pageSize || 10 });
            },
          }}
        />
      </Modal>
    </div>
  );
};

export default List;
