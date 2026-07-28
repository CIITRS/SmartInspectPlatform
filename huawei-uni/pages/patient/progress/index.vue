<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">进度查询</text>
      <text class="page-desc">查看全部样本检测时间线</text>
    </view>

    <view class="range-tabs">
      <view
        v-for="option in rangeOptions"
        :key="option.value"
        class="range-tab"
        :class="{ active: selectedMonths === option.value }"
        @click="changeRange(option.value)"
      >
        <text>{{ option.label }}</text>
      </view>
    </view>

    <view v-if="loading" class="state-box"><text class="state-text">加载中...</text></view>
    <view v-else-if="samples.length === 0" class="state-box">
      <text class="empty-icon">🧪</text>
      <text class="state-text">暂无样本记录</text>
    </view>

    <view v-else class="sample-list">
      <view v-for="sample in samples" :key="sample.id" class="sample-card">
        <view class="sample-head" @click="toggleSample(sample.id)">
          <view>
            <text class="sample-code">{{ sample.sample_code }}</text>
            <text class="sample-meta">{{ sample.sample_type || '样本' }}</text>
          </view>
          <view class="head-right">
            <text class="sample-status" :class="'s-' + sample.sample_status">{{ statusMap[sample.sample_status] || sample.sample_status }}</text>
            <text class="arrow">{{ openedId === sample.id ? '⌃' : '⌄' }}</text>
          </view>
        </view>

        <view v-if="openedId === sample.id" class="sample-detail">
          <view v-for="step in buildTimeline(sample)" :key="step.key" class="timeline-row">
            <view class="dot" :class="{ done: step.done }"></view>
            <view class="line-content">
              <text class="step-title" :class="{ muted: !step.done }">{{ step.title }}</text>
              <text v-if="step.time" class="step-time">{{ step.time }}</text>
            </view>
          </view>
          <view class="detail-grid">
            <view class="info-row" v-if="sample.collection_date"><text class="lbl">采集日期</text><text class="val">{{ sample.collection_date }}</text></view>
            <view class="info-row" v-if="sample.receive_date"><text class="lbl">接收日期</text><text class="val">{{ sample.receive_date }}</text></view>
            <view class="info-row" v-if="sample.treatment_stage"><text class="lbl">治疗阶段</text><text class="val">{{ sample.treatment_stage }}</text></view>
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
      loading: true,
      openedId: null,
      selectedMonths: 3,
      rangeOptions: [
        { label: '1个月', value: 1 },
        { label: '3个月', value: 3 },
        { label: '6个月', value: 6 },
        { label: '1年', value: 12 }
      ],
      samples: [],
      statusMap: { pending: '已创建', created: '已创建', collected: '已采集', received: '已接收', processing: '检测中', tested: '已检测', completed: '报告完成' },
      statusOrder: { pending: 1, created: 1, collected: 2, received: 3, processing: 4, tested: 5, completed: 6 }
    }
  },
  onLoad() {
    this.loadSamples()
  },
  methods: {
    async loadSamples() {
      this.loading = true
      try {
        const res = await uniAPI.getSamples({ months: this.selectedMonths })
        if (res.success && res.data) {
          this.samples = res.data.list || []
          if (this.samples.length > 0) this.openedId = this.samples[0].id
        }
      } catch (e) {
        console.error('加载进度失败', e)
      } finally {
        this.loading = false
      }
    },
    toggleSample(id) {
      this.openedId = this.openedId === id ? null : id
    },
    changeRange(months) {
      if (this.selectedMonths === months) return
      this.selectedMonths = months
      this.openedId = null
      this.loadSamples()
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
.page-container { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.range-tabs { display: grid; grid-template-columns: repeat(4, 1fr); gap: 12rpx; margin-bottom: 24rpx; }
.range-tab { height: 64rpx; display: flex; align-items: center; justify-content: center; background: #fff; border: 1rpx solid #e5e9f0; border-radius: 12rpx; font-size: 24rpx; color: #606f7b; box-sizing: border-box; }
.range-tab.active { background: #1677ff; border-color: #1677ff; color: #fff; font-weight: 600; }
.state-box { display: flex; flex-direction: column; align-items: center; padding: 120rpx 0; }
.empty-icon { font-size: 76rpx; margin-bottom: 20rpx; }
.state-text { font-size: 26rpx; color: #8c9aa8; }
.sample-card { background: #fff; border-radius: 20rpx; margin-bottom: 24rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); overflow: hidden; }
.sample-head { display: flex; justify-content: space-between; align-items: center; gap: 20rpx; padding: 28rpx; }
.sample-code { display: block; font-size: 28rpx; color: #1f2d3d; font-weight: 600; margin-bottom: 8rpx; }
.sample-meta { display: block; font-size: 22rpx; color: #8c9aa8; }
.head-right { display: flex; align-items: center; gap: 14rpx; }
.sample-status { font-size: 22rpx; padding: 4rpx 16rpx; border-radius: 20rpx; }
.arrow { color: #c0c6cc; font-size: 28rpx; }
.s-pending, .s-created { background: #f5f5f5; color: #8c9aa8; }
.s-collected { background: #e6f7ff; color: #1677ff; }
.s-received, .s-tested, .s-completed { background: #f0f9eb; color: #52c41a; }
.s-processing { background: #fdf6ec; color: #e6a23c; }
.sample-detail { padding: 0 28rpx 28rpx; border-top: 1rpx solid #f0f2f5; }
.timeline-row { display: flex; gap: 20rpx; position: relative; padding: 24rpx 0 0; }
.dot { width: 20rpx; height: 20rpx; border-radius: 50%; background: #d9dde3; margin-top: 8rpx; }
.dot.done { background: #1677ff; }
.line-content { flex: 1; padding-bottom: 8rpx; }
.step-title { display: block; font-size: 26rpx; color: #1f2d3d; font-weight: 600; }
.step-title.muted { color: #a0b0c0; font-weight: 400; }
.step-time { display: block; font-size: 22rpx; color: #8c9aa8; margin-top: 6rpx; }
.detail-grid { margin-top: 12rpx; padding-top: 16rpx; border-top: 1rpx dashed #f0f2f5; }
.info-row { display: flex; justify-content: space-between; padding: 8rpx 0; }
.lbl { font-size: 24rpx; color: #8c9aa8; }
.val { font-size: 24rpx; color: #1f2d3d; font-weight: 500; }
</style>
