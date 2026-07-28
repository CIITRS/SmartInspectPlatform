<template>
  <view class="page-container">
    <view class="page-header">
      <view>
        <text class="page-title">随访管理</text>
        <text class="page-desc">上传报告图片和诊断信息</text>
      </view>
      <button class="add-btn" size="mini" @click="openForm">新增随访单</button>
    </view>

    <view v-if="loading" class="state-box"><text class="state-text">加载中...</text></view>
    <view v-else-if="followUps.length === 0" class="state-box">
      <text class="state-title">暂无随访记录</text>
      <text class="state-text">新增随访单后可在这里查看</text>
    </view>
    <view v-else class="list-box">
      <view v-for="item in followUps" :key="item.id" class="follow-card">
        <view class="card-head">
          <text class="card-title">随访单 #{{ item.id }}</text>
          <text class="card-time">{{ item.created_at || '' }}</text>
        </view>
        <view v-if="item.diagnosis_info" class="content-block">
          <text class="block-label">诊断信息</text>
          <text class="block-text">{{ item.diagnosis_info }}</text>
        </view>
        <view v-if="item.report_notes" class="content-block">
          <text class="block-label">报告说明</text>
          <text class="block-text">{{ item.report_notes }}</text>
        </view>
        <view v-if="item.images && item.images.length" class="image-row">
          <image
            v-for="(img, index) in item.images"
            :key="index"
            :src="img"
            class="thumb"
            mode="aspectFill"
            @click="previewImages(item.images, index)"
          ></image>
        </view>
      </view>
    </view>

    <uni-popup ref="formPopup" type="bottom">
      <view class="form-panel">
        <view class="form-head">
          <text class="form-title">新增随访单</text>
          <text class="close-btn" @click="closeForm">✕</text>
        </view>
        <view class="form-body">
          <view class="field">
            <text class="field-label">诊断信息</text>
            <textarea v-model="form.diagnosis_info" class="textarea" maxlength="800" placeholder="请输入诊断信息"></textarea>
          </view>
          <view class="field">
            <text class="field-label">报告说明</text>
            <textarea v-model="form.report_notes" class="textarea" maxlength="800" placeholder="请输入报告说明或补充描述"></textarea>
          </view>
          <view class="field">
            <view class="upload-head">
              <text class="field-label">报告图片</text>
              <text class="upload-count">{{ form.images.length }}/3</text>
            </view>
            <view class="upload-grid">
              <view v-for="(img, index) in form.images" :key="index" class="upload-item">
                <image :src="img" class="upload-image" mode="aspectFill" @click="previewImages(form.images, index)"></image>
                <text class="remove-image" @click.stop="removeImage(index)">✕</text>
              </view>
              <view v-if="form.images.length < 3" class="upload-add" @click="chooseImages">
                <text class="plus">+</text>
              </view>
            </view>
          </view>
        </view>
        <view class="form-actions">
          <button class="cancel-btn" @click="closeForm">取消</button>
          <button class="submit-btn" :loading="submitting" @click="submitForm">提交</button>
        </view>
      </view>
    </uni-popup>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      loading: true,
      submitting: false,
      followUps: [],
      form: {
        diagnosis_info: '',
        report_notes: '',
        images: []
      }
    }
  },
  onLoad() {
    this.loadFollowUps()
  },
  methods: {
    async loadFollowUps() {
      this.loading = true
      try {
        const res = await uniAPI.getFollowUps()
        if (res.success && res.data) {
          this.followUps = res.data.list || []
        }
      } catch (e) {
        console.error('加载随访单失败', e)
      } finally {
        this.loading = false
      }
    },
    openForm() {
      this.resetForm()
      this.$refs.formPopup.open()
    },
    closeForm() {
      this.$refs.formPopup.close()
    },
    resetForm() {
      this.form = { diagnosis_info: '', report_notes: '', images: [] }
    },
    chooseImages() {
      const remain = 3 - this.form.images.length
      if (remain <= 0) return
      uni.chooseImage({
        count: remain,
        sizeType: ['compressed'],
        sourceType: ['album', 'camera'],
        success: async (res) => {
          const paths = res.tempFilePaths || []
          const dataUrls = []
          for (const path of paths) {
            dataUrls.push(await this.fileToDataUrl(path))
          }
          this.form.images = this.form.images.concat(dataUrls).slice(0, 3)
        }
      })
    },
    fileToDataUrl(path) {
      return new Promise((resolve) => {
        // #ifdef MP-WEIXIN
        uni.getFileSystemManager().readFile({
          filePath: path,
          encoding: 'base64',
          success: (res) => {
            resolve('data:image/jpeg;base64,' + res.data)
          },
          fail: () => resolve(path)
        })
        // #endif
        // #ifndef MP-WEIXIN
        resolve(path)
        // #endif
      })
    },
    removeImage(index) {
      this.form.images.splice(index, 1)
    },
    previewImages(images, index) {
      uni.previewImage({ urls: images, current: images[index] })
    },
    async submitForm() {
      if (!this.form.diagnosis_info && !this.form.report_notes && this.form.images.length === 0) {
        uni.showToast({ title: '请填写内容或上传图片', icon: 'none' })
        return
      }
      this.submitting = true
      try {
        const res = await uniAPI.createFollowUp(this.form)
        if (res.success) {
          uni.showToast({ title: '提交成功', icon: 'success' })
          this.closeForm()
          await this.loadFollowUps()
        } else {
          uni.showToast({ title: res.message || '提交失败', icon: 'none' })
        }
      } catch (e) {
        console.error('提交随访单失败', e)
        uni.showToast({ title: '提交失败', icon: 'none' })
      } finally {
        this.submitting = false
      }
    }
  }
}
</script>

<style scoped>
.page-container { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.page-header { display: flex; align-items: center; justify-content: space-between; gap: 20rpx; margin-bottom: 28rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.add-btn { margin: 0; background: #1677ff; color: #fff; border-radius: 12rpx; font-size: 24rpx; line-height: 56rpx; }
.state-box { display: flex; flex-direction: column; align-items: center; padding: 120rpx 0; }
.state-title { font-size: 30rpx; color: #1f2d3d; font-weight: 600; margin-bottom: 12rpx; }
.state-text { font-size: 26rpx; color: #8c9aa8; }
.follow-card { background: #fff; border-radius: 16rpx; padding: 28rpx; margin-bottom: 20rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.card-head { display: flex; justify-content: space-between; align-items: center; gap: 16rpx; margin-bottom: 18rpx; }
.card-title { font-size: 28rpx; font-weight: 600; color: #1f2d3d; }
.card-time { font-size: 22rpx; color: #8c9aa8; }
.content-block { margin-top: 18rpx; }
.block-label { display: block; font-size: 22rpx; color: #8c9aa8; margin-bottom: 8rpx; }
.block-text { display: block; font-size: 26rpx; color: #1f2d3d; line-height: 1.6; white-space: pre-wrap; }
.image-row { display: flex; gap: 14rpx; margin-top: 20rpx; }
.thumb { width: 148rpx; height: 148rpx; border-radius: 12rpx; background: #eef2f6; }
.form-panel { background: #fff; border-radius: 28rpx 28rpx 0 0; overflow: hidden; }
.form-head { display: flex; align-items: center; justify-content: space-between; padding: 28rpx 32rpx; border-bottom: 1rpx solid #f0f2f5; }
.form-title { font-size: 32rpx; font-weight: 700; color: #1f2d3d; }
.close-btn { font-size: 32rpx; color: #8c9aa8; padding: 8rpx; }
.form-body { padding: 28rpx 32rpx; max-height: 70vh; overflow-y: auto; box-sizing: border-box; }
.field { margin-bottom: 26rpx; }
.field-label { display: block; font-size: 26rpx; font-weight: 600; color: #1f2d3d; margin-bottom: 12rpx; }
.textarea { width: 100%; height: 160rpx; padding: 18rpx; background: #f7f9fc; border: 1rpx solid #e5e9f0; border-radius: 12rpx; font-size: 26rpx; color: #1f2d3d; box-sizing: border-box; }
.upload-head { display: flex; align-items: center; justify-content: space-between; }
.upload-count { font-size: 24rpx; color: #8c9aa8; }
.upload-grid { display: flex; flex-wrap: wrap; gap: 16rpx; }
.upload-item, .upload-add { position: relative; width: 156rpx; height: 156rpx; border-radius: 12rpx; overflow: hidden; background: #f7f9fc; border: 1rpx solid #e5e9f0; box-sizing: border-box; }
.upload-image { width: 100%; height: 100%; }
.remove-image { position: absolute; right: 6rpx; top: 6rpx; width: 36rpx; height: 36rpx; border-radius: 50%; background: rgba(0,0,0,0.55); color: #fff; font-size: 22rpx; text-align: center; line-height: 36rpx; }
.upload-add { display: flex; align-items: center; justify-content: center; }
.plus { font-size: 56rpx; color: #8c9aa8; line-height: 1; }
.form-actions { display: grid; grid-template-columns: 1fr 1fr; gap: 18rpx; padding: 20rpx 32rpx 32rpx; border-top: 1rpx solid #f0f2f5; }
.cancel-btn, .submit-btn { height: 76rpx; line-height: 76rpx; border-radius: 12rpx; font-size: 28rpx; }
.cancel-btn { background: #f5f7fa; color: #606f7b; }
.submit-btn { background: #1677ff; color: #fff; }
</style>
