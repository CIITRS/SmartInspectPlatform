<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">预约检测</text>
      <text class="page-desc">预约试剂盒或扫码邮寄已绑定的样本</text>
    </view>

    <view class="action-stack">
      <button class="primary-action" @click="goToMailForm">预约试剂盒邮寄</button>
      <button class="primary-action" @click="openScanModal">扫码邮寄样本</button>
    </view>

    <view class="records-section">
      <view class="section-header"><text class="section-title">预约记录</text><button class="refresh-button" @click="loadRequests">刷新</button></view>
      <view v-if="loading" class="empty-container"><text class="empty-text">加载中...</text></view>
      <view v-else-if="requests.length === 0" class="empty-container"><text class="empty-text">暂无预约记录</text></view>
      <view v-else class="request-list">
        <view v-for="item in requests" :key="item.id" class="request-card clickable" @click="openRequestDetail(item)">
          <view class="request-header">
            <text class="request-name">{{ item.receiver_name || '检测预约' }}</text>
            <text class="status-tag" :class="{ delivered: item.express_status === 'delivered', shipped: item.status === 'shipped' }">{{ appointmentStatus(item) }}</text>
          </view>
          <view class="request-row"><text class="row-label">套餐</text><text class="row-value">{{ item.package_progress || item.package_name || '单次检测' }}</text></view>
          <view class="request-row"><text class="row-label">检查癌型</text><text class="row-value">{{ item.cancer_type || '-' }}</text></view>
          <text class="detail-entry">查看预约详情 ›</text>
        </view>
      </view>
    </view>

    <view v-if="scanModalOpen" class="modal-mask" @click="closeScanModal" @touchmove.stop.prevent>
      <view class="center-modal scan-modal" @click.stop>
        <text class="modal-title">请扫码</text>
        <text v-if="!scannedSampleCode" class="modal-desc">请扫描样本管上的条形码或二维码</text>
        <view v-else class="sample-code-box"><text class="sample-code-label">样本管码</text><text class="sample-code-value">{{ scannedSampleCode }}</text></view>
        <button v-if="!scannedSampleCode" class="modal-primary" @click="scanSampleCode">开始扫码</button>
        <view v-else class="modal-actions">
          <button class="modal-secondary" @click="scanSampleCode">重新扫码</button>
          <button class="modal-primary" @click="goToSampleMail">邮寄</button>
        </view>
        <button class="modal-cancel" @click="closeScanModal">取消</button>
      </view>
    </view>

    <view v-if="detailOpen" class="modal-mask" @click="detailOpen = false" @touchmove.stop.prevent>
      <view class="center-modal detail-modal" @click.stop>
        <view class="detail-head"><text class="modal-title">预约详情</text><text class="close-button" @click="detailOpen = false">×</text></view>
        <scroll-view scroll-y class="detail-scroll">
          <view class="detail-row"><text class="detail-label">检查癌型</text><text class="detail-value">{{ currentRequest.cancer_type || '-' }}</text></view>
          <view class="detail-row"><text class="detail-label">类型</text><text class="detail-value">{{ currentRequest.detection_mode || '-' }}</text></view>
          <view class="detail-row"><text class="detail-label">套餐</text><text class="detail-value">{{ currentRequest.package_name || '单次检测' }}</text></view>
          <view class="detail-row"><text class="detail-label">检测次数</text><text class="detail-value">{{ currentRequest.package_progress || '-' }}</text></view>
          <view class="detail-row"><text class="detail-label">预约日期</text><text class="detail-value">{{ currentRequest.detection_date || currentRequest.created_at || '-' }}</text></view>
          <view class="detail-row"><text class="detail-label">收件地址</text><text class="detail-value">{{ currentRequest.full_address || '-' }}</text></view>
          <view v-if="currentRequest.tracking_number" class="detail-row"><text class="detail-label">快递单号</text><text class="detail-value">{{ currentRequest.express_company || '' }} {{ currentRequest.tracking_number }}</text></view>
          <view v-if="currentRequest.tracking_number" class="tracking-section">
            <text class="tracking-title">快递路径</text>
            <text v-if="currentRequest.express_status === 'delivered'" class="delivered-line">{{ signedTime(currentRequest.express_delivered_at) }} 已签收</text>
            <text v-else-if="currentRequest.express_last_query_error" class="tracking-error">{{ currentRequest.express_last_query_error }}</text>
            <view v-else v-for="(event, index) in currentRequest.express_route || []" :key="index" class="route-item">
              <text class="route-status">{{ event.status }}</text><text class="route-time">{{ event.time }}</text>
            </view>
            <text v-if="currentRequest.express_status !== 'delivered' && !(currentRequest.express_route || []).length && !currentRequest.express_last_query_error" class="empty-text">暂无物流轨迹</text>
          </view>
        </scroll-view>
      </view>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return { loading: false, requests: [], scanModalOpen: false, scannedSampleCode: '', detailOpen: false, currentRequest: {} }
  },
  onLoad(options) {
    uni.setNavigationBarTitle({ title: '预约检测' })
    this.loadRequests()
    if (String(options && options.scan || '') === '1') this.$nextTick(() => this.openScanModal())
  },
  onShow() { this.loadRequests() },
  methods: {
    goToMailForm() { uni.navigateTo({ url: '/pages/patient/appointment-mail/index' }) },
    async loadRequests() {
      this.loading = true
      try {
        const response = await uniAPI.getSampleBoxRequests()
        if (response.success && response.data) this.requests = response.data.list || []
      } catch (error) {
        uni.showToast({ title: '预约记录加载失败', icon: 'none' })
      } finally { this.loading = false }
    },
    openScanModal() {
      this.scannedSampleCode = ''
      this.scanModalOpen = true
      this.$nextTick(() => this.scanSampleCode())
    },
    scanSampleCode() {
      uni.scanCode({
        onlyFromCamera: true,
        scanType: ['barCode', 'qrCode'],
        success: ({ result }) => {
          const code = String(result || '').trim().toUpperCase()
          if (!code) return uni.showToast({ title: '未识别到样本管码', icon: 'none' })
          this.scannedSampleCode = code
        },
        fail: (error) => {
          if (!String(error && error.errMsg || '').includes('cancel')) uni.showToast({ title: '扫码失败，请重试', icon: 'none' })
        }
      })
    },
    closeScanModal() { this.scanModalOpen = false },
    goToSampleMail() {
      if (!this.scannedSampleCode) return
      const code = encodeURIComponent(this.scannedSampleCode)
      this.closeScanModal()
      uni.navigateTo({ url: `/pages/patient/mail-sample/index?sample_code=${code}` })
    },
    openRequestDetail(item) { this.currentRequest = item || {}; this.detailOpen = true },
    appointmentStatus(item) {
      if (item.express_status === 'delivered') return '已签收'
      if (item.status === 'shipped') return '已邮寄'
      return '待邮寄'
    },
    signedTime(value) {
      const match = String(value || '').match(/\b(\d{2}:\d{2})(?::\d{2})?\b/)
      return match ? match[1] : '--:--'
    }
  }
}
</script>

<style scoped>
.page-container { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.page-header { margin-bottom: 28rpx; }
.page-title { display: block; color: #1f2d3d; font-size: 38rpx; font-weight: 700; }
.page-desc { display: block; margin-top: 8rpx; color: #7b8794; font-size: 24rpx; }
.action-stack { display: flex; flex-direction: column; gap: 20rpx; }
.primary-action { display: flex; align-items: center; justify-content: center; width: 100%; min-height: 92rpx; margin: 0; border: none; border-radius: 16rpx; background: #1677ff; color: #fff; font-size: 29rpx; font-weight: 700; }
.primary-action::after, .refresh-button::after, .modal-primary::after, .modal-secondary::after, .modal-cancel::after { border: none; }
.records-section { margin-top: 28rpx; padding: 28rpx; border-radius: 18rpx; background: #fff; box-shadow: 0 4rpx 16rpx rgba(31,45,61,.05); }
.section-header, .request-header, .request-row, .detail-head, .detail-row { display: flex; align-items: flex-start; justify-content: space-between; gap: 20rpx; }
.section-header { align-items: center; margin-bottom: 16rpx; }
.section-title { color: #1f2d3d; font-size: 30rpx; font-weight: 700; }
.refresh-button { min-width: 112rpx; height: 64rpx; margin: 0; border: none; border-radius: 12rpx; background: #edf2f7; color: #4d5b6a; font-size: 24rpx; line-height: 64rpx; }
.empty-container { display: flex; justify-content: center; padding: 72rpx 0; }.empty-text { color: #8c9aa8; font-size: 24rpx; }
.request-card { padding: 24rpx 0; border-bottom: 1rpx solid #edf0f5; }.request-card:last-child { border-bottom: none; }.clickable:active { opacity: .72; }
.request-header { margin-bottom: 12rpx; }.request-name { color: #1f2d3d; font-size: 28rpx; font-weight: 700; }
.status-tag { flex: none; padding: 6rpx 16rpx; border-radius: 999rpx; background: #fff7e6; color: #d46b08; font-size: 22rpx; }.status-tag.shipped { background: #e6f4ff; color: #1677ff; }.status-tag.delivered { background: #f0f9eb; color: #389e0d; }
.request-row { padding: 6rpx 0; }.row-label { flex: none; width: 120rpx; color: #8c9aa8; font-size: 24rpx; }.row-value { flex: 1; color: #1f2d3d; font-size: 24rpx; text-align: right; }
.detail-entry { display: block; margin-top: 14rpx; color: #1677ff; font-size: 24rpx; text-align: right; }
.modal-mask { position: fixed; z-index: 1100; inset: 0; display: flex; align-items: center; justify-content: center; padding: 32rpx; background: rgba(0,0,0,.52); box-sizing: border-box; }
.center-modal { width: 100%; max-width: 680rpx; max-height: 82vh; padding: 32rpx; border-radius: 24rpx; background: #fff; box-sizing: border-box; box-shadow: 0 18rpx 60rpx rgba(0,0,0,.2); }
.scan-modal { text-align: center; }.modal-title { color: #1f2d3d; font-size: 32rpx; font-weight: 700; }.modal-desc { display: block; margin: 18rpx 0 28rpx; color: #7b8794; font-size: 25rpx; }
.sample-code-box { margin: 28rpx 0; padding: 24rpx; border-radius: 16rpx; background: #f0f6ff; }.sample-code-label { display: block; color: #7b8794; font-size: 23rpx; }.sample-code-value { display: block; margin-top: 10rpx; color: #1677ff; font-size: 32rpx; font-weight: 700; word-break: break-all; }
.modal-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 20rpx; }.modal-primary, .modal-secondary, .modal-cancel { width: 100%; min-height: 88rpx; margin: 0; border: none; border-radius: 14rpx; font-size: 27rpx; }.modal-primary { background: #1677ff; color: #fff; }.modal-secondary { background: #eaf3ff; color: #1677ff; }.modal-cancel { margin-top: 18rpx; background: #f4f5f6; color: #607080; }
.detail-modal { padding: 0; overflow: hidden; }.detail-head { align-items: center; padding: 28rpx 30rpx; border-bottom: 1rpx solid #edf0f3; }.close-button { width: 72rpx; height: 72rpx; color: #607080; font-size: 50rpx; line-height: 68rpx; text-align: center; }.detail-scroll { max-height: 68vh; padding: 14rpx 30rpx 30rpx; box-sizing: border-box; }
.detail-row { padding: 16rpx 0; border-bottom: 1rpx solid #f2f3f5; }.detail-label { flex: 0 0 140rpx; color: #8c9aa8; font-size: 24rpx; }.detail-value { flex: 1; color: #1f2d3d; font-size: 25rpx; text-align: right; word-break: break-all; }
.tracking-section { margin-top: 24rpx; padding: 22rpx; border-radius: 16rpx; background: #f8fafc; }.tracking-title { display: block; margin-bottom: 14rpx; color: #1f2d3d; font-size: 27rpx; font-weight: 700; }.delivered-line { color: #389e0d; font-size: 26rpx; font-weight: 600; }.tracking-error { color: #d93025; font-size: 23rpx; }.route-item { padding: 14rpx 0; border-bottom: 1rpx solid #e8edf2; }.route-status { display: block; color: #1f2d3d; font-size: 24rpx; }.route-time { display: block; margin-top: 6rpx; color: #8c9aa8; font-size: 21rpx; }
</style>
