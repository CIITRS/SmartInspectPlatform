<template>
  <view class="page">
    <view class="header">
      <text class="title">新增样本</text>
      <text class="desc">为当前患者新增样本</text>
    </view>

    <view v-if="patientIds.length === 0" class="empty">
      <text>请先从患者列表选择患者</text>
      <button class="link-btn" @click="goPatients">选择患者</button>
    </view>

    <view v-else>
      <view v-if="historicalSample" class="history-tip">
        <text class="history-title">该患者曾经检查为：</text>
        <text class="history-text">检测癌种：{{ historicalSample.cancer_type_name || '-' }}，样本类型：{{ historicalSample.sample_type_name || '-' }}，报告类型：{{ historicalSample.report_type_label || '-' }}，送检单位：{{ historicalSample.organization || '-' }}</text>
        <button class="reuse-btn" @click="reuseHistoricalSample">复用信息</button>
      </view>
      <view class="form-card">
      <view class="form-item">
        <text class="label">检测方式</text>
        <view class="segmented">
          <view class="seg-item" :class="{ active: form.service_mode === 'single' }" @click="form.service_mode = 'single'; form.sale_package_id = 0">单次检测</view>
          <view class="seg-item" :class="{ active: form.service_mode === 'package' }" @click="form.service_mode = 'package'">套餐联检</view>
        </view>
      </view>
      <view v-if="form.service_mode === 'package'" class="form-item">
        <text class="label">检测套餐</text>
        <picker :range="packages" range-key="display_name" @change="onPackageChange">
          <view class="picker" :class="{ placeholder: !form.sale_package_id }">{{ selectedOptionName(packages, form.sale_package_id, '请选择套餐') }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">检测癌种</text>
        <picker :range="cancerTypes" range-key="name" @change="onCancerTypeChange">
          <view class="picker" :class="{ placeholder: !form.cancer_type_id }">{{ selectedOptionName(cancerTypes, form.cancer_type_id, '请选择检测癌种') }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">样本类型</text>
        <picker :range="sampleTypes" range-key="name" @change="onSampleTypeChange">
          <view class="picker" :class="{ placeholder: !form.sample_type_id }">{{ selectedOptionName(sampleTypes, form.sample_type_id, '请选择样本类型') }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">报告类型</text>
        <view class="segmented">
          <view v-for="item in reportTypes" :key="item.value" class="seg-item" :class="{ active: form.report_type === item.value }" @click="form.report_type = item.value">{{ item.label }}</view>
        </view>
      </view>
      <view class="form-item">
        <text class="label">治疗阶段</text>
        <picker :range="treatmentStages" range-key="name" @change="onTreatmentStageChange">
          <view class="picker" :class="{ placeholder: !form.treatment_stage_id }">{{ selectedOptionName(treatmentStages, form.treatment_stage_id, '请选择治疗阶段') }}</view>
        </picker>
      </view>
      <view class="form-item">
        <text class="label">{{ patientIds.length === 1 ? '样本编号后4位' : '起始后4位' }}</text>
        <view class="inline-row">
          <text class="prefix-box">{{ samplePrefix || '前缀加载中' }}</text>
          <input v-model="suffixValue" type="number" maxlength="4" class="input inline-input" placeholder="例如 0001" />
          <button class="scan-btn" @click="scanSampleCode">扫码</button>
        </view>
        <text class="hint">前缀由系统生成，只填写后4位；扫码完整编号时会自动取后4位</text>
      </view>
      <view class="form-item">
        <text class="label">送检单位</text>
        <picker :range="organizationTypes" @change="onOrganizationTypeChange">
          <view class="picker">{{ organizationType }}</view>
        </picker>
        <input v-if="organizationType === '单位'" v-model="form.organization" class="input mt" placeholder="请输入送检单位" />
      </view>
      <view class="form-item">
        <text class="label">备注</text>
        <input v-model="form.notes" class="input" placeholder="选填" />
      </view>
      <view class="form-item">
        <text class="label">回寄快递（绑定样本后可登记）</text>
        <view class="inline-row">
          <input v-model="form.return_express_company" class="input express-company" placeholder="快递公司" />
          <input v-model="form.return_tracking_number" class="input" placeholder="回寄快递单号（选填）" />
        </view>
      </view>
      </view>

      <view class="consent-card">
        <view class="consent-title">知情同意书</view>
        <view v-if="consentSigned" class="consent-signed">该患者已于 {{ consentSignedAt || '此前' }} 签署，无需重复填写。</view>
        <template v-else>
          <scroll-view scroll-y class="consent-text"><text>{{ consentText }}</text></scroll-view>
          <input v-model="form.consent_signed_name" class="input" placeholder="请输入签署人姓名" />
          <view class="signature-head">
            <text>请在下方手写签名</text>
            <text class="clear-sign" @click="clearSignature">清空</text>
          </view>
          <canvas canvas-id="signatureCanvas" id="signatureCanvas" class="signature-canvas"
            disable-scroll
            @touchstart="signatureStart"
            @touchmove="signatureMove"
            @touchend="signatureEnd"></canvas>
          <view class="agree-row" @click="consentAgreed = !consentAgreed">
            <text class="check">{{ consentAgreed ? '☑' : '☐' }}</text>
            <text>本人已阅读并同意以上内容，确认签名真实有效</text>
          </view>
        </template>
      </view>
    </view>

    <button v-if="patientIds.length > 0" class="submit-btn" :disabled="submitting" @click="submit">
      {{ submitting ? '提交中...' : '新增样本' }}
    </button>

  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      patientIds: [],
      sampleTypes: [],
      cancerTypes: [],
      treatmentStages: [],
      packages: [],
      consentSigned: false,
      consentSignedAt: '',
      consentText: '',
      consentAgreed: false,
      signatureHasInk: false,
      signatureDrawing: false,
      signatureLastPoint: null,
      reportTypes: [
        { value: 'normal', label: '高敏（MePlex高敏98CpG）' },
        { value: 'high', label: '超敏（MePlex超敏180CpG）' }
      ],
      organizationTypes: ['个人', '单位'],
      organizationType: '个人',
      form: {
        sample_type_id: 0,
        cancer_type_id: 0,
        treatment_stage_id: 0,
        report_type: '',
        start_sequence: '',
        manual_suffix: '',
        organization: '个人送检',
        notes: '',
        service_mode: 'single',
        sale_package_id: 0,
        consent_signed_name: '',
        return_express_company: '',
        return_tracking_number: ''
      },
      samplePrefix: '',
      historicalSample: null,
      submitting: false
    }
  },
  computed: {
    suffixValue: {
      get() {
        return this.patientIds.length === 1 ? this.form.manual_suffix : this.form.start_sequence
      },
      set(value) {
        const suffix = String(value || '').replace(/\D/g, '').slice(0, 4)
        if (this.patientIds.length === 1) this.form.manual_suffix = suffix
        else this.form.start_sequence = suffix
      }
    }
  },
  onLoad(options) {
    const ids = String((options && options.patient_ids) || '')
      .split(',')
      .map(id => Number(id))
      .filter(id => id > 0)
    this.patientIds = ids
    this.loadOptions()
  },
  methods: {
    async loadOptions() {
      try {
        const res = await uniAPI.getEmployeeSampleOptions(this.patientIds)
        const data = res.data || {}
        this.sampleTypes = this.toList(data.sample_types)
        this.cancerTypes = this.toList(data.cancer_types)
        this.treatmentStages = this.toList(data.treatment_stages)
        this.packages = this.toList(data.packages).map(item => ({
          ...item,
          display_name: `${item.name}（${item.detection_count || 1}次）`
        }))
        this.consentSigned = Boolean(data.consent_signed)
        this.consentSignedAt = data.consent_signed_at || ''
        this.consentText = data.consent_text || ''
        this.samplePrefix = data.sample_prefix || ''
        this.historicalSample = data.historical_sample || null
        if (!this.suffixValue && Number(data.next_sequence) > 0) {
          this.suffixValue = String(data.next_sequence).padStart(4, '0')
        }
      } catch (error) {
        uni.showToast({ title: '选项加载失败', icon: 'none' })
      }
    },
    toList(data) {
      if (Array.isArray(data)) return data
      if (data && Array.isArray(data.list)) return data.list
      return []
    },
    selectedOptionName(options, id, placeholder) {
      const selected = options.find(item => Number(item.id) === Number(id))
      return selected ? selected.name : placeholder
    },
    onCancerTypeChange(e) {
      const selected = this.cancerTypes[Number(e.detail.value)]
      this.form.cancer_type_id = selected ? Number(selected.id) : 0
    },
    onSampleTypeChange(e) {
      const selected = this.sampleTypes[Number(e.detail.value)]
      this.form.sample_type_id = selected ? Number(selected.id) : 0
    },
    onTreatmentStageChange(e) {
      const selected = this.treatmentStages[Number(e.detail.value)]
      this.form.treatment_stage_id = selected ? Number(selected.id) : 0
    },
    onPackageChange(e) {
      const selected = this.packages[Number(e.detail.value)]
      this.form.sale_package_id = selected ? Number(selected.id) : 0
    },
    signatureStart(e) {
      const point = e.touches && e.touches[0]
      if (!point) return
      this.signatureDrawing = true
      this.signatureLastPoint = { x: point.x, y: point.y }
    },
    signatureMove(e) {
      if (!this.signatureDrawing || !this.signatureLastPoint) return
      const point = e.touches && e.touches[0]
      if (!point) return
      const context = uni.createCanvasContext('signatureCanvas', this)
      context.setStrokeStyle('#111827')
      context.setLineWidth(3)
      context.setLineCap('round')
      context.beginPath()
      context.moveTo(this.signatureLastPoint.x, this.signatureLastPoint.y)
      context.lineTo(point.x, point.y)
      context.stroke()
      context.draw(true)
      this.signatureLastPoint = { x: point.x, y: point.y }
      this.signatureHasInk = true
    },
    signatureEnd() {
      this.signatureDrawing = false
      this.signatureLastPoint = null
    },
    clearSignature() {
      const context = uni.createCanvasContext('signatureCanvas', this)
      context.clearRect(0, 0, 1000, 500)
      context.draw()
      this.signatureHasInk = false
    },
    exportSignature() {
      return new Promise((resolve, reject) => {
        uni.canvasToTempFilePath({
          canvasId: 'signatureCanvas',
          fileType: 'png',
          quality: 1,
          success: (result) => {
            try {
              const fs = uni.getFileSystemManager()
              fs.readFile({
                filePath: result.tempFilePath,
                encoding: 'base64',
                success: (readResult) => resolve(`data:image/png;base64,${readResult.data}`),
                fail: reject
              })
            } catch (error) {
              reject(error)
            }
          },
          fail: reject
        }, this)
      })
    },
    reuseHistoricalSample() {
      const item = this.historicalSample
      if (!item) return
      this.form.cancer_type_id = Number(item.cancer_type_id) || 0
      this.form.sample_type_id = Number(item.sample_type_id) || 0
      this.form.report_type = item.report_type || ''
      this.form.organization = item.organization || '个人送检'
      this.organizationType = this.form.organization === '个人送检' ? '个人' : '单位'
      // 治疗阶段属于本次检测信息，不复用历史值。
      uni.showToast({ title: '已复用历史信息', icon: 'success' })
    },
    scanSampleCode() {
      uni.scanCode({
        onlyFromCamera: false,
        success: (res) => {
          const code = String(res.result || '').trim()
          if (code.length >= 4) {
            this.suffixValue = code.slice(-4)
            uni.showToast({ title: '已识别后4位', icon: 'success' })
          } else {
            uni.showToast({ title: '未识别到有效编号', icon: 'none' })
          }
        },
        fail: () => {
          uni.showToast({ title: '扫码取消或失败', icon: 'none' })
        }
      })
    },
    onOrganizationTypeChange(e) {
      this.organizationType = this.organizationTypes[Number(e.detail.value)] || '个人'
      if (this.organizationType === '个人') this.form.organization = '个人送检'
      else this.form.organization = ''
    },
    goPatients() {
      uni.redirectTo({ url: '/pages/employee/patients/index?select=1' })
    },
    async submit() {
      if (!this.form.cancer_type_id) { uni.showToast({ title: '请选择检测癌种', icon: 'none' }); return }
      if (!this.form.sample_type_id) { uni.showToast({ title: '请选择样本类型', icon: 'none' }); return }
      if (!this.form.report_type) { uni.showToast({ title: '请选择高敏或超敏', icon: 'none' }); return }
      if (this.form.service_mode === 'package' && !this.form.sale_package_id) { uni.showToast({ title: '请选择检测套餐', icon: 'none' }); return }
      if (!this.form.treatment_stage_id) { uni.showToast({ title: '请选择治疗阶段', icon: 'none' }); return }
      const suffix = this.suffixValue
      if (!/^\d{4}$/.test(suffix)) {
        uni.showToast({ title: '请输入4位数字', icon: 'none' })
        return
      }
      if (this.organizationType === '单位' && !String(this.form.organization || '').trim()) {
        uni.showToast({ title: '请输入送检单位', icon: 'none' })
        return
      }
      if (!this.consentSigned) {
        if (!String(this.form.consent_signed_name || '').trim()) { uni.showToast({ title: '请输入签署人姓名', icon: 'none' }); return }
        if (!this.signatureHasInk) { uni.showToast({ title: '请完成手写签名', icon: 'none' }); return }
        if (!this.consentAgreed) { uni.showToast({ title: '请确认同意知情同意书', icon: 'none' }); return }
      }
      this.submitting = true
      try {
        let consentSignature = ''
        if (!this.consentSigned) consentSignature = await this.exportSignature()
        const payload = {
          ...this.form,
          consent_signature: consentSignature,
          patient_ids: this.patientIds,
          start_sequence: Number(this.form.start_sequence) || 0,
          manual_suffix: this.form.manual_suffix || ''
        }
        if (this.patientIds.length === 1) {
          payload.start_sequence = 0
          payload.manual_suffix = suffix
        } else {
          payload.start_sequence = Number(suffix)
          payload.manual_suffix = ''
        }
        const res = await uniAPI.allocateEmployeeSamples(payload)
        if (res.success && res.data) {
          uni.showToast({ title: '新增成功', icon: 'success' })
          setTimeout(() => uni.navigateBack(), 700)
        } else {
          uni.showToast({ title: res.message || '新增失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: error.message || '网络错误', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.header { margin-bottom: 24rpx; }
.title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.empty { display: flex; flex-direction: column; align-items: center; gap: 24rpx; padding: 120rpx 0; color: #8c9aa8; font-size: 28rpx; }
.link-btn { width: 240rpx; height: 76rpx; line-height: 76rpx; border-radius: 14rpx; background: #1677ff; color: #fff; font-size: 26rpx; border: none; }
.form-card { background: #fff; border-radius: 20rpx; padding: 12rpx 28rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.history-tip { margin-bottom: 20rpx; padding: 24rpx; border-radius: 18rpx; background: #eef6ff; border: 1rpx solid #bae0ff; }
.history-title { display: block; color: #0958d9; font-size: 27rpx; font-weight: 700; margin-bottom: 10rpx; }
.history-text { display: block; color: #345; font-size: 24rpx; line-height: 1.7; }
.reuse-btn { margin: 18rpx 0 0; width: 180rpx; height: 62rpx; line-height: 62rpx; border: none; border-radius: 12rpx; background: #1677ff; color: #fff; font-size: 24rpx; }
.form-item { padding: 20rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.form-item:last-child { border-bottom: none; }
.label { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 12rpx; }
.picker, .input { width: 100%; min-height: 72rpx; line-height: 72rpx; padding: 0 20rpx; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; font-size: 26rpx; color: #1f2d3d; }
.placeholder { color: #9ca3af; }
.segmented { display: flex; gap: 16rpx; }
.seg-item { flex: 1; min-height: 72rpx; padding: 14rpx 10rpx; display: flex; align-items: center; justify-content: center; text-align: center; border-radius: 12rpx; background: #f3f6fa; color: #606f7b; font-size: 23rpx; box-sizing: border-box; }
.seg-item.active { background: #1677ff; color: #fff; }
.mt { margin-top: 14rpx; }
.inline-row { display: flex; align-items: center; gap: 16rpx; }
.prefix-box { min-width: 220rpx; height: 72rpx; line-height: 72rpx; padding: 0 18rpx; border-radius: 12rpx; background: #eef1f5; color: #8c9aa8; font-size: 26rpx; box-sizing: border-box; }
.inline-input { flex: 1; }
.scan-btn { width: 160rpx; height: 72rpx; line-height: 72rpx; border-radius: 12rpx; background: #f0f5ff; color: #1677ff; font-size: 26rpx; border: 2rpx solid #1677ff; }
.hint { display: block; margin-top: 8rpx; font-size: 22rpx; color: #8c9aa8; }
.submit-btn { margin-top: 28rpx; width: 100%; height: 88rpx; line-height: 88rpx; border-radius: 16rpx; border: none; background: #1677ff; color: #fff; font-size: 30rpx; font-weight: 600; }
.express-company { max-width: 220rpx; }
.consent-card { margin-top: 22rpx; padding: 26rpx; border-radius: 20rpx; background: #fff; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.consent-title { font-size: 30rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 16rpx; }
.consent-signed { padding: 20rpx; border-radius: 12rpx; background: #f6ffed; color: #389e0d; font-size: 25rpx; }
.consent-text { height: 220rpx; padding: 18rpx; margin-bottom: 18rpx; box-sizing: border-box; border-radius: 12rpx; background: #f8fafc; color: #475569; font-size: 24rpx; line-height: 1.7; }
.signature-head { display: flex; justify-content: space-between; margin: 18rpx 0 10rpx; color: #64748b; font-size: 24rpx; }
.clear-sign { color: #1677ff; }
.signature-canvas { width: 100%; height: 260rpx; border: 2rpx dashed #94a3b8; border-radius: 12rpx; background: #fff; }
.agree-row { display: flex; gap: 10rpx; align-items: flex-start; margin-top: 16rpx; color: #475569; font-size: 23rpx; line-height: 1.5; }
.check { color: #1677ff; font-size: 30rpx; }
</style>
