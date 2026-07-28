<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">预约试剂盒邮寄</text>
      <text class="page-desc">填写收件地址后，工作人员会安排邮寄</text>
    </view>

    <view class="form-panel">
      <view class="form-item">
        <text class="form-label">收件人姓名</text>
        <input v-model="form.receiver_name" class="form-input" placeholder="请输入收件人姓名" />
      </view>
      <view class="form-item">
        <text class="form-label">收件人电话</text>
        <input v-model="form.receiver_phone" class="form-input" type="number" maxlength="11" placeholder="请输入联系电话" />
      </view>
      <view class="form-item">
        <text class="form-label">省市区</text>
        <picker mode="region" :value="region" @change="onRegionChange">
          <view class="region-picker" :class="{ empty: !regionText }">
            {{ regionText || '请选择省 / 市 / 区' }}
          </view>
        </picker>
      </view>
      <view class="form-item">
        <text class="form-label">详细地址</text>
        <textarea v-model="form.detail_address" class="form-textarea" placeholder="请输入街道、门牌号等详细地址" />
      </view>
      <view class="form-item">
        <text class="form-label">备注</text>
        <input v-model="form.notes" class="form-input" placeholder="可填写方便收件的时间等" />
      </view>
    </view>

    <button class="submit-button" :loading="submitting" @click="submitMailRequest">
      {{ submitting ? '提交中...' : '提交预约' }}
    </button>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      submitting: false,
      region: [],
      form: {
        receiver_name: '',
        receiver_phone: '',
        province: '',
        city: '',
        district: '',
        detail_address: '',
        notes: ''
      }
    }
  },
  computed: {
    regionText() {
      return this.region.length ? this.region.join(' / ') : ''
    }
  },
  onLoad() {
    uni.setNavigationBarTitle({ title: '预约试剂盒邮寄' })
    this.loadPatientInfo()
  },
  methods: {
    async loadPatientInfo() {
      try {
        const response = await uniAPI.getPatientInfo()
        const patient = response.data || {}
        this.form.receiver_name = patient.name || ''
        this.form.receiver_phone = patient.phone || ''
      } catch (error) {
        console.error('Load patient info failed:', error)
      }
    },
    onRegionChange(event) {
      this.region = event.detail.value || []
      this.form.province = this.region[0] || ''
      this.form.city = this.region[1] || ''
      this.form.district = this.region[2] || ''
    },
    validateForm() {
      if (!String(this.form.receiver_name || '').trim()) return '请输入收件人姓名'
      if (!/^1\d{10}$/.test(String(this.form.receiver_phone || '').trim())) return '请输入正确收件人电话'
      if (!this.form.province || !this.form.city || !this.form.district) return '请选择省市区'
      if (!String(this.form.detail_address || '').trim()) return '请输入详细地址'
      return ''
    },
    async submitMailRequest() {
      const error = this.validateForm()
      if (error) {
        uni.showToast({ title: error, icon: 'none' })
        return
      }
      this.submitting = true
      try {
        const payload = {
          ...this.form,
          receiver_address: `${this.form.province}${this.form.city}${this.form.district}${this.form.detail_address}`
        }
        const response = await uniAPI.createSampleBoxRequest(payload)
        if (response.success) {
          uni.showToast({ title: '预约成功', icon: 'success' })
          setTimeout(() => {
            uni.navigateBack()
          }, 700)
        } else {
          uni.showToast({ title: response.message || '预约失败', icon: 'none' })
        }
      } catch (error) {
        console.error('Submit mail request failed:', error)
        uni.showToast({ title: error.message || '预约失败', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page-container {
  min-height: 100vh;
  padding: 32rpx;
  background: #f5f7fa;
  box-sizing: border-box;
}

.page-header {
  margin-bottom: 28rpx;
}

.page-title {
  display: block;
  color: #1f2d3d;
  font-size: 38rpx;
  font-weight: 700;
  line-height: 1.35;
}

.page-desc {
  display: block;
  margin-top: 8rpx;
  color: #7b8794;
  font-size: 24rpx;
  line-height: 1.5;
}

.form-panel {
  padding: 28rpx;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 4rpx 16rpx rgba(31, 45, 61, 0.05);
}

.form-item {
  margin-bottom: 24rpx;
}

.form-item:last-child {
  margin-bottom: 0;
}

.form-label {
  display: block;
  margin-bottom: 10rpx;
  color: #4d5b6a;
  font-size: 24rpx;
}

.form-input,
.region-picker,
.form-textarea {
  width: 100%;
  min-height: 84rpx;
  padding: 0 22rpx;
  border: 2rpx solid #e5eaf0;
  border-radius: 12rpx;
  background: #fafbfc;
  box-sizing: border-box;
  color: #1f2d3d;
  font-size: 26rpx;
  line-height: 84rpx;
}

.region-picker.empty {
  color: #a9b4bf;
}

.form-textarea {
  height: 170rpx;
  padding-top: 20rpx;
  line-height: 1.5;
}

.submit-button {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 100%;
  height: 88rpx;
  margin-top: 28rpx;
  border-radius: 12rpx;
  background: #1677ff;
  color: #fff;
  font-size: 28rpx;
  font-weight: 700;
  text-align: center;
}

.submit-button::after {
  border: none;
}
</style>
