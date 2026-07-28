<template>
  <view class="privacy-page">
    <scroll-view scroll-y class="scroll-container">
      <view class="content">
        <text class="page-title" user-select="true">华微智慧小程序隐私保护指引</text>

        <view class="section">
          <text class="paragraph" user-select="true">本指引是华微智慧小程序开发者华微智检医疗科技(哈尔滨)有限公司（以下简称“开发者”）为处理你的个人信息而制定。</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">一、开发者处理的信息</text>
          <text class="paragraph" user-select="true">根据法律规定，开发者仅处理实现小程序功能所必要的信息。</text>
          <text v-for="(item, index) in infoItems" :key="index" class="list-item" user-select="true">{{ index + 1 }}. {{ item }}</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">二、未成年人保护</text>
          <text class="paragraph" user-select="true">根据相关法律法规的规定，若你是14周岁以下的未成年人，你需要和你的监护人共同仔细阅读本指引，并在征得监护人明示同意后继续使用小程序服务。开发者将根据相关法律法规的规定及本指引内容，处理经监护人同意而收集的未成年人用户信息，并通过【本指引你的权益部分】披露的内容保障未成年人在个人信息处理活动中的各项权利。</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">三、你的权益</text>
          <text class="paragraph" user-select="true">关于你的个人信息，你可以通过以下方式与开发者联系，行使查阅、复制、更正、删除等法定权利。</text>
          <text class="paragraph" user-select="true">若你在小程序中注册了账号，你可以通过以下方式与开发者联系，申请注销你在小程序中使用的账号。在受理你的申请后，开发者承诺在十五个工作日内完成核查和处理，并按照法律法规要求处理你的相关信息。</text>
          <text class="contact-line" user-select="true">电话：4008788674</text>
          <text class="contact-line" user-select="true">邮箱：HW@huaweibio.com.cn</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">四、开发者对信息的存储</text>
          <text class="paragraph" user-select="true">开发者承诺，除法律法规另有规定外，开发者对你的信息的保存期限应当为实现处理目的所必要的最短时间。</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">五、信息的使用规则</text>
          <text class="paragraph" user-select="true">开发者将会在本指引所明示的用途内使用收集的信息。</text>
          <text class="paragraph" user-select="true">如开发者使用你的信息超出本指引目的或合理范围，开发者必须在变更使用目的或范围前，再次以小程序首页弹窗、短信等方式告知并征得你的明示同意。</text>
        </view>

        <view class="section">
          <text class="section-title" user-select="true">六、信息对外提供</text>
          <text class="paragraph" user-select="true">开发者承诺，不会主动共享或转让你的信息至任何第三方，如存在确需共享或转让时，开发者应当直接征得或确认第三方征得你的单独同意。</text>
          <text class="paragraph" user-select="true">开发者承诺，不会对外公开披露你的信息，如必须公开披露时，开发者应当向你告知公开披露的目的、披露信息的类型及可能涉及的信息，并征得你的单独同意。</text>
          <text class="paragraph" user-select="true">你认为开发者未遵守上述约定，或有其他的投诉建议、或未成年人个人信息保护相关问题，可通过以下方式与开发者联系；或者向微信进行投诉。</text>
          <text class="contact-line" user-select="true">电话：4008788674</text>
          <text class="contact-line" user-select="true">邮箱：HW@huaweibio.com.cn</text>
        </view>

        <view class="section">
          <text class="paragraph" user-select="true">更新日期：2026-06-06</text>
          <text class="paragraph" user-select="true">本小程序已对用户的信息处理进行了逐一、如实的说明，并保证如有变更会及时更新指引。</text>
        </view>
      </view>
    </scroll-view>
  </view>
</template>

<script>
export default {
  data() {
    return {
      infoItems: [
        '开发者将在获取你的明示同意后，收集你的微信昵称、头像，用途是【为了同步系统内用户头像或昵称】。',
        '开发者将在获取你的明示同意后，收集你的位置信息，用途是【用于选择样本邮寄地址或者报告收货地址】。',
        '开发者收集你的地址，用于样本邮寄报告邮寄。',
        '开发者收集你的身份证号码，用于便于实名认证区分不同用户。',
        '开发者将在获取你的明示同意后，收集你的手机号，用途是【便于登录或注册识别您的身份】。',
        '开发者获取你选择的位置信息，用于选择样本邮寄地址或者报告收货地址。',
        '开发者收集你选中的文件，用于提交您的曾经检测报告文件。',
        '开发者收集你选中的照片或视频信息，用于提交您的曾经检测报告图片。',
        '开发者将在获取你的明示同意后，使用你的相册（仅写入）权限，用途是【保存报告电子版图片】。',
        '开发者收集你的邮箱，用于便于为您发送电子版报告、通知。'
      ]
    }
  }
}
</script>

<style scoped>
.privacy-page {
  min-height: 100vh;
  background-color: #F5F7FA;
}

.scroll-container {
  height: 100vh;
}

.content {
  padding: 32rpx;
  box-sizing: border-box;
}

.page-title {
  display: block;
  font-size: 36rpx;
  line-height: 1.45;
  font-weight: 700;
  color: #1F2D3D;
  margin-bottom: 32rpx;
}

.section {
  margin-bottom: 34rpx;
}

.section-title {
  display: block;
  font-size: 30rpx;
  line-height: 1.5;
  font-weight: 600;
  color: #1F2D3D;
  margin-bottom: 16rpx;
}

.paragraph,
.list-item,
.contact-line {
  display: block;
  font-size: 26rpx;
  line-height: 1.8;
  color: #1F2D3D;
  margin-bottom: 12rpx;
}

.list-item {
  padding-left: 8rpx;
}

.contact-line {
  font-weight: 500;
}

@media (max-width: 375px) {
  .content {
    padding: 24rpx 24rpx 48rpx;
  }

  .page-title {
    font-size: 34rpx;
  }

  .paragraph,
  .list-item,
  .contact-line {
    font-size: 24rpx;
  }
}
</style>
