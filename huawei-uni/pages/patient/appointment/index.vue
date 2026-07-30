<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">物流中心</text>
      <text class="page-desc">申请试剂盒，并查看寄出样本当前所在环节</text>
    </view>

    <view class="action-grid">
      <button class="action-card primary" @click="goToMailForm">
        <text class="action-title">预约试剂盒邮寄</text>
        <text class="action-desc">填写收件地址，等待工作人员邮寄</text>
      </button>
      <button class="action-card" :loading="managerLoading" @click="contactManager">
        <text class="action-title">联系专属客户经理</text>
        <text class="action-desc">{{ managerText }}</text>
      </button>
    </view>

    <view class="records-section">
      <view class="section-header">
        <text class="section-title">我的样本位置</text>
      </view>
      <view v-if="samples.length === 0" class="empty-container">
        <text class="empty-text">暂无寄出样本</text>
      </view>
      <view v-for="sample in samples" :key="sample.id" class="request-card">
        <view class="request-header">
          <text class="request-name">{{ sample.sample_code }}</text>
          <text class="status-tag" :class="{ shipped: sample.express_status === 'delivered' }">
            {{ sampleLocation(sample) }}
          </text>
        </view>
        <view class="request-row">
          <text class="row-label">运单</text>
          <text class="row-value">{{ sample.express_company || '' }} {{ sample.tracking_number || '待填写' }}</text>
        </view>
        <view v-if="sample.latest_event_status" class="request-row address-row">
          <text class="row-label">最新动态</text>
          <text class="row-value">{{ sample.latest_event_status }}</text>
        </view>
      </view>
    </view>

    <view class="records-section">
      <view class="section-header">
        <text class="section-title">预约记录</text>
        <button class="refresh-button" @click="loadRequests">刷新</button>
      </view>
      <view v-if="loading" class="empty-container">
        <text class="empty-text">加载中...</text>
      </view>
      <view v-else-if="requests.length === 0" class="empty-container">
        <text class="empty-text">暂无预约记录</text>
      </view>
      <view v-else class="request-list">
        <view v-for="item in requests" :key="item.id" class="request-card">
          <view class="request-header">
            <text class="request-name">{{ item.receiver_name || '收件人' }}</text>
            <text class="status-tag" :class="statusClass(item.status)">{{ item.status_text || statusText(item.status) }}</text>
          </view>
          <view class="request-row">
            <text class="row-label">电话</text>
            <text class="row-value">{{ item.receiver_phone || '-' }}</text>
          </view>
          <view class="request-row address-row">
            <text class="row-label">地址</text>
            <text class="row-value">{{ item.full_address || '-' }}</text>
          </view>
          <view v-if="item.tracking_number" class="request-row">
            <text class="row-label">运单号</text>
            <text class="row-value">{{ item.express_company ? item.express_company + ' ' : '' }}{{ item.tracking_number }}</text>
          </view>
          <view class="request-row">
            <text class="row-label">提交时间</text>
            <text class="row-value">{{ item.created_at || '-' }}</text>
          </view>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      loading: false,
      managerLoading: false,
      manager: null,
      requests: [],
      samples: []
    }
  },
  computed: {
    managerText() {
      if (!this.manager) return '获取姓名和电话'
      return `${this.manager.name || '客户经理'} ${this.manager.phone || ''}`.trim()
    }
  },
  onLoad() {
    uni.setNavigationBarTitle({ title: '物流中心' })
    this.loadRequests()
    this.loadSamples()
  },
  onShow() {
    this.loadRequests()
    this.loadSamples()
  },
  methods: {
    goToMailForm() {
      uni.navigateTo({ url: '/pages/patient/appointment-mail/index' })
    },
    async loadSamples() {
      try {
        const response = await uniAPI.getMailSamples()
        this.samples = response.data?.list || []
      } catch (error) {
        console.error('Load sample logistics failed:', error)
      }
    },
    sampleLocation(sample) {
      if (sample.express_status && sample.express_status !== 'delivered') return '寄往实验室途中'
      const map = {
        created: '患者处，待寄回',
        collected: '患者处，待寄回',
        received: '实验室已签收',
        testing: '实验室检测中',
        tested: '检测已完成',
        completed: '检测已完成'
      }
      return map[sample.sample_status] || '待确认'
    },
    async loadRequests() {
      this.loading = true
      try {
        const response = await uniAPI.getSampleBoxRequests()
        if (response.success && response.data) {
          this.requests = response.data.list || []
        }
      } catch (error) {
        console.error('Load mail requests failed:', error)
        uni.showToast({ title: '预约记录加载失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    },
    async contactManager() {
      this.managerLoading = true
      try {
        const response = await uniAPI.getPatientManager()
        this.manager = response.data || {}
        if (this.manager.phone) {
          uni.showModal({
            title: this.manager.name || '专属客户经理',
            content: this.manager.phone,
            confirmText: '拨打电话',
            success: (res) => {
              if (res.confirm) {
                uni.makePhoneCall({ phoneNumber: this.manager.phone })
              }
            }
          })
        } else {
          uni.showModal({
            title: this.manager.name || '专属客户经理',
            content: '暂未配置联系电话，请稍后联系工作人员。',
            showCancel: false
          })
        }
      } catch (error) {
        console.error('Load manager failed:', error)
        uni.showToast({ title: '获取客户经理失败', icon: 'none' })
      } finally {
        this.managerLoading = false
      }
    },
    statusText(status) {
      const map = { requested: '待邮寄', shipped: '已邮寄' }
      return map[status] || status || '待邮寄'
    },
    statusClass(status) {
      return {
        shipped: status === 'shipped'
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

.action-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 16rpx;
}

.action-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 148rpx;
  padding: 24rpx 16rpx;
  border: 2rpx solid #e5eaf0;
  border-radius: 16rpx;
  background: #fff;
  color: #1f2d3d;
  line-height: 1.4;
  text-align: center;
  box-shadow: 0 4rpx 16rpx rgba(31, 45, 61, 0.05);
}

.action-card::after {
  border: none;
}

.action-card.primary {
  border-color: #1677ff;
  background: #eef6ff;
}

.action-title {
  display: block;
  font-size: 28rpx;
  font-weight: 700;
}

.action-desc {
  display: block;
  margin-top: 8rpx;
  color: #7b8794;
  font-size: 22rpx;
  line-height: 1.35;
}

.form-panel,
.records-section {
  margin-top: 24rpx;
  padding: 28rpx;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 4rpx 16rpx rgba(31, 45, 61, 0.05);
}

.form-title,
.section-title {
  color: #1f2d3d;
  font-size: 30rpx;
  font-weight: 700;
}

.form-item {
  margin-top: 22rpx;
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
  height: 150rpx;
  padding-top: 20rpx;
  line-height: 1.5;
}

.submit-button {
  display: flex;
  align-items: center;
  justify-content: center;
  height: 88rpx;
  margin-top: 28rpx;
  border-radius: 12rpx;
  background: #1677ff;
  color: #fff;
  font-size: 28rpx;
  font-weight: 700;
  text-align: center;
}

.submit-button::after,
.refresh-button::after {
  border: none;
}

.section-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 18rpx;
}

.refresh-button {
  display: flex;
  align-items: center;
  justify-content: center;
  margin-right: -10rpx;
  min-width: 112rpx;
  height: 56rpx;
  padding: 0 18rpx;
  border-radius: 10rpx;
  background: #edf2f7;
  color: #4d5b6a;
  font-size: 24rpx;
  line-height: 56rpx;
  text-align: center;
}

.empty-container {
  display: flex;
  align-items: center;
  justify-content: center;
  min-height: 160rpx;
}

.empty-text {
  color: #8c9aa8;
  font-size: 26rpx;
}

.request-card {
  padding: 22rpx 0;
  border-bottom: 1rpx solid #edf0f5;
}

.request-card:last-child {
  border-bottom: none;
}

.request-header,
.request-row {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 20rpx;
}

.request-header {
  margin-bottom: 14rpx;
}

.request-name {
  color: #1f2d3d;
  font-size: 28rpx;
  font-weight: 700;
}

.status-tag {
  flex: none;
  padding: 6rpx 16rpx;
  border-radius: 999rpx;
  background: #fff7e6;
  color: #d46b08;
  font-size: 22rpx;
}

.status-tag.shipped {
  background: #f0f9eb;
  color: #389e0d;
}

.request-row {
  padding: 6rpx 0;
}

.row-label {
  flex: none;
  width: 112rpx;
  color: #8c9aa8;
  font-size: 24rpx;
}

.row-value {
  flex: 1;
  color: #1f2d3d;
  font-size: 24rpx;
  line-height: 1.45;
  text-align: right;
  word-break: break-all;
}

.address-row .row-value {
  text-align: left;
}
</style>
