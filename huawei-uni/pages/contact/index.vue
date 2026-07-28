<template>
  <view class="page-container">
    <view class="page-header">
      <text class="page-title">联系我们</text>
      <text class="page-desc">华微智检客户服务</text>
    </view>

    <view class="contact-card" @click="callPhone">
      <view class="contact-content">
        <text class="contact-title">电话咨询</text>
        <text class="contact-main">400-8788-674</text>
        <text class="contact-sub">周一至周五 9:00-18:00</text>
      </view>
    </view>

    <view class="contact-card" @click="copyEmail">
      <view class="contact-content">
        <text class="contact-title">邮件联系</text>
        <text class="contact-main">HW@huaweibio.com.cn</text>
        <text class="contact-sub">48小时内回复</text>
      </view>
    </view>

    <view class="contact-card">
      <view class="contact-content">
        <text class="contact-title">公司地址</text>
        <text class="contact-main">哈尔滨市香坊区旭升南街23-8号2层</text>
        <text class="contact-sub">研发实验中心</text>
      </view>
    </view>
  </view>
</template>

<script>
export default {
  methods: {
    callPhone() {
      uni.makePhoneCall({ phoneNumber: '400-8788-674' })
    },
    copyEmail() {
      uni.setClipboardData({
        data: 'HW@huaweibio.com.cn',
        success: () => uni.showToast({ title: '邮箱已复制', icon: 'success' })
      })
    }
  }
}
</script>

<style scoped>
.page-container { min-height: 100vh; padding: 32rpx; background: #F5F7FA; box-sizing: border-box; }
.page-header { margin-bottom: 32rpx; }
.page-title { display: block; font-size: 36rpx; font-weight: 700; color: #1f2d3d; margin-bottom: 8rpx; }
.page-desc { display: block; font-size: 24rpx; color: #8c9aa8; }
.contact-card { background: #fff; border-radius: 20rpx; padding: 32rpx; margin-bottom: 24rpx; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.contact-content { flex: 1; min-width: 0; }
.contact-title { display: block; font-size: 24rpx; color: #8c9aa8; margin-bottom: 10rpx; }
.contact-main { display: block; font-size: 30rpx; color: #1f2d3d; font-weight: 600; line-height: 1.45; word-break: break-all; margin-bottom: 8rpx; }
.contact-sub { display: block; font-size: 24rpx; color: #1677ff; }
</style>
