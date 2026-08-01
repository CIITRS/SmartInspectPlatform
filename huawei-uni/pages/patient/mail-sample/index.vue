<template>
  <view class="page-container">
    <view class="page-header"><text class="page-title">邮寄样本</text><text class="page-desc">填写寄件信息并提交物流单号</text></view>
    <view class="sample-banner"><text class="sample-label">样本管码</text><text class="sample-code">{{ form.sample_code || '请返回重新扫码' }}</text></view>
    <view class="recommend-tip">建议使用京东物流或顺丰速运寄回样本</view>
    <view class="form-card">
      <view class="form-item"><text class="form-label">寄件人姓名</text><input v-model="form.sender_name" placeholder="请输入姓名" class="form-input" /></view>
      <view class="form-item"><text class="form-label">手机号</text><input v-model="form.sender_phone" type="number" maxlength="11" placeholder="顺丰查询需要寄件人手机号" class="form-input" /></view>
      <view class="form-item"><text class="form-label">寄件地址</text><input v-model="form.sender_address" placeholder="请输入寄件地址" class="form-input" /></view>
      <view class="form-item">
        <text class="form-label">快递公司</text>
        <picker :range="expressCompanies" range-key="name" @change="onExpressCompanyChange"><view class="form-input picker-value">{{ selectedExpressCompany.name }}</view></picker>
      </view>
      <view class="form-item">
        <text class="form-label required">快递单号</text>
        <view class="tracking-input-wrap">
          <input v-model="form.tracking_no" placeholder="请输入或扫描快递单号" class="form-input tracking-input" />
          <button class="scan-button" aria-label="扫码录入快递单号" @click="scanTrackingNumber"><view class="scan-icon"><view class="scan-line"></view></view><text>扫码</text></button>
        </view>
      </view>
      <view class="form-item"><text class="form-label">备注（选填）</text><input v-model="form.notes" placeholder="特殊说明" class="form-input" /></view>
    </view>
    <button class="submit-btn" @click="submitForm" :disabled="submitting">{{ submitting ? '提交中...' : '提交邮寄信息' }}</button>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      submitting: false,
      expressCompanyIndex: 0,
      expressCompanies: [{ name: '京东物流', type: 'auto' }, { name: '顺丰速运', type: 'auto' }, { name: '其他（自动识别）', type: 'auto' }],
      form: { sample_code: '', sender_name: '', sender_phone: '', sender_address: '', express_type: 'auto', express_company: '京东物流', tracking_no: '', notes: '' }
    }
  },
  computed: { selectedExpressCompany() { return this.expressCompanies[this.expressCompanyIndex] || this.expressCompanies[0] } },
  onLoad(options) {
    this.form.sample_code = decodeURIComponent(String(options && options.sample_code || '')).trim().toUpperCase()
    if (!this.form.sample_code) uni.showToast({ title: '请先扫码获取样本管码', icon: 'none' })
  },
  methods: {
    onExpressCompanyChange(event) {
      const index = Number(event.detail.value || 0)
      const company = this.expressCompanies[index] || this.expressCompanies[0]
      this.expressCompanyIndex = index
      this.form.express_type = 'auto'
      this.form.express_company = company.name
    },
    scanTrackingNumber() {
      uni.scanCode({
        onlyFromCamera: true,
        scanType: ['barCode', 'qrCode'],
        success: ({ result }) => { this.form.tracking_no = String(result || '').trim() },
        fail: (error) => { if (!String(error && error.errMsg || '').includes('cancel')) uni.showToast({ title: '扫码失败，请重试', icon: 'none' }) }
      })
    },
    validateForm() {
      if (!this.form.sample_code) return '请先扫码获取样本管码'
      if (!String(this.form.sender_name || '').trim()) return '请输入寄件人姓名'
      if (!/^1\d{10}$/.test(String(this.form.sender_phone || '').trim())) return '请输入正确手机号'
      if (!String(this.form.tracking_no || '').trim()) return '快递单号不能为空'
      return ''
    },
    async submitForm() {
      const error = this.validateForm()
      if (error) return uni.showToast({ title: error, icon: 'none' })
      this.submitting = true
      try {
        const res = await uniAPI.createMailSample({ ...this.form, express_type: 'auto' })
        if (!res.success) throw new Error(res.message || '提交失败')
        uni.showToast({ title: '提交成功', icon: 'success' })
        setTimeout(() => uni.navigateBack(), 700)
      } catch (error) {
        uni.showToast({ title: error.message || '提交失败', icon: 'none' })
      } finally { this.submitting = false }
    }
  }
}
</script>

<style scoped>
.page-container { min-height: 100vh; padding: 32rpx 32rpx calc(40rpx + env(safe-area-inset-bottom)); background: #f5f7fa; box-sizing: border-box; }
.page-header { margin-bottom: 24rpx; }.page-title { display: block; color: #1f2d3d; font-size: 38rpx; font-weight: 700; }.page-desc { display: block; margin-top: 8rpx; color: #8c9aa8; font-size: 24rpx; }
.sample-banner { padding: 26rpx; border-radius: 18rpx; background: #eaf3ff; text-align: center; }.sample-label { display: block; color: #6b7c8f; font-size: 23rpx; }.sample-code { display: block; margin-top: 8rpx; color: #1677ff; font-size: 32rpx; font-weight: 700; word-break: break-all; }
.recommend-tip { margin: 18rpx 0 24rpx; padding: 18rpx 22rpx; border-radius: 12rpx; background: #fff7e6; color: #ad6800; font-size: 23rpx; line-height: 1.5; }
.form-card { padding: 10rpx 28rpx; border-radius: 20rpx; background: #fff; box-shadow: 0 2rpx 12rpx rgba(22,119,255,.06); }.form-item { padding: 20rpx 0; border-bottom: 1rpx solid #f0f2f5; }.form-item:last-child { border-bottom: none; }
.form-label { display: block; margin-bottom: 12rpx; color: #4d5b6a; font-size: 24rpx; }.form-label.required::after { content: ' *'; color: #d93025; }
.form-input { width: 100%; height: 84rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; color: #1f2d3d; font-size: 26rpx; }.picker-value { display: flex; align-items: center; }
.tracking-input-wrap { display: flex; align-items: center; gap: 16rpx; }.tracking-input { flex: 1; min-width: 0; }.scan-button { display: flex; flex-direction: column; align-items: center; justify-content: center; flex: 0 0 96rpx; width: 96rpx; height: 84rpx; margin: 0; padding: 0; border: none; border-radius: 12rpx; background: #eaf3ff; color: #1677ff; font-size: 20rpx; line-height: 1.2; }.scan-button::after, .submit-btn::after { border: none; }
.scan-icon { position: relative; width: 30rpx; height: 30rpx; margin-bottom: 5rpx; border: 4rpx solid #1677ff; border-top-color: transparent; border-bottom-color: transparent; }.scan-line { position: absolute; left: 2rpx; right: 2rpx; top: 11rpx; height: 3rpx; background: #1677ff; }
.submit-btn { width: 100%; height: 92rpx; margin-top: 28rpx; border: none; border-radius: 16rpx; background: #1677ff; color: #fff; font-size: 29rpx; font-weight: 700; }.submit-btn[disabled] { opacity: .6; }
</style>
