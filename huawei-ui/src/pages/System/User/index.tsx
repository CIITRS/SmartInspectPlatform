import React, { useState, useEffect } from 'react';
import { Table, Button, Input, Form, Row, Col, Modal, Select, Switch, App, Tabs, Card, Cascader, Tree, Descriptions, Tag } from 'antd';
import type { CascaderProps } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined, KeyOutlined, SafetyCertificateOutlined } from '@ant-design/icons';
import routesConfig from '@/../config/routes';

const { Option } = Select;
const { TextArea } = Input;

const formatDateTime = (dateString: string | undefined | null): string => {
  if (!dateString) return '-';
  
  // 如果已经是中文格式，直接返回
  if (dateString.includes('年') && dateString.includes('月')) {
    return dateString;
  }
  
  try {
    const date = new Date(dateString);
    if (isNaN(date.getTime())) {
      return dateString;
    }
    const year = date.getFullYear();
    const month = String(date.getMonth() + 1).padStart(2, '0');
    const day = String(date.getDate()).padStart(2, '0');
    const hours = String(date.getHours()).padStart(2, '0');
    const minutes = String(date.getMinutes()).padStart(2, '0');
    const seconds = String(date.getSeconds()).padStart(2, '0');
    return `${year}年${month}月${day}日 ${hours}:${minutes}:${seconds}`;
  } catch {
    return dateString || '-';
  }
};

interface CascaderOption {
  value: string | number;
  label: string;
  children?: CascaderOption[];
  disableCheckbox?: boolean;
}

// 从路由配置生成权限选项
const generateCascaderOptionsFromRoutes = (routes: any[]): CascaderOption[] => {
  const options: CascaderOption[] = [];
  
  // 递归处理路由配置
  const processRoute = (route: any, parentPath: string = '') => {
    // 跳过隐藏在菜单中的路由
    if (route.hideInMenu) return;
    
    // 跳过没有name的路由
    if (!route.name) return;
    
    // 跳过layout为false的路由（如登录页）
    if (route.layout === false) return;
    
    // 生成权限值
    const routePath = route.path === '/' ? 'dashboard' : route.path.replace(/^\//, '');
    const permissionValue = routePath.replace(/\//g, '_');
    
    const option: CascaderOption = {
      value: permissionValue,
      label: route.name,
    };
    
    // 处理子路由
    if (route.routes && route.routes.length > 0) {
      const childrenOptions: CascaderOption[] = [];
      
      route.routes.forEach((childRoute: any) => {
        // 跳过隐藏在菜单中的子路由
        if (childRoute.hideInMenu) return;
        // 跳过没有name的子路由
        if (!childRoute.name) return;
        
        const childPath = childRoute.path;
        const childPermissionValue = childPath.replace(/^\//, '').replace(/\//g, '_');
        
        childrenOptions.push({
          value: childPermissionValue,
          label: childRoute.name,
        });
      });
      
      if (childrenOptions.length > 0) {
        option.children = childrenOptions;
      }
    }
    
    options.push(option);
  };
  
  // 处理所有路由
  routes.forEach(route => {
    processRoute(route);
  });
  
  return options;
};

const User: React.FC = () => {
  // 用户管理状态
  const [userForm] = Form.useForm();
  const [users, setUsers] = useState<any[]>([]);
  const [userLoading, setUserLoading] = useState(true);
  const [userPagination, setUserPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [_userSearchParams, setUserSearchParams] = useState({});
  const [userModalVisible, setUserModalVisible] = useState(false);
  const [userModalForm] = Form.useForm();
  const [editingUserId, setEditingUserId] = useState<number | null>(null);
  const [userDetailVisible, setUserDetailVisible] = useState(false);
  const [selectedUser, setSelectedUser] = useState<any>(null);
  const [permissionModalVisible, setPermissionModalVisible] = useState(false);
  const [permissionUser, setPermissionUser] = useState<any>(null);
  const [permissionSource, setPermissionSource] = useState<'role' | 'user'>('role');

  // 部门管理状态
  const [departmentForm] = Form.useForm();
  const [departments, setDepartments] = useState<any[]>([]);
  const [departmentTree, setDepartmentTree] = useState<any[]>([]);
  const [departmentLoading, setDepartmentLoading] = useState(true);
  const [departmentPagination, setDepartmentPagination] = useState({ current: 1, pageSize: 10, total: 0 });
  const [_departmentSearchParams, setDepartmentSearchParams] = useState({});
  const [departmentModalVisible, setDepartmentModalVisible] = useState(false);
  const [departmentModalForm] = Form.useForm();
  const [editingDepartmentId, setEditingDepartmentId] = useState<string | null>(null);

  // 角色管理状态
	const [roles, setRoles] = useState<any[]>([]);
	const [rolesLoading, setRolesLoading] = useState(true);
	const [roleModalVisible, setRoleModalVisible] = useState(false);
	const [roleModalForm] = Form.useForm();
	const [editingRoleId, setEditingRoleId] = useState<number | null>(null);
	
	// 从路由配置生成页面权限级联选择器选项
	const [cascaderOptions, setCascaderOptions] = useState<CascaderOption[]>([]);
	
	// 初始化时从路由配置生成权限选项
	useEffect(() => {
		const options = generateCascaderOptionsFromRoutes(routesConfig);
		setCascaderOptions(options);
	}, []);
	
	// 选中的权限值
	const [selectedPermissions, setSelectedPermissions] = useState<string[][]>([]);

  const { message: appMessage } = App.useApp();

  // 用户管理方法
  const fetchUsers = async (params: any = {}) => {
    setUserLoading(true);
    try {
      const response = await fetch('/api/system/users', {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        // 后端返回的是包含list和total字段的对象
        const userList = result.data.list || [];
        setUsers(userList);
        setUserPagination({
          ...userPagination,
          total: result.data.total || 0,
          current: params.page || 1,
        });
      } else {
        appMessage.error('获取用户列表失败');
      }
    } catch (_error) {
      appMessage.error('获取用户列表失败');
    } finally {
      setUserLoading(false);
    }
  };

  const handleUserSearch = (values: any) => {
    setUserSearchParams(values);
    fetchUsers({ page: 1 });
  };

  const handleUserCreate = () => {
    setEditingUserId(null);
    userModalForm.resetFields();
    userModalForm.setFieldsValue({ status: 1 });
    setUserModalVisible(true);
  };

  const handleUserEdit = (record: any) => {
    const roleIds = Array.isArray(record.role_ids) && record.role_ids.length > 0
      ? record.role_ids.map((id: any) => Number(id)).filter(Boolean)
      : record.role_id ? [Number(record.role_id)] : [];
    setEditingUserId(record.id);
    userModalForm.setFieldsValue({
      ...record,
      role_ids: roleIds,
      role_id: roleIds[0],
      department_id: record.department_id ? Number(record.department_id) : undefined,
      status: Number(record.status ?? 1),
    });
    setUserModalVisible(true);
  };

  const handleUserDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该用户吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/system/users/${id}`, {
            method: 'DELETE',
          });
          const result = await response.json();
          if (result.code === 200) {
            appMessage.success('删除成功');
            fetchUsers();
          } else {
            appMessage.error('删除失败');
          }
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleUserModalSubmit = async (values: any) => {
    try {
      const url = editingUserId ? `/api/system/users/${editingUserId}` : '/api/system/users';
      const method = editingUserId ? 'PUT' : 'POST';
      
      // 直接发送明文密码到后端，后端会使用项目现有规则加密
      const roleIds = Array.isArray(values.role_ids)
        ? values.role_ids.map((id: any) => Number(id)).filter(Boolean)
        : [];
      const userData = { ...values, role_ids: roleIds, role_id: roleIds[0] };
      
      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(userData),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success(editingUserId ? '用户更新成功' : '用户创建成功');
        setUserModalVisible(false);
        fetchUsers();
      } else {
        appMessage.error(result.message || (editingUserId ? '用户更新失败' : '用户创建失败'));
      }
    } catch (_error) {
      appMessage.error(editingUserId ? '用户更新失败' : '用户创建失败');
    }
  };

  const handleUserStatusChange = async (id: string, status: number) => {
    try {
      const response = await fetch(`/api/system/users/${id}/status`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ status }),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('状态更新成功');
        fetchUsers();
      } else {
        appMessage.error('状态更新失败');
      }
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  const handleUserAIAccessChange = async (id: number, allowed: boolean) => {
    try {
      const response = await fetch(`/api/system/users/${id}/ai-allowed`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ allowed }),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('AI访问权限更新成功');
        fetchUsers();
      } else {
        appMessage.error(result.message || 'AI访问权限更新失败');
      }
    } catch (_error) {
      appMessage.error('AI访问权限更新失败');
    }
  };

  const handleResetUserPassword = (record: any) => {
    Modal.confirm({
      title: '确认重置密码',
      content: `确定要将 ${record.real_name || record.username} 的密码重置为 Hw123456 吗？`,
      okText: '重置',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/system/users/${record.id}/reset-password`, {
            method: 'POST',
          });
          const result = await response.json();
          if (result.code === 200) {
            appMessage.success('密码已重置为 Hw123456');
          } else {
            appMessage.error(result.message || '密码重置失败');
          }
        } catch (_error) {
          appMessage.error('密码重置失败');
        }
      },
    });
  };

  const permissionsToCascaderValue = (tree: any[]): string[][] => {
    const values: string[][] = [];
    const walk = (node: any, parents: string[]) => {
      const id = String(node.id || node.key || '');
      if (!id) return;
      const path = [...parents, id];
      if (node.checked) values.push(path);
      (node.children || []).forEach((child: any) => walk(child, path));
    };
    (tree || []).forEach((node: any) => walk(node, []));
    return values;
  };

  const openUserPermissions = async (record: any) => {
    setPermissionUser(record);
    setPermissionModalVisible(true);
    setSelectedPermissions([]);
    try {
      const response = await fetch(`/api/system/users/${record.id}/permissions`);
      const result = await response.json();
      if (result.code === 200) {
        const data = result.data || {};
        setPermissionSource(data.source || 'role');
        setSelectedPermissions(permissionsToCascaderValue(data.permissions || []));
      } else {
        appMessage.error(result.message || '获取用户权限失败');
      }
    } catch (_error) {
      appMessage.error('获取用户权限失败');
    }
  };

  const saveUserPermissions = async () => {
    if (!permissionUser) return;
    const buildPermissionTree = (options: CascaderOption[], selected: string[][]): any[] => (
      options.map((option) => {
        const value = String(option.value);
        const node: any = {
          id: value,
          key: value,
          title: option.label,
          checked: selected.some(path => path[0] === value),
        };
        if (option.children) {
          node.children = buildPermissionTree(option.children, selected.filter(path => path[0] === value).map(path => path.slice(1)));
        }
        return node;
      })
    );
    try {
      const response = await fetch(`/api/system/users/${permissionUser.id}/permissions`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ permissions: buildPermissionTree(cascaderOptions, selectedPermissions) }),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('用户权限已保存');
        setPermissionSource('user');
        setPermissionModalVisible(false);
      } else {
        appMessage.error(result.message || '保存用户权限失败');
      }
    } catch (_error) {
      appMessage.error('保存用户权限失败');
    }
  };

  const clearUserPermissions = async () => {
    if (!permissionUser) return;
    try {
      const response = await fetch(`/api/system/users/${permissionUser.id}/permissions`, { method: 'DELETE' });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('已恢复角色默认权限');
        setPermissionModalVisible(false);
      } else {
        appMessage.error(result.message || '恢复默认权限失败');
      }
    } catch (_error) {
      appMessage.error('恢复默认权限失败');
    }
  };

  // 部门管理方法
  const fetchDepartments = async (params: any = {}) => {
    setDepartmentLoading(true);
    try {
      const response = await fetch('/api/system/departments', {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        // 处理部门数据，确保status字段正确
        const processedDepartments = (result.data.list || []).map((dept: any) => {
          // 确保status字段存在且为数字类型
          if (dept.status === undefined && dept.isActive !== undefined) {
            dept.status = dept.isActive;
          }
          return dept;
        });
        setDepartments(processedDepartments);
        setDepartmentPagination({
          ...departmentPagination,
          total: result.data.total || 0,
          current: params.page || 1,
        });
      } else {
        appMessage.error('获取部门列表失败');
      }
    } catch (_error) {
      appMessage.error('获取部门列表失败');
    } finally {
      setDepartmentLoading(false);
    }
  };

  const handleDepartmentSearch = (values: any) => {
    setDepartmentSearchParams(values);
    fetchDepartments({ page: 1 });
  };

  const handleDepartmentCreate = () => {
    setEditingDepartmentId(null);
    departmentModalForm.resetFields();
    // 新建部门时默认启用，设置默认值为1
    departmentModalForm.setFieldsValue({ status: 1 });
    setDepartmentModalVisible(true);
  };

  const handleDepartmentEdit = (record: any) => {
    setEditingDepartmentId(record.id);
    departmentModalForm.setFieldsValue(record);
    setDepartmentModalVisible(true);
  };

  const handleDepartmentDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该部门吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/system/departments/${id}`, {
            method: 'DELETE',
          });
          const result = await response.json();
          if (result.code === 200) {
            appMessage.success('删除成功');
            fetchDepartments();
          } else {
            appMessage.error('删除失败');
          }
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleDepartmentStatusChange = async (id: string, status: number) => {
    try {
      const response = await fetch(`/api/system/departments/${id}/status`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ status }),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('状态更新成功');
        fetchDepartments();
      } else {
        appMessage.error('状态更新失败');
      }
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  const handleDepartmentModalSubmit = async (values: any) => {
    try {
      const url = editingDepartmentId ? `/api/system/departments/${editingDepartmentId}` : '/api/system/departments';
      const method = editingDepartmentId ? 'PUT' : 'POST';
      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(values),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success(editingDepartmentId ? '部门更新成功' : '部门创建成功');
        setDepartmentModalVisible(false);
        fetchDepartments();
        fetchDepartmentTree();
      } else {
        appMessage.error(editingDepartmentId ? '部门更新失败' : '部门创建失败');
      }
    } catch (_error) {
      appMessage.error(editingDepartmentId ? '部门更新失败' : '部门创建失败');
    }
  };

  // 打开用户详情
  const handleViewUserDetail = (user: any) => {
    setSelectedUser(user);
    setUserDetailVisible(true);
  };

  // 用户管理列定义
  const userColumns = [
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { 
      title: '真实姓名', 
      dataIndex: 'real_name', 
      key: 'real_name',
      render: (name: string, record: any) => (
        <a 
          onClick={() => handleViewUserDetail(record)} 
          style={{ color: '#1890ff', cursor: 'pointer' }}
        >
          {name}
        </a>
      )
    },
    { title: '工号', dataIndex: 'employee_id', key: 'employee_id', render: (employeeId: string) => {
      // 确保工号信息正确显示，处理空值情况
      if (employeeId === null || employeeId === undefined || employeeId === '') {
        return '-';
      }
      return employeeId;
    } },
    {
      title: '角色',
      dataIndex: 'role_name',
      key: 'role_name',
      render: (roleName: string, record: any) => {
        const roleNames = Array.isArray(record.role_names) && record.role_names.length > 0
          ? record.role_names
          : roleName ? String(roleName).split('、') : [];
        return roleNames.length > 0 ? (
          <>
            {roleNames.map((name: string) => (
              <Tag color="blue" key={name}>{name}</Tag>
            ))}
          </>
        ) : (
          <Tag color="default">-</Tag>
        );
      },
    },
    { 
      title: '状态', 
      dataIndex: 'status', 
      key: 'status',
      render: (status: number, _record: any) => (
        <Switch
          checked={status === 1}
          onChange={(checked) => handleUserStatusChange(_record.id, checked ? 1 : 0)}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    { 
      title: 'AI 权限', 
      dataIndex: 'ai_allowed', 
      key: 'ai_allowed',
      render: (aiAllowed: number, _record: any) => (
        <Switch
          checked={aiAllowed === 1}
          onChange={(checked) => handleUserAIAccessChange(_record.id, checked)}
          checkedChildren="开启"
          unCheckedChildren="关闭"
        />
      ),
    },
    { title: '最后登录时间', dataIndex: 'last_login_time', key: 'last_login_time', render: (time: string) => {
      // 后端已经格式化为"2026年06月01日 22:22:22"格式,这里做容错处理
      if (!time || time === '') {
        return '从未登录';
      }
      return time;
    }},
    { 
      title: '操作', 
      key: 'action', 
      render: (_text: any, record: any) => (
        <>
          <Button 
            type="link" 
            icon={<EditOutlined />} 
            onClick={() => handleUserEdit(record)} 
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button
            type="link"
            icon={<KeyOutlined />}
            onClick={() => handleResetUserPassword(record)}
            style={{ marginRight: 8 }}
          >
            重置密码
          </Button>
          <Button
            type="link"
            icon={<SafetyCertificateOutlined />}
            onClick={() => openUserPermissions(record)}
            style={{ marginRight: 8 }}
          >
            权限
          </Button>
          <Button 
            type="link" 
            danger 
            icon={<DeleteOutlined />} 
            onClick={() => handleUserDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 部门管理列定义
  const departmentColumns = [
    { title: '部门名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: number, record: any) => (
        <Switch
          checked={status === 1}
          onChange={(checked) => handleDepartmentStatusChange(record.id, checked ? 1 : 0)}
          checkedChildren="启用"
          unCheckedChildren="禁用"
        />
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_text: any, record: any) => (
        <>
          <Button
            type="link"
            icon={<EditOutlined />}
            onClick={() => handleDepartmentEdit(record)}
            style={{ marginRight: 8 }}
          >
            编辑
          </Button>
          <Button
            type="link"
            danger
            icon={<DeleteOutlined />}
            onClick={() => handleDepartmentDelete(record.id)}
          >
            删除
          </Button>
        </>
      ),
    },
  ];

  // 获取部门树形结构
  const fetchDepartmentTree = async () => {
    try {
      const response = await fetch('/api/system/departments/tree', {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        // 处理部门树形数据，确保status字段正确
        const processedTree = (result.data || []).map((dept: any) => {
          // 确保status字段存在且为数字类型
          if (dept.status === undefined && dept.isActive !== undefined) {
            dept.status = dept.isActive;
          }
          // 递归处理子部门
          if (dept.children && dept.children.length > 0) {
            dept.children = dept.children.map((child: any) => {
              if (child.status === undefined && child.isActive !== undefined) {
                child.status = child.isActive;
              }
              return child;
            });
          }
          return dept;
        });
        setDepartmentTree(processedTree);
      } else {
        appMessage.error('获取部门树形结构失败');
      }
    } catch (_error) {
      appMessage.error('获取部门树形结构失败');
    }
  };

  // 初始化数据
  useEffect(() => {
    fetchUsers();
    fetchDepartments();
    fetchDepartmentTree();
    fetchRoles();
  }, []);

  // 获取角色列表
  const fetchRoles = async () => {
    setRolesLoading(true);
    try {
      const response = await fetch('/api/system/roles', {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        // 处理角色数据，确保status字段正确
        const processedRoles = (result.data.list || []).map((role: any) => {
          // 确保status字段存在且为数字类型
          if (role.status === undefined && role.isActive !== undefined) {
            role.status = role.isActive;
          }
          return role;
        });
        setRoles(processedRoles);
      } else {
        appMessage.error('获取角色列表失败');
      }
    } catch (_error) {
      appMessage.error('获取角色列表失败');
    } finally {
      setRolesLoading(false);
    }
  };

  // 获取角色权限
  const fetchRolePermissions = async (roleId: number) => {
    try {
      const response = await fetch(`/api/system/roles/${roleId}/permissions`, {
        method: 'GET',
      });
      const result = await response.json();
      if (result.code === 200) {
        // 处理后端返回的权限数据，转换为级联选择器需要的格式
        const processPermissions = (tree: any[]): string[][] => {
          const permissions: string[][] = [];
          
          const traverse = (node: any, path: string[]) => {
            if (node.checked) {
              permissions.push([...path, node.id]);
            }
            if (node.children) {
              node.children.forEach((child: any) => {
                traverse(child, [...path, node.id]);
              });
            }
          };
          
          tree.forEach(node => {
            traverse(node, []);
          });
          
          return permissions;
        };
        
        setSelectedPermissions(processPermissions(result.data || []));
      } else {
        appMessage.error('获取角色权限失败');
      }
    } catch (_error) {
      appMessage.error('获取角色权限失败');
    }
  };

  // 角色管理方法
  const handleRoleCreate = () => {
    setEditingRoleId(null);
    roleModalForm.resetFields();
    // 新建角色时状态默认值设置为空
    roleModalForm.setFieldsValue({ status: null });
    // 重置权限选择为默认状态（默认全选）
    const generateDefaultPermissions = (options: CascaderOption[]): string[][] => {
      const permissions: string[][] = [];
      
      const processOption = (option: CascaderOption, parentPath: string[] = []) => {
        // 添加当前选项
        permissions.push([...parentPath, String(option.value)]);
        
        // 处理子选项
        if (option.children && option.children.length > 0) {
          option.children.forEach(child => {
            processOption(child, [...parentPath, String(option.value)]);
          });
        }
      };
      
      options.forEach(option => {
        processOption(option);
      });
      
      return permissions;
    };
    
    setSelectedPermissions(generateDefaultPermissions(cascaderOptions));
    setRoleModalVisible(true);
  };

  const handleRoleEdit = (record: any) => {
    setEditingRoleId(record.id);
    roleModalForm.setFieldsValue(record);
    // 获取角色权限
    fetchRolePermissions(record.id);
    setRoleModalVisible(true);
  };

  const handleRoleDelete = (id: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '确定要删除该角色吗？',
      okText: '确定',
      okType: 'danger',
      cancelText: '取消',
      onOk: async () => {
        try {
          const response = await fetch(`/api/system/roles/${id}`, {
            method: 'DELETE',
          });
          const result = await response.json();
          if (result.code === 200) {
            appMessage.success('删除成功');
            fetchRoles();
          } else {
            appMessage.error('删除失败');
          }
        } catch (_error) {
          appMessage.error('删除失败');
        }
      },
    });
  };

  const handleRoleStatusChange = async (id: string, status: number) => {
    try {
      const response = await fetch(`/api/system/roles/${id}/status`, {
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify({ status }),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success('状态更新成功');
        fetchRoles();
      } else {
        appMessage.error('状态更新失败');
      }
    } catch (_error) {
      appMessage.error('状态更新失败');
    }
  };

  // 级联选择器 onChange 处理函数
  const handlePermissionChange: CascaderProps<CascaderOption, 'value', true>['onChange'] = (value: (string | number)[][]) => {
    setSelectedPermissions((value || []).map(item => item.map(String)));
  };

  const handleRoleModalSubmit = async (values: any) => {
    try {
      // 处理页面权限数据，将级联选择器的值转换为后端需要的树形结构
      const buildPermissionTree = (options: CascaderOption[], selected: string[][]): any[] => {
        return options.map((option) => {
          const node = {
            id: option.value,
            key: option.value,
            checked: selected.some(path => path[0] === option.value),
          } as any;
          
          if (option.children) {
            node.children = buildPermissionTree(option.children, selected.filter(path => path[0] === option.value));
          }
          
          return node;
        });
      };
      
      // 构建包含权限的请求数据
      const roleData = {
        ...values,
        permissions: buildPermissionTree(cascaderOptions, selectedPermissions),
      };
      
      const url = editingRoleId ? `/api/system/roles/${editingRoleId}` : '/api/system/roles';
      const method = editingRoleId ? 'PUT' : 'POST';
      const response = await fetch(url, {
        method,
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(roleData),
      });
      const result = await response.json();
      if (result.code === 200) {
        appMessage.success(editingRoleId ? '角色更新成功' : '角色创建成功');
        setRoleModalVisible(false);
        fetchRoles();
      } else {
        appMessage.error(editingRoleId ? '角色更新失败' : '角色创建失败');
      }
    } catch (_error) {
      appMessage.error(editingRoleId ? '角色更新失败' : '角色创建失败');
    }
  };

  // 定义tabs的items
  const tabsItems = [
    {
      key: 'user',
      label: '用户管理',
      children: (
        <>
          <Form form={userForm} layout="inline" onFinish={handleUserSearch} style={{ marginBottom: 16 }}>
            <Row gutter={16}>
              <Col span={8}>
                <Form.Item name="username">
                  <Input placeholder="用户名" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item name="real_name">
                  <Input placeholder="真实姓名" />
                </Form.Item>
              </Col>
              <Col span={8}>
                <Form.Item>
                  <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                    查询
                  </Button>
                  <Button type="default" onClick={() => userForm.resetFields()}>重置</Button>
                </Form.Item>
              </Col>
            </Row>
          </Form>

          <div style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleUserCreate}>
              新增用户
            </Button>
          </div>

          <Table
            columns={userColumns}
            dataSource={users}
            rowKey="id"
            loading={userLoading}
            pagination={userPagination}
            onChange={(page) => fetchUsers({ page: page.current, pageSize: page.pageSize })}
          />

          <Modal
            title={editingUserId ? '编辑用户' : '新增用户'}
            open={userModalVisible}
            onCancel={() => setUserModalVisible(false)}
            footer={null}
          >
            <Form
              form={userModalForm}
              layout="vertical"
              onFinish={handleUserModalSubmit}
            >
              <Form.Item
                name="username"
                label="用户名"
                rules={[{ required: true, message: '请输入用户名' }]}
              >
                <Input placeholder="请输入用户名" disabled={!!editingUserId} />
              </Form.Item>

              {!editingUserId && (
                <Form.Item
                  name="password"
                  label="密码"
                  extra="留空时自动生成：Hw123456 + 姓名拼音，例如 Hw123456zhaohui"
                >
                  <Input.Password placeholder="留空自动生成默认密码" />
                </Form.Item>
              )}

              <Form.Item
                name="real_name"
                label="真实姓名"
                rules={[{ required: true, message: '请输入真实姓名' }]}
              >
                <Input placeholder="请输入真实姓名" />
              </Form.Item>

              <Form.Item
                name="employee_id"
                label="工号"
                rules={[{ required: true, message: '请输入工号' }]}
              >
                <Input placeholder="请输入工号" />
              </Form.Item>

              <Form.Item
                name="phone"
                label="联系电话"
                rules={[{ required: true, message: '请输入联系电话' }]}
              >
                <Input placeholder="请输入联系电话" />
              </Form.Item>

              <Form.Item
                name="role_ids"
                label="角色"
                rules={[{ required: true, message: '请选择角色' }]}
              >
                <Select mode="multiple" placeholder="请选择角色" loading={rolesLoading} maxTagCount="responsive">
                  {roles.map(role => (
                    <Option key={role.id} value={role.id}>{role.name}</Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="department_id"
                label="部门"
                rules={[{ required: true, message: '请选择部门' }]}
              >
                <Select placeholder="请选择部门">
                  {departments.map(dept => (
                    <Option key={dept.id} value={dept.id}>{dept.name}</Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="status"
                label="状态"
                rules={[{ required: true, message: '请选择状态' }]}
              >
                <Select placeholder="请选择状态">
                  <Option value={1}>启用</Option>
                  <Option value={0}>禁用</Option>
                </Select>
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setUserModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
    {
      key: 'department',
      label: '部门管理',
      children: (
        <>
          <div style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleDepartmentCreate}>
              新增部门
            </Button>
          </div>

          <Row gutter={16}>
            <Col span={8}>
              <Card title="部门树形结构" style={{ height: 'calc(100vh - 300px)', overflow: 'auto' }}>
              <style>{`
                .department-tree {
                  height: 100%;
                }
                .department-tree .ant-tree-treenode {
                  margin: 2px 0;
                }
                .department-tree .ant-tree-indent-unit {
                  width: 20px;
                }
                .department-tree .ant-tree-node-content-wrapper {
                  padding: 4px 8px;
                  border-radius: 4px;
                  transition: all 0.3s;
                }
                .department-tree .ant-tree-node-content-wrapper:hover {
                  background-color: #f5f5f5;
                }
              `}</style>
              <Tree
                className="department-tree"
                treeData={departmentTree.map(item => {
                  // 递归处理所有子部门，确保每个节点都有title和key
                  const processNode = (node: any) => ({
                    ...node,
                    title: node.name,
                    key: `dept_${node.id}`, // 添加前缀确保key唯一
                    children: (node.children || []).map((child: any) => processNode(child))
                  });
                  return processNode(item);
                })}
                showLine
                defaultExpandAll
                onExpand={(expandedKeys: React.Key[], info: any) => {
                  console.log('Expanded keys:', expandedKeys);
                  console.log('Expanded node:', info.node);
                }}
                onSelect={(_selectedKeys: React.Key[], info: any) => {
                  console.log('Selected node:', info.node);
                }}
              />
            </Card>
            </Col>
            <Col span={16}>
              <Form form={departmentForm} layout="inline" onFinish={handleDepartmentSearch} style={{ marginBottom: 16 }}>
                <Row gutter={16}>
                  <Col span={12}>
                    <Form.Item name="name">
                      <Input placeholder="部门名称" />
                    </Form.Item>
                  </Col>
                  <Col span={12}>
                    <Form.Item>
                      <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                        查询
                      </Button>
                      <Button type="default" onClick={() => departmentForm.resetFields()}>重置</Button>
                    </Form.Item>
                  </Col>
                </Row>
              </Form>

              <Table
                columns={departmentColumns}
                dataSource={departments}
                rowKey="id"
                loading={departmentLoading}
                pagination={departmentPagination}
                onChange={(page) => fetchDepartments({ page: page.current, pageSize: page.pageSize })}
              />
            </Col>
          </Row>

          <Modal
            title={editingDepartmentId ? '编辑部门' : '新增部门'}
            open={departmentModalVisible}
            onCancel={() => setDepartmentModalVisible(false)}
            footer={null}
          >
            <Form
              form={departmentModalForm}
              layout="vertical"
              onFinish={handleDepartmentModalSubmit}
            >
              <Form.Item
                name="name"
                label="部门名称"
                rules={[{ required: true, message: '请输入部门名称' }]}
              >
                <Input placeholder="请输入部门名称" />
              </Form.Item>

              <Form.Item
                name="parent_id"
                label="父部门"
              >
                <Select placeholder="选择父部门">
                  <Option value={null}>无父部门</Option>
                  {departments.map((dept) => (
                    <Option key={dept.id} value={dept.id}>
                      {dept.name}
                    </Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={4} placeholder="请输入部门描述" />
              </Form.Item>

              <Form.Item
                name="status"
                label="状态"
                rules={[{ required: true, message: '请选择状态' }]}
              >
                <Select placeholder="请选择状态">
                  <Option value={1}>启用</Option>
                  <Option value={0}>禁用</Option>
                </Select>
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setDepartmentModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
    {
      key: 'role',
      label: '角色管理',
      children: (
        <>
          <div style={{ marginBottom: 16 }}>
            <Button type="primary" icon={<PlusOutlined />} onClick={handleRoleCreate}>
              新增角色
            </Button>
          </div>

          <Table
            columns={[
              { title: '角色名称', dataIndex: 'name', key: 'name' },
              { title: '描述', dataIndex: 'description', key: 'description' },
              {
                title: '状态',
                dataIndex: 'status',
                key: 'status',
                render: (status: number, _record: any) => (
                  <Switch
                    checked={status === 1}
                    onChange={(checked) => handleRoleStatusChange(_record.id, checked ? 1 : 0)}
                    checkedChildren="启用"
                    unCheckedChildren="禁用"
                  />
                ),
              },

              {
                title: '操作',
                key: 'action',
                render: (_text: any, record: any) => (
                  <>
                    <Button
                      type="link"
                      icon={<EditOutlined />}
                      onClick={() => handleRoleEdit(record)}
                      style={{ marginRight: 8 }}
                    >
                      编辑
                    </Button>
                    <Button
                      type="link"
                      danger
                      icon={<DeleteOutlined />}
                      onClick={() => handleRoleDelete(record.id)}
                    >
                      删除
                    </Button>
                  </>
                ),
              },
            ]}
            dataSource={roles}
            rowKey="id"
            loading={rolesLoading}
          />

          <Modal
            title={editingRoleId ? '编辑角色' : '新增角色'}
            open={roleModalVisible}
            onCancel={() => setRoleModalVisible(false)}
            footer={null}
            width={800}
          >
            <Form
              form={roleModalForm}
              layout="vertical"
              onFinish={handleRoleModalSubmit}
            >
              <Form.Item
                name="name"
                label="角色名称"
                rules={[{ required: true, message: '请输入角色名称' }]}
              >
                <Input placeholder="请输入角色名称" />
              </Form.Item>

              <Form.Item
                name="description"
                label="描述"
              >
                <TextArea rows={4} placeholder="请输入角色描述" />
              </Form.Item>

              <Form.Item
                name="status"
                label="状态"
                rules={[{ required: true, message: '请选择状态' }]}
              >
                <Select placeholder="请选择状态">
                  <Option value={1}>启用</Option>
                  <Option value={0}>禁用</Option>
                </Select>
              </Form.Item>

              <Form.Item
                label="页面权限"
              >
                <Card title="页面权限配置" bordered={false}>
                  <Cascader
                    style={{ width: '100%' }}
                    options={cascaderOptions}
                    onChange={handlePermissionChange}
                    multiple
                    maxTagCount="responsive"
                    value={selectedPermissions}
                  />
                </Card>
              </Form.Item>

              <Form.Item>
                <Button type="primary" htmlType="submit" style={{ marginRight: 8 }}>
                  保存
                </Button>
                <Button onClick={() => setRoleModalVisible(false)}>
                  取消
                </Button>
              </Form.Item>
            </Form>
          </Modal>
        </>
      ),
    },
  ];

  return (
    <div>
      <Tabs defaultActiveKey="user" items={tabsItems} />
      <Modal
        title={`设置权限${permissionUser ? `：${permissionUser.real_name || permissionUser.username}` : ''}`}
        open={permissionModalVisible}
        onCancel={() => setPermissionModalVisible(false)}
        width={820}
        okText="保存个人权限"
        cancelText="取消"
        onOk={saveUserPermissions}
        footer={(_, { OkBtn, CancelBtn }) => (
          <>
            <Button onClick={clearUserPermissions}>恢复角色默认</Button>
            <CancelBtn />
            <OkBtn />
          </>
        )}
      >
        <Card
          title="页面权限"
          size="small"
          extra={<Tag color={permissionSource === 'user' ? 'blue' : 'default'}>{permissionSource === 'user' ? '个人权限' : '角色默认'}</Tag>}
        >
          <Cascader
            style={{ width: '100%' }}
            options={cascaderOptions}
            onChange={handlePermissionChange}
            multiple
            maxTagCount="responsive"
            value={selectedPermissions}
            placeholder="请选择该用户可访问的页面"
          />
        </Card>
      </Modal>
      
      {/* 用户详情模态框 */}
      <Modal
        title="用户详情"
        open={userDetailVisible}
        onCancel={() => setUserDetailVisible(false)}
        footer={[
          <Button key="close" onClick={() => setUserDetailVisible(false)}>
            关闭
          </Button>
        ]}
        width={600}
      >
        {selectedUser && (
          <div style={{ padding: '20px 0' }}>
            <Descriptions bordered column={1} size="middle">
              <Descriptions.Item label="用户名">{selectedUser.username}</Descriptions.Item>
              <Descriptions.Item label="真实姓名">{selectedUser.real_name || '-'}</Descriptions.Item>
              <Descriptions.Item label="工号">
                {selectedUser.employee_id || '-'}
              </Descriptions.Item>
              <Descriptions.Item label="联系电话">{selectedUser.phone || '-'}</Descriptions.Item>
              <Descriptions.Item label="角色">
                {Array.isArray(selectedUser.role_names) && selectedUser.role_names.length > 0 ? (
                  selectedUser.role_names.map((name: string) => <Tag color="blue" key={name}>{name}</Tag>)
                ) : (
                  <Tag color={selectedUser.role_name ? 'blue' : 'default'}>
                    {selectedUser.role_name || '-'}
                  </Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="部门">
                <Tag color={selectedUser.department_name ? 'green' : 'default'}>
                  {selectedUser.department_name || '-'}
                </Tag>
              </Descriptions.Item>
              <Descriptions.Item label="状态">
                {selectedUser.status === 1 ? (
                  <Tag color="success">启用</Tag>
                ) : (
                  <Tag color="error">禁用</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="是否绑定小程序">
                {selectedUser.bind_mini_program === '是' ? (
                  <Tag color="green">已绑定</Tag>
                ) : (
                  <Tag color="default">未绑定</Tag>
                )}
              </Descriptions.Item>
              <Descriptions.Item label="最后登录时间">
                {formatDateTime(selectedUser.last_login_time) || '从未登录'}
              </Descriptions.Item>
              <Descriptions.Item label="创建时间">
                {formatDateTime(selectedUser.createdAt)}
              </Descriptions.Item>
              <Descriptions.Item label="更新时间">
                {formatDateTime(selectedUser.updatedAt)}
              </Descriptions.Item>
            </Descriptions>
          </div>
        )}
      </Modal>
    </div>
  );
};

export default User;
