<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">报告查询</text>
      <text class="page-desc">查看所有检验报告</text>
    </view>

    <view class="search-bar">
      <input
        v-model="patientName"
        class="search-input"
        confirm-type="search"
        placeholder="输入患者姓名搜索"
        @confirm="loadReports"
      />
      <button v-if="patientName" class="clear-btn" @click="clearSearch">清空</button>
      <button class="search-btn" @click="loadReports">搜索</button>
    </view>

    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else-if="reports.length === 0" class="empty-container">
      <text class="empty-text">暂无报告记录</text>
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
          <text class="report-sample">样本: {{ item.sample_code || '-' }}</text>
          <text class="report-time">{{ formatDate(item.generated_time) || '-' }}</text>
          <text class="view-status" :class="{ viewed: item.patient_viewed }">
            {{ item.patient_viewed ? `患者已查阅${item.patient_viewed_at ? ' · ' + item.patient_viewed_at : ''}` : '患者未查阅' }}
          </text>
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
      patientName: '',
      statusMap: {
        draft: { text: '草稿', class: 'status-draft' },
        generated: { text: '已生成', class: 'status-ok' },
        pending: { text: '待审核', class: 'status-pending' },
        reviewed: { text: '已审核', class: 'status-ok' },
        published: { text: '已发布', class: 'status-ok' },
        rejected: { text: '已拒绝', class: 'status-error' },
        generating: { text: '生成中', class: 'status-pending' }
      }
    }
  },
  onLoad() {
    this.loadReports()
  },
  onShow() {
    if (!this.loading) {
      this.loadReports()
    }
  },
  methods: {
    async loadReports() {
      this.loading = true
      try {
        const response = await uniAPI.getEmployeeReports({
          patient_name: this.patientName.trim()
        })
        if (response.success && response.data) {
          this.reports = response.data.list || []
        }
      } catch (error) {
        console.error('Load reports failed:', error)
      } finally {
        this.loading = false
      }
    },
    clearSearch() {
      this.patientName = ''
      this.loadReports()
    },
    getStatusText(status) {
      const statusInfo = this.statusMap[status]
      return statusInfo ? statusInfo.text : status
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
        const date = new Date(dateStr)
        const year = date.getFullYear()
        const month = String(date.getMonth() + 1).padStart(2, '0')
        const day = String(date.getDate()).padStart(2, '0')
        return `${year}-${month}-${day}`
      } catch {
        return dateStr
      }
    },
    viewDetail(item) {
      uni.navigateTo({
        url: `/pages/employee/report-detail/index?id=${item.id}`
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

.search-bar {
  display: flex;
  align-items: center;
  gap: 12rpx;
  margin-bottom: 24rpx;
}

.search-input {
  flex: 1;
  height: 72rpx;
  padding: 0 24rpx;
  border-radius: 12rpx;
  background: #fff;
  font-size: 24rpx;
  color: #1f2d3d;
  box-sizing: border-box;
}

.search-btn,
.clear-btn {
  height: 72rpx;
  line-height: 72rpx;
  margin: 0;
  padding: 0 24rpx;
  border-radius: 12rpx;
  font-size: 24rpx;
}

.search-btn {
  color: #fff;
  background: #1677ff;
}

.clear-btn {
  color: #6b7785;
  background: #eef2f6;
}

.loading-container, .empty-container {
  display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 120rpx 0;
}
.loading-text { font-size: 28rpx; color: #8c9aa8; }
.empty-icon { font-size: 80rpx; margin-bottom: 24rpx; }
.empty-text { font-size: 28rpx; color: #1f2d3d; font-weight: 500; }

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
.view-status { display: inline-block; margin-top: 10rpx; padding: 5rpx 14rpx; border-radius: 999rpx; background: #fff7e6; color: #d46b08; font-size: 21rpx; }
.view-status.viewed { background: #f6ffed; color: #389e0d; }

.report-right {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.report-status {
  font-size: 22rpx;
  padding: 4rpx 16rpx;
  border-radius: 20rpx;
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
