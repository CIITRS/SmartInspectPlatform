<template>
  <view class="page-container">
    <view class="page-header">
      <view>
        <text class="page-title">样本邮寄</text>
        <text class="page-desc">查看邮寄记录，也可以新增样本邮寄</text>
      </view>
      <button class="add-btn" @click="showForm = !showForm">{{ showForm ? '收起' : '新增' }}</button>
    </view>

    <view v-if="showForm" class="form-card">
      <view class="form-item">
        <text class="form-label">寄件人姓名</text>
        <input v-model="form.sender_name" placeholder="请输入姓名" class="form-input" />
      </view>
      <view class="form-item">
        <text class="form-label">手机号</text>
        <input v-model="form.sender_phone" type="number" maxlength="11" placeholder="请输入手机号" class="form-input" />
      </view>
      <view class="form-item">
        <text class="form-label">寄件地址</text>
        <input v-model="form.sender_address" placeholder="请输入寄件地址" class="form-input" />
      </view>
      <view class="form-item">
        <text class="form-label">快递公司</text>
        <picker :range="expressCompanies" range-key="name" @change="onExpressCompanyChange">
          <view class="form-input picker-value">{{ selectedExpressCompany.name }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="form-label">快递单号（选填）</text>
        <input v-model="form.tracking_no" placeholder="已寄出请填快递单号" class="form-input" />
      </view>
      <view class="form-item">
        <text class="form-label">备注（选填）</text>
        <input v-model="form.notes" placeholder="特殊说明" class="form-input" />
      </view>
      <button class="submit-btn" @click="submitForm" :disabled="submitting">
        {{ submitting ? '提交中...' : '提交邮寄信息' }}
      </button>
    </view>

    <view v-if="loading" class="state-box"><text class="state-text">加载中...</text></view>
    <view v-else-if="mailSamples.length === 0" class="state-box">
      <text class="empty-icon">📦</text>
      <text class="state-text">暂无邮寄样本</text>
    </view>
    <view v-else class="list-box">
      <view v-for="item in mailSamples" :key="item.id" class="mail-card">
        <view class="mail-head">
          <text class="sample-code">{{ item.sample_code }}</text>
          <text class="sample-status" :class="'s-' + item.sample_status">{{ statusMap[item.sample_status] || item.sample_status }}</text>
        </view>
        <view class="info-row"><text class="lbl">提交时间</text><text class="val">{{ item.created_at || item.collection_date || '-' }}</text></view>
        <view class="info-row" v-if="item.sender_name"><text class="lbl">寄件人</text><text class="val">{{ item.sender_name }}</text></view>
        <view class="info-row" v-if="item.sender_phone"><text class="lbl">手机号</text><text class="val">{{ item.sender_phone }}</text></view>
        <view class="info-row" v-if="item.sender_address"><text class="lbl">寄件地址</text><text class="val address">{{ item.sender_address }}</text></view>
        <view class="info-row" v-if="item.tracking_number">
          <text class="lbl">快递单号</text>
          <view class="tracking-wrap">
            <text class="val tracking">{{ item.tracking_number }}</text>
            <text class="copy-btn" @click="copyTracking(item.tracking_number)">复制</text>
          </view>
        </view>
        <view class="info-row" v-if="item.express_status"><text class="lbl">快递状态</text><text class="val">{{ expressStatusMap[item.express_status] || item.express_status }}</text></view>
        <view class="info-row" v-if="item.delivered_at"><text class="lbl">签收时间</text><text class="val">{{ item.delivered_at }}</text></view>
        <view class="info-row" v-if="item.latest_event_status"><text class="lbl">最新动态</text><text class="val address">{{ item.latest_event_status }}</text></view>
        <button
          v-if="item.tracking_number"
          class="query-btn"
          :loading="queryingId === item.id"
          @click="queryExpress(item)"
        >查询物流</button>
        <view v-if="trackingDetails[item.id]" class="tracking-detail">
          <text v-if="trackingDetails[item.id].last_query_error" class="tracking-error">{{ trackingDetails[item.id].last_query_error }}</text>
          <text v-else-if="trackingDetails[item.id].status === 'delivered'" class="signed-tip">已签收，中间物流路径已清除</text>
          <view v-else v-for="(event, index) in trackingDetails[item.id].route || []" :key="index" class="route-item">
            <text class="route-status">{{ event.status }}</text>
            <text class="route-time">{{ event.time }}</text>
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
      submitting: false,
      queryingId: 0,
      showForm: false,
      mailSamples: [],
      trackingDetails: {},
      expressCompanyIndex: 0,
      expressCompanies: [
        { name: '自动识别', type: 'auto' },
        { name: '顺丰速运', type: 'sfexpress' },
        { name: '圆通快递', type: 'yuantong' },
        { name: '中通快递', type: 'zhongtong' },
        { name: '韵达快递', type: 'yunda' },
        { name: '申通快递', type: 'shentong' },
        { name: '京东物流', type: 'jd' },
        { name: '邮政 EMS', type: 'ems' }
      ],
      form: { sender_name: '', sender_phone: '', sender_address: '', express_type: 'auto', express_company: '自动识别', tracking_no: '', notes: '' },
      statusMap: { created: '已创建', collected: '已采集', received: '已接收', processing: '检测中', tested: '已检测', completed: '已完成' },
      expressStatusMap: { pending: '待揽件', picked_up: '已揽件', in_transit: '运输中', delivered: '已签收', exception: '物流异常', returned: '退件' }
    }
  },
  computed: {
    selectedExpressCompany() {
      return this.expressCompanies[this.expressCompanyIndex] || this.expressCompanies[0]
    }
  },
  onLoad() {
    this.loadMailSamples()
  },
  onShow() {
    this.loadMailSamples()
  },
  methods: {
    async loadMailSamples() {
      this.loading = true
      try {
        const res = await uniAPI.getMailSamples()
        if (res.success && res.data) {
          this.mailSamples = res.data.list || []
        }
      } catch (e) {
        console.error('加载邮寄样本失败', e)
      } finally {
        this.loading = false
      }
    },
    async submitForm() {
      if (!this.form.sender_name) { uni.showToast({ title: '请输入姓名', icon: 'none' }); return }
      if (!this.form.sender_phone) { uni.showToast({ title: '请输入手机号', icon: 'none' }); return }
      this.submitting = true
      try {
        const res = await uniAPI.createMailSample(this.form)
        if (res.success) {
          uni.showToast({ title: '提交成功', icon: 'success' })
          this.expressCompanyIndex = 0
          this.form = { sender_name: '', sender_phone: '', sender_address: '', express_type: 'auto', express_company: '自动识别', tracking_no: '', notes: '' }
          this.showForm = false
          this.loadMailSamples()
        } else {
          uni.showToast({ title: res.message || '提交失败', icon: 'none' })
        }
      } catch (e) {
        uni.showToast({ title: '网络错误', icon: 'none' })
      } finally {
        this.submitting = false
      }
    },
    onExpressCompanyChange(event) {
      const index = Number(event.detail.value || 0)
      const company = this.expressCompanies[index] || this.expressCompanies[0]
      this.expressCompanyIndex = index
      this.form.express_type = company.type
      this.form.express_company = company.name
    },
    async queryExpress(item) {
      this.queryingId = item.id
      try {
        const res = await uniAPI.getExpress(item.id)
        if (res.success && res.data) {
          this.$set(this.trackingDetails, item.id, res.data)
          item.express_status = res.data.status
          item.delivered_at = res.data.delivered_at || item.delivered_at
          item.latest_event_status = res.data.latest_event_status || item.latest_event_status
        } else {
          uni.showToast({ title: res.message || '暂无物流信息', icon: 'none' })
        }
      } catch (e) {
        uni.showToast({ title: '物流查询失败', icon: 'none' })
      } finally {
        this.queryingId = 0
      }
    },
    copyTracking(code) {
      uni.setClipboardData({
        data: code,
        success: () => uni.showToast({ title: '快递单号已复制', icon: 'success' })
      })
    }
  }
}
</script>

<style scoped>
.page-container { padding: 32rpx; min-height: 100vh; background: #F5F7FA; box-sizing: border-box; }
.page-header { display: flex; justify-content: space-between; align-items: flex-start; margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.add-btn { margin: 0; width: 128rpx; height: 64rpx; line-height: 64rpx; border-radius: 12rpx; background: #1677ff; color: #fff; font-size: 24rpx; border: none; }
.form-card, .mail-card { background: #fff; border-radius: 20rpx; padding: 16rpx 28rpx; margin-bottom: 24rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.form-item { padding: 20rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.form-label { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 12rpx; }
.form-input { width: 100%; height: 72rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; }
.picker-value { display: flex; align-items: center; color: #1f2d3d; }
.submit-btn { margin-top: 24rpx; width: 100%; height: 88rpx; border-radius: 16rpx; background: #1677ff; color: #fff; font-size: 28rpx; border: none; }
.submit-btn[disabled] { opacity: 0.6; }
.state-box { display: flex; flex-direction: column; align-items: center; padding: 96rpx 0; }
.empty-icon { font-size: 72rpx; margin-bottom: 20rpx; }
.state-text { font-size: 26rpx; color: #8c9aa8; }
.mail-head { display: flex; justify-content: space-between; align-items: center; padding: 12rpx 0 18rpx; border-bottom: 1rpx solid #f0f2f5; margin-bottom: 12rpx; }
.sample-code { font-size: 28rpx; color: #1f2d3d; font-weight: 600; }
.sample-status { font-size: 22rpx; padding: 4rpx 16rpx; border-radius: 20rpx; }
.s-created { background: #f5f5f5; color: #8c9aa8; }
.s-collected { background: #e6f7ff; color: #1677ff; }
.s-received, .s-tested, .s-completed { background: #f0f9eb; color: #52c41a; }
.s-processing { background: #fdf6ec; color: #e6a23c; }
.info-row { display: flex; justify-content: space-between; gap: 24rpx; padding: 10rpx 0; }
.lbl { flex: 0 0 150rpx; font-size: 24rpx; color: #8c9aa8; }
.val { flex: 1; font-size: 24rpx; color: #1f2d3d; text-align: right; font-weight: 500; }
.address { word-break: break-all; }
.tracking-wrap { flex: 1; display: flex; justify-content: flex-end; align-items: center; gap: 12rpx; }
.tracking { color: #1677ff; }
.copy-btn { flex: 0 0 auto; font-size: 20rpx; color: #fff; background: #1677ff; padding: 4rpx 12rpx; border-radius: 8rpx; }
.query-btn { margin-top: 16rpx; width: 100%; height: 68rpx; line-height: 68rpx; border-radius: 12rpx; background: #eef5ff; color: #1677ff; font-size: 24rpx; border: none; }
.tracking-detail { margin-top: 16rpx; padding: 18rpx; border-radius: 12rpx; background: #f8fafc; }
.route-item { padding: 12rpx 0; border-bottom: 1rpx solid #edf0f3; }
.route-status { display: block; font-size: 23rpx; color: #1f2d3d; }
.route-time { display: block; margin-top: 6rpx; font-size: 20rpx; color: #8c9aa8; }
.tracking-error { color: #f56c6c; font-size: 22rpx; }
.signed-tip { color: #52c41a; font-size: 22rpx; }
</style>
