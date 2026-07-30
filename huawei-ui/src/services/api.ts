import { request } from '@umijs/max';

const inFlightGetRequests = new Map<string, Promise<any>>();

const stableStringify = (value: any): string => {
  if (value === null || value === undefined) return String(value);
  if (typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(stableStringify).join(',')}]`;
  return `{${Object.keys(value).sort().map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`).join(',')}}`;
};

const getRequestKey = (url: string, options: any) => stableStringify({
  url,
  params: options?.params || {},
  data: options?.data || undefined,
});

const apiRequest = <T = any>(url: string, options: any = {}) => {
  const { skipDedupe, ...requestOptions } = options || {};
  const method = String(requestOptions.method || 'GET').toUpperCase();
  if (method !== 'GET' || skipDedupe) {
    return request<T>(url, requestOptions);
  }

  const key = getRequestKey(url, requestOptions);
  const pending = inFlightGetRequests.get(key);
  if (pending) return pending as Promise<T>;

  const promise = request<T>(url, requestOptions).finally(() => {
    inFlightGetRequests.delete(key);
  });
  inFlightGetRequests.set(key, promise);
  return promise;
};

// 认证相关API
export async function login(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ token: string; user: any }>('/api/auth/login', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function getCurrentUser(options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/auth/me', {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取用户信息失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function changePassword(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>('/api/auth/changePassword', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '密码修改失败');
      }
      
      return apiResponse;
    },
  });
}

export async function updateUserInfo(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>('/api/auth/update', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function updateUsername(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>('/api/auth/updateUsername', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '用户名修改失败');
      }
      
      return apiResponse;
    },
  });
}

// 工具函数：下划线转驼峰
function toCamelCase(obj: any): any {
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }
  if (Array.isArray(obj)) {
    return obj.map(item => toCamelCase(item));
  }
  const camelObj: any = {};
  for (const key in obj) {
    if (obj.hasOwnProperty(key)) {
      const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
      camelObj[camelKey] = toCamelCase(obj[key]);
    }
  }
  return camelObj;
}

// 患者管理相关API
export async function listPatients(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/patients', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      console.log('API原始响应:', response);
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式1: { code: 200, message: '...', data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式2: { success: true, data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      console.log('实际数据:', actualData);
      console.log('患者列表长度:', actualData.list.length);
      
      // 转换列表中的每个患者对象，将下划线命名转换为驼峰命名
      const transformedList = actualData.list.map((patient: any) => {
        const transformedPatient: any = {};
        for (const key in patient) {
          if (patient.hasOwnProperty(key)) {
            // 转换下划线命名为驼峰命名
            const camelKey = key.replace(/_([a-z])/g, (_, letter) => letter.toUpperCase());
            transformedPatient[camelKey] = patient[key];
          }
        }
        return transformedPatient;
      });

      // 返回前端组件期望的格式
      return {
        data: {
          list: transformedList,
          total: actualData.total
        }
      };
    },
  });
}

export async function getPatientById(patientCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/patients/${patientCode}`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function getPatientDetail(patientCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/patients/${patientCode}`, {
    method: 'GET',
    ...(options || {}),
  });
}

// 验证身份证号是否存在
export async function checkIdCard(idCard: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: { exists: boolean; patient: any } }>('/api/patients/checkIdCard', {
    method: 'GET',
    params: { id_card: idCard },
    ...(options || {}),
    resolveResponse: handleResponse,
  });
}

// 工具函数：驼峰转下划线
function toSnakeCase(obj: any): any {
  // 处理日期对象，包括 dayjs 对象
  if (obj === null || typeof obj !== 'object') {
    return obj;
  }
  // 检查是否为 dayjs 对象（具有 $d 属性的日期对象）
  if (obj.$d && obj.$d instanceof Date) {
    return obj.toISOString();
  }
  // 检查是否为原生 Date 对象
  if (obj instanceof Date) {
    return obj.toISOString();
  }
  if (Array.isArray(obj)) {
    return obj.map(item => toSnakeCase(item));
  }
  // 检查是否为 FormData 对象
  if (obj instanceof FormData) {
    return obj;
  }
  const snakeObj: any = {};
  for (const key in obj) {
    if (Object.prototype.hasOwnProperty.call(obj, key)) {
      // 跳过不可枚举属性和函数
      if (typeof obj[key] === 'function') continue;
      // 如果 key 已经是 snake_case，直接保留
      if (key.includes('_')) {
        snakeObj[key] = toSnakeCase(obj[key]);
        continue;
      }
      // 否则进行驼峰转下划线
      let snakeKey = '';
      for (let i = 0; i < key.length; i++) {
        const char = key[i];
        if (char === char.toUpperCase() && i > 0) {
          snakeKey += '_' + char.toLowerCase();
        } else {
          snakeKey += char.toLowerCase();
        }
      }
      snakeObj[snakeKey] = toSnakeCase(obj[key]);
    }
  }
  return snakeObj;
}

export async function createPatient(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/patients', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function updatePatient(patientCode: string, body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/patients/${patientCode}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function deletePatient(patientCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/patients/${patientCode}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

// 样本管理相关API
export async function createSample(body: any, options?: { [key: string]: any }) {
  // 直接使用传入的body，因为已经手动设置了正确的字段名格式
  return apiRequest<{ data: any }>('/api/samples', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function allocateSamples(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/samples/allocate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function sampleReceived(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/samples/receive', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function getSampleReceivePreview(sampleCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/samples/receive-preview', {
    method: 'GET',
    params: { sample_code: sampleCode },
    ...(options || {}),
  });
}

export async function batchReceiveSamples(body: { sample_codes: string[] }, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/samples/detect_batchReceive', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function updateSample(id: string, body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/samples/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function deleteSample(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/samples/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

// 结果管理相关API
export async function listResults(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/results', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

// 获取样本列表
export async function getSamples(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>(`/api/samples`, {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

// 获取患者样本列表
export async function getSamplesByPatientId(patientCode: string, options?: { [key: string]: any }) {
  return getSamples({ patient_code: patientCode }, options);
}

// 获取患者检测结果
export async function getResultsByPatientId(patientCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>(`/api/results/patient/${patientCode}`, {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理，当患者没有检测结果时返回空数组
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let data: any[] = [];
      
      if (apiResponse.code === 200) {
        // 格式1: { code: 200, message: '...', data: { results: [] } }
        success = true;
        data = apiResponse.data?.results || [];
      } else if (apiResponse.success === true) {
        // 格式2: { success: true, data: { results: [] } }
        success = true;
        data = apiResponse.data?.results || [];
      } else if (apiResponse.code === 404) {
        // 处理404错误，返回空数组
        return {
          data: []
        };
      }
      
      if (!success) {
        // 其他错误，直接返回空数组
        return {
          data: []
        };
      }
      
      // 返回前端组件期望的格式
      return {
        data: data
      };
    },
  });
}

// 获取患者结果对比
export async function getPatientResultsCompare(patientCode: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/results/patient/${patientCode}/compare`, {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取患者历史结果失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function createResult(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/results', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function importResults(file: FormData, onUploadProgress?: (progressEvent: any) => void, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: any }>('/api/results/import', {
    method: 'POST',
    data: file,
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    onUploadProgress,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      let data: any = null;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '导入失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message,
        data
      };
    },
  });
}

// 检查样本是否已存在结果
export async function checkExistingResults(sampleCodes: string[], options?: { [key: string]: any }) {
  return apiRequest<{ data: { existingSamples: string[] } }>('/api/results/checkExisting', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { sample_codes: sampleCodes },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let data: any = null;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { existingSamples: [] } }
        success = true;
        data = apiResponse.data;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { existingSamples: [] } }
        success = true;
        data = apiResponse.data;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '检查样本失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data
      };
    },
  });
}

export async function downloadTemplate(cancerTypeId: string, modelId: string, options?: { [key: string]: any }) {
  return apiRequest<Blob>(`/api/results/template/${cancerTypeId}`, {
    method: 'GET',
    params: { modelId },
    responseType: 'blob',
    ...(options || {}),
  });
}

// 患者端API
export async function queryDetectionStatus(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/patient/query-status', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

// 系统设置相关API - 模型设置
export async function listModels(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>('/api/system/modelSettings', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createModel(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/system/modelSettings', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function updateModel(id: string, body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/system/modelSettings/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      
      if (apiResponse.code === 200) {
        success = true;
      } else if (apiResponse.success === true) {
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '更新失败');
      }
      
      return apiResponse;
    },
  });
}

export async function getModelGeneThresholds(id: string | number, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>(`/api/system/modelSettings/${id}/geneThresholds`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function updateModelGeneThresholds(id: string | number, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any }>(`/api/system/modelSettings/${id}/geneThresholds`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function deleteModel(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/system/modelSettings/${id}`, {
    method: 'DELETE',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      
      if (apiResponse.code === 200) {
        success = true;
      } else if (apiResponse.success === true) {
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '删除失败');
      }
      
      return apiResponse;
    },
  });
}

// 系统设置相关API - 基因设置
export async function listGenes(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>('/api/system/geneSettings', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createGene(body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/system/geneSettings', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function updateGene(id: string, body: any, options?: { [key: string]: any }) {
  // 将驼峰命名转换为下划线命名，匹配后端期望的格式
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/system/geneSettings/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function updateGenePanels(id: string | number, panelIds: number[], options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any }>(`/api/system/geneSettings/${id}/panels`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { panelIds },
    ...(options || {}),
  });
}

export async function deleteGene(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/system/geneSettings/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

// Panel管理相关API
export async function listPanels(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>('/api/system/panels', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createPanel(body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/system/panels', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function updatePanel(id: string, body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/system/panels/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function deletePanel(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/system/panels/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

export async function getPanelGenes(panelId: string, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>(`/api/system/panels/${panelId}/genes`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function updatePanelGenes(panelId: string, geneIds: number[], options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/system/panels/${panelId}/genes`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { geneIds },
    ...(options || {}),
  });
}

export async function listCancerTypes(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[]; total: number }>('/api/system/cancerTypes/list', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [], total: 0 }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [], total: 0 }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data,
        total: actualData.total
      };
    },
  });
}

// 通用响应处理函数
function handleResponse(response: any) {
  const apiResponse = response.data;
  
  // 兼容两种后端返回格式
  let success = false;
  
  if (apiResponse.code === 200 || apiResponse.code === 201) {
    // 格式: { code: 200, message: '...', data: {} }
    success = true;
  } else if (apiResponse.success === true) {
    // 格式: { success: true, data: {} }
    success = true;
  }
  
  if (!success) {
    throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
  }
  
  return apiResponse;
}

export async function createCancerType(body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>('/api/system/cancerTypes', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
    resolveResponse: handleResponse,
  });
}

export async function updateCancerType(id: string, body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ data: any }>(`/api/system/cancerTypes/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
    resolveResponse: handleResponse,
  });
}

export async function deleteCancerType(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean }>(`/api/system/cancerTypes/${id}`, {
    method: 'DELETE',
    ...(options || {}),
    resolveResponse: handleResponse,
  });
}

// 系统设置相关API - 样本类型

export async function getSampleTypes(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/sampleTypes', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

export async function listSampleTypes(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any[] }>('/api/system/sampleTypes/list', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createSampleType(data: any, options?: { [key: string]: any }) {
  return apiRequest('/api/system/sampleTypes', {
    method: 'POST',
    data,
    ...(options || {}),
  });
}

export async function updateSampleType(id: string, data: any, options?: { [key: string]: any }) {
  const snakeCaseData = toSnakeCase(data);
  return apiRequest(`/api/system/sampleTypes/${id}`, {
    method: 'PUT',
    data: snakeCaseData,
    ...(options || {}),
  });
}

export async function deleteSampleType(id: string, options?: { [key: string]: any }) {
  return apiRequest(`/api/system/sampleTypes/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

// 系统设置相关API - 治疗阶段

export async function getTreatmentStages(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/treatmentStages', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

// 系统设置相关API - 部门管理

export async function listDepartments(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/departments', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

export async function listDepartmentsTree(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/departments/tree', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

// 系统设置相关API - 用户管理

export async function listUsers(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/users', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

// 系统设置相关API - 角色管理

export async function listRoles(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/system/roles', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: [] }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: [] }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

// 报告管理相关API
export async function getSamplesWithoutReports(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/reports/samplesWithoutReports', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { list: [], total: 0 } }
        actualData = apiResponse;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { list: [], total: 0 } }
        actualData = apiResponse;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取未生成报告的样本列表失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData.data
      };
    },
  });
}

export async function getPendingReviewReports(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/reports/pendingReview', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function getGeneratingReports(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/reports/generating', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function getReportPdfStatus(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/${id}/pdf/status`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function generateReport(body: any, options?: { [key: string]: any }) {
  // 直接使用驼峰命名的参数，后端已经支持驼峰命名
  return apiRequest<{ data: any }>('/api/reports/generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function reviewReport(id: string, body: any, options?: { [key: string]: any }) {
  // 直接使用驼峰命名的参数，后端已经支持驼峰命名
  return apiRequest<{ data: any }>(`/api/reports/review/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function listReports(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/reports', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function listAppointments(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/appointments', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function updateAppointment(id: string | number, body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ success: boolean; message: string }>(`/api/appointments/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
  });
}

export async function uploadAppointmentTracking(file: FormData, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data: any }>('/api/appointments/tracking-upload', {
    method: 'POST',
    data: file,
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    ...(options || {}),
  });
}

export async function getReportById(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/${id}`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function getPatientHistoricalReports(patientId: string | number, params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>(`/api/reports/patient/${patientId}`, {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function downloadReport(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/patient/download/${id}`, {
    method: 'GET',
    ...(options || {}),
  });
}

export async function batchDownloadReports(body: { ids: number[]; version: 'concise' | 'full' }, options?: { [key: string]: any }) {
  return apiRequest<{ data: { downloadUrl: string; fileName: string; count: number } }>('/api/reports/batch-download', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function updateReportStatus(id: string, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/status/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let data: any = null;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: null }
        success = true;
        data = apiResponse.data;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: null }
        success = true;
        data = apiResponse.data;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '更新报告状态失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: data
      };
    },
  });
}

export async function deleteReport(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

export async function updateReport(id: string, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 更新样本基因表达值
export async function updateSampleGeneData(sampleId: string, geneData: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/samples/${sampleId}/geneData`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { gene_data: geneData },
    ...(options || {}),
  });
}

// 公式计算相关API
export async function calculateFormula(formula: string, variables: any, thresholds?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { result: number } }>('/api/formula/calculate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      formula,
      variables,
      thresholds,
    },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { result: 0 } }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { result: 0 } }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '公式计算失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function calculateModelFormula(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ code: number; success: boolean; message: string; data: any }>('/api/formula/modelCalculate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 箱线图数据相关API
export async function getBoxplotData(geneSymbol: string, cancerTypeId?: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/results/boxplot', {
    method: 'GET',
    params: {
      geneSymbol,
      cancerTypeId,
    },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取箱线图数据失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

// 更新结果的信号值
export async function updateResultSignalValue(resultId: number, signalValue: number, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/results/update-signal', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      resultId,
      signalValue,
    },
    ...(options || {}),
  });
}

// 模板管理相关API
export async function getTemplates(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/reports/templates', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createTemplate(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/reports/templates', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function updateTemplate(id: string, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/templates/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function deleteTemplate(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/reports/templates/${id}`, {
    method: 'DELETE',
    ...(options || {}),
  });
}

export async function getReportPositions(options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/system/report-positions', {
    method: 'GET',
    ...(options || {}),
  });
}

export async function getSystemBootstrap(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: Record<string, any> }>('/api/system/bootstrap', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function updateReportPosition(id: number, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/system/report-positions/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 批量生成报告
export async function batchGenerateReports(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/reports/batch-generate', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 销售模组相关API

// 套餐管理
export async function listPackages(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/sales/packages', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createPackage(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/packages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function updatePackage(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/packages', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function listPatientPackages(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/sales/patient-packages', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function bindPatientPackage(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/patient-packages', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function listSalesAssignmentPatients(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/sales/assignment-patients', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function assignSalesToPatient(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/assign-patient', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 订单管理
export async function listOrders(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/sales/orders', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

export async function createOrder(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/orders', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 检测计划管理
export async function listDetectionPlans(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/sales/detectionPlans', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}



// 销售统计
export async function getSalesStatistics(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/sales/statistics', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

// 获取用户信息
export async function getUserInfo(options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/auth/me', {
    method: 'GET',
    ...(options || {}),
  });
}

// 获取订单列表
export async function getOrders(options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/sales/orders', {
    method: 'GET',
    ...(options || {}),
  });
}

// 获取检测计划
export async function getDetectionPlans(orderId: number, options?: { [key: string]: any }) {
  return apiRequest<{ data: any[] }>('/api/sales/detectionPlans', {
    method: 'GET',
    params: { sale_orderId: orderId },
    ...(options || {}),
  });
}

// 更新检测计划
export async function updateDetectionPlan(body: any, options?: { [key: string]: any }) {
  const { id, ...rest } = body;
  return apiRequest<{ data: any }>(`/api/sales/detectionPlans/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: rest,
    ...(options || {}),
  });
}

// 批次管理相关API
export async function importBatch(file: FormData, onUploadProgress?: (progressEvent: any) => void, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: any }>('/api/batch/import', {
    method: 'POST',
    data: file,
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    onUploadProgress,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      let data: any = null;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '导入失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message,
        data
      };
    },
  });
}

export async function listBatches(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/batch/list', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function getBatchDetail(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/batch/detail/${id}`, {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function searchBatchesByPatient(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[]; total: number } }>('/api/batch/search', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { list: [], total: 0 } }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '请求失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

export async function updateBatchStatus(id: string, body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>(`/api/batch/status/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '更新状态失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message
      };
    },
  });
}

// 批次结果导出API
export async function exportBatchResult(batchId: string, options?: { [key: string]: any }) {
  return apiRequest<Blob>(`/api/batch/export/${batchId}`, {
    method: 'GET',
    responseType: 'blob',
    ...(options || {}),
  });
}

// 更新批次检测类型API
export async function updateBatchCancerType(id: string, cancerTypeId: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>(`/api/batch/cancerType/${id}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { cancerTypeId },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(message || apiResponse.errorMessage || '请求失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

// 更新单个样本检测类型API
export async function updateSampleCancerType(batchId: string, sampleCode: string, cancerTypeId: string, modelId?: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>(`/api/batch/sampleCancerType/${batchId}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: { sampleCode, cancerTypeId, modelId },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(message || apiResponse.errorMessage || '请求失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

// 自动匹配检测类型API
export async function autoMatchCancerType(batchId: string, options?: { [key: string]: any }) {
  return apiRequest<{
    success: boolean;
    message: string;
    data: {
      matchedCancerTypes: any[];
      recommendedCancerType: any;
    }
  }>(`/api/batch/auto-match-cancer-type/${batchId}`, {
    method: 'POST',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      let data = { matchedCancerTypes: [], recommendedCancerType: null };
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
        data = apiResponse.data || data;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
        data = apiResponse.data || data;
      }
      
      if (!success) {
        throw new Error(message || apiResponse.errorMessage || '请求失败');
      }
      
      return {
        success,
        message,
        data
      };
    },
  });
}

// 获取可选择的检测类型列表API
export async function getSelectableCancerTypes(currentCancerTypeId: string | number = 0, options?: { [key: string]: any }) {
  return apiRequest<{
    success: boolean;
    message: string;
    data: any[];
  }>(`/api/system/cancerTypes/selectable/${currentCancerTypeId}`, {
    method: 'GET',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      let data: any[] = [];
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
        data = apiResponse.data || data;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
        data = apiResponse.data || data;
      }
      
      if (!success) {
        throw new Error(message || apiResponse.errorMessage || '请求失败');
      }
      
      return {
        success,
        message,
        data
      };
    },
  });
}

// 基因匹配API
export async function matchGenes(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/results/match-genes', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 批次基因匹配API（兼容旧路由）
export async function batchMatchGenes(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/batch/match-genes', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

export async function applyBatchGeneMatches(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>('/api/batch/gene-matches', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 获取模型列表API
export async function getModels(params?: any, options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[] } }>('/api/system/modelSettings', {
    method: 'GET',
    params: { ...params },
    ...(options || {}),
  });
}

// 应用模型API
export async function applyModel(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>('/api/batch/apply-model', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
  });
}

// 提交批次API
export async function submitBatch(id: string, options?: { [key: string]: any }) {
  const { forceOverwrite, ...restOptions } = options || {};
  return apiRequest<{ success: boolean; message: string }>(`/api/batch/submit/${id}`, {
    method: 'POST',
    params: forceOverwrite ? { force: 1 } : undefined,
    headers: {
      'Content-Type': 'application/json',
    },
    data: forceOverwrite ? { force: true, forceOverwrite: true } : undefined,
    ...restOptions,
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '提交失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message
      };
    },
  });
}

export async function getBatchDuplicateSamples(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: { batchId: number; batchCode: string; duplicateSamples: any[] } }>(`/api/batch/duplicates/${id}`, {
    method: 'GET',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      const success = apiResponse.code === 200 || apiResponse.success === true;
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取重复样本失败');
      }
      return { data: apiResponse.data };
    },
  });
}

export async function createBatchRetestSamples(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: any }>('/api/batch/duplicates/retest', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      const success = apiResponse.code === 200 || apiResponse.success === true;
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '创建复检样本失败');
      }
      return { success, message: apiResponse.message, data: apiResponse.data };
    },
  });
}

// 删除批次API
export async function deleteBatch(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>(`/api/batch/${id}`, {
    method: 'DELETE',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '删除失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message
      };
    },
  });
}

// 重置已提交批次API
export async function resetSubmittedBatch(id: string, options?: { [key: string]: any }) {
  const { force, ...restOptions } = options || {};
  return apiRequest<{ success: boolean; message: string; data?: any; code?: number }>(`/api/batch/submitted/${id}`, {
    method: 'DELETE',
    params: force ? { force: 1 } : undefined,
    ...(restOptions || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      let success = false;
      let message = '';

      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }

      if (apiResponse.code === 409 && apiResponse.data?.requiresConfirmation) {
        return {
          success: false,
          code: 409,
          message: apiResponse.message,
          data: apiResponse.data,
        };
      }

      if (!success) {
        throw new Error(apiResponse.message || '删除失败');
      }

      return {
        success,
        message,
        data: apiResponse.data,
      };
    },
  });
}

// 部分退回已提交批次样本API
export async function partialResetSubmittedBatch(id: string, body: { sampleCodes: string[]; force?: boolean }, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: any; code?: number }>(`/api/batch/submitted/${id}/partial-reset`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      if (apiResponse.code === 409 && apiResponse.data?.requiresConfirmation) {
        return {
          success: false,
          code: 409,
          message: apiResponse.message,
          data: apiResponse.data,
        };
      }
      if (apiResponse.code === 200 || apiResponse.success === true) {
        return {
          success: true,
          message: apiResponse.message,
          data: apiResponse.data,
        };
      }
      throw new Error(apiResponse.message || '退回失败');
    },
  });
}

// 删除批次中的样本API
export async function deleteSampleFromBatch(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: { batchDeleted?: boolean; batchId?: number; sampleCodes?: string[] } }>('/api/batch/deleteSample', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '删除样本失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message,
        data: apiResponse.data,
      };
    },
  });
}

// 多文件上传接口
export async function batchMultiUpload(formData: FormData, onUploadProgress?: (progressEvent: any) => void, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data?: any }>('/api/batch/multi-upload', {
    method: 'POST',
    data: formData,
    headers: {
      'Content-Type': 'multipart/form-data',
    },
    onUploadProgress,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      let data: any = null;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...', data: {} }
        success = true;
        message = apiResponse.message;
        data = apiResponse.data;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '上传失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message,
        data
      };
    },
  });
}

// 获取检测人员列表
export async function getTesters(options?: { [key: string]: any }) {
  return apiRequest<{ data: { list: any[] } }>('/api/testers', {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData: any;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: { list: [] } }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: { list: [] } }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取检测人员列表失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

// 获取多平台批次详情
export async function getBatchMultiDetail(id: string, options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/batch/multi-detail/${id}`, {
    method: 'GET',
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let actualData: any;
      let success = false;
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...', data: {} }
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, data: {} }
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || apiResponse.errorMessage || '获取批次详情失败');
      }
      
      // 返回前端组件期望的格式
      return {
        data: actualData
      };
    },
  });
}

// 合并样本数据
export async function mergeBatchData(body: any, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>('/api/batch/merge-data', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: body,
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '保存数据失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message
      };
    },
  });
}

// 清除缓存API
export async function clearCache(cacheType?: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>('/api/system/cache/clear', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      cacheType: cacheType || 'all',
    },
    ...(options || {}),
    // 自定义响应处理
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      // 兼容两种后端返回格式
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        // 格式: { code: 200, message: '...' }
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        // 格式: { success: true, message: '...' }
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '清除缓存失败');
      }
      
      // 返回前端组件期望的格式
      return {
        success,
        message
      };
    },
  });
}

// 设置单个样本接收时间API
export async function setSampleReceiveDate(sampleCode: string, receiveDate: string, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>('/api/samples/receive-date', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      sample_code: sampleCode,
      receive_date: receiveDate,
    },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '设置接收时间失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

// 批量设置样本接收时间API
export async function batchSetSampleReceiveDate(body: { sampleCodes: string[]; receiveDate: string }, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string }>('/api/samples/batch-receive-date', {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {
      sample_codes: body.sampleCodes,
      receive_date: body.receiveDate,
    },
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '批量设置接收时间失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

// 快递运单相关API
export async function getSampleExpress(sampleId: string, direction = 'inbound', options?: { [key: string]: any }) {
  return apiRequest<{ data: any }>(`/api/express/${sampleId}?direction=${encodeURIComponent(direction)}`, {
    method: 'GET',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let actualData: any;
      let success = false;
      
      if (apiResponse.code === 200) {
        actualData = apiResponse.data;
        success = true;
      } else if (apiResponse.success === true) {
        actualData = apiResponse.data;
        success = true;
      }
      
      if (!success) {
        return { data: null };
      }
      
      return {
        data: actualData
      };
    },
  });
}

export async function saveSampleExpress(sampleId: string, body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase({ ...body, sampleId: Number(sampleId) });
  return apiRequest<{ success: boolean; message: string; data?: any }>(`/api/express/create`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '保存快递运单失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

export async function updateSampleExpress(sampleId: string, body: any, options?: { [key: string]: any }) {
  const snakeCaseBody = toSnakeCase(body);
  return apiRequest<{ success: boolean; message: string }>(`/api/express/${sampleId}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/json',
    },
    data: snakeCaseBody,
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      
      let success = false;
      let message = '';
      
      if (apiResponse.code === 200) {
        success = true;
        message = apiResponse.message;
      } else if (apiResponse.success === true) {
        success = true;
        message = apiResponse.message;
      }
      
      if (!success) {
        throw new Error(apiResponse.message || '更新快递运单失败');
      }
      
      return {
        success,
        message
      };
    },
  });
}

export async function refreshSampleExpress(expressId: string | number, options?: { [key: string]: any }) {
  return apiRequest<{ success: boolean; message: string; data: any }>(`/api/express/${expressId}/query`, {
    method: 'POST',
    ...(options || {}),
    resolveResponse: (response: any) => {
      const apiResponse = response.data;
      if (apiResponse.code !== 200 && apiResponse.success !== true) {
        throw new Error(apiResponse.message || '物流查询失败');
      }
      return {
        success: true,
        message: apiResponse.message,
        data: apiResponse.data,
      };
    },
  });
}
