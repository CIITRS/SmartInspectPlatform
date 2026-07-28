/**
 * @see https://umijs.org/docs/max/access#access
 * */

// 定义用户类型
interface CurrentUser {
  id: number;
  username: string;
  real_name: string;
  employee_id?: string;
  phone: string;
  role: {
    id: number;
    name: string;
    description: string;
  };
  role_names?: string[];
  status: number;
  department: string;
  permissions: PermissionItem[];
  last_login_time: string | null;
}

// 定义权限项类型
interface PermissionItem {
  id: string;
  title: string;
  key: string;
  checked: boolean;
  children: PermissionItem[];
}

// 检查用户是否有权限访问指定页面
function hasPermission(
  currentUser: CurrentUser | undefined,
  pageKey: string
): boolean {
  if (!currentUser || !currentUser.permissions) {
    return false;
  }

  // 递归检查权限树
  function checkPermissionTree(permissions: PermissionItem[]): boolean {
    if (!permissions) {
      return false;
    }
    for (const item of permissions) {
      if (item.key === pageKey && item.checked) {
        return true;
      }
      if (item.children && item.children.length > 0) {
        if (checkPermissionTree(item.children)) {
          return true;
        }
      }
    }
    return false;
  }

  return checkPermissionTree(currentUser.permissions || []);
}

export default function access(
  initialState: { currentUser?: CurrentUser } | undefined,
) {
  const { currentUser } = initialState ?? {};
  
  // 基于权限树的访问控制
  const canAccessDashboard = hasPermission(currentUser, 'dashboard');
  const canAccessPatient = hasPermission(currentUser, 'patient');
  const canAccessSample = hasPermission(currentUser, 'sample');
  const canAccessResult = hasPermission(currentUser, 'result');
  const canAccessReport = hasPermission(currentUser, 'report');
  const canAccessSystem = hasPermission(currentUser, 'system');
  const canAccessUsers = hasPermission(currentUser, 'users') || canAccessSystem;
  const canAccessAnnouncement = hasPermission(currentUser, 'announcement');
  const canAccessAIManagement = hasPermission(currentUser, 'ai-management');
  const canAccessSales = hasPermission(currentUser, 'sales');
  const canAccessAppointment = hasPermission(currentUser, 'appointment') || canAccessSales;
  const roleName = Array.isArray(currentUser?.role_names) && currentUser.role_names.length > 0
    ? currentUser.role_names.join('、')
    : currentUser?.role?.name || '';
  // 检查是否为销售角色
  const canSales = /销售/.test(roleName);
  
  // 检查是否为管理员角色
  const canAdmin = /管理员|IT/.test(roleName);
  

  
  return {
    canAdmin,
    canAccessDashboard,
    canAccessPatient,
    canAccessSample,
    canAccessResult,
    canAccessReport,
    canAccessSystem,
    canAccessUsers,
    canAccessAnnouncement,
    canAccessAIManagement,
    canAccessSales,
    canAccessAppointment,
    canSales,
  };
}
