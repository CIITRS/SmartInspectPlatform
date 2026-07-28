<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">帮助中心</text>
      <text class="page-desc">常见问题与服务说明</text>
    </view>

    <view v-if="loading" class="state-box"><text class="state-text">加载中...</text></view>
    <view v-else>
      <view v-for="category in categories" :key="category.name" class="category">
        <text class="category-title">{{ category.name }}</text>
        <view class="faq-card">
          <view v-for="(item, index) in category.items" :key="index" class="faq-item" @click="toggle(category.name, index)">
            <view class="faq-question">
              <text class="question-text">{{ item.question }}</text>
              <text class="question-arrow">{{ openedKey === category.name + '-' + index ? '⌃' : '⌄' }}</text>
            </view>
            <text v-if="openedKey === category.name + '-' + index" class="faq-answer" user-select="true">{{ item.answer }}</text>
          </view>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { uniAPI } from '../../api/index.js'

export default {
  data() {
    return {
      loading: true,
      openedKey: '',
      categories: []
    }
  },
  onLoad() {
    this.loadHelp()
  },
  methods: {
    async loadHelp() {
      this.loading = true
      try {
        const res = await uniAPI.getHelpCenter()
        const data = res && res.data ? res.data : {}
        this.categories = data.categories || []
      } catch (e) {
        console.error('加载帮助中心失败', e)
      } finally {
        this.loading = false
      }
    },
    toggle(name, index) {
      const key = name + '-' + index
      this.openedKey = this.openedKey === key ? '' : key
    }
  }
}
</script>

<style scoped>
.page-container { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.state-box { display: flex; justify-content: center; padding: 120rpx 0; }
.state-text { font-size: 26rpx; color: #8c9aa8; }
.category { margin-bottom: 28rpx; }
.category-title { display: block; font-size: 24rpx; color: #8c9aa8; margin: 0 0 12rpx 8rpx; }
.faq-card { background: #fff; border-radius: 20rpx; overflow: hidden; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.faq-item { padding: 28rpx; border-bottom: 1rpx solid #f0f2f5; }
.faq-item:last-child { border-bottom: none; }
.faq-question { display: flex; justify-content: space-between; align-items: center; gap: 24rpx; }
.question-text { flex: 1; font-size: 28rpx; color: #1f2d3d; font-weight: 600; line-height: 1.45; }
.question-arrow { flex: 0 0 auto; font-size: 28rpx; color: #c0c6cc; }
.faq-answer { display: block; margin-top: 18rpx; font-size: 24rpx; color: #5f6f7f; line-height: 1.65; }
</style>
