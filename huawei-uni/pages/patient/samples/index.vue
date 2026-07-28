<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">我的样本</text>
      <text class="page-desc">查看样本采集和检测状态</text>
    </view>
    <view v-if="loading" class="loading-box"><text class="loading-text">加载中...</text></view>
    <view v-else-if="samples.length === 0" class="empty-box">
      <text class="empty-icon">🧪</text>
      <text class="empty-text">暂无样本记录</text>
    </view>
    <view v-else class="list-box">
      <view v-for="item in samples" :key="item.id" class="sample-card" @click="showSampleDetail(item)">
        <view class="sample-header">
          <text class="sample-code">{{ item.sample_code }}</text>
          <text class="sample-status" :class="'s-' + item.sample_status">{{ statusMap[item.sample_status] || item.sample_status }}</text>
        </view>
        <view class="sample-info">
          <view class="info-row"><text class="lbl">采集日期</text><text class="val">{{ item.collection_date || '-' }}</text></view>
          <view class="info-row" v-if="item.sample_type"><text class="lbl">样本类型</text><text class="val">{{ item.sample_type }}</text></view>
          <view class="info-row" v-if="item.treatment_stage"><text class="lbl">治疗阶段</text><text class="val">{{ item.treatment_stage }}</text></view>
          <view class="info-row" v-if="item.receive_date"><text class="lbl">接收日期</text><text class="val">{{ item.receive_date }}</text></view>
        </view>
      </view>
    </view>

    <!-- 样本详情弹窗 -->
    <uni-popup ref="detailPopup" type="center">
      <view class="detail-popup" v-if="currentSample">
        <view class="popup-header">
          <text class="popup-title">样本详情</text>
          <text class="popup-close" @click="closeDetailPopup">✕</text>
        </view>
        <view class="popup-body">
          <view class="info-row"><text class="lbl">样本编号</text><text class="val">{{ currentSample.sample_code }}</text></view>
          <view class="info-row"><text class="lbl">状态</text><text class="val status-text" :class="'s-' + currentSample.sample_status">{{ statusMap[currentSample.sample_status] || currentSample.sample_status }}</text></view>
          <view class="info-row" v-if="currentSample.collection_date"><text class="lbl">采集日期</text><text class="val">{{ currentSample.collection_date }}</text></view>
          <view class="info-row" v-if="currentSample.sample_type"><text class="lbl">样本类型</text><text class="val">{{ currentSample.sample_type }}</text></view>
          <view class="info-row" v-if="currentSample.treatment_stage"><text class="lbl">治疗阶段</text><text class="val">{{ currentSample.treatment_stage }}</text></view>
          <view class="info-row" v-if="currentSample.receive_date"><text class="lbl">接收日期</text><text class="val">{{ currentSample.receive_date }}</text></view>

          <view class="timeline-section">
            <text class="section-title">样本时间线</text>
            <view v-for="step in buildTimeline(currentSample)" :key="step.key" class="timeline-row">
              <view class="dot" :class="{ done: step.done }"></view>
              <view class="line-content">
                <text class="step-title" :class="{ muted: !step.done }">{{ step.title }}</text>
                <text v-if="step.time" class="step-time">{{ step.time }}</text>
              </view>
            </view>
          </view>
        </view>
      </view>
    </uni-popup>
  </view>
</template>
<script>
import { uniAPI } from '../../../api/index.js'
export default {
  data() {
    return {
      samples: [], loading: true,
      statusMap: { pending: '已创建', created: '已创建', collected: '已采集', received: '已接收', processing: '检测中', tested: '已检测', completed: '已完成' },
      statusOrder: { pending: 1, created: 1, collected: 2, received: 3, processing: 4, tested: 5, completed: 6 },
      currentSample: null,
      detailPopup: null
    }
  },
  onLoad() { this.load() },
  methods: {
    async load() {
      try {
        const res = await uniAPI.getSamples()
        if (res.success && res.data) this.samples = res.data.list || []
      } catch (e) { console.error(e) }
      finally { this.loading = false }
    },
    async showSampleDetail(item) {
      this.currentSample = item
      this.detailPopup = this.$refs.detailPopup
      this.detailPopup.open()
    },
    closeDetailPopup() {
      this.detailPopup.close()
      this.currentSample = null
    },
    buildTimeline(sample) {
      const current = this.statusOrder[sample.sample_status] || 0
      return [
        { key: 'created', title: '样本已创建', done: true, time: sample.sample_created_at || '' },
        { key: 'collected', title: '样本已采集', done: current >= 2, time: sample.collection_date || '' },
        { key: 'received', title: '实验室已接收', done: current >= 3, time: sample.receive_date || '' },
        { key: 'processing', title: '检测处理中', done: current >= 4, time: '' },
        { key: 'tested', title: '检测完成', done: current >= 5 || !!sample.test_completed_at, time: sample.test_completed_at || '' },
        { key: 'completed', title: '报告完成', done: current >= 6 || !!sample.report_reviewed_time, time: sample.report_reviewed_time || sample.report_generated_time || '' }
      ]
    }
  }
}
</script>
<style scoped>
.page-container { padding: 32rpx; min-height: 100vh; background: #F5F7FA; box-sizing: border-box; }
.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.loading-box, .empty-box { display: flex; flex-direction: column; align-items: center; padding: 120rpx 0; }
.loading-text { font-size: 28rpx; color: #8c9aa8; }
.empty-icon { font-size: 80rpx; margin-bottom: 24rpx; }
.empty-text { font-size: 28rpx; color: #1f2d3d; }
.sample-card { background: #fff; border-radius: 20rpx; padding: 28rpx; margin-bottom: 20rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.sample-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16rpx; padding-bottom: 16rpx; border-bottom: 1rpx solid #f0f2f5; }
.sample-code { font-size: 28rpx; font-weight: 600; color: #1f2d3d; }
.sample-status { font-size: 22rpx; padding: 4rpx 16rpx; border-radius: 20rpx; }
.s-pending, .s-created { background: #f5f5f5; color: #8c9aa8; }
.s-collected { background: #e6f7ff; color: #1677ff; }
.s-received { background: #f0f9eb; color: #67c23a; }
.s-processing { background: #fdf6ec; color: #e6a23c; }
.s-tested { background: #f0f9eb; color: #67c23a; }
.s-completed { background: #f0f9eb; color: #52c41a; }
.info-row { display: flex; justify-content: space-between; padding: 8rpx 0; }
.lbl { font-size: 24rpx; color: #8c9aa8; }
.val { font-size: 24rpx; color: #1f2d3d; font-weight: 500; }

/* 详情弹窗样式 */
.detail-popup { width: 600rpx; background: #fff; border-radius: 24rpx; overflow: hidden; }
.popup-header { display: flex; justify-content: space-between; align-items: center; padding: 32rpx; border-bottom: 1rpx solid #f0f2f5; }
.popup-title { font-size: 32rpx; font-weight: 600; color: #1f2d3d; }
.popup-close { font-size: 32rpx; color: #8c9aa8; padding: 8rpx; }
.popup-body { padding: 32rpx; max-height: 600rpx; overflow-y: auto; }

.status-text { padding: 2rpx 8rpx; border-radius: 6rpx; }
.timeline-section { margin-top: 24rpx; padding-top: 20rpx; border-top: 1rpx solid #f0f2f5; }
.section-title { display: block; font-size: 28rpx; font-weight: 600; color: #1f2d3d; margin-bottom: 8rpx; }
.timeline-row { display: flex; gap: 18rpx; padding-top: 20rpx; }
.dot { width: 18rpx; height: 18rpx; border-radius: 50%; background: #d9dde3; margin-top: 8rpx; flex-shrink: 0; }
.dot.done { background: #1677ff; }
.line-content { flex: 1; min-width: 0; }
.step-title { display: block; font-size: 24rpx; color: #1f2d3d; font-weight: 600; }
.step-title.muted { color: #a0b0c0; font-weight: 400; }
.step-time { display: block; font-size: 22rpx; color: #8c9aa8; margin-top: 4rpx; }
</style>
