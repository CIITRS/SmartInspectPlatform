<template>
  <view class="page">
    <view v-if="loading" class="state"><text>加载中...</text></view>

    <view v-else-if="!patient.id" class="state"><text>未找到患者</text></view>

    <view v-else class="content">
      <view class="patient-card">
        <view class="patient-head">
          <view>
            <text class="name">{{ patient.name || '-' }}</text>
            <text class="meta">{{ patient.patient_code || '-' }}</text>
          </view>
          <text class="gender">{{ patient.gender || '-' }}</text>
        </view>
        <view class="info-row"><text class="lbl">手机号</text><text class="val">{{ patient.phone || '-' }}</text></view>
        <view class="info-row"><text class="lbl">证件类型</text><text class="val">{{ patient.id_document_type || '-' }}</text></view>
        <view class="info-row"><text class="lbl">证件号</text><text class="val">{{ patient.id_document_no || patient.id_card || '-' }}</text></view>
        <view class="info-row"><text class="lbl">生日</text><text class="val">{{ patient.birthday || '-' }}</text></view>
        <view class="info-row"><text class="lbl">诊断</text><text class="val">{{ patient.diagnosis || '-' }}</text></view>
        <view class="info-row"><text class="lbl">吸烟史</text><text class="val">{{ patient.smoking_status || '-' }}</text></view>
        <view class="info-row"><text class="lbl">地址</text><text class="val">{{ patient.address || '-' }}</text></view>
      </view>

      <view class="patient-card">
        <view class="section-head compact">
          <text class="section-title">病理与预后信息</text>
          <button class="mini-btn" @click="completePatient">完善</button>
        </view>
        <view class="info-row"><text class="lbl">癌直径</text><text class="val">{{ patient.cancer_diameter || '-' }}</text></view>
        <view class="info-row"><text class="lbl">病理信息</text><text class="val">{{ patient.cancer_pathology || '-' }}</text></view>
        <view class="info-row"><text class="lbl">预后信息</text><text class="val">{{ patient.prognosis_info || '-' }}</text></view>
        <view class="info-row"><text class="lbl">其他信息</text><text class="val">{{ patient.other_info || '-' }}</text></view>
        <view v-if="followUps.length" class="follow-list">
          <view v-for="item in followUps" :key="item.id" class="follow-item">
            <view class="info-row"><text class="lbl">完善时间</text><text class="val">{{ item.created_at || '-' }}</text></view>
            <view class="info-row" v-if="item.diagnosis_info"><text class="lbl">检测信息</text><text class="val">{{ item.diagnosis_info }}</text></view>
            <view class="info-row" v-if="item.report_notes"><text class="lbl">结果说明</text><text class="val">{{ item.report_notes }}</text></view>
            <view class="info-row" v-if="item.images && item.images.length"><text class="lbl">报告文件</text><text class="val">{{ item.images.join('，') }}</text></view>
          </view>
        </view>
      </view>

      <view class="section-head">
        <text class="section-title">样本列表</text>
        <text class="section-count">{{ samples.length }} 个</text>
      </view>

      <view v-if="samples.length === 0" class="empty-card">
        <text>暂无样本</text>
      </view>

      <view v-else class="sample-list">
        <view v-for="item in samples" :key="item.id" class="sample-card" :class="{ clickable: item.has_report }" @click="openSample(item)">
          <view class="sample-head">
            <text class="sample-code">{{ item.sample_code || '-' }}</text>
            <text class="sample-status" :class="'s-' + item.sample_status">{{ statusMap[item.sample_status] || item.sample_status || '-' }}</text>
          </view>
          <view class="info-row" v-if="item.sample_type"><text class="lbl">样本类型</text><text class="val">{{ item.sample_type }}</text></view>
          <view class="info-row" v-if="item.cancer_type"><text class="lbl">检测癌种</text><text class="val">{{ item.cancer_type }}</text></view>
          <view class="info-row" v-if="item.report_type_label"><text class="lbl">报告类型</text><text class="val">{{ item.report_type_label }}</text></view>
          <view class="info-row" v-if="item.treatment_stage"><text class="lbl">患者状态</text><text class="val">{{ item.treatment_stage }}</text></view>
          <view class="info-row"><text class="lbl">创建时间</text><text class="val">{{ item.sample_created_at || item.collection_date || '-' }}</text></view>
          <view class="info-row" v-if="item.receive_date"><text class="lbl">接收日期</text><text class="val">{{ item.receive_date }}</text></view>
          <view class="info-row" v-if="item.report_reviewed_time || item.report_generated_time">
            <text class="lbl">报告时间</text>
            <text class="val">{{ item.report_reviewed_time || item.report_generated_time }}</text>
          </view>
          <view class="info-row" v-if="item.notes"><text class="lbl">备注</text><text class="val">{{ item.notes }}</text></view>
          <text v-if="item.has_report" class="view-hint">已出报告，点击查看样本详情 ›</text>
        </view>
      </view>
    </view>

    <view class="bottom-bar">
      <button class="add-btn" :disabled="!patient.id" @click="addSample">新增样本</button>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      patientId: 0,
      loading: true,
      patient: {},
      samples: [],
      followUps: [],
      statusMap: {
        created: '已创建',
        collected: '已采集',
        received: '已接收',
        processing: '处理中',
        tested: '已检测',
        completed: '已完成'
      }
    }
  },
  onLoad(options) {
    this.patientId = Number(options && options.id) || 0
    this.loadDetail()
  },
  onShow() {
    if (this.patientId && !this.loading) this.loadDetail()
  },
  methods: {
    async loadDetail() {
      if (!this.patientId) {
        this.loading = false
        uni.showToast({ title: '患者ID无效', icon: 'none' })
        return
      }
      this.loading = true
      try {
        const res = await uniAPI.getEmployeePatientDetail(this.patientId)
        if (res.success && res.data) {
          this.patient = res.data.patient || {}
          this.samples = Array.isArray(res.data.samples) ? res.data.samples : []
          this.followUps = Array.isArray(res.data.follow_ups) ? res.data.follow_ups : []
        } else {
          uni.showToast({ title: res.message || '加载失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: error.message || '加载失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    },
    addSample() {
      if (!this.patient.id) return
      uni.navigateTo({ url: `/pages/employee/sample-allocate/index?patient_ids=${this.patient.id}` })
    },
    completePatient() {
      if (!this.patient.id) return
      uni.navigateTo({ url: `/pages/employee/patient-complete/index?id=${this.patient.id}` })
    },
    openSample(item) {
      if (!item || !item.has_report) return
      uni.navigateTo({ url: `/pages/employee/sample-detail/index?id=${item.id}` })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx 32rpx 132rpx; background: #f5f7fa; box-sizing: border-box; }
.content { width: 100%; }
.state { display: flex; justify-content: center; padding: 160rpx 0; color: #8c9aa8; font-size: 28rpx; }
.patient-card, .sample-card, .empty-card { background: #fff; border-radius: 18rpx; padding: 26rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.patient-card { margin-bottom: 28rpx; }
.patient-head { display: flex; align-items: flex-start; justify-content: space-between; gap: 20rpx; padding-bottom: 20rpx; margin-bottom: 10rpx; border-bottom: 1rpx solid #f0f2f5; }
.name { display: block; font-size: 34rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.meta { display: block; font-size: 24rpx; color: #1677ff; }
.gender { flex-shrink: 0; min-width: 72rpx; height: 44rpx; line-height: 44rpx; text-align: center; border-radius: 22rpx; background: #f0f5ff; color: #1677ff; font-size: 24rpx; }
.info-row { display: flex; gap: 18rpx; margin-top: 14rpx; font-size: 25rpx; line-height: 1.45; }
.lbl { width: 112rpx; color: #8c9aa8; flex-shrink: 0; }
.val { flex: 1; color: #1f2d3d; word-break: break-all; }
.section-head { display: flex; align-items: center; justify-content: space-between; margin: 0 4rpx 16rpx; }
.section-head.compact { margin-bottom: 10rpx; }
.section-title { font-size: 30rpx; font-weight: 700; color: #1f2d3d; }
.section-count { font-size: 24rpx; color: #8c9aa8; }
.mini-btn { width: 128rpx; height: 58rpx; line-height: 58rpx; border-radius: 12rpx; border: none; background: #f0f5ff; color: #1677ff; font-size: 24rpx; }
.follow-list { margin-top: 18rpx; border-top: 1rpx solid #f0f2f5; }
.follow-item { padding-top: 16rpx; }
.empty-card { display: flex; justify-content: center; padding: 72rpx 0; color: #8c9aa8; font-size: 26rpx; }
.sample-list { display: flex; flex-direction: column; gap: 20rpx; }
.sample-card.clickable { border: 1rpx solid #bae0ff; }
.view-hint { display: block; margin-top: 18rpx; color: #1677ff; font-size: 24rpx; text-align: right; }
.sample-head { display: flex; align-items: center; justify-content: space-between; gap: 18rpx; padding-bottom: 16rpx; margin-bottom: 10rpx; border-bottom: 1rpx solid #f0f2f5; }
.sample-code { flex: 1; min-width: 0; color: #1f2d3d; font-size: 29rpx; font-weight: 700; word-break: break-all; }
.sample-status { flex-shrink: 0; font-size: 22rpx; padding: 6rpx 16rpx; border-radius: 999rpx; background: #f2f3f5; color: #606266; }
.s-created { background: #e6f4ff; color: #1677ff; }
.s-collected { background: #f0f5ff; color: #2f54eb; }
.s-received { background: #f6ffed; color: #52c41a; }
.s-processing { background: #fff7e6; color: #fa8c16; }
.s-tested, .s-completed { background: #f6ffed; color: #389e0d; }
.bottom-bar { position: fixed; left: 0; right: 0; bottom: 0; padding: 18rpx 32rpx calc(18rpx + env(safe-area-inset-bottom)); background: rgba(255,255,255,0.96); box-shadow: 0 -4rpx 18rpx rgba(31,45,61,0.08); box-sizing: border-box; }
.add-btn { width: 100%; height: 88rpx; line-height: 88rpx; border-radius: 16rpx; border: none; background: #1677ff; color: #fff; font-size: 30rpx; font-weight: 600; }
.add-btn[disabled] { background: #a0cfff; color: #fff; }
</style>
