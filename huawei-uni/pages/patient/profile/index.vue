<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">信息完善</text>
      <text class="page-desc">完善您的个人信息</text>
    </view>

    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else class="form-container">
      <view class="form-card">
        <view class="form-item">
          <text class="form-label">姓名</text>
          <input v-model="form.name" placeholder="请输入姓名" class="form-input" />
        </view>

        <view class="form-item">
          <text class="form-label">性别</text>
          <view class="gender-group">
            <view class="gender-item" :class="{ active: form.gender === '男' }" @click="form.gender = '男'">
              <text>男</text>
            </view>
            <view class="gender-item" :class="{ active: form.gender === '女' }" @click="form.gender = '女'">
              <text>女</text>
            </view>
          </view>
        </view>

        <view class="form-item">
          <text class="form-label">身份证件类型</text>
          <input v-model="form.id_document_type" placeholder="身份证件类型" class="form-input" disabled />
        </view>

        <view class="form-item">
          <text class="form-label">身份证件号</text>
          <input v-model="form.id_document_no" placeholder="身份证件号" class="form-input" disabled />
        </view>

        <view class="form-item">
          <text class="form-label">手机号</text>
          <input v-model="form.phone" placeholder="手机号" class="form-input" disabled />
        </view>

        <view class="form-item">
          <text class="form-label">地址</text>
          <input v-model="form.address" placeholder="请输入地址" class="form-input" />
        </view>

        <view class="form-item">
          <text class="form-label">诊断信息</text>
          <textarea v-model="form.diagnosis" placeholder="请输入诊断信息（选填）" class="form-textarea" />
        </view>

        <view class="form-item">
          <text class="form-label">吸烟状况</text>
          <input v-model="form.smoking_status" placeholder="请输入吸烟状况（选填）" class="form-input" />
        </view>
      </view>

      <button class="submit-btn" @click="submitForm" :disabled="submitting">
        {{ submitting ? '提交中...' : '保存信息' }}
      </button>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      loading: true,
      submitting: false,
      form: {
        name: '',
        gender: '男',
        id_document_type: '',
        id_document_no: '',
        id_card: '',
        phone: '',
        address: '',
        diagnosis: '',
        cancer_diameter: '',
        smoking_status: ''
      }
    }
  },
  onLoad() {
    this.loadInfo()
  },
  methods: {
    async loadInfo() {
      try {
        const response = await uniAPI.getPatientInfo()
        if (response.success && response.data) {
          const d = response.data
          this.form.name = d.name || ''
          this.form.gender = d.gender || '男'
          this.form.id_document_type = d.id_document_type || ''
          this.form.id_document_no = d.id_document_no || d.id_card || ''
          this.form.id_card = d.id_card || ''
          this.form.phone = d.phone || ''
          this.form.address = d.address || ''
          this.form.diagnosis = d.diagnosis || ''
          this.form.cancer_diameter = d.cancer_diameter || ''
          this.form.smoking_status = d.smoking_status || ''
        }
      } catch (error) {
        console.error('Load info failed:', error)
      } finally {
        this.loading = false
      }
    },
    async submitForm() {
      if (!this.form.name) {
        uni.showToast({ title: '请输入姓名', icon: 'none' })
        return
      }

      this.submitting = true
      try {
        const response = await uniAPI.updatePatientInfo(this.form)
        if (response.success) {
          uni.showToast({ title: '保存成功', icon: 'success' })
          setTimeout(() => { uni.navigateBack() }, 1500)
        } else {
          uni.showToast({ title: response.message || '保存失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: '网络错误', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page-container {
  padding: 32rpx;
  min-height: 100vh;
  background-color: #F5F7FA;
  box-sizing: border-box;
}

.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }

.loading-container {
  display: flex; align-items: center; justify-content: center; padding: 120rpx 0;
}
.loading-text { font-size: 28rpx; color: #8c9aa8; }

.form-card {
  background: #fff;
  border-radius: 20rpx;
  padding: 16rpx 28rpx;
  box-shadow: 0 2rpx 12rpx rgba(22, 119, 255, 0.06);
}

.form-item {
  padding: 20rpx 0;
  border-bottom: 1rpx solid #f0f2f5;
}

.form-item:last-child { border-bottom: none; }

.form-label {
  display: block;
  font-size: 24rpx;
  color: #8c9aa8;
  margin-bottom: 12rpx;
}

.form-input {
  width: 100%;
  height: 72rpx;
  padding: 0 20rpx;
  border: 2rpx solid #e5e7eb;
  border-radius: 12rpx;
  background: #f9fafb;
  box-sizing: border-box;
  font-size: 26rpx;
  color: #1f2d3d;
}

.form-textarea {
  width: 100%;
  height: 160rpx;
  padding: 16rpx 20rpx;
  border: 2rpx solid #e5e7eb;
  border-radius: 12rpx;
  background: #f9fafb;
  box-sizing: border-box;
  font-size: 26rpx;
  color: #1f2d3d;
}

.gender-group {
  display: flex;
  gap: 20rpx;
}

.gender-item {
  flex: 1;
  height: 72rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  border: 2rpx solid #e5e7eb;
  border-radius: 12rpx;
  font-size: 26rpx;
  color: #1f2d3d;
  background: #f9fafb;
  transition: all 0.2s;
}

.gender-item.active {
  border-color: #1677ff;
  background: #e6f7ff;
  color: #1677ff;
  font-weight: 500;
}

.submit-btn {
  margin-top: 40rpx;
  width: 100%;
  height: 96rpx;
  border-radius: 16rpx;
  background: linear-gradient(135deg, #1677ff 0%, #409eff 100%);
  color: #fff;
  font-size: 28rpx;
  font-weight: 500;
  border: none;
  box-shadow: 0 4rpx 12rpx rgba(22, 119, 255, 0.3);
}

.submit-btn[disabled] {
  opacity: 0.6;
}
</style>
