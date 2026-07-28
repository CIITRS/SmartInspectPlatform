<template>
  <view class="page">
    <view class="card">
      <view class="manager">
        <text class="manager-title">{{ manager.name || '我的邀请码' }}</text>
        <text class="manager-sub">客户扫码后将绑定为您的客户</text>
      </view>

      <view class="qr-box">
        <image v-if="qrcodeUrl" :src="fullQrcodeUrl" class="qr-image" mode="aspectFit"></image>
        <view v-else class="qr-placeholder">
          <text class="qr-text">小程序码未生成</text>
          <text class="qr-sub">请检查微信 AppID/AppSecret 配置</text>
        </view>
      </view>

      <view class="path-box">
        <text class="path-label">工号</text>
        <text class="path-text">{{ employeeCode }}</text>
      </view>
    </view>

    <button class="copy-btn" @click="copyEmployeeCode">复制工号</button>
    <button class="refresh-btn" @click="loadCode">刷新</button>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

const API_ORIGIN = 'https://bgpt.huaweibio.com.cn'

export default {
  data() {
    return {
      manager: {},
      invitePath: '',
      qrcodeUrl: '',
      loading: false
    }
  },
  computed: {
    fullQrcodeUrl() {
      if (!this.qrcodeUrl) return ''
      if (/^https?:\/\//.test(this.qrcodeUrl)) return this.qrcodeUrl
      return API_ORIGIN + this.qrcodeUrl
    },
    employeeCode() {
      return this.manager.employee_id || this.manager.username || (this.invitePath ? this.invitePath.replace(/^.*sales_id=/, '') : '-')
    }
  },
  onLoad() {
    this.loadCode()
  },
  methods: {
    async loadCode() {
      this.loading = true
      uni.showLoading({ title: '加载中...' })
      try {
        const res = await uniAPI.getInviteCode()
        if (res.success && res.data) {
          this.manager = res.data.manager || {}
          this.invitePath = res.data.invite_path || ''
          this.qrcodeUrl = res.data.qrcode_url || ''
        } else {
          uni.showToast({ title: res.message || '获取失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: '网络错误', icon: 'none' })
      } finally {
        uni.hideLoading()
        this.loading = false
      }
    },
    copyEmployeeCode() {
      if (!this.employeeCode || this.employeeCode === '-') return
      uni.setClipboardData({
        data: this.employeeCode,
        success: () => uni.showToast({ title: '已复制', icon: 'success' })
      })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.card { background: #fff; border-radius: 20rpx; padding: 36rpx 28rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.manager { text-align: center; margin-bottom: 28rpx; }
.manager-title { display: block; font-size: 34rpx; font-weight: 700; color: #1f2d3d; }
.manager-sub { display: block; margin-top: 10rpx; font-size: 24rpx; color: #8c9aa8; }
.qr-box { width: 430rpx; height: 430rpx; margin: 0 auto 28rpx; border: 2rpx solid #e5e7eb; border-radius: 16rpx; display: flex; align-items: center; justify-content: center; background: #f9fafb; }
.qr-image { width: 390rpx; height: 390rpx; }
.qr-placeholder { text-align: center; padding: 30rpx; }
.qr-text { display: block; font-size: 28rpx; color: #1f2d3d; }
.qr-sub { display: block; margin-top: 12rpx; font-size: 22rpx; color: #8c9aa8; }
.path-box { padding: 22rpx; background: #f6f9ff; border-radius: 12rpx; }
.path-label { display: block; font-size: 22rpx; color: #8c9aa8; margin-bottom: 8rpx; }
.path-text { display: block; font-size: 24rpx; color: #1f2d3d; word-break: break-all; }
.copy-btn, .refresh-btn { margin-top: 28rpx; width: 100%; height: 88rpx; border-radius: 14rpx; font-size: 28rpx; border: none; }
.copy-btn { background: #1677ff; color: #fff; }
.refresh-btn { background: #fff; color: #1677ff; border: 2rpx solid #1677ff; }
</style>
