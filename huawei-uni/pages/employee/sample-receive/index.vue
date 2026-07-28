<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">样本接收</text>
      <text class="page-desc">接收待处理的样本</text>
    </view>

    <view class="scan-panel">
      <button class="scan-btn" @click="scanAndReceive">扫码接收样本</button>
      <button class="refresh-btn" @click="loadSamples">刷新</button>
    </view>

    <!-- 批量接收按钮 -->
    <view v-if="samples.length > 0" class="batch-action">
      <view class="select-all" @click="toggleSelectAll">
        <view class="checkbox" :class="{ checked: allSelected }">
          <text v-if="allSelected" class="check-icon">✓</text>
        </view>
        <text class="select-text">全选 ({{ selectedCount }}/{{ samples.length }})</text>
      </view>
      <button class="batch-btn" :disabled="selectedCount === 0" @click="batchReceive">
        批量接收 ({{ selectedCount }})
      </button>
    </view>

    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else-if="samples.length === 0" class="empty-container">
      <text class="empty-icon">📦</text>
      <text class="empty-text">暂无待接收样本</text>
      <text class="empty-desc">所有样本已接收完成</text>
    </view>

    <view v-else class="list-container">
      <view v-for="item in samples" :key="item.id" class="sample-card">
        <view class="sample-checkbox" @click="toggleSelect(item.sample_code)">
          <view class="checkbox" :class="{ checked: selectedIds.includes(item.sample_code) }">
            <text v-if="selectedIds.includes(item.sample_code)" class="check-icon">✓</text>
          </view>
        </view>
        <view class="sample-content" @click="viewDetail(item)">
          <view class="sample-header">
            <text class="sample-code">{{ item.sample_code }}</text>
            <text class="sample-status" :class="'s-' + item.sample_status">{{ statusMap[item.sample_status] || item.sample_status }}</text>
          </view>
          <view class="sample-info">
            <view class="info-row"><text class="lbl">患者姓名</text><text class="val">{{ item.patient_name || '-' }}</text></view>
            <view class="info-row"><text class="lbl">采样日期</text><text class="val">{{ item.collection_date || '-' }}</text></view>
            <view class="info-row" v-if="item.sample_type"><text class="lbl">样本类型</text><text class="val">{{ item.sample_type }}</text></view>
          </view>
        </view>
        <view class="sample-action">
          <button class="receive-btn" @click="receiveSample(item)">接收</button>
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
      samples: [],
      loading: true,
      selectedIds: [],
      statusMap: {
        created: '已创建',
        collected: '已采集',
        received: '已接收',
        processing: '检测中',
        tested: '已检测',
        completed: '已完成'
      }
    }
  },
  computed: {
    allSelected() {
      return this.samples.length > 0 && this.selectedIds.length === this.samples.length
    },
    selectedCount() {
      return this.selectedIds.length
    }
  },
  onLoad() {
    this.loadSamples()
  },
  onShow() {
    if (!this.loading) {
      this.loadSamples()
    }
  },
  methods: {
    async loadSamples() {
      this.loading = true
      this.selectedIds = []
      try {
        const response = await uniAPI.getPendingSamples()
        if (response.success && response.data) {
          this.samples = response.data.list || []
        }
      } catch (error) {
        console.error('Load samples failed:', error)
      } finally {
        this.loading = false
      }
    },
    toggleSelect(sampleCode) {
      const index = this.selectedIds.indexOf(sampleCode)
      if (index > -1) {
        this.selectedIds.splice(index, 1)
      } else {
        this.selectedIds.push(sampleCode)
      }
    },
    toggleSelectAll() {
      if (this.allSelected) {
        this.selectedIds = []
      } else {
        this.selectedIds = this.samples.map(item => item.sample_code)
      }
    },
    viewDetail(item) {
      uni.showToast({ title: `样本 ${item.sample_code}`, icon: 'none' })
    },
    scanAndReceive() {
      uni.scanCode({
        onlyFromCamera: true,
        scanType: ['barCode', 'qrCode'],
        success: (res) => {
          const sampleCode = String(res.result || '').trim()
          if (!sampleCode) {
            uni.showToast({ title: '未识别到样本编号', icon: 'none' })
            return
          }
          this.confirmReceiveByCode(sampleCode)
        },
        fail: () => {
          uni.showToast({ title: '扫码已取消', icon: 'none' })
        }
      })
    },
    confirmReceiveByCode(sampleCode) {
      uni.showModal({
        title: '扫码结果',
        content: `识别到样本 ${sampleCode}，是否确认接收？`,
        success: async (res) => {
          if (res.confirm) {
            await this.receiveByCode(sampleCode)
          }
        }
      })
    },
    async receiveByCode(sampleCode) {
      uni.showLoading({ title: '接收中...' })
      try {
        const response = await uniAPI.receiveSample({ sample_code: sampleCode })
        uni.hideLoading()
        if (response.success) {
          const data = response.data || {}
          uni.showModal({
            title: '接收成功',
            content: `该样本的检测癌种为：${data.cancer_type_name || '-'}，需要测试的Panel为：${data.panel_summary || '-'}`,
            showCancel: false
          })
          this.loadSamples()
        } else {
          uni.showToast({ title: response.message || '接收失败', icon: 'none' })
        }
      } catch (error) {
        uni.hideLoading()
        uni.showToast({ title: error.message || '网络错误', icon: 'none' })
      }
    },
    async receiveSample(item) {
      uni.showModal({
        title: '确认接收',
        content: `确定要接收样本 ${item.sample_code} 吗？`,
        success: async (res) => {
          if (res.confirm) {
            await this.receiveByCode(item.sample_code)
          }
        }
      })
    },
    async batchReceive() {
      if (this.selectedIds.length === 0) {
        uni.showToast({ title: '请选择要接收的样本', icon: 'none' })
        return
      }
      uni.showModal({
        title: '批量接收',
        content: `确定要接收选中的 ${this.selectedIds.length} 个样本吗？`,
        success: async (res) => {
          if (res.confirm) {
            try {
              const response = await uniAPI.batchReceiveSamples({ sample_codes: this.selectedIds })
              if (response.success) {
                const groups = response.data && response.data.panel_groups ? response.data.panel_groups : []
                const content = groups.length
                  ? groups.map(group => `${group.panel} 需要检测的样本为：${(group.sample_codes || []).join('，')}`).join('\n')
                  : '批量接收成功'
                uni.showModal({
                  title: '批量接收成功',
                  content,
                  showCancel: false
                })
                this.loadSamples()
              } else {
                uni.showToast({ title: response.message || '批量接收失败', icon: 'none' })
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

.page-header { margin-bottom: 24rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }

.scan-panel {
  display: flex;
  gap: 16rpx;
  margin-bottom: 24rpx;
}

.scan-btn {
  flex: 1;
  height: 88rpx;
  border-radius: 16rpx;
  background: #1677ff;
  color: #fff;
  border: none;
  font-size: 28rpx;
  font-weight: 600;
}

.refresh-btn {
  width: 160rpx;
  height: 88rpx;
  border-radius: 16rpx;
  background: #fff;
  color: #1677ff;
  border: 2rpx solid #1677ff;
  font-size: 26rpx;
}

.batch-action {
  display: flex;
  justify-content: space-between;
  align-items: center;
  background: #fff;
  border-radius: 16rpx;
  padding: 20rpx 24rpx;
  margin-bottom: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
}

.select-all {
  display: flex;
  align-items: center;
  gap: 12rpx;
}

.checkbox {
  width: 40rpx;
  height: 40rpx;
  border: 2rpx solid #d9d9d9;
  border-radius: 8rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
}

.checkbox.checked {
  background-color: #1677ff;
  border-color: #1677ff;
}

.check-icon {
  color: #fff;
  font-size: 24rpx;
  font-weight: bold;
}

.select-text {
  font-size: 24rpx;
  color: #1f2d3d;
}

.batch-btn {
  height: 64rpx;
  padding: 0 24rpx;
  background-color: #1677ff;
  color: #fff;
  border: none;
  border-radius: 12rpx;
  font-size: 24rpx;
}

.batch-btn[disabled] {
  background-color: #d9d9d9;
}

.loading-container, .empty-container {
  display: flex; flex-direction: column; align-items: center; justify-content: center; padding: 120rpx 0;
}
.loading-text { font-size: 28rpx; color: #8c9aa8; }
.empty-icon { font-size: 80rpx; margin-bottom: 24rpx; }
.empty-text { font-size: 28rpx; color: #1f2d3d; font-weight: 500; margin-bottom: 8rpx; }
.empty-desc { font-size: 24rpx; color: #8c9aa8; }

.sample-card {
  display: flex;
  align-items: center;
  background: #fff;
  border-radius: 20rpx;
  padding: 24rpx;
  margin-bottom: 16rpx;
  box-shadow: 0 2rpx 12rpx rgba(22, 119, 255, 0.06);
}

.sample-checkbox {
  padding-right: 16rpx;
}

.sample-content {
  flex: 1;
}

.sample-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12rpx;
}

.sample-code {
  font-size: 28rpx;
  font-weight: 600;
  color: #1f2d3d;
}

.sample-status {
  font-size: 22rpx;
  padding: 4rpx 16rpx;
  border-radius: 20rpx;
}

.s-created { background: #f5f5f5; color: #8c9aa8; }
.s-collected { background: #e6f7ff; color: #1677ff; }
.s-received { background: #f0f9eb; color: #67c23a; }

.sample-info {
  padding-left: 40rpx;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 6rpx 0;
}

.lbl {
  font-size: 22rpx;
  color: #8c9aa8;
}

.val {
  font-size: 22rpx;
  color: #1f2d3d;
}

.sample-action {
  padding-left: 16rpx;
}

.receive-btn {
  height: 64rpx;
  padding: 0 24rpx;
  background-color: #e6f7ff;
  color: #1677ff;
  border: none;
  border-radius: 12rpx;
  font-size: 24rpx;
}
</style>
