<template>
  <view class="page">
    <view class="header">
      <text class="title">样本详情</text>
      <text class="desc">当前员工创建的全部样本</text>
    </view>
    <view v-if="loading" class="state">加载中...</view>
    <view v-else-if="samples.length === 0" class="state">暂无创建的样本</view>
    <view v-else class="list">
      <view v-for="item in samples" :key="item.id" class="card" @click="openDetail(item)">
        <view class="head">
          <text class="code">{{ item.sample_code }}</text>
          <text class="status">{{ statusMap[item.sample_status] || item.sample_status }}</text>
        </view>
        <view class="row"><text class="label">患者</text><text>{{ item.patient_name || '-' }}（{{ item.patient_code || '-' }}）</text></view>
        <view class="row"><text class="label">检测癌种</text><text>{{ item.cancer_type || '-' }}</text></view>
        <view class="row"><text class="label">样本类型</text><text>{{ item.sample_type || '-' }}</text></view>
        <view class="row"><text class="label">报告类型</text><text>{{ item.report_type_label || '-' }}</text></view>
        <view class="row"><text class="label">送检单位</text><text>{{ item.organization || '-' }}</text></view>
        <button v-if="item.can_delete" class="delete-btn" @click.stop="confirmDelete(item)">删除样本</button>
        <text v-else class="locked">已进入检测或报告流程，不可删除</text>
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
      samples: [],
      statusMap: { created: '已创建', collected: '已采集', received: '已接收', processing: '处理中', tested: '已检测', completed: '已完成' }
    }
  },
  onShow() { this.loadSamples() },
  methods: {
    async loadSamples() {
      this.loading = true
      try {
        const res = await uniAPI.getEmployeeSamples()
        this.samples = Array.isArray(res.data && res.data.list) ? res.data.list : []
      } catch (error) {
        uni.showToast({ title: error.message || '加载失败', icon: 'none' })
      } finally { this.loading = false }
    },
    openDetail(item) {
      uni.navigateTo({ url: `/pages/employee/sample-detail/index?id=${item.id}` })
    },
    confirmDelete(item) {
      uni.showActionSheet({
        itemList: ['管码可继续使用（进入号池）', '管码不可继续使用'],
        success: (choice) => {
          const reusable = choice.tapIndex === 0
          this.confirmDeleteChoice(item, reusable)
        }
      })
    },
    confirmDeleteChoice(item, reusable) {
      uni.showModal({
        title: '删除样本',
        content: reusable
          ? `确认删除样本 ${item.sample_code}？该管码会进入号池，后续可再次使用。`
          : `确认删除样本 ${item.sample_code}？该管码将被停用，后续不能再次使用。`,
        confirmColor: '#ff4d4f',
        success: async (res) => {
          if (!res.confirm) return
          try {
            const result = await uniAPI.deleteEmployeeSample(item.id, reusable)
            uni.showToast({ title: result.message || '删除成功', icon: 'success' })
            this.loadSamples()
          } catch (error) {
            // 请求层已经展示后端的精确错误，只避免再次弹出重复提示。
          }
        }
      })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.header { margin-bottom: 24rpx; }
.title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; }
.desc { display: block; margin-top: 8rpx; color: #8c9aa8; font-size: 24rpx; }
.state { padding: 140rpx 0; text-align: center; color: #8c9aa8; }
.list { display: flex; flex-direction: column; gap: 20rpx; }
.card { padding: 26rpx; border-radius: 18rpx; background: #fff; box-shadow: 0 2rpx 12rpx rgba(22,119,255,.06); }
.head { display: flex; justify-content: space-between; padding-bottom: 14rpx; margin-bottom: 8rpx; border-bottom: 1rpx solid #f0f2f5; }
.code { font-size: 29rpx; font-weight: 700; color: #1f2d3d; }
.status { color: #1677ff; font-size: 23rpx; }
.row { display: flex; gap: 16rpx; margin-top: 13rpx; font-size: 25rpx; color: #1f2d3d; }
.label { width: 120rpx; flex-shrink: 0; color: #8c9aa8; }
.delete-btn { margin: 22rpx 0 0; height: 66rpx; line-height: 66rpx; border: 1rpx solid #ffccc7; border-radius: 12rpx; color: #cf1322; background: #fff2f0; font-size: 24rpx; }
.locked { display: block; margin-top: 18rpx; color: #b0b6bd; font-size: 22rpx; text-align: right; }
</style>
