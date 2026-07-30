// API服务封装

// 根据环境变量判断API地址
// 开发环境使用本地服务，生产环境使用正式服务器
const getBaseUrl = () => {
  // #ifdef MP-WEIXIN
  // 小程序环境使用正式服务器
  return 'https://bgpt.huaweibio.com.cn/api';
  // #endif
};

export const BASE_URL = getBaseUrl();

// 请求封装
function request(url, options = {}) {
  return new Promise((resolve, reject) => {
    // 获取 session_id
    const sessionId = uni.getStorageSync('miniapp_session_id');
    
    uni.request({
      url: BASE_URL + url,
      method: options.method || 'GET',
      data: options.data || {},
      header: {
        'Content-Type': 'application/json',
        'X-Miniapp-Session': sessionId || '',
        ...options.header
      },
      success: (res) => {
        // 处理 401 未授权
        if (res.statusCode === 401) {
          // 清除本地 session
          uni.removeStorageSync('miniapp_session_id');
          // 跳转到登录页
          uni.showToast({
            title: '请先登录',
            icon: 'none',
            duration: 1200
          });
          uni.redirectTo({
            url: '/pages/login/index'
          });
          reject({ code: 401, message: '未授权，请先登录' });
          return;
        }
        
        if (res.statusCode === 200) {
          // 处理业务错误
          if (res.data && res.data.code === 401) {
            uni.removeStorageSync('miniapp_session_id');
            uni.showToast({
              title: res.data.message || '请先登录',
              icon: 'none',
              duration: 1200
            });
            uni.redirectTo({
              url: '/pages/login/index'
            });
            reject(res.data);
            return;
          }
          resolve(res.data);
        } else {
          // 显示错误信息
          const errorMsg = res.data?.message || '请求失败';
          if (options.showError !== false) {
            uni.showToast({
              title: errorMsg,
              icon: 'none',
              duration: 2000
            });
          }
          reject(res.data);
        }
      },
      fail: (err) => {
        // 网络错误处理
        if (options.showError !== false) {
          uni.showToast({
            title: '网络错误，请检查网络连接',
            icon: 'none',
            duration: 2000
          });
        }
        reject({ code: -1, message: '网络错误', error: err });
      }
    });
  });
}

// 认证相关API
export const authAPI = {
  // 登录
  login: (data) => {
    return request('/auth/login', {
      method: 'POST',
      data
    });
  },
  // 获取用户信息
  getMe: () => {
    return request('/auth/me');
  },
  // 退出登录
  logout: () => {
    return request('/auth/logout', {
      method: 'POST'
    });
  },
  // 根据手机号查询身份
  phoneIdentities: (data) => {
    return request('/auth/phone-identities', {
      method: 'POST',
      data
    });
  },
  // 发送短信验证码
  smsSend: (data) => {
    return request('/auth/sms/send', {
      method: 'POST',
      data
    });
  },
  // 短信验证码登录
  smsLogin: (data) => {
    return request('/auth/sms/login', {
      method: 'POST',
      data
    });
  },
  // 一键登录
  oneClickLogin: (data) => {
    return request('/auth/oneclick/login', {
      method: 'POST',
      data
    });
  },
  switchIdentity: (data) => {
    return request('/auth/miniapp/switch-identity', {
      method: 'POST',
      data
    });
  },
  bindPhone: (data) => {
    return request('/auth/bind-phone', {
      method: 'POST',
      data
    });
  },
  // 首次登录患者建档
  registerPatientFirstLogin: (data) => {
    return request('/auth/miniapp/register-patient', {
      method: 'POST',
      data
    });
  },
  checkIdCard: (idCard, documentType = '居民身份证') => {
    return request('/auth/miniapp/check-id-card', {
      data: { id_card: idCard, id_document_no: idCard, id_document_type: documentType }
    });
  },
  // 获取邀请页客户经理
  getInviteManager: (salesId) => {
    return request('/auth/miniapp/invite-manager', {
      data: { sales_id: salesId }
    });
  },
  // 销售码邀请注册
  inviteRegister: (data) => {
    return request('/auth/miniapp/invite-register', {
      method: 'POST',
      data
    });
  }
};

// 患者相关API
export const patientAPI = {
  // 获取患者列表
  getPatients: (params) => {
    return request('/patients', {
      data: params
    });
  },
  // 获取患者详情
  getPatientById: (id) => {
    return request(`/patients/${id}`);
  },
  // 创建患者
  createPatient: (data) => {
    return request('/patients', {
      method: 'POST',
      data
    });
  },
  // 更新患者
  updatePatient: (id, data) => {
    return request(`/patients/${id}`, {
      method: 'PUT',
      data
    });
  }
};

// 样本相关API
export const sampleAPI = {
  // 获取样本列表
  getSamples: (params) => {
    return request('/samples', {
      data: params
    });
  },
  // 创建样本
  createSample: (data) => {
    return request('/samples', {
      method: 'POST',
      data
    });
  },
  // 样本接收
  sampleReceived: (data) => {
    return request('/samples/receive', {
      method: 'POST',
      data
    });
  },
  // 批量接收样本
  batchReceiveSamples: (data) => {
    return request('/samples/detect_batchReceive', {
      method: 'POST',
      data
    });
  }
};

// 报告相关API
export const reportAPI = {
  // 获取报告列表
  getReports: (params) => {
    return request('/reports', {
      data: params
    });
  },
  // 获取患者历史报告
  getPatientReports: (patientId) => {
    return request(`/reports/patient/${patientId}`);
  },
  // 获取报告详情
  getReportBySampleCode: (sampleCode) => {
    return request(`/reports/${sampleCode}`);
  },
  // 生成报告
  generateReport: (data) => {
    return request('/reports/generate', {
      method: 'POST',
      data
    });
  },
  // 审核报告
  reviewReport: (id, data) => {
    return request(`/reports/review/${id}`, {
      method: 'PUT',
      data
    });
  }
};

// 批次相关API
export const batchAPI = {
  // 获取批次列表
  getBatches: (params) => {
    return request('/batch/list', {
      data: params
    });
  },
  // 获取批次详情
  getBatchDetail: (batchCode) => {
    return request(`/batch/detail/${batchCode}`);
  },
  // 获取批次样本
  getBatchSamples: (batchCode) => {
    return request(`/batch/samples/${batchCode}`);
  },
  // 更新批次状态
  updateBatchStatus: (batchCode, data) => {
    return request(`/batch/status/${batchCode}`, {
      method: 'PUT',
      data
    });
  },
  // 提交批次
  submitBatch: (batchCode) => {
    return request(`/batch/submit/${batchCode}`, {
      method: 'POST'
    });
  }
};

// 结果相关API
export const resultAPI = {
  // 获取结果列表
  getResults: (params) => {
    return request('/results', {
      data: params
    });
  },
  // 获取患者结果
  getPatientResults: (patientCode) => {
    return request(`/results/patient/${patientCode}`);
  },
  // 获取患者结果对比
  getPatientResultsCompare: (patientCode) => {
    return request(`/results/patient/${patientCode}/compare`);
  }
};

// 系统相关API
export const systemAPI = {
  // 获取样本类型
  getSampleTypes: () => {
    return request('/system/sampleTypes');
  },
  // 获取癌症类型
  getCancerTypes: () => {
    return request('/system/cancerTypes/list');
  },
  // 获取治疗阶段
  getTreatmentStages: () => {
    return request('/system/treatmentStages');
  },
  // 获取部门列表
  getDepartments: () => {
    return request('/system/departments');
  }
};

// AI助手相关API
export const aiAPI = {
  // 实时问答（非流式）
  chat: (data) => {
    return request('/ai/chat', {
      method: 'POST',
      data
    });
  },
  // 实时问答（流式）
  chatStream: (data, callbacks) => {
    const { onChunk, onComplete, onError } = callbacks || {};
    
    // 获取 session_id
    const sessionId = uni.getStorageSync('miniapp_session_id');
    
    // 直接使用 uni.request 处理流式数据
    const requestTask = uni.request({
      url: BASE_URL + '/ai/chat',
      method: 'POST',
      data: data,
      header: {
        'Content-Type': 'application/json',
        'X-Miniapp-Session': sessionId || ''
      },
      responseType: 'text',
      enableChunked: true, // 启用分块传输
      success: (res) => {
        // 处理完整响应（非流式情况或流式结束）
        if (onComplete) {
          onComplete(res);
        }
      },
      fail: (err) => {
        if (onError) {
          onError(err);
        } else {
          uni.showToast({
            title: '网络错误，请检查网络连接',
            icon: 'none',
            duration: 2000
          });
        }
      }
    });
    
    // uni-app 的 uni.request 不直接支持 onProgress，但我们可以使用替代方法
    // 这里我们使用一个轮询机制或等待支持的方式，或者我们降级为非流式
    
    // 实际上，对于 uni-app，在小程序环境中流式支持有限
    // 我们先保持这个框架，然后在实际页面中处理
    
    return requestTask;
  },
  // 报告分析
  analyzeReport: (data) => {
    const filePath = typeof data === 'string' ? data : data.filePath
    const sessionId = uni.getStorageSync('miniapp_session_id')
    return new Promise((resolve, reject) => {
      uni.uploadFile({
        url: BASE_URL + '/ai/report-analysis',
        filePath,
        name: 'file',
        header: { 'X-Miniapp-Session': sessionId || '' },
        success: (res) => {
          try {
            const payload = typeof res.data === 'string' ? JSON.parse(res.data) : res.data
            if (res.statusCode >= 200 && res.statusCode < 300 && payload.success) resolve(payload)
            else reject(new Error(payload.message || '报告分析失败'))
          } catch (error) {
            reject(error)
          }
        },
        fail: reject
      })
    })
  }
};

// 小程序专用API
export const uniAPI = {
  // 获取患者信息
  getPatientInfo: () => {
    return request('/uni/patient/info');
  },
  // 更新患者信息
  updatePatientInfo: (data) => {
    return request('/uni/patient/info', {
      method: 'PUT',
      data
    });
  },
  // 获取检测计划/预约列表
  getDetectionPlans: () => {
    return request('/uni/detection-plans');
  },
  // 获取我的套餐
  getMyPackages: () => {
    return request('/uni/packages');
  },
  getPackageOptions: () => {
    return request('/uni/package-options');
  },
  applyPackage: (data) => {
    return request('/uni/package-applications', {
      method: 'POST',
      data
    });
  },
  // 预约邮寄采样盒
  createSampleBoxRequest: (data) => {
    return request('/uni/sample-box-request', {
      method: 'POST',
      data
    });
  },
  // 获取试剂盒邮寄预约记录
  getSampleBoxRequests: () => {
    return request('/uni/sample-box-requests');
  },
  // 获取专属客户经理
  getPatientManager: () => {
    return request('/uni/patient/manager');
  },
  // 获取患者报告列表
  getReports: () => {
    return request('/uni/reports');
  },
  // 获取报告详情
  getReportDetail: (id) => {
    return request(`/uni/reports/${id}`);
  },
  // 获取样本列表
  getSamples: (params = {}) => {
    return request('/uni/samples', {
      data: params
    });
  },
  // 提交邮寄样本
  createMailSample: (data) => {
    return request('/uni/mail-sample', {
      method: 'POST',
      data
    });
  },
  // 获取邮寄样本列表
  getMailSamples: () => {
    return request('/uni/mail-samples');
  },
  // 获取帮助中心内容
  getHelpCenter: () => {
    return request('/uni/help-center');
  },
  // 获取随访单列表
  getFollowUps: () => {
    return request('/uni/follow-ups');
  },
  // 新增随访单
  createFollowUp: (data) => {
    return request('/uni/follow-ups', {
      method: 'POST',
      data
    });
  },
  // 获取员工待办统计
  getEmployeeStats: () => {
    return request('/uni/employee/stats');
  },
  // 获取员工报告列表
  getEmployeeReports: (params = {}) => {
    return request('/uni/employee/reports', {
      data: params
    });
  },
  // 获取员工患者列表
  getEmployeePatients: (params = {}) => {
    return request('/uni/employee/patients', {
      data: params
    });
  },
  getEmployeePatientGroups: () => request('/uni/employee/patient-groups'),
  createEmployeePatientGroup: (data) => request('/uni/employee/patient-groups', { method: 'POST', data }),
  deleteEmployeePatientGroup: (id) => request(`/uni/employee/patient-groups/${id}`, { method: 'DELETE' }),
  setEmployeePatientGroup: (patientId, groupId) => request(`/uni/employee/patients/${patientId}/group`, {
    method: 'PUT',
    data: { group_id: Number(groupId) || 0 }
  }),
  // 获取员工端患者详情
  getEmployeePatientDetail: (id) => {
    return request(`/uni/employee/patients/${id}`);
  },
  getEmployeePatientReportPreview: (id, fileUrl) => {
    return request(`/uni/employee/patients/${id}/report-files/preview?file_url=${encodeURIComponent(fileUrl)}`);
  },
  getEmployeePatientReportAnalysis: (id, fileUrl) => {
    return request(`/uni/employee/patients/${id}/report-files/analysis?file_url=${encodeURIComponent(fileUrl)}`, {
      showError: false
    });
  },
  analyzeEmployeePatientReport: (id, fileUrl, force = false) => {
    return request(`/uni/employee/patients/${id}/report-files/analysis${force ? '?force=1' : ''}`, {
      method: 'POST',
      data: { file_url: fileUrl },
      showError: false
    });
  },
  completeEmployeePatient: (id, data) => {
    return request(`/uni/employee/patients/${id}/completion`, {
      method: 'POST',
      data
    });
  },
  // 员工新患录入
  createEmployeePatient: (data) => {
    return request('/uni/employee/patients', {
      method: 'POST',
      data
    });
  },
  // 获取员工新增样本选项
  getEmployeeSampleOptions: (patientIds = []) => {
    const ids = Array.isArray(patientIds) ? patientIds.filter(Boolean).join(',') : String(patientIds || '')
    return request(`/uni/employee/sample-options${ids ? `?patient_ids=${encodeURIComponent(ids)}` : ''}`, { showError: false });
  },
  // 获取待审核报告列表
  getPendingReports: () => {
    return request('/uni/employee/reports/pending');
  },
  // 获取真实审核人候选
  getReportReviewers: () => {
    return request('/uni/employee/report-reviewers');
  },
  // 审核报告
  reviewReport: (id, data) => {
    return request(`/uni/employee/reports/${id}/review`, {
      method: 'PUT',
      data
    });
  },
  // 获取报告PDF下载链接
  downloadReportPDF: (id, mode = 'view') => {
    return request(`/uni/reports/${id}/pdf/download?mode=${encodeURIComponent(mode)}`);
  },
  // 获取报告预览图片
  getReportPreviewImage: (id) => {
    return request(`/uni/reports/${id}/preview-image`);
  },
  // 获取待接收样本列表
  getPendingSamples: () => {
    return request('/uni/employee/samples/pending');
  },
  // 分配样本编号
  allocateEmployeeSamples: (data) => {
    return request('/uni/employee/samples/allocate', {
      method: 'POST',
      data
    });
  },
  // 当前员工创建的样本及样本详情
  getEmployeeSamples: () => {
    return request('/uni/employee/samples', { showError: false });
  },
  getEmployeeSampleDetail: (id) => {
    return request(`/uni/employee/samples/${id}`);
  },
  deleteEmployeeSample: (id, reusable) => {
    return request(`/uni/employee/samples/${id}`, {
      method: 'DELETE',
      data: { reusable: Boolean(reusable) }
    });
  },
  // 接收样本
  receiveSample: (data) => {
    return request('/uni/employee/samples/receive', {
      method: 'POST',
      data
    });
  },
  // 批量接收样本
  batchReceiveSamples: (data) => {
    return request('/uni/employee/samples/batch-receive', {
      method: 'POST',
      data
    });
  },
  // 获取员工邀请小程序码
  getInviteCode: () => {
    return request('/uni/employee/invite-code');
  },
  // 获取快递运单
  getExpress: (sampleId) => {
    return request(`/uni/samples/${sampleId}/express`);
  },
  // 创建快递运单
  createExpress: (data) => {
    return request('/express/create', {
      method: 'POST',
      data
    });
  },
  // 更新快递运单
  updateExpress: (id, data) => {
    return request(`/express/${id}`, {
      method: 'PUT',
      data
    });
  }
};
