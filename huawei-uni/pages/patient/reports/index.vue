<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">检验报告</text>
      <text class="page-desc">查看您的检验结果</text>
    </view>

    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else-if="reports.length === 0" class="empty-container">
      <text class="empty-icon">📋</text>
      <text class="empty-text">暂无检验报告</text>
      <text class="empty-desc">完成检测后报告将在此显示</text>
    </view>

    <view v-else class="list-container">
      <view v-for="item in reports" :key="item.id" class="report-card" @click="viewDetail(item)">
        <view class="report-left">
          <view class="report-header-row">
            <text class="report-type">{{ item.sample_code || '检验报告' }}</text>
            <text class="report-status" :class="getStatusClass(item.status)">
              {{ getStatusText(item.status) }}
            </text>
          </view>
          <text class="report-type-badge">{{ item.report_type_label || getReportTypeLabel(item.report_type) }}</text>
          <text class="report-patient">患者: {{ item.patient_name || '-' }}</text>
          <text v-if="item.status !== 'no_report'" class="report-time">{{ formatDate(item.generated_time) || '-' }}</text>
        </view>
        <view class="report-right">
          <text class="report-arrow">›</text>
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
      reports: [],
      loading: true,
      statusMap: {
        draft: { text: '草稿', class: 'status-draft' },
        generated: { text: '已生成', class: 'status-ok' },
        pending: { text: '待审核', class: 'status-pending' },
        reviewed: { text: '已审核', class: 'status-ok' },
        published: { text: '已发布', class: 'status-ok' },
        rejected: { text: '已拒绝', class: 'status-error' },
        no_report: { text: '未出报告', class: 'status-draft' }
      }
    }
  },
  onLoad() {
    this.loadReports()
  },
  onShow() {
    // 每次显示页面时刷新数据
    if (!this.loading) {
      this.loadReports()
    }
  },
  methods: {
    async loadReports() {
      this.loading = true
      try {
        const response = await uniAPI.getReports()
        if (response.success && response.data) {
          this.reports = response.data.list || []
        }
      } catch (error) {
        console.error('Load reports failed:', error)
        // 错误已在 request 中统一处理
      } finally {
        this.loading = false
      }
    },
    getStatusText(status) {
      const statusInfo = this.statusMap[status]
      return statusInfo ? statusInfo.text : '未知'
    },
    getStatusClass(status) {
      const statusInfo = this.statusMap[status]
      return statusInfo ? statusInfo.class : 'status-draft'
    },
    getReportTypeLabel(type) {
      if (type === 'high') return '超敏'
      if (type === 'screening') return '健康筛查'
      return '高敏'
    },
    formatDate(dateStr) {
      if (!dateStr) return ''
      try {
        let normalizedDateStr = dateStr
        if (normalizedDateStr.includes(' ') && !normalizedDateStr.includes('T')) {
          normalizedDateStr = normalizedDateStr.replace(' ', 'T')
          const parts = normalizedDateStr.split(':')
          if (parts.length === 1) {
            normalizedDateStr += 'T00:00:00'
          } else if (parts.length === 2) {
            normalizedDateStr += ':00'
          }
        }
        const date = new Date(normalizedDateStr)
        if (isNaN(date.getTime())) {
          return dateStr
        }
        const year = date.getFullYear()
        const month = String(date.getMonth() + 1).padStart(2, '0')
        const day = String(date.getDate()).padStart(2, '0')
        return `${year}-${month}-${day}`
      } catch {
        return dateStr
      }
    },
    viewDetail(item) {
      if (item.status === 'no_report') {
        uni.showToast({ title: '未出报告', icon: 'none' })
        return
      }
      if (item.status !== 'reviewed' && item.status !== 'published') {
        uni.showToast({ title: '报告暂不可查看', icon: 'none' })
        return
      }
      // 跳转到报告详情页
      uni.navigateTo({
        url: `/pages/patient/report-detail/index?id=${item.id}`
      })
    }
  }
}
</script>

<style scoped>
.page-container {
  padding: 32rpx;
  min-height: 100vh;
  background-color: #F5F7FA;
  box-sizing: border-box;
}

.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }

.loading-container, .empty-container {
  display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 120rpx 0;
}
.loading-text { font-size: 28rpx; color: #8c9aa8; }
.empty-icon { font-size: 80rpx; margin-bottom: 24rpx; }
.empty-text { font-size: 28rpx; color: #1f2d3d; font-weight: 500; margin-bottom: 8rpx; }
.empty-desc { font-size: 24rpx; color: #8c9aa8; }

.report-card {
  background: #fff;
  border-radius: 20rpx;
  padding: 28rpx;
  margin-bottom: 20rpx;
  box-shadow: 0 2rpx 12rpx rgba(22, 119, 255, 0.06);
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.report-left { flex: 1; }

.report-header-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8rpx;
}

.report-type {
  font-size: 28rpx;
  font-weight: 600;
  color: #1f2d3d;
}

.report-no {
  display: block;
  font-size: 22rpx;
  color: #8c9aa8;
  margin-bottom: 4rpx;
}

.report-patient {
  display: block;
  font-size: 22rpx;
  color: #8c9aa8;
  margin-bottom: 4rpx;
}

.report-type-badge {
  display: inline-flex;
  align-items: center;
  margin: 2rpx 0 10rpx;
  padding: 6rpx 18rpx;
  border-radius: 999rpx;
  background: #e8f7ef;
  color: #16a34a;
  font-size: 22rpx;
  font-weight: 600;
}

.report-sample {
  display: block;
  font-size: 22rpx;
  color: #8c9aa8;
  margin-bottom: 4rpx;
}

.report-time {
  display: block;
  font-size: 22rpx;
  color: #a0b0c0;
}

.report-right {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.report-status {
  font-size: 20rpx;
  padding: 4rpx 12rpx;
  border-radius: 16rpx;
}

.status-ok { background: #f0f9eb; color: #67c23a; }
.status-pending { background: #fdf6ec; color: #e6a23c; }
.status-draft { background: #f5f5f5; color: #8c9aa8; }
.status-error { background: #fef0f0; color: #f56c6c; }

.report-arrow {
  font-size: 32rpx;
  color: #c0c6cc;
}
</style>
