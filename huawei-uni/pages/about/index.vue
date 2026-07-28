<template>
  <view class="about-page">
    <!-- 顶部品牌区域 -->
    <view class="brand-section">
      <image src="../../static/logo.svg" class="brand-logo"></image>
      <text class="brand-name">华微智检</text>
      <text class="brand-subtitle">专业的医疗检测服务平台</text>
    </view>

    <!-- 功能列表 -->
    <view class="function-section">
      <view class="function-card">
        <view class="function-item" @click="goToPrivacy">
          <text class="item-text">隐私协议</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
        <view class="function-item" @click="goToService">
          <text class="item-text">用户服务协议</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
      </view>
    </view>

    <!-- 底部信息 -->
    <view class="footer-section">
      <view class="info-item">
        <text class="info-label">版本号</text>
        <text class="info-value">{{ versionName }}</text>
      </view>
      <view class="icp-item">
        <text class="icp-text">黑ICP备2026004750号-2X</text>
      </view>
    </view>
  </view>
</template>

<script>
import manifest from '../../manifest.json'

export default {
  data() {
    return {
      versionName: manifest.versionName || '1.0.0'
    }
  },
  methods: {
    goToPrivacy() {
      uni.navigateTo({ url: '/pages/about/privacy' })
    },
    goToService() {
      uni.navigateTo({ url: '/pages/about/service' })
    }
  }
}
</script>

<style scoped>
.about-page {
  padding: 32rpx;
  min-height: 100vh;
  background-color: #F5F7FA;
  display: flex;
  flex-direction: column;
}

/* 顶部品牌区域 */
.brand-section {
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 80rpx 0 64rpx;
}

.brand-logo {
  width: 160rpx;
  height: 160rpx;
  margin-bottom: 24rpx;
}

.brand-name {
  font-size: 36rpx;
  font-weight: 600;
  color: #1F2D3D;
  margin-bottom: 12rpx;
}

.brand-subtitle {
  font-size: 24rpx;
  color: #8C9AA8;
}

/* 功能列表 */
.function-section {
  margin-bottom: 64rpx;
}

.function-card {
  background-color: #fff;
  border-radius: 16rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
  overflow: hidden;
}

.function-item {
  height: 96rpx;
  display: flex;
  align-items: center;
  padding: 0 24rpx;
  border-bottom: 1rpx solid #F0F2F5;
  transition: all 0.3s ease;
}

.function-item:last-child {
  border-bottom: none;
}

.function-item:hover {
  background-color: #F9FAFB;
}

.item-text {
  flex: 1;
  font-size: 24rpx;
  color: #1F2D3D;
}

.item-arrow {
  font-size: 24rpx;
  color: #C0C6CC;
}

/* 底部信息 */
.footer-section {
  margin-top: auto;
  padding-bottom: 48rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
}

.info-item {
  display: flex;
  align-items: center;
  gap: 16rpx;
  margin-bottom: 24rpx;
}

.info-label {
  font-size: 24rpx;
  color: #8C9AA8;
}

.info-value {
  font-size: 24rpx;
  color: #1F2D3D;
  font-weight: 500;
}

.icp-item {
  padding: 12rpx 24rpx;
}

.icp-text {
  font-size: 22rpx;
  color: #8C9AA8;
}

/* 响应式调整 */
@media (max-width: 375px) {
  .about-page {
    padding: 24rpx 24rpx 48rpx;
  }
  
  .brand-section {
    padding: 64rpx 0 48rpx;
  }
  
  .brand-logo {
    width: 128rpx;
    height: 128rpx;
  }
  
  .brand-name {
    font-size: 32rpx;
  }
  
  .brand-subtitle {
    font-size: 22rpx;
  }
  
  .function-item {
    padding: 0 20rpx;
  }
  
  .item-text {
    font-size: 22rpx;
  }
}
</style>
