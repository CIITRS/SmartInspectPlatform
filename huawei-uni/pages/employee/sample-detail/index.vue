<template>
  <view class="page">
    <view v-if="loading" class="state">加载中...</view>
    <view v-else-if="!sample.id" class="state">样本不存在或无权查看</view>
    <view v-else class="card">
      <text class="code">{{ sample.sample_code }}</text>
      <view class="row"><text class="label">患者</text><text>{{ sample.patient_name || '-' }}（{{ sample.patient_code || '-' }}）</text></view>
      <view class="row"><text class="label">检测癌种</text><text>{{ sample.cancer_type || '-' }}</text></view>
      <view class="row"><text class="label">样本类型</text><text>{{ sample.sample_type || '-' }}</text></view>
      <view class="row"><text class="label">报告类型</text><text>{{ sample.report_type_label || '-' }}</text></view>
      <view class="row"><text class="label">送检单位</text><text>{{ sample.organization || '-' }}</text></view>
      <view class="row"><text class="label">治疗阶段</text><text>{{ sample.treatment_stage || '-' }}</text></view>
      <view class="row"><text class="label">采样日期</text><text>{{ sample.collection_date || '-' }}</text></view>
      <view class="row"><text class="label">接收日期</text><text>{{ sample.receive_date || '-' }}</text></view>
      <view class="row"><text class="label">备注</text><text>{{ sample.notes || '-' }}</text></view>
      <button v-if="sample.has_report" class="report-btn" @click="viewReport">查看报告</button>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'
export default {
  data() { return { id: 0, loading: true, sample: {} } },
  onLoad(options) { this.id = Number(options && options.id) || 0; this.loadDetail() },
  methods: {
    async loadDetail() {
      try {
        const res = await uniAPI.getEmployeeSampleDetail(this.id)
        this.sample = res.data || {}
      } catch (error) {
        this.sample = {}
      } finally { this.loading = false }
    },
    viewReport() {
      if (!this.sample.report_id) return
      uni.navigateTo({ url: `/pages/employee/report-detail/index?id=${this.sample.report_id}` })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.state { padding: 160rpx 0; text-align: center; color: #8c9aa8; }
.card { padding: 30rpx; border-radius: 20rpx; background: #fff; box-shadow: 0 2rpx 12rpx rgba(22,119,255,.06); }
.code { display: block; padding-bottom: 22rpx; margin-bottom: 10rpx; border-bottom: 1rpx solid #f0f2f5; color: #1f2d3d; font-size: 34rpx; font-weight: 700; }
.row { display: flex; gap: 18rpx; margin-top: 18rpx; font-size: 26rpx; line-height: 1.5; color: #1f2d3d; }
.label { width: 130rpx; flex-shrink: 0; color: #8c9aa8; }
.report-btn { margin-top: 34rpx; height: 82rpx; line-height: 82rpx; border: none; border-radius: 14rpx; background: #1677ff; color: #fff; font-size: 28rpx; }
</style>
