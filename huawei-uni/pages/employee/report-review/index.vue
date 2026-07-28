<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">报告审核</text>
      <text class="page-desc">审核待审核的检验报告</text>
    </view>

    <view class="toolbar">
      <button class="refresh-btn" @click="loadReports">刷新</button>
    </view>

    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else-if="reports.length === 0" class="empty-container">
      <text class="empty-icon">✅</text>
      <text class="empty-text">暂无待审核报告</text>
      <text class="empty-desc">所有报告已审核完成</text>
    </view>

    <view v-else class="list-container">
      <view v-for="item in reports" :key="item.id" class="report-card">
        <view class="report-header" @click="viewDetail(item)">
          <view class="report-info">
            <text class="report-no">{{ item.sample_code || '未知样本' }}</text>
            <text class="report-type-badge">{{ item.report_type_label || getReportTypeLabel(item.report_type) }}</text>
            <text class="report-patient">患者: {{ item.patient_name || '-' }}</text>
          </view>
          <text class="report-status">{{ statusMap[item.status] || '待审核' }}</text>
          <text class="report-arrow">›</text>
        </view>
        <view class="report-body">
          <view class="info-row">
            <text class="info-label">生成时间</text>
            <text class="info-value">{{ item.generated_time || '-' }}</text>
          </view>
          <view class="info-row">
            <text class="info-label">生成人员</text>
            <text class="info-value">{{ item.generated_by || '-' }}</text>
          </view>
        </view>
        <view class="report-actions">
          <button class="action-btn reject-btn" @click="rejectReport(item)">拒绝</button>
          <button class="action-btn approve-btn" @click="approveReport(item)">审核通过</button>
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
        pending: '待审核',
        generated: '待审核',
        reviewed: '已审核',
        rejected: '已拒绝'
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
        const response = await uniAPI.getPendingReports()
        if (response.success && response.data) {
          this.reports = response.data.list || []
        }
      } catch (error) {
        console.error('Load pending reports failed:', error)
      } finally {
        this.loading = false
      }
    },
    viewDetail(item) {
      uni.navigateTo({
        url: `/pages/employee/report-detail/index?id=${item.id}`
      })
    },
    getReportTypeLabel(type) {
      if (type === 'high') return '超敏'
      if (type === 'screening') return '健康筛查'
      return '高敏'
    },
    async selectReviewerIfNeeded() {
      const res = await uniAPI.getReportReviewers()
      const data = res.data || {}
      if (!data.requires_reviewer) return null
      const reviewers = data.list || []
      if (!reviewers.length) {
        uni.showToast({ title: '暂无可选真实审核人', icon: 'none' })
        throw new Error('no reviewers')
      }
      const itemList = reviewers.map(item => `${item.name || item.username}（${item.employee_id || item.username || '-'}）`)
      const selectedIndex = await new Promise((resolve, reject) => {
        uni.showActionSheet({
          itemList,
          success: (actionRes) => resolve(actionRes.tapIndex),
          fail: reject
        })
      })
      return reviewers[selectedIndex] && reviewers[selectedIndex].id
    },
    async approveReport(item) {
      uni.showModal({
        title: '确认审核',
        content: `确定要审核通过报告 ${item.sample_code} 吗？`,
        success: async (res) => {
          if (res.confirm) {
            try {
              const reviewerId = await this.selectReviewerIfNeeded()
              const payload = { status: 'reviewed' }
              if (reviewerId) payload.reviewer_id = reviewerId
              const response = await uniAPI.reviewReport(item.id, payload)
              if (response.success) {
                uni.showToast({ title: '审核通过', icon: 'success' })
                this.loadReports()
              } else {
                uni.showToast({ title: response.message || '审核失败', icon: 'none' })
              }
            } catch (error) {
              if (String(error && error.errMsg || error && error.message || '').includes('cancel')) return
              uni.showToast({ title: '网络错误', icon: 'none' })
            }
          }
        }
      })
    },
    async rejectReport(item) {
      uni.showModal({
        title: '拒绝审核',
        content: '请输入拒绝原因',
        editable: true,
        success: async (res) => {
          if (res.confirm) {
            const reason = String(res.content || '').trim()
            if (!reason) {
              uni.showToast({ title: '请填写拒绝原因', icon: 'none' })
              return
            }
            try {
              const response = await uniAPI.reviewReport(item.id, { status: 'rejected', rejectedReason: reason })
              if (response.success) {
                uni.showToast({ title: '已拒绝', icon: 'success' })
                this.loadReports()
              } else {
                uni.showToast({ title: response.message || '操作失败', icon: 'none' })
              }
            } catch (error) {
              uni.showToast({ title: '网络错误', icon: 'none' })
            }
          }
        }
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

.toolbar {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 20rpx;
}

.refresh-btn {
  width: 160rpx;
  height: 72rpx;
  border-radius: 12rpx;
  background: #fff;
  color: #1677ff;
  border: 2rpx solid #1677ff;
  font-size: 26rpx;
}

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
}

.report-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20rpx;
  padding-bottom: 16rpx;
  border-bottom: 1rpx solid #f0f2f5;
}

.report-info {
  flex: 1;
}

.report-no {
  display: block;
  font-size: 28rpx;
  font-weight: 600;
  color: #1f2d3d;
  margin-bottom: 4rpx;
}

.report-patient {
  display: block;
  font-size: 22rpx;
  color: #8c9aa8;
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

.report-arrow {
  font-size: 32rpx;
  color: #c0c6cc;
  margin-left: 12rpx;
}

.report-status {
  font-size: 22rpx;
  padding: 6rpx 14rpx;
  border-radius: 20rpx;
  background: #fff7e6;
  color: #d46b08;
  margin-left: 16rpx;
}

.report-body {
  margin-bottom: 20rpx;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 8rpx 0;
}

.info-label {
  font-size: 24rpx;
  color: #8c9aa8;
}

.info-value {
  font-size: 24rpx;
  color: #1f2d3d;
}

.report-actions {
  display: flex;
  gap: 16rpx;
}

.action-btn {
  flex: 1;
  height: 72rpx;
  border-radius: 12rpx;
  font-size: 26rpx;
  font-weight: 500;
  border: none;
}

.reject-btn {
  background-color: #fef0f0;
  color: #f56c6c;
}

.approve-btn {
  background-color: #e6f7ff;
  color: #1677ff;
}
</style>
