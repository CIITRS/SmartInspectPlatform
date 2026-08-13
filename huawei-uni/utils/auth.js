export function isValidPhone(phone) {
  return /^1\d{10}$/.test(String(phone || '').trim())
}

export function normalizePhone(phone) {
  return String(phone || '').trim()
}

function normalizeIdentity(raw) {
  if (!raw) {
    return null
  }

  const info = raw.info || raw
  const identityType = raw.identity_type || (raw.patient_id ? 'patient' : 'employee')

  if (identityType === 'patient') {
    const title = info.name || raw.name || info.patient_code || 'Patient'
    const subtitle = info.patient_code || raw.patient_code || info.id_card || 'Patient Identity'

    return {
      identity_type: 'patient',
      title,
      subtitle,
      info
    }
  }

  const roleName = info.role && info.role.name ? info.role.name : ''
  const title = info.real_name || raw.real_name || info.username || raw.username || 'Employee'
  const subtitle = roleName || info.username || 'Employee Identity'

  return {
    identity_type: 'employee',
    title,
    subtitle,
    info
  }
}

export function parseLoginPayload(data) {
  const payload = data || {}

  if (payload.need_register) {
    return {
      sessionId: '',
      needRegister: true,
      phone: payload.phone || '',
      message: payload.message || '首次登录需要填写患者基本信息'
    }
  }

  const rawIdentityList = Array.isArray(payload.identity_list)
    ? payload.identity_list
    : Array.isArray(payload.identities)
      ? payload.identities
      : []

  const identityList = rawIdentityList.map(normalizeIdentity).filter(Boolean)
  const sessionId = payload.session_id || ''

  if (payload.need_select || identityList.length > 1) {
    return {
      sessionId,
      needRegister: false,
      needSelect: true,
      identityList
    }
  }

  if (payload.user_info) {
    return {
      sessionId,
      needRegister: false,
      needSelect: false,
      identity: normalizeIdentity(payload.user_info)
    }
  }

  if (identityList.length === 1) {
    return {
      sessionId,
      needRegister: false,
      needSelect: false,
      identity: identityList[0]
    }
  }

  return {
    sessionId,
    needRegister: false,
    needSelect: false,
    identity: null
  }
}

export function saveLoginState({ phone, sessionId, identity, identityList }) {
  const info = identity && identity.info ? identity.info : {}
  const identityType = identity ? identity.identity_type : ''
  const userInfo = {
    identity: identityType,
    phone,
    sessionId,
    userInfo: info,
    patient: identityType === 'patient' ? info : null,
    employee: identityType === 'employee' ? info : null,
    identityList: Array.isArray(identityList) ? identityList : []
  }

  uni.setStorageSync('userInfo', userInfo)
  uni.setStorageSync('miniapp_session_id', sessionId)
  return userInfo
}

export function navigateToHome(identityType) {
  if (identityType === 'patient' && uni.getStorageSync('pending_sample_entry')) {
    uni.redirectTo({
      url: '/pages/employee/sample-allocate/index?self=1'
    })
    return
  }
  uni.switchTab({
    url: '/pages/home/index'
  })
}

function decodeScene(value) {
  try {
    return decodeURIComponent(String(value || '').replace(/\+/g, '%20'))
  } catch (error) {
    return String(value || '')
  }
}

export function captureSampleEntryLaunch(options = {}) {
  const query = options.query || options
  const q = decodeScene(query.q)
  const scene = decodeScene(query.scene || q)
  const action = String(query.action || query.type || '').toLowerCase()
  const isEntry = query.sample_entry === '1' || action === 'sample-entry' || /sample[-_ ]?entry|样本录入/i.test(`${scene} ${q}`)
  if (!isEntry) return false

  uni.setStorageSync('pending_sample_entry', '1')
  const match = scene.match(/(?:sample_code|tube_code|tube)=([^&]+)/i)
  const sampleCode = String(query.sample_code || query.tube_code || (match && match[1]) || '').trim()
  if (sampleCode) uni.setStorageSync('pending_sample_code', sampleCode)
  return true
}

export function routePendingSampleEntry() {
  if (!uni.getStorageSync('pending_sample_entry')) return false
  const pages = getCurrentPages()
  const route = pages.length ? pages[pages.length - 1].route : ''
  if (/pages\/(login|patient\/profile|employee\/sample-allocate)/.test(route)) return true

  const state = uni.getStorageSync('userInfo') || {}
  if (!state.identity || !uni.getStorageSync('miniapp_session_id')) {
    uni.redirectTo({ url: '/pages/login/index' })
    return true
  }
  if (state.identity !== 'patient') {
    uni.showToast({ title: '请切换患者身份后录入样本', icon: 'none' })
    return true
  }
  uni.redirectTo({ url: '/pages/employee/sample-allocate/index?self=1' })
  return true
}

export function refreshTabBarFromStorage() {
  // 这个函数的主要作用是根据当前存储的用户身份状态来刷新页面
  // 现在由各页面自己处理身份检查，这里暂时保持空实现
  // 将来可以扩展为处理 tabBar 相关的逻辑
}
