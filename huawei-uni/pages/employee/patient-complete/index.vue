<template>
  <view class="page">
    <view v-if="loading" class="state">加载中...</view>
    <view v-else class="form-card">
      <view class="patient-title">
        <text class="name">{{ patient.name || '-' }}</text>
        <text class="code">{{ patient.patient_code || '-' }}</text>
      </view>

      <view class="form-item">
        <text class="label">癌直径</text>
        <input v-model="form.cancer_diameter" class="input" placeholder="销售输入什么就显示什么" />
      </view>
      <view class="form-item">
        <text class="label">癌症病理信息</text>
        <textarea v-model="form.cancer_pathology" class="textarea" placeholder="请输入病理信息" />
      </view>
      <view class="form-item">
        <text class="label">预后信息</text>
        <textarea v-model="form.prognosis_info" class="textarea" placeholder="请输入预后信息" />
      </view>
      <view class="form-item">
        <text class="label">检测信息</text>
        <textarea v-model="form.diagnosis_info" class="textarea" placeholder="请输入检测信息" />
      </view>
      <view class="form-item">
        <text class="label">其他信息</text>
        <textarea v-model="form.other_info" class="textarea" placeholder="请输入其他信息" />
      </view>
      <view class="form-item">
        <text class="label">结果说明</text>
        <textarea v-model="form.report_notes" class="textarea" placeholder="请输入结果说明" />
      </view>

      <view class="form-item">
        <view class="upload-head">
          <text class="label no-margin">报告文件</text>
          <button class="upload-btn" :disabled="uploading" @click="chooseFile">{{ uploading ? '上传中...' : '上传' }}</button>
        </view>
        <view v-if="form.report_files.length === 0" class="empty-file">未上传</view>
        <view v-else class="file-list">
          <view v-for="(file, index) in form.report_files" :key="file" class="file-row">
            <text class="file-name">{{ fileName(file) }}</text>
            <text class="remove" @click="removeFile(index)">删除</text>
          </view>
        </view>
      </view>
    </view>

    <button class="submit-btn" :disabled="submitting" @click="submit">{{ submitting ? '保存中...' : '保存完善信息' }}</button>
  </view>
</template>

<script>
import { BASE_URL, uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      patientId: 0,
      loading: true,
      submitting: false,
      uploading: false,
      patient: {},
      form: {
        cancer_diameter: '',
        cancer_pathology: '',
        prognosis_info: '',
        diagnosis_info: '',
        other_info: '',
        report_notes: '',
        report_files: []
      }
    }
  },
  onLoad(options) {
    this.patientId = Number(options && options.id) || 0
    this.loadDetail()
  },
  methods: {
    async loadDetail() {
      if (!this.patientId) {
        this.loading = false
        uni.showToast({ title: '患者ID无效', icon: 'none' })
        return
      }
      try {
        const res = await uniAPI.getEmployeePatientDetail(this.patientId)
        const data = res.data || {}
        this.patient = data.patient || {}
        this.form.cancer_diameter = this.patient.cancer_diameter || ''
        this.form.cancer_pathology = this.patient.cancer_pathology || ''
        this.form.prognosis_info = this.patient.prognosis_info || ''
        this.form.other_info = this.patient.other_info || ''
        this.form.report_files = String(this.patient.report_files || '').split(',').map(v => v.trim()).filter(Boolean)
      } catch (error) {
        uni.showToast({ title: '加载失败', icon: 'none' })
      } finally {
        this.loading = false
      }
    },
    fileName(file) {
      const clean = String(file || '').split('?')[0]
      return clean.split('/').pop() || clean
    },
    removeFile(index) {
      this.form.report_files.splice(index, 1)
    },
    chooseFile() {
      uni.chooseMessageFile({
        count: 1,
        type: 'file',
        success: (res) => {
          const file = (res.tempFiles || [])[0]
          if (file && file.path) this.uploadFile(file.path)
        }
      })
    },
    uploadFile(path) {
      this.uploading = true
      uni.uploadFile({
        url: BASE_URL + '/uni/employee/patient-report/upload',
        filePath: path,
        name: 'file',
        formData: { patient_code: this.patient.patient_code || '' },
        header: { 'X-Miniapp-Session': uni.getStorageSync('miniapp_session_id') || '' },
        success: (res) => {
          let data = {}
          try { data = JSON.parse(res.data || '{}') } catch (e) {}
          if (data.success && data.data) {
            this.form.report_files.push(data.data.url || data.data.path)
            uni.showToast({ title: '上传成功', icon: 'success' })
          } else {
            uni.showToast({ title: data.message || '上传失败', icon: 'none' })
          }
        },
        fail: () => uni.showToast({ title: '上传失败', icon: 'none' }),
        complete: () => { this.uploading = false }
      })
    },
    async submit() {
      this.submitting = true
      try {
        const res = await uniAPI.completeEmployeePatient(this.patientId, this.form)
        if (res.success) {
          uni.showToast({ title: '保存成功', icon: 'success' })
          setTimeout(() => uni.navigateBack(), 700)
        } else {
          uni.showToast({ title: res.message || '保存失败', icon: 'none' })
        }
      } catch (error) {
        uni.showToast({ title: error.message || '保存失败', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx 32rpx 132rpx; background: #f5f7fa; box-sizing: border-box; }
.state { text-align: center; padding: 120rpx 0; color: #8c9aa8; font-size: 28rpx; }
.form-card { background: #fff; border-radius: 18rpx; padding: 26rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.patient-title { padding-bottom: 20rpx; margin-bottom: 12rpx; border-bottom: 1rpx solid #eef1f5; }
.name { display: block; font-size: 34rpx; font-weight: 700; color: #1f2d3d; }
.code { display: block; margin-top: 8rpx; color: #1677ff; font-size: 24rpx; }
.form-item { padding: 18rpx 0; border-bottom: 1rpx solid #f0f2f5; }
.form-item:last-child { border-bottom: none; }
.label { display: block; margin-bottom: 12rpx; color: #64748b; font-size: 25rpx; }
.no-margin { margin-bottom: 0; }
.input, .textarea { width: 100%; border: 2rpx solid #e5e7eb; border-radius: 12rpx; background: #f9fafb; box-sizing: border-box; color: #1f2d3d; font-size: 26rpx; }
.input { height: 72rpx; line-height: 72rpx; padding: 0 20rpx; }
.textarea { min-height: 150rpx; padding: 18rpx 20rpx; line-height: 1.5; }
.upload-head { display: flex; align-items: center; justify-content: space-between; }
.upload-btn { width: 136rpx; height: 60rpx; line-height: 60rpx; border-radius: 12rpx; background: #1677ff; color: #fff; border: none; font-size: 24rpx; }
.empty-file { margin-top: 14rpx; color: #8c9aa8; font-size: 24rpx; }
.file-list { margin-top: 12rpx; display: flex; flex-direction: column; gap: 10rpx; }
.file-row { display: flex; align-items: center; justify-content: space-between; gap: 14rpx; padding: 14rpx; border-radius: 12rpx; background: #f5f7fa; }
.file-name { flex: 1; min-width: 0; color: #1f2d3d; font-size: 24rpx; word-break: break-all; }
.remove { color: #ff4d4f; font-size: 24rpx; }
.submit-btn { position: fixed; left: 32rpx; right: 32rpx; bottom: calc(22rpx + env(safe-area-inset-bottom)); height: 88rpx; line-height: 88rpx; border: none; border-radius: 16rpx; background: #1677ff; color: #fff; font-size: 30rpx; font-weight: 700; }
.submit-btn[disabled], .upload-btn[disabled] { opacity: 0.65; }
</style>
