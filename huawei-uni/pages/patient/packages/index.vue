<template>
  <view class="page">
    <view class="page-header">
      <text class="page-title">我的套餐</text>
      <text class="page-desc">查看购买套餐、下一次预计检测时间和采样盒预约</text>
    </view>

    <view v-if="loading" class="state-box">
      <text class="state-text">加载中...</text>
    </view>

    <view v-else-if="packages.length === 0" class="state-box">
      <text class="empty-icon">📋</text>
      <text class="state-title">暂无套餐</text>
      <text class="state-text">购买套餐后会在这里显示检测计划</text>
    </view>

    <view v-else class="package-list">
      <view v-for="item in packages" :key="item.id" class="package-card">
        <view class="package-head">
          <view>
            <text class="package-name">{{ item.package_name }}</text>
            <text class="order-no">{{ item.order_no }}</text>
          </view>
          <text class="status-tag" :class="'s-' + item.status">{{ statusText(item.status) }}</text>
        </view>

        <view class="next-box">
          <text class="next-label">下一次预计检测</text>
          <text class="next-date">{{ item.next_detection_date || '待安排' }}</text>
        </view>

        <view class="meta-grid">
          <view class="meta-item">
            <text class="meta-label">检测次数</text>
            <text class="meta-value">{{ item.detection_count }} 次</text>
          </view>
          <view class="meta-item">
            <text class="meta-label">检测间隔</text>
            <text class="meta-value">{{ item.interval_days }} 天</text>
          </view>
          <view class="meta-item">
            <text class="meta-label">付款状态</text>
            <text class="meta-value">{{ paymentText(item.payment_status) }}</text>
          </view>
          <view class="meta-item">
            <text class="meta-label">购买时间</text>
            <text class="meta-value">{{ item.created_at || '-' }}</text>
          </view>
        </view>

        <view class="plans">
          <view v-for="plan in item.plans" :key="plan.id" class="plan-row">
            <text class="plan-index">第 {{ plan.detection_number }} 次</text>
            <text class="plan-date">{{ plan.detection_date || '待确定' }}</text>
            <text class="plan-status">{{ planStatusText(plan.status) }}</text>
          </view>
        </view>

        <button class="box-btn" @click="openBoxForm(item)">预约邮寄采样盒</button>
      </view>
    </view>

    <view v-if="showBoxForm" class="mask" @click="closeBoxForm">
      <view class="form-panel" @click.stop>
        <view class="form-head">
          <text class="form-title">预约邮寄采样盒</text>
          <text class="close" @click="closeBoxForm">×</text>
        </view>
        <view class="form-body">
          <view class="form-item">
            <text class="form-label">收件人</text>
            <input v-model="boxForm.receiver_name" class="form-input" placeholder="请输入收件人姓名" />
          </view>
          <view class="form-item">
            <text class="form-label">手机号</text>
            <input v-model="boxForm.receiver_phone" type="number" maxlength="11" class="form-input" placeholder="请输入手机号" />
          </view>
          <view class="form-item">
            <text class="form-label">收件地址</text>
            <input v-model="boxForm.receiver_address" class="form-input" placeholder="请输入收件地址" />
          </view>
          <view class="form-item">
            <text class="form-label">期望寄出日期</text>
            <picker mode="date" :value="boxForm.expected_send_date" @change="onDateChange">
              <view class="date-picker">{{ boxForm.expected_send_date || '请选择日期' }}</view>
            </picker>
          </view>
          <view class="form-item">
            <text class="form-label">备注</text>
            <input v-model="boxForm.notes" class="form-input" placeholder="选填" />
          </view>
        </view>
        <view class="form-actions">
          <button class="cancel-btn" @click="closeBoxForm">取消</button>
          <button class="submit-btn" :disabled="submitting" @click="submitBoxRequest">
            {{ submitting ? '提交中...' : '提交预约' }}
          </button>
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
      loading: true,
      submitting: false,
      packages: [],
      showBoxForm: false,
      currentPackage: null,
      boxForm: {
        receiver_name: '',
        receiver_phone: '',
        receiver_address: '',
        expected_send_date: '',
        notes: ''
      }
    }
  },
  onLoad() {
    this.loadPackages()
  },
  onShow() {
    this.loadPackages()
  },
  methods: {
    async loadPackages() {
      this.loading = true
      try {
        const res = await uniAPI.getMyPackages()
        if (res.success && res.data) {
          this.packages = res.data.list || []
        }
      } catch (e) {
        uni.showToast({ title: '加载套餐失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    },
    statusText(status) {
      const map = { pending: '待确认', active: '进行中', completed: '已完成', cancelled: '已取消' }
      return map[status] || status || '-'
    },
    paymentText(status) {
      const map = { pending: '待支付', paid: '已支付', refunded: '已退款' }
      return map[status] || status || '-'
    },
    planStatusText(status) {
      const map = { scheduled: '待检测', completed: '已完成', cancelled: '已取消' }
      return map[status] || status || '-'
    },
    openBoxForm(item) {
      this.currentPackage = item
      this.boxForm = {
        receiver_name: item.patient_name || '',
        receiver_phone: item.patient_phone || '',
        receiver_address: item.patient_address || '',
        expected_send_date: item.next_detection_date || '',
        notes: ''
      }
      this.showBoxForm = true
    },
    closeBoxForm() {
      this.showBoxForm = false
      this.currentPackage = null
    },
    onDateChange(e) {
      this.boxForm.expected_send_date = e.detail.value
    },
    async submitBoxRequest() {
      if (!this.currentPackage) return
      if (!this.boxForm.receiver_name) { uni.showToast({ title: '请输入收件人', icon: 'none' }); return }
      if (!this.boxForm.receiver_phone) { uni.showToast({ title: '请输入手机号', icon: 'none' }); return }
      if (!this.boxForm.receiver_address) { uni.showToast({ title: '请输入收件地址', icon: 'none' }); return }

      this.submitting = true
      try {
        const res = await uniAPI.createSampleBoxRequest({
          order_id: this.currentPackage.order_id,
          plan_id: this.currentPackage.next_plan_id || 0,
          ...this.boxForm
        })
        if (res.success) {
          uni.showToast({ title: '预约成功', icon: 'success' })
          this.closeBoxForm()
        } else {
          uni.showToast({ title: res.message || '预约失败', icon: 'none' })
        }
      } catch (e) {
        uni.showToast({ title: '网络错误', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.page-header { margin-bottom: 28rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; line-height: 1.5; }
.state-box { display: flex; flex-direction: column; align-items: center; padding: 120rpx 0; }
.empty-icon { font-size: 72rpx; margin-bottom: 20rpx; }
.state-title { font-size: 28rpx; color: #1f2d3d; font-weight: 600; margin-bottom: 8rpx; }
.state-text { font-size: 24rpx; color: #8c9aa8; }
.package-card { background: #fff; border-radius: 20rpx; padding: 28rpx; margin-bottom: 24rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.package-head { display: flex; justify-content: space-between; gap: 20rpx; align-items: flex-start; padding-bottom: 20rpx; border-bottom: 1rpx solid #f0f2f5; }
.package-name { display: block; font-size: 30rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.order-no { display: block; font-size: 22rpx; color: #8c9aa8; }
.status-tag { flex: 0 0 auto; font-size: 22rpx; padding: 6rpx 16rpx; border-radius: 20rpx; background: #e6f7ff; color: #1677ff; }
.s-completed { background: #f0f9eb; color: #52c41a; }
.s-cancelled { background: #fef0f0; color: #f56c6c; }
.next-box { margin: 24rpx 0; padding: 24rpx; border-radius: 16rpx; background: #f6f9ff; }
.next-label { display: block; font-size: 24rpx; color: #64748b; margin-bottom: 8rpx; }
.next-date { display: block; font-size: 38rpx; color: #1677ff; font-weight: 700; }
.meta-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx; margin-bottom: 22rpx; }
.meta-item { padding: 18rpx; border-radius: 12rpx; background: #fafbfc; }
.meta-label { display: block; font-size: 22rpx; color: #8c9aa8; margin-bottom: 6rpx; }
.meta-value { display: block; font-size: 24rpx; color: #1f2d3d; font-weight: 600; }
.plans { border-top: 1rpx solid #f0f2f5; padding-top: 14rpx; }
.plan-row { display: grid; grid-template-columns: 140rpx 1fr 120rpx; gap: 12rpx; padding: 12rpx 0; align-items: center; }
.plan-index, .plan-date, .plan-status { font-size: 24rpx; color: #1f2d3d; }
.plan-status { color: #8c9aa8; text-align: right; }
.box-btn { margin-top: 22rpx; width: 100%; height: 84rpx; line-height: 84rpx; border: none; border-radius: 14rpx; background: #1677ff; color: #fff; font-size: 28rpx; }
.mask { position: fixed; inset: 0; background: rgba(15,23,42,0.45); display: flex; align-items: flex-end; z-index: 20; }
.form-panel { width: 100%; background: #fff; border-radius: 28rpx 28rpx 0 0; overflow: hidden; }
.form-head { display: flex; justify-content: space-between; align-items: center; padding: 28rpx 32rpx; border-bottom: 1rpx solid #f0f2f5; }
.form-title { font-size: 30rpx; color: #1f2d3d; font-weight: 700; }
.close { font-size: 40rpx; color: #8c9aa8; }
.form-body { padding: 12rpx 32rpx; max-height: 60vh; overflow-y: auto; }
.form-item { padding: 18rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.form-label { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 12rpx; }
.form-input, .date-picker { width: 100%; min-height: 72rpx; line-height: 72rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; color: #1f2d3d; }
.form-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx; padding: 24rpx 32rpx 36rpx; }
.cancel-btn, .submit-btn { height: 78rpx; line-height: 78rpx; border-radius: 12rpx; font-size: 28rpx; }
.cancel-btn { background: #fff; color: #1677ff; border: 2rpx solid #1677ff; }
.submit-btn { background: #1677ff; color: #fff; border: none; }
.submit-btn[disabled] { opacity: 0.6; }
</style>
