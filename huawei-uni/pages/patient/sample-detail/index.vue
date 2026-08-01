<template>
  <view class="page">
    <view v-if="loading" class="state"><text>加载中...</text></view>
    <view v-else-if="!sample" class="state"><text>样本不存在</text></view>
    <template v-else>
      <view class="summary">
        <view class="summary-head">
          <text class="code">{{ sample.sample_code }}</text>
          <text class="status">{{ statusMap[sample.sample_status] || sample.sample_status }}</text>
        </view>
        <view class="row"><text class="label">样本类型</text><text>{{ sample.sample_type || '-' }}</text></view>
        <view class="row"><text class="label">治疗阶段</text><text>{{ sample.treatment_stage || '-' }}</text></view>
      </view>

      <view class="section">
        <text class="section-title">样本状态</text>
        <view v-for="step in timeline" :key="step.key" class="timeline-row">
          <view class="rail"><view class="dot" :class="{ done: step.done }"></view><view class="line"></view></view>
          <view class="timeline-content">
            <text class="step-title" :class="{ muted: !step.done }">{{ step.title }}</text>
            <text v-if="step.time" class="meta">{{ step.time }}</text>
            <text v-if="step.detail" class="meta">{{ step.detail }}</text>
          </view>
        </view>
      </view>

      <view v-if="express" class="section">
        <text class="section-title">快递流程</text>
        <view class="row"><text class="label">运单号</text><text class="tracking">{{ express.tracking_number }}</text></view>
        <view class="row"><text class="label">当前状态</text><text>{{ express.latest_event_status || express.status || '-' }}</text></view>
        <view v-for="(event, index) in express.route || []" :key="index" class="event">
          <text class="event-status">{{ event.status }}</text>
          <text class="meta">{{ event.time }}</text>
        </view>
      </view>
    </template>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      id: 0,
      loading: true,
      sample: null,
      express: null,
      statusMap: { pending: '已创建', created: '已创建', collected: '已采集', received: '已接收', processing: '检测中', tested: '已检测', completed: '已完成' },
      statusOrder: { pending: 1, created: 1, collected: 2, received: 3, processing: 4, tested: 5, completed: 6 }
    }
  },
  computed: {
    timeline() {
      if (!this.sample) return []
      const sample = this.sample
      const current = this.statusOrder[sample.sample_status] || 0
      const hasReport = !!sample.report_generated_time
      return [
        { key: 'created', title: '样本创建', done: true, time: sample.sample_created_at || '' },
        { key: 'collected', title: '样本采集', done: current >= 2, time: sample.collection_date || '' },
        { key: 'transit', title: '运输中', done: !!this.express, time: this.express ? `单号：${this.express.tracking_number || '-'}` : '', detail: this.express ? (this.express.latest_event_status || '') : '暂无运单' },
        { key: 'received', title: '样本接收', done: current >= 3, time: sample.receive_date || '' },
        { key: 'tested', title: '样本检测', done: current >= 5 || !!sample.test_completed_at, time: sample.test_completed_at || '' },
        { key: 'report', title: hasReport ? '已出报告' : '未出报告', done: hasReport, time: sample.report_generated_time || '' },
        { key: 'review', title: '报告审核', done: !!sample.report_reviewed_time, time: sample.report_reviewed_time || (hasReport ? '待审核' : '') },
        { key: 'viewed', title: sample.patient_viewed ? '患者已查看' : '患者未查看', done: !!sample.patient_viewed, time: sample.patient_viewed_at || '' }
      ]
    }
  },
  onLoad(options) {
    this.id = Number(options.id || 0)
    this.load()
  },
  methods: {
    async load() {
      try {
        const [sampleRes, expressRes] = await Promise.all([
          uniAPI.getSamples(),
          uniAPI.getSampleExpress(this.id).catch(() => null)
        ])
        const list = sampleRes && sampleRes.success && sampleRes.data ? (sampleRes.data.list || []) : []
        this.sample = list.find(item => Number(item.id) === this.id) || null
        this.express = expressRes && expressRes.success ? expressRes.data : null
      } catch (error) {
        console.error('加载样本详情失败', error)
      } finally {
        this.loading = false
      }
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 28rpx; background: #f5f7fa; box-sizing: border-box; color: #1f2d3d; }
.state { padding: 160rpx 0; text-align: center; color: #8c9aa8; }
.summary, .section { padding: 28rpx; margin-bottom: 20rpx; background: #fff; border-radius: 16rpx; }
.summary-head { display: flex; align-items: center; justify-content: space-between; gap: 20rpx; padding-bottom: 20rpx; margin-bottom: 12rpx; border-bottom: 1rpx solid #edf0f3; }
.code { min-width: 0; font-size: 30rpx; font-weight: 700; overflow-wrap: anywhere; }
.status { flex-shrink: 0; padding: 6rpx 16rpx; color: #1677ff; background: #e6f4ff; border-radius: 8rpx; font-size: 22rpx; }
.row { display: flex; justify-content: space-between; gap: 24rpx; padding: 12rpx 0; font-size: 24rpx; }
.label, .meta { color: #7b8794; }
.tracking { overflow-wrap: anywhere; }
.section-title { display: block; margin-bottom: 14rpx; font-size: 28rpx; font-weight: 700; }
.timeline-row { display: flex; min-height: 100rpx; }
.rail { width: 32rpx; display: flex; flex-direction: column; align-items: center; flex-shrink: 0; }
.dot { width: 18rpx; height: 18rpx; margin-top: 8rpx; border-radius: 50%; background: #d9dde3; }
.dot.done { background: #1677ff; }
.line { width: 2rpx; flex: 1; margin: 8rpx 0; background: #e7eaee; }
.timeline-row:last-child .line { background: transparent; }
.timeline-content { flex: 1; min-width: 0; padding: 0 0 22rpx 18rpx; }
.step-title { display: block; font-size: 25rpx; font-weight: 600; }
.step-title.muted { color: #9aa6b2; font-weight: 400; }
.meta { display: block; margin-top: 6rpx; font-size: 22rpx; line-height: 1.5; overflow-wrap: anywhere; }
.event { padding: 18rpx 0; border-top: 1rpx solid #edf0f3; }
.event-status { display: block; font-size: 24rpx; line-height: 1.5; }
</style>
