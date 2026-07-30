<template>
  <view class="page">
    <view class="header">
      <text class="title">新患录入</text>
      <text class="desc">录入后会自动归属到当前员工</text>
    </view>

    <view class="form-card">
      <view class="form-item">
        <text class="label">姓名</text>
        <input v-model="form.name" class="input" placeholder="请输入姓名" />
      </view>
      <view class="form-item">
        <text class="label">性别</text>
        <view class="segmented">
          <view class="seg-item" :class="{ active: form.gender === '男' }" @click="form.gender = '男'">男</view>
          <view class="seg-item" :class="{ active: form.gender === '女' }" @click="form.gender = '女'">女</view>
        </view>
      </view>
      <view class="form-item">
        <text class="label">手机号</text>
        <input v-model="form.phone" type="number" maxlength="11" class="input" placeholder="请输入手机号（选填）" />
      </view>
      <view class="form-item">
        <text class="label">身份证件类型</text>
        <picker :range="documentTypeOptions" @change="onDocumentTypeChange">
          <view class="date-input">{{ form.id_document_type || '请选择身份证件类型' }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">身份证件号</text>
        <input v-model="form.id_document_no" :maxlength="form.id_document_type === '居民身份证' ? 18 : 50" class="input" placeholder="请输入身份证件号" @blur="checkIdCard" />
      </view>
      <view class="form-item">
        <text class="label">生日</text>
        <picker mode="date" :value="form.birthday" @change="onBirthdayChange">
          <view class="date-input">{{ form.birthday || '请选择生日' }}</view>
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
          <view class="date-input" :class="{ placeholder: !form.smoking_status }">{{ form.smoking_status || '请选择吸烟状态（选填）' }}</view>
        </picker>
      </view>
    </view>

    <button class="submit-btn" :disabled="submitting" @click="submit">
      {{ submitting ? '保存中...' : '保存患者' }}
    </button>
  </view>
</template>

<script>
import { authAPI, uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      submitting: false,
      idCardExists: false,
      documentTypeOptions: ['居民身份证', '护照', '港澳居民来往内地通行证', '台湾居民来往大陆通行证', '自编号'],
      smokingStatusOptions: ['不吸烟', '10支以内/日', '10-20支/日', '20支以上/日'],
      form: {
        name: '',
        gender: '男',
        phone: '',
        id_document_type: '居民身份证',
        id_document_no: '',
        id_card: '',
        birthday: '',
        address: '',
        diagnosis: '',
        cancer_diameter: '',
        smoking_status: '',
        patient_status: null
      }
    }
  },
  methods: {
    setPatientStatus(status) {
      this.form.patient_status = status
      if (status === 0) {
        this.form.diagnosis = ''
        this.form.cancer_diameter = ''
      }
    },
    onBirthdayChange(e) {
      this.form.birthday = e.detail.value
    },
    onSmokingStatusChange(e) {
      this.form.smoking_status = this.smokingStatusOptions[Number(e.detail.value)] || ''
    },
    onDocumentTypeChange(e) {
      this.form.id_document_type = this.documentTypeOptions[Number(e.detail.value)]
      this.idCardExists = false
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
            uni.showToast({ title: '该身份证件号已存在', icon: 'none' })
          }
        }
      } catch (error) {
        uni.showToast({ title: '身份证件号校验失败', icon: 'none' })
      }
    },
    async submit() {
      if (!this.form.name) { uni.showToast({ title: '请输入姓名', icon: 'none' }); return }
      if (this.form.phone && !/^1\d{10}$/.test(this.form.phone)) { uni.showToast({ title: '请输入正确手机号', icon: 'none' }); return }
      const documentNo = String(this.form.id_document_no || '').trim()
      if (!this.form.id_document_type || !documentNo) {
        uni.showToast({ title: '请选择身份证件类型并填写身份证件号', icon: 'none' })
        return
      }
      if (this.form.id_document_type === '居民身份证' && !/^\d{17}[\dXx]$/.test(documentNo)) {
        uni.showToast({ title: '居民身份证号格式错误', icon: 'none' })
        return
      }
      this.form.id_card = this.form.id_document_type === '居民身份证' ? documentNo : ''
      await this.checkIdCard()
      if (this.idCardExists) {
        uni.showToast({ title: '该身份证件号已存在', icon: 'none' })
        return
      }
      if (!this.form.birthday) { uni.showToast({ title: '请选择生日', icon: 'none' }); return }
      if (this.form.patient_status !== 0 && this.form.patient_status !== 1) {
        uni.showToast({ title: '请选择患者状态', icon: 'none' })
        return
      }
      if (this.form.patient_status === 1 && !String(this.form.diagnosis || '').trim()) {
        uni.showToast({ title: '请填写诊断', icon: 'none' })
        return
      }
      if (this.form.patient_status === 1 && !String(this.form.cancer_diameter || '').trim()) {
        uni.showToast({ title: '请填写肿瘤直径', icon: 'none' })
        return
      }

      this.submitting = true
      try {
        const res = await uniAPI.createEmployeePatient(this.form)
        if (res.success) {
          const patientId = res.data && res.data.id
          uni.showModal({
            title: '患者已绑定客户经理',
            content: '是否立即为该患者新增样本？新增时可选择单检或套餐，并完成知情同意签名。',
            confirmText: '新增样本',
            cancelText: '稍后处理',
            success: (modal) => {
              if (modal.confirm && patientId) {
                uni.redirectTo({ url: `/pages/employee/sample-allocate/index?patient_ids=${patientId}` })
              } else {
                uni.navigateBack()
              }
            }
          })
        } else {
          uni.showToast({ title: res.message || '保存失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: error.message || '网络错误', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.header { margin-bottom: 24rpx; }
.title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.form-card { background: #fff; border-radius: 20rpx; padding: 12rpx 28rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.form-item { padding: 20rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.form-item:last-child { border-bottom: none; }
.label { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 12rpx; }
.input, .date-input { width: 100%; min-height: 72rpx; line-height: 72rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; color: #1f2d3d; }
.placeholder { color: #9ca3af; }
.textarea { width: 100%; height: 148rpx; padding: 18rpx 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; color: #1f2d3d; }
.segmented { display: flex; gap: 16rpx; }
.seg-item { flex: 1; height: 72rpx; line-height: 72rpx; text-align: center; border-radius: 12rpx; background: #f3f6fa; color: #606f7b; font-size: 26rpx; }
.seg-item.active { background: #1677ff; color: #fff; }
.submit-btn { margin-top: 28rpx; width: 100%; height: 88rpx; line-height: 88rpx; border-radius: 16rpx; border: none; background: #1677ff; color: #fff; font-size: 30rpx; font-weight: 600; }
</style>
