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
      uni.navigateTo({ url: `/pages/patient/sample-detail/index?id=${item.id}` })
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

</style>
