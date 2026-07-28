<template>
  <view class="login-page">
    <view class="login-card">
      <image src="../../static/logo.svg" class="logo"></image>
      <text class="title">华微智检</text>
      <text class="subtitle">请选择登录方式</text>

      <view class="login-methods">
        <!-- #ifdef MP-WEIXIN -->
        <button v-if="!agreementChecked" class="method-btn primary" @click="openOneClickConsent">
          一键登录
        </button>
        <button v-else class="method-btn primary" open-type="getPhoneNumber" @getphonenumber="onGetPhoneNumber">
          一键登录
        </button>
        <!-- #endif -->
        <button class="method-btn outline" @click="goToSmsLogin">
          验证码登录
        </button>
      </view>
    </view>

    <view class="agreement-footer" @click="toggleAgreement">
      <view class="agreement-check" :class="{ checked: agreementChecked }">
        <text v-if="agreementChecked" class="agreement-checkmark">✓</text>
      </view>
      <text class="agreement-muted">已阅读并同意</text>
      <text class="agreement-link" @click.stop="goToPrivacy">隐私政策</text>
      <text class="agreement-muted">和</text>
      <text class="agreement-link" @click.stop="goToService">用户协议</text>
    </view>

    <view v-if="showAgreementModal" class="identity-mask" @click="closeAgreementModal">
      <view class="identity-popup agreement-popup" @click.stop>
        <view class="popup-header">
          <text class="popup-title">阅读并同意协议</text>
          <view class="popup-close" @click="closeAgreementModal">
            <text class="close-icon">×</text>
          </view>
        </view>
        <view class="popup-body">
          <text class="agreement-text">请先阅读并同意《隐私政策》和《用户协议》后继续使用。</text>
          <view class="agreement-actions">
            <button class="method-btn outline" @click="closeAgreementModal">暂不同意</button>
            <!-- #ifdef MP-WEIXIN -->
            <button
              v-if="pendingAgreementAction === 'oneClick'"
              class="method-btn primary"
              open-type="getPhoneNumber"
              @getphonenumber="agreeAndGetPhoneNumber"
            >
              同意并继续
            </button>
            <!-- #endif -->
            <button v-if="pendingAgreementAction !== 'oneClick'" class="method-btn primary" @click="acceptAgreement">
              同意并继续
            </button>
          </view>
        </view>
      </view>
    </view>

    <!-- 首次登录患者建档弹窗 -->
    <view v-if="showRegisterModal" class="identity-mask">
      <view class="identity-popup register-popup" @click.stop>
        <view class="popup-header">
          <text class="popup-title">首次登录需填写患者基本信息</text>
        </view>
        <view class="popup-body">
          <view class="form-item">
            <text class="label">姓名</text>
            <input v-model="registerForm.name" class="input" placeholder="请输入姓名" />
          </view>
          <view class="form-item">
            <text class="label">身份证号</text>
            <input v-model="registerForm.id_card" class="input" maxlength="18" placeholder="请输入身份证号" />
          </view>
          <view class="form-item">
            <text class="label">电话</text>
            <input v-model="registerForm.phone" class="input" type="number" maxlength="11" placeholder="请输入电话" />
          </view>
          <button class="method-btn primary" :disabled="registering" @click="submitRegister">
            {{ registering ? '提交中...' : '提交并登录' }}
          </button>
        </view>
      </view>
    </view>

    <!-- 身份选择弹窗 -->
    <view v-if="showIdentitySelect" class="identity-mask" @click="closeIdentitySelect">
      <view class="identity-popup" @click.stop>
        <view class="popup-header">
          <text class="popup-title">请选择登录身份</text>
          <view class="popup-close" @click="closeIdentitySelect">
            <text class="close-icon">×</text>
          </view>
        </view>
        <view class="popup-body">
          <view
            v-for="(identity, index) in identityList"
            :key="identity.identity_type + '-' + index"
            class="identity-item"
            @click="selectIdentity(identity)"
          >
            <view class="identity-info">
              <text class="identity-name">{{ identity.title }}</text>
              <text class="identity-desc">{{ identity.subtitle }}</text>
            </view>
            <text class="identity-arrow">›</text>
          </view>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { authAPI } from '../../api/index.js'
import {
  isValidPhone,
  normalizePhone,
  parseLoginPayload,
  saveLoginState,
  navigateToHome
} from '../../utils/auth.js'

export default {
  data() {
    return {
      showIdentitySelect: false,
      showRegisterModal: false,
      showAgreementModal: false,
      pendingAgreementAction: '',
      agreementChecked: false,
      registering: false,
      identityList: [],
      currentSessionId: '',
      registerForm: {
        name: '',
        id_card: '',
        phone: ''
      }
    }
  },
  methods: {
    toggleAgreement() {
      this.agreementChecked = !this.agreementChecked
    },
    confirmAgreement(action) {
      if (this.agreementChecked) {
        this.runAgreementAction(action)
        return
      }
      this.pendingAgreementAction = action
      this.showAgreementModal = true
    },
    closeAgreementModal() {
      this.showAgreementModal = false
      this.pendingAgreementAction = ''
    },
    acceptAgreement() {
      const action = this.pendingAgreementAction
      this.agreementChecked = true
      this.closeAgreementModal()
      this.runAgreementAction(action)
    },
    agreeAndGetPhoneNumber(e) {
      this.agreementChecked = true
      this.closeAgreementModal()
      this.onGetPhoneNumber(e)
    },
    runAgreementAction(action) {
      if (action === 'sms') {
        uni.navigateTo({
          url: `/pages/login/sms?agreed=${this.agreementChecked ? '1' : '0'}`
        })
      }
    },
    openOneClickConsent() {
      this.confirmAgreement('oneClick')
    },
    goToSmsLogin() {
      this.confirmAgreement('sms')
    },
    goToPrivacy() {
      uni.navigateTo({ url: '/pages/about/privacy' })
    },
    goToService() {
      uni.navigateTo({ url: '/pages/about/service' })
    },
    // 微信一键登录 - 获取手机号
    async onGetPhoneNumber(e) {
      if (!this.agreementChecked) {
        uni.showToast({
          title: '请先阅读并同意协议',
          icon: 'none'
        })
        return
      }
      if (e.detail.errMsg !== 'getPhoneNumber:ok') {
        uni.showToast({
          title: '取消了授权',
          icon: 'none'
        })
        return
      }

      uni.showLoading({
        title: '登录中...'
      })

      try {
        // 检测是否为开发者工具环境或手机号凭证不可用
        const isDevtoolsOrNoEncrypt = 
          !e.detail.code || 
          e.detail.code === 'the code is a mock one' ||
          e.detail.code.startsWith('mock_')

        let loginData

        if (isDevtoolsOrNoEncrypt) {
          // 开发者工具环境：使用模拟手机号
          loginData = {
            phone: '13800138000',
            autoLogin: false
          }
        } else {
          // 真实微信环境：优先使用新版手机号code，保留加密数据作为后端兼容兜底
          loginData = {
            phoneCode: e.detail.code,
            encryptedData: e.detail.encryptedData,
            iv: e.detail.iv
          }

          if (e.detail.encryptedData && e.detail.iv) {
            try {
              const loginRes = await new Promise((resolve, reject) => {
                uni.login({
                  provider: 'weixin',
                  success: resolve,
                  fail: reject
                })
              })
              if (loginRes && loginRes.code) {
                loginData.jsCode = loginRes.code
              }
            } catch (loginError) {
              console.warn('Get Wechat jsCode failed:', loginError)
            }
          }
        }

        const response = await authAPI.oneClickLogin(loginData)
        uni.hideLoading()

        if (!response.success) {
          uni.showToast({
            title: response.message || '登录失败',
            icon: 'none'
          })
          return
        }

        this.handleLoginResponse(response.data)
      } catch (error) {
        uni.hideLoading()
        uni.showToast({
          title: '网络错误，请稍后重试',
          icon: 'none'
        })
        console.error('One click login failed:', error)
      }
    },
    handleLoginResponse(data) {
      const result = parseLoginPayload(data)

      if (result.needRegister) {
        this.registerForm.phone = normalizePhone(result.phone || this.registerForm.phone)
        this.showRegisterModal = true
        uni.showToast({
          title: result.message || '首次登录需要填写患者基本信息',
          icon: 'none'
        })
        return
      }

      this.currentSessionId = result.sessionId || ''

      if (result.needSelect) {
        this.identityList = result.identityList
        if (this.currentSessionId) {
          uni.setStorageSync('miniapp_session_id', this.currentSessionId)
        }
        this.showIdentitySelect = true
        return
      }

      if (!result.identity) {
        uni.showToast({
          title: '未获取到登录身份',
          icon: 'none'
        })
        return
      }

      this.finishLogin(result.identity)
    },
    closeIdentitySelect() {
      this.showIdentitySelect = false
    },
    async selectIdentity(identity) {
      try {
        const response = await authAPI.switchIdentity({
          identity_type: identity.identity_type,
          user_id: identity.info?.user_id || identity.user_id || 0,
          patient_id: identity.info?.patient_id || identity.patient_id || 0
        })
        this.showIdentitySelect = false
        if (response.success && response.data) {
          const result = parseLoginPayload({
            session_id: response.data.session_id || this.currentSessionId,
            user_info: response.data.user_info
          })
          this.finishLogin(result.identity || identity, response.data.identity_list || this.identityList)
          return
        }
      } catch (error) {
        uni.showToast({ title: '切换身份失败', icon: 'none' })
        return
      }
      this.showIdentitySelect = false
      this.finishLogin(identity, this.identityList)
    },
    finishLogin(identity, identityList = this.identityList) {
      saveLoginState({
        phone: this.registerForm.phone || identity.info?.phone || '',
        sessionId: this.currentSessionId,
        identity,
        identityList
      })
      // 先更新 tabbar 的身份类型
      navigateToHome(identity.identity_type)
    },
    async submitRegister() {
      const name = String(this.registerForm.name || '').trim()
      const idCard = String(this.registerForm.id_card || '').trim()
      const phone = normalizePhone(this.registerForm.phone)

      if (!name) {
        uni.showToast({ title: '请输入姓名', icon: 'none' })
        return
      }
      if (!idCard || idCard.length !== 18) {
        uni.showToast({ title: '请输入18位身份证号', icon: 'none' })
        return
      }
      if (!isValidPhone(phone)) {
        uni.showToast({ title: '请输入正确的电话', icon: 'none' })
        return
      }

      this.registering = true
      try {
        const subscribeAccepted = await this.requestReportSubscribe()
        const jsCode = await this.getWechatJsCode()
        const response = await authAPI.registerPatientFirstLogin({
          name,
          id_card: idCard,
          phone,
          jsCode,
          report_subscribe_accepted: subscribeAccepted
        })
        if (!response.success) {
          uni.showToast({ title: response.message || '建档失败', icon: 'none' })
          return
        }
        this.showRegisterModal = false
        this.handleLoginResponse(response.data)
      } catch (error) {
        uni.showToast({ title: '网络错误，请稍后重试', icon: 'none' })
      } finally {
        this.registering = false
      }
    },
    async requestReportSubscribe() {
      if (!uni.requestSubscribeMessage) return false
      try {
        const result = await new Promise((resolve, reject) => {
          uni.requestSubscribeMessage({
            tmplIds: ['etRGY-LJcMas11zwBIpayTEF1THGdUG_sAoNb2XQoro'],
            success: resolve,
            fail: reject
          })
        })
        return result && result['etRGY-LJcMas11zwBIpayTEF1THGdUG_sAoNb2XQoro'] === 'accept'
      } catch (error) {
        return false
      }
    },
    async getWechatJsCode() {
      try {
        const loginRes = await new Promise((resolve, reject) => {
          uni.login({ provider: 'weixin', success: resolve, fail: reject })
        })
        return loginRes && loginRes.code ? loginRes.code : ''
      } catch (error) {
        return ''
      }
    }
  }
}
</script>

<style scoped>
.login-page {
  min-height: 100vh;
  padding: 40rpx;
  background: linear-gradient(135deg, #f5f7fa 0%, #eaf3ff 100%);
  box-sizing: border-box;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.login-card {
  width: 100%;
  max-width: 520rpx;
  margin: 0 auto;
  background: #fff;
  border-radius: 24rpx;
  padding: 64rpx 32rpx 48rpx;
  box-shadow: 0 8rpx 24rpx rgba(22, 119, 255, 0.12);
  text-align: center;
  box-sizing: border-box;
}

.agreement-footer {
  margin-top: 28rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-wrap: wrap;
  gap: 8rpx;
}

.agreement-check {
  width: 28rpx;
  height: 28rpx;
  border: 2rpx solid #c0c6cc;
  border-radius: 50%;
  box-sizing: border-box;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #fff;
}

.agreement-check.checked {
  border-color: #1677ff;
  background: #1677ff;
}

.agreement-checkmark {
  font-size: 20rpx;
  line-height: 1;
  color: #fff;
}

.agreement-muted {
  font-size: 22rpx;
  color: #8c9aa8;
}

.agreement-link {
  font-size: 22rpx;
  color: #1677ff;
}

.agreement-text {
  display: block;
  margin-bottom: 28rpx;
  font-size: 26rpx;
  line-height: 1.7;
  color: #1f2d3d;
}

.agreement-actions {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 20rpx;
}

.logo {
  width: 120rpx;
  height: 120rpx;
  margin-bottom: 24rpx;
}

.title {
  display: block;
  font-size: 36rpx;
  font-weight: 600;
  color: #1f2d3d;
}

.subtitle {
  display: block;
  margin: 12rpx 0 48rpx;
  font-size: 22rpx;
  color: #8c9aa8;
}

.login-methods {
  display: flex;
  flex-direction: column;
  gap: 20rpx;
}

.method-btn {
  width: 100%;
  height: 96rpx;
  border-radius: 16rpx;
  font-size: 28rpx;
  font-weight: 500;
  line-height: normal;
  text-align: center;
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 0;
  margin: 0;
}

.method-btn.primary {
  background: linear-gradient(135deg, #1677ff 0%, #409eff 100%);
  color: #fff;
  border: none;
  box-shadow: 0 4rpx 12rpx rgba(22, 119, 255, 0.3);
}

.method-btn.outline {
  background: #fff;
  color: #1677ff;
  border: 2rpx solid #1677ff;
}

/* 身份选择弹窗 */
.identity-mask {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  z-index: 999;
  display: flex;
  align-items: flex-end;
  animation: fadeIn 0.2s ease;
}

@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}

.identity-popup {
  width: 100%;
  background: #fff;
  border-top-left-radius: 32rpx;
  border-top-right-radius: 32rpx;
  padding: 0 32rpx 48rpx;
  box-sizing: border-box;
  animation: slideUp 0.3s ease;
}

@keyframes slideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.popup-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 32rpx 0 24rpx;
  border-bottom: 1rpx solid #f0f2f5;
}

.popup-title {
  font-size: 30rpx;
  font-weight: 600;
  color: #1f2d3d;
}

.popup-close {
  width: 48rpx;
  height: 48rpx;
  display: flex;
  align-items: center;
  justify-content: center;
}

.close-icon {
  font-size: 36rpx;
  color: #8c9aa8;
}

.popup-body {
  padding-top: 24rpx;
}


.register-popup {
  padding-bottom: 40rpx;
}

.form-item {
  margin-bottom: 24rpx;
}

.label {
  display: block;
  margin-bottom: 12rpx;
  font-size: 24rpx;
  color: #1f2d3d;
  font-weight: 500;
}

.input {
  width: 100%;
  height: 84rpx;
  padding: 0 24rpx;
  border: 2rpx solid #e5e7eb;
  border-radius: 16rpx;
  background: #f9fafb;
  box-sizing: border-box;
  font-size: 24rpx;
}

.identity-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 28rpx 24rpx;
  border: 2rpx solid #e5e7eb;
  border-radius: 16rpx;
  background: #fff;
  transition: all 0.2s ease;
}

.identity-item + .identity-item {
  margin-top: 16rpx;
}

.identity-item:active {
  background: #f0f7ff;
  border-color: #1677ff;
}

.identity-info {
  flex: 1;
}

.identity-name {
  display: block;
  font-size: 28rpx;
  color: #1f2d3d;
  font-weight: 500;
}

.identity-desc {
  display: block;
  margin-top: 8rpx;
  font-size: 22rpx;
  color: #8c9aa8;
}

.identity-arrow {
  font-size: 32rpx;
  color: #c0c6cc;
  margin-left: 16rpx;
}
</style>
