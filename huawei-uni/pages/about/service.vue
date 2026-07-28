<template>
  <view class="service-page">
    <scroll-view scroll-y class="scroll-container">
      <view class="content">
        <view class="title-section">
          <text class="title">用户服务协议</text>
          <text class="update-time">更新时间：2026年1月1日</text>
        </view>

        <view class="section">
          <text class="section-title">1. 协议接受</text>
          <text class="section-text">欢迎使用华微智检平台。在使用我们的服务之前，请您仔细阅读本用户服务协议。如果您不同意本协议的任何内容，请不要使用我们的服务。您使用服务的行为将被视为对本协议的接受。</text>
        </view>

        <view class="section">
          <text class="section-title">2. 服务描述</text>
          <text class="section-text">华微智检平台为用户提供医疗检测预约、样本管理、检测结果查询等服务。我们致力于为用户提供专业、便捷、可靠的医疗检测服务。</text>
        </view>

        <view class="section">
          <text class="section-title">3. 用户账户</text>
          <text class="section-text">您需要注册并登录账户才能使用我们的部分服务。您应妥善保管账户信息，对账户下的所有活动负责。如发现账户被非法使用，请立即通知我们。</text>
        </view>

        <view class="section">
          <text class="section-title">4. 用户行为规范</text>
          <text class="section-text">您在使用服务时应遵守法律法规，不得从事任何违法活动，不得侵犯他人合法权益，不得干扰或破坏服务的正常运行。</text>
        </view>

        <view class="section">
          <text class="section-title">5. 服务变更与终止</text>
          <text class="section-text">我们可能会根据业务需要变更或终止部分或全部服务。我们会提前通知用户，并尽力减少对用户的影响。</text>
        </view>

        <view class="section">
          <text class="section-title">6. 知识产权</text>
          <text class="section-text">华微智检平台及其相关内容的知识产权归我们所有。未经许可，您不得复制、修改、传播或以其他方式使用这些内容。</text>
        </view>

        <view class="section">
          <text class="section-title">7. 免责声明</text>
          <text class="section-text">在法律允许的范围内，我们不对因不可抗力、网络故障等原因导致的服务中断或数据丢失承担责任。我们的服务按“现状”提供，不保证服务不会中断或完全无误。</text>
        </view>

        <view class="section">
          <text class="section-title">8. 协议修改</text>
          <text class="section-text">我们有权适时修改本协议。修改后的协议会在平台上发布，发布后即生效。继续使用服务即表示您接受修改后的协议。</text>
        </view>

        <view class="section">
          <text class="section-title">9. 争议解决</text>
          <text class="section-text">如发生争议，双方应友好协商解决。协商不成的，任何一方均可向有管辖权的人民法院提起诉讼。</text>
        </view>

        <view class="section">
          <text class="section-title">10. 联系我们</text>
          <text class="section-text">如有任何疑问，请通过客服渠道与我们联系。</text>
        </view>
      </view>
    </scroll-view>
  </view>
</template>

<script>
export default {
  data() {
    return {}
  }
}
</script>

<style scoped>
.service-page {
  min-height: 100vh;
  background-color: #F5F7FA;
}

.scroll-container {
  height: 100vh;
}

.content {
  padding: 32rpx;
}

.title-section {
  background-color: #fff;
  border-radius: 16rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
}

.title {
  font-size: 32rpx;
  font-weight: 600;
  color: #1F2D3D;
  display: block;
  margin-bottom: 16rpx;
}

.update-time {
  font-size: 22rpx;
  color: #8C9AA8;
  display: block;
}

.section {
  background-color: #fff;
  border-radius: 16rpx;
  padding: 32rpx;
  margin-bottom: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
}

.section-title {
  font-size: 26rpx;
  font-weight: 600;
  color: #1F2D3D;
  display: block;
  margin-bottom: 16rpx;
}

.section-text {
  font-size: 24rpx;
  color: #4E5969;
  line-height: 1.8;
  display: block;
}

/* 响应式调整 */
@media (max-width: 375px) {
  .content {
    padding: 24rpx 24rpx 48rpx;
  }
  
  .title-section,
  .section {
    padding: 24rpx;
  }
  
  .title {
    font-size: 28rpx;
  }
  
  .section-title {
    font-size: 24rpx;
  }
  
  .section-text {
    font-size: 22rpx;
  }
}
</style>
