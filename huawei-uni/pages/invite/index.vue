<template>
  <view class="page">
    <view class="form-card">
      <view class="form-item">
        <text class="label">姓名</text>
        <input v-model="form.name" class="input" placeholder="请输入姓名" />
      </view>
      <view class="form-item">
        <text class="label">身份证件类型</text>
        <picker :range="documentTypeOptions" @change="onDocumentTypeChange">
          <view class="input picker-text">{{ form.id_document_type || '请选择身份证件类型' }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">身份证件号</text>
        <input v-model="form.id_document_no" class="input" :maxlength="form.id_document_type === '居民身份证' ? 18 : 50" placeholder="请输入身份证件号" @blur="checkIdCard" />
      </view>
      <view class="form-item">
        <text class="label">联系电话</text>
        <input v-model="form.phone" class="input" type="number" maxlength="11" placeholder="请输入联系电话" />
      </view>
      <view class="form-item">
        <text class="label">验证码</text>
        <view class="code-row">
          <input v-model="form.sms_code" class="input code-input" type="number" maxlength="6" placeholder="请输入短信验证码" />
          <button class="code-btn" :disabled="codeSending || countdown > 0" @click="sendCode">
            {{ countdown > 0 ? countdown + 's' : '获取验证码' }}
          </button>
        </view>
      </view>
      <view class="form-item">
        <text class="label">性别</text>
        <picker :range="genderOptions" @change="onGenderChange">
          <view class="input picker-text">{{ form.gender || '请选择性别' }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">出生日期</text>
        <picker mode="date" :value="form.birthday" @change="onBirthdayChange">
          <view class="input picker-text">{{ form.birthday || '请选择出生日期' }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">地址</text>
        <input v-model="form.address" class="input" placeholder="请输入地址" />
      </view>
      <view class="form-item">
        <text class="label">患者状态</text>
        <view class="segmented">
          <view class="seg-item" :class="{ active: form.patient_status === 1 }" @click="setPatientStatus(1)">患病</view>
          <view class="seg-item" :class="{ active: form.patient_status === 0 }" @click="setPatientStatus(0)">健康</view>
        </view>
      </view>
      <view v-if="form.patient_status === 1" class="form-item">
        <text class="label">诊断</text>
        <textarea v-model="form.diagnosis" class="textarea" placeholder="请输入诊断信息（必填）" />
      </view>
      <view v-if="form.patient_status === 1" class="form-item">
        <text class="label">肿瘤直径</text>
        <input v-model="form.cancer_diameter" class="input" placeholder="请输入肿瘤直径，单位：cm（必填）" />
      </view>
      <view class="form-item">
        <text class="label">吸烟状况</text>
        <picker :range="smokingStatusOptions" @change="onSmokingStatusChange">
          <view class="input picker-text">{{ form.smoking_status || '请选择吸烟状态（选填）' }}</view>
        </picker>
      </view>
      <view class="manager-line">专属客户经理：{{ managerName || '加载中' }}</view>
    </view>

    <button class="submit-btn" @click="submit" :disabled="submitting">
      {{ submitting ? '提交中...' : '提交并绑定客户经理' }}
    </button>
  </view>
</template>

<script>
import { authAPI } from '../../api/index.js'
import { parseLoginPayload, saveLoginState, navigateToHome } from '../../utils/auth.js'

export default {
  data() {
    return {
      salesId: 0,
      managerName: '',
      submitting: false,
      codeSending: false,
      countdown: 0,
      timer: null,
      idCardExists: false,
      idCardBindable: false,
      bindExistingConfirmed: false,
      genderOptions: ['男', '女'],
      documentTypeOptions: ['居民身份证', '护照', '港澳居民来往内地通行证', '台湾居民来往大陆通行证', '自编号'],
      smokingStatusOptions: ['不吸烟', '10支以内/日', '10-20支/日', '20支以上/日'],
      form: {
        name: '',
        id_document_type: '居民身份证',
        id_document_no: '',
        id_card: '',
        phone: '',
        sms_code: '',
        gender: '男',
        birthday: '',
        address: '',
        diagnosis: '',
        cancer_diameter: '',
        smoking_status: '',
        patient_status: null
      }
    }
  },
  onLoad(options) {
    const scene = decodeURIComponent(options.scene || '')
    const sceneSalesId = scene.split('&').map(item => item.split('=')).find(([key]) => key === 'sales_id')
    this.salesId = Number(options.sales_id || (sceneSalesId && sceneSalesId[1]) || 0)
    this.loadManager()
  },
  onUnload() {
    if (this.timer) clearInterval(this.timer)
  },
  methods: {
    setPatientStatus(status) {
      this.form.patient_status = status
      if (status === 0) {
        this.form.diagnosis = ''
        this.form.cancer_diameter = ''
      }
    },
    onGenderChange(e) {
      this.form.gender = this.genderOptions[Number(e.detail.value)]
    },
    onBirthdayChange(e) {
      this.form.birthday = e.detail.value
    },
    onDocumentTypeChange(e) {
      this.form.id_document_type = this.documentTypeOptions[Number(e.detail.value)]
      this.idCardExists = false
      this.idCardBindable = false
      this.bindExistingConfirmed = false
      if (this.form.id_document_type === '自编号') {
        uni.showModal({
          title: '提示',
          content: '此选项仅用于存量用户，请及时提醒患者完善身份信息和手机号。',
          showCancel: false
        })
      }
    },
    async checkIdCard() {
      const idCard = String(this.form.id_document_no || '').trim()
      this.idCardExists = false
      this.idCardBindable = false
      this.bindExistingConfirmed = false
      if (!idCard) return
      if (this.form.id_document_type === '居民身份证' && !/^\d{17}[\dXx]$/.test(idCard)) {
        uni.showToast({ title: '居民身份证号格式错误', icon: 'none' })
        return
      }
      try {
        const res = await authAPI.checkIdCard(idCard, this.form.id_document_type)
        if (res.success && res.data) {
          if (res.data.gender) this.form.gender = res.data.gender
          if (res.data.birthday) this.form.birthday = res.data.birthday
          if (res.data.exists) {
            this.idCardExists = true
            this.idCardBindable = !!res.data.bindable
            if (this.idCardBindable) {
              const patientName = res.data.patient && res.data.patient.name ? res.data.patient.name : '该患者'
              const confirmed = await new Promise((resolve) => {
                uni.showModal({
                  title: '患者已有信息',
                  content: `${patientName}已有建档信息，将使用当前手机号进行绑定。`,
                  confirmText: '确定绑定',
                  success: (modalRes) => resolve(!!modalRes.confirm),
                  fail: () => resolve(false)
                })
              })
              this.bindExistingConfirmed = !!confirmed
            } else {
              uni.showToast({ title: '该身份证件号已存在', icon: 'none' })
            }
          }
        }
      } catch (error) {
        uni.showToast({ title: '身份证件号校验失败', icon: 'none' })
      }
    },
    onSmokingStatusChange(e) {
      this.form.smoking_status = this.smokingStatusOptions[Number(e.detail.value)] || ''
    },
    async loadManager() {
      if (!this.salesId) {
        uni.showToast({ title: '邀请参数无效', icon: 'none' })
        return
      }
      try {
        const res = await authAPI.getInviteManager(this.salesId)
        if (res.success && res.data) {
          this.managerName = res.data.name || res.data.real_name || ''
        }
      } catch (error) {
        uni.showToast({ title: '客户经理信息加载失败', icon: 'none' })
      }
    },
    validatePhone() {
      if (!/^1\d{10}$/.test(String(this.form.phone || '').trim())) {
        uni.showToast({ title: '请输入正确手机号', icon: 'none' })
        return false
      }
      return true
    },
    async sendCode() {
      if (!this.validatePhone()) return
      this.codeSending = true
      try {
        const res = await authAPI.smsSend({
          phone: this.form.phone,
          purpose: 'invite_register',
          client: 'invite'
        })
        if (!res.success) {
          uni.showToast({ title: res.message || '发送失败', icon: 'none' })
          return
        }
        this.countdown = 60
        this.timer = setInterval(() => {
          this.countdown -= 1
          if (this.countdown <= 0 && this.timer) {
            clearInterval(this.timer)
            this.timer = null
          }
        }, 1000)
      } catch (error) {
        uni.showToast({ title: '验证码发送失败', icon: 'none' })
      } finally {
        this.codeSending = false
      }
    },
    validate() {
      if (!this.salesId) {
        uni.showToast({ title: '邀请参数无效', icon: 'none' })
        return false
      }
      if (!String(this.form.name || '').trim()) {
        uni.showToast({ title: '请输入姓名', icon: 'none' })
        return false
      }
      const documentNo = String(this.form.id_document_no || '').trim()
      if (!this.form.id_document_type || !documentNo) {
        uni.showToast({ title: '请选择身份证件类型并填写身份证件号', icon: 'none' })
        return false
      }
      if (this.form.id_document_type === '居民身份证' && !/^\d{17}[\dXx]$/.test(documentNo)) {
        uni.showToast({ title: '请输入18位居民身份证号', icon: 'none' })
        return false
      }
      if (this.idCardExists && (!this.idCardBindable || !this.bindExistingConfirmed)) {
        uni.showToast({ title: '该身份证件号已存在', icon: 'none' })
        return false
      }
      if (!this.validatePhone()) return false
      if (!String(this.form.sms_code || '').trim()) {
        uni.showToast({ title: '请输入验证码', icon: 'none' })
        return false
      }
      if (!this.form.gender || !this.form.birthday) {
        uni.showToast({ title: '请完善性别和生日', icon: 'none' })
        return false
      }
      if (this.form.patient_status !== 0 && this.form.patient_status !== 1) {
        uni.showToast({ title: '请选择患者状态', icon: 'none' })
        return false
      }
      if (this.form.patient_status === 1 && !String(this.form.diagnosis || '').trim()) {
        uni.showToast({ title: '请填写诊断', icon: 'none' })
        return false
      }
      if (this.form.patient_status === 1 && !String(this.form.cancer_diameter || '').trim()) {
        uni.showToast({ title: '请填写肿瘤直径', icon: 'none' })
        return false
      }
      this.form.id_card = this.form.id_document_type === '居民身份证' ? documentNo : ''
      return true
    },
    async submit() {
      await this.checkIdCard()
      if (!this.validate()) return
      this.submitting = true
      try {
        const subscribeAccepted = await this.requestReportSubscribe()
        const jsCode = await this.getWechatJsCode()
        const res = await authAPI.inviteRegister({
          sales_id: this.salesId,
          ...this.form,
          jsCode,
          report_subscribe_accepted: subscribeAccepted
        })
        if (!res.success) {
          uni.showToast({ title: res.message || '提交失败', icon: 'none' })
          return
        }
        const result = parseLoginPayload(res.data)
        if (result.identity) {
          saveLoginState({ sessionId: result.sessionId, identity: result.identity, phone: result.identity.info.phone || '' })
        }
        uni.showToast({ title: '提交成功', icon: 'success' })
        setTimeout(() => navigateToHome('patient'), 800)
      } catch (error) {
        uni.showToast({ title: '网络错误，请稍后重试', icon: 'none' })
      } finally {
        this.submitting = false
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
.page { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.form-card { background: #fff; border-radius: 20rpx; padding: 16rpx 28rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.form-item { padding: 20rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.label { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 12rpx; }
.input { width: 100%; min-height: 72rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; color: #1f2d3d; }
.textarea { width: 100%; height: 150rpx; padding: 16rpx 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; }
.picker-text { display: flex; align-items: center; color: #1f2d3d; }
.segmented { display: flex; gap: 16rpx; }
.seg-item { flex: 1; height: 72rpx; line-height: 72rpx; text-align: center; border-radius: 12rpx; background: #f3f6fa; color: #606f7b; font-size: 26rpx; }
.seg-item.active { background: #1677ff; color: #fff; }
.code-row { display: flex; gap: 16rpx; align-items: center; }
.code-input { flex: 1; }
.code-btn { width: 210rpx; height: 72rpx; line-height: 72rpx; border-radius: 12rpx; background: #1677ff; color: #fff; font-size: 24rpx; padding: 0; border: none; }
.code-btn[disabled] { opacity: 0.55; }
.manager-line { padding: 26rpx 0 12rpx; font-size: 26rpx; color: #1677ff; font-weight: 600; }
.submit-btn { margin-top: 40rpx; width: 100%; height: 96rpx; border-radius: 16rpx; background: #1677ff; color: #fff; font-size: 28rpx; font-weight: 500; border: none; }
.submit-btn[disabled] { opacity: 0.6; }
</style>
