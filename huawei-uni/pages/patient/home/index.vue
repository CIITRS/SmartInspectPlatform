<template>
  <view class="patient-home">
    <!-- 顶部图片区 -->
    <view class="top-banner">
      <image src="../../../static/logo.svg" class="banner-image" mode="aspectFit"></image>
      <view class="banner-content">
        <text class="banner-title">华微智检</text>
        <text class="banner-subtitle">您的专业健康管家</text>
      </view>
    </view>
    
    <!-- 主要功能入口 -->
    <view class="main-functions">
      <view class="function-card" @click="信息完善">
        <view class="function-content">
          <text class="function-title">信息完善</text>
          <text class="function-desc">完善个人信息</text>
        </view>
        <view class="function-icon">
          <image src="../../../static/infor.png" class="icon-img" mode="aspectFit"></image>
        </view>
      </view>
      <view class="function-card" @click="查看结果">
        <view class="function-content">
          <text class="function-title">查看结果</text>
          <text class="function-desc">下载检验报告</text>
        </view>
        <view class="function-icon">
          <image src="../../../static/report.png" class="icon-img" mode="aspectFit"></image>
        </view>
      </view>
    </view>
    
    <!-- 服务分区 -->
    <!-- 样本服务 -->
    <view class="service-section">
      <view class="section-header">
        <text class="section-title">样本服务</text>
      </view>
      <view class="service-items">
        <view class="sub-function-item" @click="我的样本">
          <view class="sub-function-icon-circle">
            <image src="../../../static/Blood.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">我的样本</text>
        </view>
        <view class="sub-function-item" @click="预约检测">
          <view class="sub-function-icon-circle">
            <image src="../../../static/blood-appoint.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">预约检测</text>
        </view>
        <view class="sub-function-item" @click="邮寄样本">
          <view class="sub-function-icon-circle">
            <image src="../../../static/post.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">样本邮寄</text>
        </view>
        <view class="sub-function-item" @click="进度查询">
          <view class="sub-function-icon-circle">
            <image src="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='32' height='32' viewBox='0 0 24 24' fill='%231677FF'%3E%3Cpath d='M12 2C6.48 2 2 6.48 2 12C2 17.52 6.48 22 12 22C17.52 22 22 17.52 22 12C22 6.48 17.52 2 12 2ZM13 17H11V11H13V17ZM13 9H11V7H13V9Z'/%3E%3C/svg%3E" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">进度查询</text>
        </view>
      </view>
    </view>
    
    <!-- 患者服务 -->
    <view class="service-section">
      <view class="section-header">
        <text class="section-title">患者服务</text>
      </view>
      <view class="service-items">
        <view class="sub-function-item" @click="修改信息">
          <view class="sub-function-icon-circle">
            <image src="../../../static/infor.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">修改信息</text>
        </view>
        <view class="sub-function-item" @click="科学疗养">
          <view class="sub-function-icon-circle">
            <image src="../../../static/heart.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">科学疗养</text>
        </view>
      </view>
    </view>
    
    <!-- 综合服务 -->
    <view class="service-section">
      <view class="section-header">
        <text class="section-title">综合服务</text>
      </view>
      <view class="service-items">
        <view class="sub-function-item" @click="随访管理">
          <view class="sub-function-icon-circle">
            <image src="../../../static/follow-up.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">随访管理</text>
        </view>
        <view class="sub-function-item" @click="帮助中心">
          <view class="sub-function-icon-circle">
            <image src="../../../static/help.png" class="circle-icon" mode="aspectFit"></image>
          </view>
          <text class="sub-function-text">帮助中心</text>
        </view>
      </view>
    </view>
  
  </view>
</template>

<script>
import { refreshTabBarFromStorage } from '../../../utils/auth.js'
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      userInfo: uni.getStorageSync('userInfo') || {},
      recentReports: []
    }
  },
  onLoad() {
    const userInfo = uni.getStorageSync('userInfo') || {}
    if (userInfo.identity === 'employee') {
      uni.switchTab({
        url: '/pages/employee/home/index'
      })
    }
  },
  onShow() {
    refreshTabBarFromStorage()
    this.promptAssignManager()
  },
  methods: {
    async promptAssignManager() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      if (userInfo.identity !== 'patient') return
      const patient = userInfo.patient || userInfo.patientInfo || userInfo
      const source = patient.patient_source || patient.patientSource || userInfo.patient_source || userInfo.patientSource
      const salesPerson = patient.sales_person || patient.salesPerson || userInfo.sales_person || userInfo.salesPerson
      if (source !== 'miniapp_self' || salesPerson) return
      const promptKey = `manager_assign_prompt_${patient.id || patient.patient_id || patient.patientCode || 'self'}`
      if (uni.getStorageSync(promptKey)) return
      try {
        const res = await uniAPI.getPatientManager()
        if (res && res.success && res.data && (res.data.name || res.data.phone)) return
      } catch (error) {
        // 未分配客户经理时继续提示用户联系。
      }
      uni.setStorageSync(promptKey, '1')
      uni.showModal({
        title: '联系我们',
        content: '为了更好服务您，请您点击联系我们为您分配专属客户经理。',
        confirmText: '联系我们',
        cancelText: '取消',
        success: (res) => {
          if (res.confirm) {
            uni.navigateTo({ url: '/pages/contact/index' })
          }
        }
      })
    },
    checkLogin() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      if (!userInfo.identity) {
        uni.navigateTo({ url: '/pages/login/index' })
        return false
      }
      return true
    },
    预约检测() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/appointment/index' })
    },
    查看结果() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/reports/index' })
    },
    信息完善() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/profile/index' })
    },
    邮寄样本() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/mail-sample/index' })
    },
    我的样本() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/samples/index' })
    },
    进度查询() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/progress/index' })
    },
    修改信息() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/profile/index' })
    },
    科学疗养() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/science-care/index' })
    },
    随访管理() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/follow-up/index' })
    },
    帮助中心() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/help-center/index' })
    }
  }
}
</script>

<style scoped>
.patient-home {
  padding: 32rpx;
  min-height: 100vh;
  background-color: #F5F7FA;
  box-sizing: border-box;
}

/* 顶部图片区 */
.top-banner {
  width: 100%;
  height: 240rpx;
  background: linear-gradient(135deg, #E6F4FF 0%, #D0E8FF 100%);
  border-radius: 24rpx;
  margin-bottom: 32rpx;
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 24rpx;
  padding: 0 32rpx;
  box-sizing: border-box;
  box-shadow: 0 4rpx 16rpx rgba(0, 119, 255, 0.10);
  overflow: hidden;
}

.banner-image {
  width: 120rpx;
  height: 120rpx;
  opacity: 0.9;
  flex: 0 0 120rpx;
}

.banner-content {
  flex: 1;
  min-width: 0;
  margin-left: 0;
}

.banner-title {
  font-size: 32rpx;
  font-weight: bold;
  color: #1677FF;
  margin-bottom: 8rpx;
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.banner-subtitle {
  font-size: 24rpx;
  color: #1677FF;
  display: block;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
/* 主要功能入口 */
.main-functions {
  display: flex;
  gap: 24rpx;
  margin-bottom: 32rpx;
}

.function-card {
  flex: 1;
  background-color: #fff;
  border-radius: 20rpx;
  padding: 32rpx 24rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.08);
  display: flex;
  flex-direction: row;
  align-items: center;
  justify-content: space-between;
  transition: all 0.3s ease;
  min-height: 160rpx;
}

.function-card:hover {
  transform: translateY(-4rpx);
  box-shadow: 0 8rpx 24rpx rgba(22, 119, 255, 0.12);
}

.function-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.function-icon {
  width: 96rpx;
  height: 96rpx;
  min-width: 96rpx;
  min-height: 96rpx;
  margin-left: 16rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-img {
  width: 96rpx;
  height: 96rpx;
  min-width: 96rpx;
  min-height: 96rpx;
  flex-shrink: 0;
}

.function-title {
  font-size: 28rpx;
  font-weight: 600;
  color: #1F2D3D;
  margin-bottom: 8rpx;
}

.function-desc {
  font-size: 20rpx;
  color: #8C9AA8;
  text-align: left;
}

/* 服务分区 */
.service-section {
  margin-bottom: 32rpx;
  background-color: #fff;
  border-radius: 20rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.08);
  overflow: hidden;
}

.section-header {
  background-color: #E6F4FF;
  padding: 24rpx;
}

.section-title {
  font-size: 28rpx;
  font-weight: 600;
  color: #1F2D3D;
}

.service-items {
  padding: 24rpx;
  background-color: #fff;
}

.sub-function-item {
  display: inline-block;
  width: 25%;
  text-align: center;
  margin-bottom: 24rpx;
}

.sub-function-item:nth-last-child(-n+4) {
  margin-bottom: 0;
}

.sub-function-icon-circle {
  width: 100rpx;
  height: 100rpx;
  border-radius: 50%;
  background-color: #fff;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0 auto 16rpx;
  transition: all 0.3s ease;
}

.sub-function-icon-circle:hover {
  transform: translateY(-4rpx);
  box-shadow: 0 8rpx 24rpx rgba(22, 119, 255, 0.18);
}

.circle-icon {
  width: 48rpx;
  height: 48rpx;
}

.sub-function-text {
  font-size: 22rpx;
  color: #1F2D3D;
  display: block;
}

/* 最近报告 */
.recent-reports {
  margin-bottom: 32rpx;
}

.report-card {
  background-color: #fff;
  border-radius: 16rpx;
  padding: 24rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
}

.report-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16rpx;
}

.report-name {
  font-size: 24rpx;
  font-weight: 500;
  color: #1F2D3D;
}

.report-date {
  font-size: 20rpx;
  color: #8C9AA8;
}

.report-status {
  font-size: 20rpx;
  color: #67C23A;
  margin-bottom: 20rpx;
  display: inline-block;
  padding: 4rpx 12rpx;
  background-color: #F0F9EB;
  border-radius: 12rpx;
}

.report-status.abnormal {
  color: #F56C6C;
  background-color: #FEF0F0;
}

.view-report-btn {
  width: 100%;
  height: 64rpx;
  background-color: #EAF3FF;
  color: #1677FF;
  border: none;
  border-radius: 12rpx;
  font-size: 24rpx;
  font-weight: 500;
}

/* 响应式调整 */
@media (max-width: 375px) {
  .patient-home {
    padding: 24rpx 24rpx 140rpx;
  }
  
  .top-banner {
    height: 200rpx;
    padding: 0 32rpx;
  }
  
  .banner-title {
    font-size: 28rpx;
  }
  
  .banner-subtitle {
    font-size: 22rpx;
  }
  
  .function-card {
    padding: 24rpx 16rpx;
    min-height: 140rpx;
  }
  
  .function-icon {
    width: 80rpx;
    height: 80rpx;
    min-width: 80rpx;
    min-height: 80rpx;
    margin-left: 12rpx;
  }
  
  .icon-img {
    width: 80rpx;
    height: 80rpx;
    min-width: 80rpx;
    min-height: 80rpx;
    flex-shrink: 0;
  }
  
  .function-title {
    font-size: 26rpx;
  }
  
  .function-desc {
    font-size: 18rpx;
  }
  
  .sub-function-item {
    width: 25%;
  }
  
  .sub-function-icon-circle {
    width: 80rpx;
    height: 80rpx;
  }
  
  .circle-icon {
    width: 40rpx;
    height: 40rpx;
  }
  
  .sub-function-text {
    font-size: 20rpx;
  }
}
</style>
