<template>
  <view class="page-container">
    <view v-if="loading" class="loading-container">
      <text class="loading-text">加载中...</text>
    </view>

    <view v-else class="detail-container">
      <view class="report-header">
        <view class="header-top">
          <text class="report-title">{{ report.report_type || '检验报告' }}</text>
          <text class="report-status" :class="'s-' + report.status">{{ statusMap[report.status] || report.status }}</text>
        </view>
        <view class="sample-info">
          <text class="sample-label">样本编号</text>
          <text class="sample-value">{{ report.sample_code || '-' }}</text>
        </view>
        <text class="report-type-badge">{{ report.report_type_label || getReportTypeLabel(report.report_type) }}</text>
      </view>

      <view class="section">
        <view class="section-title">患者信息</view>
        <view class="info-grid-3">
          <view class="info-item">
            <text class="lbl">姓名</text>
            <text class="val">{{ report.patient_name || '-' }}</text>
          </view>
          <view class="info-item">
            <text class="lbl">性别</text>
            <text class="val">{{ report.patient_gender || '-' }}</text>
          </view>
          <view class="info-item">
            <text class="lbl">年龄</text>
            <text class="val">{{ report.patient_age || '-' }}</text>
          </view>
        </view>
        <view class="info-grid-2">
          <view class="info-item">
            <text class="lbl">样本类型</text>
            <text class="val">{{ report.sample_type || '-' }}</text>
          </view>
          <view class="info-item">
            <text class="lbl">采样时间</text>
            <text class="val">{{ report.collection_time || '-' }}</text>
          </view>
        </view>
      </view>

      <view class="section" v-if="report.calculation_result !== undefined">
        <view class="section-title">检测结果</view>
        <view class="result-card">
          <view class="result-item">
            <text class="result-label">信号值</text>
            <text class="result-value">{{ formatNumber(report.calculation_result) }}</text>
          </view>
        </view>
        <view class="explanation-box" v-if="report.signal_value_explanation">
          <text class="explanation-title">信号值说明</text>
          <text class="explanation-content" user-select="true">{{ report.signal_value_explanation }}</text>
        </view>
      </view>

      <view class="section" v-if="report.items && report.items.length > 0">
        <view class="section-title">检测项目</view>
        <view class="items-table">
          <view class="table-header">
            <text class="th th-name">项目名称</text>
            <text class="th">结果</text>
            <text class="th">参考值</text>
          </view>
          <view v-for="(item, index) in report.items" :key="index" class="table-row">
            <text class="td td-name">{{ item.item_name || '-' }}</text>
            <text class="td" :class="{ abnormal: item.is_abnormal }">{{ item.result || '-' }}</text>
            <text class="td td-ref">{{ item.reference || '-' }}</text>
          </view>
        </view>
      </view>

      <view class="section" v-if="report.result_explanation">
        <view class="section-title">结果说明</view>
        <view class="result-explanation">
          <text user-select="true">{{ report.result_explanation }}</text>
        </view>
      </view>

      <view class="section" v-if="trendPoints.length > 1">
        <view class="section-title">历史趋势</view>
        <canvas canvas-id="trendChart" id="trendChart" class="trend-chart"></canvas>
        <view class="trend-row" v-for="(item, index) in trendPoints" :key="index">
          <text class="trend-time">{{ item.time || '-' }}</text>
          <text class="trend-signal">{{ formatNumber(item.signal) }}</text>
          <text class="trend-mark" :class="trendClass(item.trend)">{{ item.trend || '-' }}</text>
          <text class="trend-type">{{ item.type || '-' }}</text>
        </view>
      </view>

      <view class="section">
        <view class="section-title">报告信息</view>
        <view class="info-list">
          <view class="info-row">
            <text class="lbl">报告时间</text>
            <text class="val">{{ report.report_time || report.created_at || '-' }}</text>
          </view>
          <view class="info-row">
            <text class="lbl">检验者</text>
            <text class="val">{{ report.inspector || report.tested_by || '-' }}</text>
          </view>
          <view class="info-row">
            <text class="lbl">审核者</text>
            <text class="val">{{ report.reviewer || report.reviewed_by || '-' }}</text>
          </view>
        </view>
      </view>

      <view v-if="report.status === 'reviewed' || report.status === 'published'" class="action-bar">
        <button class="action-btn preview" @click="viewPDF">查看PDF报告</button>
        <button class="action-btn manual" @click="downloadInstruction">下载说明书</button>
      </view>
    </view>

    <!-- 报告预览模态框 -->
    <view v-if="previewVisible" class="preview-modal" @click="closePreview">
      <view class="preview-content" @click.stop>
        <view class="preview-header">
          <text class="preview-title">报告预览</text>
          <text class="preview-close" @click="closePreview">×</text>
        </view>
        <scroll-view class="preview-body" scroll-y>
          <view class="preview-image-wrap">
            <text v-if="previewLoading" class="preview-loading">生成预览中...</text>
            <image
              v-else-if="previewImageUrl"
              class="preview-image"
              :src="previewImageUrl"
              mode="widthFix"
              show-menu-by-longpress
            />
            <text v-else class="preview-loading">暂无预览图片</text>
          </view>
        </scroll-view>
      </view>
    </view>
  </view>
</template>

<script>
import { BASE_URL, uniAPI } from '../../../api/index.js'

const API_ORIGIN = BASE_URL.replace(/\/api\/?$/, '')

const resolveDownloadUrl = (url) => {
  if (!url) return ''
  if (/^https?:\/\//i.test(url)) return url
  return API_ORIGIN + (url.startsWith('/') ? url : `/${url}`)
}

export default {
  data() {
    return {
      report: {},
      loading: true,
      previewVisible: false,
      previewImageUrl: '',
      previewLoading: false,
      statusMap: {
        draft: '草稿',
        generated: '已生成',
        pending: '待审核',
        reviewed: '已审核',
        published: '已发布',
        rejected: '已拒绝',
        generating: '生成中'
      }
    }
  },
  computed: {
    trendPoints() {
      const values = Array.isArray(this.report.trend_values) ? this.report.trend_values : []
      return values
        .filter(item => item &&
          String(item.time || '').trim() &&
          item.signal !== undefined &&
          item.signal !== null &&
          Number.isFinite(Number(item.signal)))
        .map(item => ({
          ...item,
          signal: Number(item.signal || 0),
          time: item.time || ''
        }))
        .sort((a, b) => String(a.time || '').localeCompare(String(b.time || '')))
    }
  },
  onLoad(options) {
    if (options && options.id) {
      this.loadReport(options.id)
    }
  },
  onShareAppMessage() {
    return {
      title: `${this.report.patient_name || ''}检验报告`,
      path: `/pages/patient/report-detail/index?id=${this.report.id || ''}`
    }
  },
  methods: {
    formatNumber(num) {
      if (num === undefined || num === null) return '-'
      return Number(num).toFixed(1)
    },
    getReportTypeLabel(type) {
      if (type === 'high') return '超敏'
      if (type === 'screening') return '健康筛查'
      return '高敏'
    },
    async loadReport(id) {
      this.loading = true
      try {
        const response = await uniAPI.getReportDetail(id)
        if (response.success && response.data) {
          this.report = response.data
          this.$nextTick(() => this.drawTrendChart())
        }
      } catch (error) {
        console.error('Load report detail failed:', error)
      } finally {
        this.loading = false
      }
    },
    async previewReport() {
      this.previewVisible = true
      if (this.previewImageUrl || this.previewLoading) return
      this.previewLoading = true
      try {
        const response = await uniAPI.getReportPreviewImage(this.report.id)
        const rawUrl = response.data?.url
        if (response.success && rawUrl) {
          this.previewImageUrl = resolveDownloadUrl(rawUrl)
        } else {
          uni.showToast({ title: '生成预览失败', icon: 'none' })
        }
      } catch (error) {
        console.error('生成预览失败:', error)
        uni.showToast({ title: '生成预览失败', icon: 'none' })
      } finally {
        this.previewLoading = false
      }
    },
    closePreview() {
      this.previewVisible = false
    },
    trendClass(trend) {
      if (trend === '↑') return 'up'
      if (trend === '↓') return 'down'
      return ''
    },
    drawTrendChart() {
      const points = this.trendPoints
      if (points.length < 2) return
      const ctx = uni.createCanvasContext('trendChart', this)
      const width = 320
      const height = 150
      const padding = 28
      const values = points.map(item => item.signal)
      const minValue = Math.min(...values, 0)
      const maxValue = Math.max(...values, 50)
      const span = maxValue - minValue || 1
      const xStep = (width - padding * 2) / (points.length - 1)
      const toY = value => height - padding - ((value - minValue) / span) * (height - padding * 2)

      ctx.clearRect(0, 0, width, height)
      ctx.setStrokeStyle('#d8e0ea')
      ctx.setLineWidth(1)
      ctx.moveTo(padding, padding)
      ctx.lineTo(padding, height - padding)
      ctx.lineTo(width - padding, height - padding)
      ctx.stroke()

      ctx.beginPath()
      ctx.setStrokeStyle('#1677ff')
      ctx.setLineWidth(2)
      points.forEach((item, index) => {
        const x = padding + index * xStep
        const y = toY(item.signal)
        if (index === 0) ctx.moveTo(x, y)
        else ctx.lineTo(x, y)
      })
      ctx.stroke()

      points.forEach((item, index) => {
        const x = padding + index * xStep
        const y = toY(item.signal)
        ctx.beginPath()
        ctx.setFillStyle('#1677ff')
        ctx.arc(x, y, 4, 0, Math.PI * 2)
        ctx.fill()
        ctx.setFillStyle('#5f6f7f')
        ctx.setFontSize(10)
        ctx.fillText(String(item.signal.toFixed(1)), x - 10, y - 8)
      })
      ctx.draw()
    },
    openPdf(downloadUrl, fileName) {
      const sessionId = uni.getStorageSync('miniapp_session_id')
      const safeName = `${fileName || 'report'}_${Date.now()}.pdf`
      const filePath = typeof wx !== 'undefined' && wx.env && wx.env.USER_DATA_PATH
        ? `${wx.env.USER_DATA_PATH}/${safeName}`
        : undefined
      uni.downloadFile({
        url: downloadUrl,
        filePath,
        header: {
          'X-Miniapp-Session': sessionId || ''
        },
        success: (res) => {
          uni.hideLoading()
          if (res.statusCode === 200) {
            const localPath = res.filePath || res.tempFilePath
            uni.openDocument({
              filePath: localPath,
              fileType: 'pdf',
              showMenu: false,
              success: () => {},
              fail: (err) => {
                console.error('打开PDF失败:', err)
                uni.showToast({ title: '打开PDF失败', icon: 'none' })
              }
            })
          } else {
            uni.showToast({ title: '下载失败', icon: 'none' })
          }
        },
        fail: (err) => {
          uni.hideLoading()
          console.error('下载失败:', err)
          uni.showToast({ title: '下载失败', icon: 'none' })
        }
      })
    },
    async viewPDF() {
      uni.showLoading({ title: '打开中...' })
      try {
        const response = await uniAPI.downloadReportPDF(this.report.id, 'view')
        const rawUrl = response.data?.url || response.data?.downloadUrl
        if (response.success && rawUrl) {
          const downloadUrl = resolveDownloadUrl(rawUrl)
          this.openPdf(downloadUrl, response.data?.report_name || `report_${this.report.id}`)
        } else {
          uni.hideLoading()
          uni.showToast({ title: '获取下载链接失败', icon: 'none' })
        }
      } catch (error) {
        uni.hideLoading()
        console.error('下载PDF失败:', error)
        uni.showToast({ title: '下载失败', icon: 'none' })
      }
    },
    downloadInstruction() {
      uni.showLoading({ title: '下载中...' })
      this.openPdf(`${API_ORIGIN}/Template/ReportInstruction.pdf`, 'ReportInstruction')
    }
  }
}
</script>

<style scoped>
.page-container {
  min-height: 100vh;
  background-color: #F5F7FA;
  padding-bottom: 120rpx;
}

.loading-container {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  padding: 120rpx 0;
}

.loading-text { font-size: 28rpx; color: #8c9aa8; }

.detail-container {
  padding: 24rpx;
}

.report-header {
  background: linear-gradient(135deg, #1677ff 0%, #69c0ff 100%);
  border-radius: 20rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
}

.header-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16rpx;
}

.report-title {
  font-size: 32rpx;
  font-weight: 700;
  color: #fff;
}

.report-status {
  font-size: 22rpx;
  padding: 6rpx 16rpx;
  border-radius: 20rpx;
  background: rgba(255, 255, 255, 0.3);
  color: #fff;
}

.report-no, .sample-no {
  display: block;
  font-size: 24rpx;
  color: rgba(255, 255, 255, 0.8);
  margin-bottom: 4rpx;
}

.sample-info {
  display: flex;
  align-items: center;
  margin-top: 12rpx;
  padding-top: 12rpx;
  border-top: 1rpx solid rgba(255, 255, 255, 0.2);
}

.sample-label {
  font-size: 24rpx;
  color: rgba(255, 255, 255, 0.7);
  margin-right: 12rpx;
}

.sample-value {
  font-size: 26rpx;
  font-weight: 600;
  color: #fff;
}

.report-type-badge {
  display: inline-flex;
  align-items: center;
  align-self: flex-start;
  margin-top: 18rpx;
  padding: 6rpx 18rpx;
  border-radius: 999rpx;
  background: #e8f7ef;
  color: #16a34a;
  font-size: 22rpx;
  font-weight: 600;
}

.section {
  background: #fff;
  border-radius: 20rpx;
  padding: 24rpx;
  margin-bottom: 20rpx;
  box-shadow: 0 2rpx 12rpx rgba(22, 119, 255, 0.06);
}

.section-title {
  font-size: 28rpx;
  font-weight: 600;
  color: #1f2d3d;
  margin-bottom: 20rpx;
  padding-bottom: 12rpx;
  border-bottom: 1rpx solid #f0f2f5;
}

.info-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16rpx;
}

.info-grid-3 {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16rpx;
  margin-bottom: 16rpx;
}

.info-grid-2 {
  display: grid;
  grid-template-columns: repeat(2, 1fr);
  gap: 16rpx;
}

.info-item {
  display: flex;
  flex-direction: column;
}

.info-item .lbl {
  font-size: 22rpx;
  color: #8c9aa8;
  margin-bottom: 4rpx;
}

.info-item .val {
  font-size: 24rpx;
  color: #1f2d3d;
}

.items-table {
  border-radius: 12rpx;
  overflow: hidden;
}

.table-header {
  display: flex;
  background-color: #f5f7fa;
}

.th {
  flex: 1;
  padding: 16rpx 12rpx;
  font-size: 22rpx;
  font-weight: 600;
  color: #8c9aa8;
  text-align: center;
}

.table-row {
  display: flex;
  border-bottom: 1rpx solid #f0f2f5;
}

.table-row:last-child {
  border-bottom: none;
}

.td {
  flex: 1;
  padding: 16rpx 12rpx;
  font-size: 24rpx;
  color: #1f2d3d;
  text-align: center;
}

.td.abnormal {
  color: #f56c6c;
  font-weight: 600;
}

.th-name {
  flex: 2;
}

.td-name {
  flex: 2;
}

.td-ref {
  font-size: 22rpx;
}

.result-card {
  background: linear-gradient(135deg, #e6f7ff 0%, #f0f9ff 100%);
  border-radius: 16rpx;
  padding: 24rpx;
  margin-bottom: 20rpx;
}

.result-item {
  display: flex;
  flex-direction: column;
  align-items: center;
}

.result-label {
  font-size: 24rpx;
  color: #8c9aa8;
  margin-bottom: 8rpx;
}

.result-value {
  font-size: 48rpx;
  font-weight: 700;
  color: #1677ff;
}

.explanation-box {
  background: #f9f9f9;
  border-radius: 12rpx;
  padding: 20rpx;
}

.explanation-title {
  display: block;
  font-size: 24rpx;
  font-weight: 600;
  color: #1f2d3d;
  margin-bottom: 12rpx;
}

.explanation-content {
  display: block;
  font-size: 24rpx;
  color: #595959;
  line-height: 1.6;
  white-space: pre-wrap;
}

.result-explanation {
  background: #fafafa;
  border-radius: 12rpx;
  padding: 20rpx;
  font-size: 24rpx;
  color: #595959;
  line-height: 1.6;
  white-space: pre-wrap;
}

.trend-row {
  display: grid;
  grid-template-columns: 2fr 1.2fr 0.7fr 1.2fr;
  align-items: center;
  gap: 12rpx;
  padding: 16rpx 0;
  border-bottom: 1rpx solid #f0f2f5;
}
.trend-chart { width: 100%; height: 300rpx; margin: 10rpx 0 18rpx; background: #f8fbff; border-radius: 12rpx; }
.trend-row:last-child { border-bottom: none; }
.trend-time, .trend-type { font-size: 24rpx; color: #606f7b; }
.trend-signal { font-size: 26rpx; font-weight: 600; color: #1f2d3d; text-align: center; }
.trend-mark { font-size: 30rpx; font-weight: 700; text-align: center; color: #8c9aa8; }
.trend-mark.up { color: #f56c6c; }
.trend-mark.down { color: #52c41a; }

.empty-items {
  text-align: center;
  padding: 40rpx;
  font-size: 26rpx;
  color: #8c9aa8;
}

.info-list {
  display: flex;
  flex-direction: column;
}

.info-row {
  display: flex;
  justify-content: space-between;
  padding: 12rpx 0;
}

.info-row .lbl {
  font-size: 24rpx;
  color: #8c9aa8;
}

.info-row .val {
  font-size: 24rpx;
  color: #1f2d3d;
}

.action-bar {
  position: relative;
  display: flex;
  gap: 24rpx;
  margin-top: 24rpx;
  padding: 20rpx 32rpx;
  background: #fff;
  box-shadow: 0 -2rpx 12rpx rgba(0, 0, 0, 0.06);
}

.action-btn {
  flex: 1;
  height: 88rpx;
  border-radius: 16rpx;
  font-size: 30rpx;
  font-weight: 500;
  border: none;
}

.action-btn.preview {
  background-color: #f0f5ff;
  color: #1677ff;
  border: 2rpx solid #1677ff;
}

.action-btn.download {
  background-color: #1677ff;
  color: #fff;
}

.action-btn.manual {
  background-color: #f6ffed;
  color: #52c41a;
  border: 2rpx solid #52c41a;
}

/* 预览模态框样式 */
.preview-modal {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.preview-content {
  width: 90%;
  max-height: 80vh;
  background: #fff;
  border-radius: 20rpx;
  overflow: hidden;
}

.preview-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 24rpx 32rpx;
  background: linear-gradient(135deg, #1677ff 0%, #69c0ff 100%);
}

.preview-title {
  font-size: 30rpx;
  font-weight: 600;
  color: #fff;
}

.preview-close {
  font-size: 48rpx;
  color: rgba(255, 255, 255, 0.8);
  line-height: 1;
}

.preview-body {
  max-height: calc(80vh - 80rpx);
}

.preview-image-wrap {
  width: 100%;
  min-height: 520rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #f5f7fa;
}

.preview-image {
  width: 100%;
  background: #fff;
}

.preview-loading {
  font-size: 28rpx;
  color: #8c9aa8;
}

.preview-report {
  padding: 24rpx;
}

.preview-section {
  margin-bottom: 24rpx;
}

.preview-section:last-child {
  margin-bottom: 0;
}

.preview-section-title {
  display: block;
  font-size: 26rpx;
  font-weight: 600;
  color: #1f2d3d;
  margin-bottom: 16rpx;
  padding-bottom: 12rpx;
  border-bottom: 1rpx solid #f0f2f5;
}

.preview-info {
  display: flex;
  justify-content: space-between;
  padding: 12rpx 0;
}

.preview-label {
  font-size: 24rpx;
  color: #8c9aa8;
}

.preview-value {
  font-size: 24rpx;
  color: #1f2d3d;
}

.preview-item {
  display: flex;
  align-items: center;
  padding: 12rpx 0;
  border-bottom: 1rpx solid #f5f7fa;
}

.preview-item:last-child {
  border-bottom: none;
}

.preview-item-name {
  flex: 2;
  font-size: 24rpx;
  color: #1f2d3d;
}

.preview-item-result {
  flex: 1;
  font-size: 24rpx;
  color: #1f2d3d;
  text-align: center;
}

.preview-item-result.abnormal {
  color: #f56c6c;
  font-weight: 600;
}

.preview-item-ref {
  flex: 1;
  font-size: 22rpx;
  color: #8c9aa8;
  text-align: right;
}

.preview-result {
  background: linear-gradient(135deg, #e6f7ff 0%, #f0f9ff 100%);
  border-radius: 12rpx;
  padding: 20rpx;
  margin-bottom: 16rpx;
}

.preview-result-label {
  display: block;
  font-size: 22rpx;
  color: #8c9aa8;
  margin-bottom: 8rpx;
  text-align: center;
}

.preview-result-value {
  display: block;
  font-size: 36rpx;
  font-weight: 700;
  color: #1677ff;
  text-align: center;
}

.preview-explanation {
  background: #f9f9f9;
  border-radius: 12rpx;
  padding: 16rpx;
  margin-top: 12rpx;
}

.preview-explanation-label {
  display: block;
  font-size: 22rpx;
  font-weight: 600;
  color: #1f2d3d;
  margin-bottom: 8rpx;
}

.preview-explanation-content {
  display: block;
  font-size: 22rpx;
  color: #595959;
  line-height: 1.5;
}

.preview-result-text {
  display: block;
  font-size: 24rpx;
  color: #595959;
  line-height: 1.6;
  background: #fafafa;
  border-radius: 12rpx;
  padding: 16rpx;
}

.preview-empty {
  text-align: center;
  padding: 40rpx;
  font-size: 26rpx;
  color: #8c9aa8;
}
</style>
