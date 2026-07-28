<template>
  <view class="home-container">
    <!-- 患者首页 -->
    <view v-if="userIdentity === 'patient' || !userIdentity" class="patient-home">
      <view class="top-banner">
        <image src="../../static/logo.svg" class="banner-image" mode="aspectFit"></image>
        <view class="banner-content">
          <text class="banner-title">华微智检</text>
          <text class="banner-subtitle">{{ userIdentity ? '您的专业健康管家' : '登录后可使用完整功能' }}</text>
        </view>
      </view>

      <view v-if="!userIdentity" class="guest-login-strip">
        <view class="guest-login-copy">
          <text class="guest-login-title">您可以先浏览功能</text>
          <text class="guest-login-desc">点击具体服务时再登录</text>
        </view>
        <button class="guest-login-btn" @click="goToLogin">登录</button>
      </view>
      
      <view class="main-functions">
        <view class="function-card" @click="信息完善">
          <view class="function-content">
            <text class="function-title">信息完善</text>
            <text class="function-desc">完善个人信息</text>
          </view>
          <view class="function-icon">
            <image src="../../static/infor.png" class="icon-img" mode="aspectFit"></image>
          </view>
        </view>
        <view class="function-card" @click="查看结果">
          <view class="function-content">
            <text class="function-title">报告查询</text>
            <text class="function-desc">下载检验报告</text>
          </view>
          <view class="function-icon">
            <image src="../../static/report.png" class="icon-img" mode="aspectFit"></image>
          </view>
        </view>
      </view>
      
      <view class="service-section">
        <view class="section-header">
          <text class="section-title">样本服务</text>
        </view>
        <view class="service-items">
          <view class="sub-function-item" @click="我的样本">
            <view class="sub-function-icon-circle">
              <image src="../../static/Blood.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">我的样本</text>
          </view>
          <view class="sub-function-item" @click="预约检测">
            <view class="sub-function-icon-circle">
              <image src="../../static/blood-appoint.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">预约检测</text>
          </view>
          <view class="sub-function-item" @click="邮寄样本">
            <view class="sub-function-icon-circle">
              <image src="../../static/post.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">样本邮寄</text>
          </view>
          <view class="sub-function-item" @click="进度查询">
            <view class="sub-function-icon-circle">
              <image :src="progressIcon" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">进度查询</text>
          </view>
        </view>
      </view>
      
      <view class="service-section">
        <view class="section-header">
          <text class="section-title">患者服务</text>
        </view>
        <view class="service-items">
          <view class="sub-function-item" @click="修改信息">
            <view class="sub-function-icon-circle">
              <image src="../../static/infor.png" class="circle-icon"></image>
            </view>
            <text class="sub-function-text">修改信息</text>
          </view>
          <view class="sub-function-item" @click="科学疗养">
            <view class="sub-function-icon-circle">
              <image src="../../static/heart.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">科学疗养</text>
          </view>
        </view>
      </view>
      
      <view class="service-section">
        <view class="section-header">
          <text class="section-title">综合服务</text>
        </view>
        <view class="service-items">
          <view class="sub-function-item" @click="随访管理">
            <view class="sub-function-icon-circle">
              <image src="../../static/follow-up.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">随访管理</text>
          </view>
          <view class="sub-function-item" @click="帮助中心">
            <view class="sub-function-icon-circle">
              <image src="../../static/help.png" class="circle-icon" mode="aspectFit"></image>
            </view>
            <text class="sub-function-text">帮助中心</text>
          </view>
        </view>
      </view>
    </view>

    <!-- 员工首页 -->
    <view v-else-if="userIdentity === 'employee'" class="employee-home">
      <view class="top-banner">
        <image src="../../static/logo.svg" class="banner-image" mode="aspectFit"></image>
        <view class="banner-content">
          <text class="banner-title">华微智检</text>
          <text class="banner-subtitle">员工工作台</text>
        </view>
      </view>
      
      <view class="todo-card">
        <view class="todo-header">
          <text class="todo-title">今日待办</text>
          <text class="todo-date">{{ currentDate }}</text>
        </view>
        <view class="todo-stats">
          <view v-if="!isSalesRole" class="stat-item" @click="goToSampleReceive">
            <text class="stat-number">{{ todoStats.pendingSamples }}</text>
            <text class="stat-label">待接收样本</text>
          </view>
          <view v-if="!isSalesRole" class="stat-divider"></view>
          <view v-if="!isSalesRole" class="stat-item" @click="goToReportReview">
            <text class="stat-number">{{ todoStats.pendingReports }}</text>
            <text class="stat-label">待审核报告</text>
          </view>
          <view v-if="!isSalesRole" class="stat-divider"></view>
          <view class="stat-item" @click="goToPatientList">
            <text class="stat-number">{{ todoStats.newPatients }}</text>
            <text class="stat-label">新患录入</text>
          </view>
        </view>
      </view>
      
      <view class="function-section">
        <view class="section-header">
          <text class="section-title">工作台</text>
        </view>
        <view class="function-grid">
          <view class="function-card" @click="新患录入">
            <view class="function-icon-bg">
              <image src="../../static/infor.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">新患录入</text>
          </view>
          <view class="function-card" @click="我的患者">
            <view class="function-icon-bg">
              <image src="../../static/patient.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">我的患者</text>
          </view>
          <view v-if="!isSalesRole" class="function-card" @click="样本接收">
            <view class="function-icon-bg">
              <image src="../../static/simple-receive.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">样本接收</text>
          </view>
          <view class="function-card" @click="新增样本">
            <view class="function-icon-bg">
              <image src="../../static/simple-add.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">新增样本</text>
          </view>
          <view class="function-card" @click="样本详情">
            <view class="function-icon-bg">
              <image src="../../static/simple-list.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">样本详情</text>
          </view>
          <view v-if="!isSalesRole" class="function-card" @click="报告审核">
            <view class="function-icon-bg">
              <image src="../../static/report.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">报告审核</text>
          </view>
          <view class="function-card" @click="我的邀请码">
            <view class="function-icon-bg">
              <image src="../../static/QRcode.png" class="function-icon" mode="aspectFit"></image>
            </view>
            <text class="function-title">我的邀请码</text>
          </view>
        </view>
      </view>
      
      <view class="function-section">
        <view class="section-header">
          <text class="section-title">快捷操作</text>
        </view>
        <view class="action-grid">
          <view class="action-card" @click="采样记录">
            <view class="action-icon-bg">
              <image src="../../static/Blood.png" class="action-icon" mode="aspectFit"></image>
            </view>
            <text class="action-title">采样记录</text>
          </view>
          <view class="action-card" @click="异常样本">
            <view class="action-icon-bg">
              <image src="../../static/heart.png" class="action-icon" mode="aspectFit"></image>
            </view>
            <text class="action-title">异常样本</text>
          </view>
          <view class="action-card" @click="消息通知">
            <view class="action-icon-bg">
              <image src="../../static/follow-up.png" class="action-icon" mode="aspectFit"></image>
            </view>
            <text class="action-title">消息通知</text>
          </view>
          <view class="action-card" @click="报告查询">
            <view class="action-icon-bg">
              <image src="../../static/report.png" class="action-icon" mode="aspectFit"></image>
            </view>
            <text class="action-title">报告查询</text>
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
      userIdentity: '',
      todoStats: {
        pendingSamples: 0,
        pendingReports: 0,
        newPatients: 0
      },
      currentDate: '',
      progressIcon: ''
    }
  },
  computed: {
    employeeInfo() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      return userInfo.employee || userInfo.userInfo || {}
    },
    roleName() {
      const role = this.employeeInfo.role || {}
      return String(role.name || this.employeeInfo.role_name || this.employeeInfo.roleName || '').trim()
    },
    isSalesRole() {
      return /销售|客户经理|sale|sales/i.test(this.roleName)
    }
  },
  onLoad() {
    this.initPage()
    this.getProgressIcon()
  },
  onShow() {
    this.checkIdentity()
    if (this.userIdentity === 'employee') {
      this.loadTodoStats()
    }
  },
  methods: {
    initPage() {
      this.checkIdentity()
      this.getCurrentDate()
    },
    checkIdentity() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      this.userIdentity = userInfo.identity || ''
    },
    getCurrentDate() {
      const date = new Date()
      const year = date.getFullYear()
      const month = String(date.getMonth() + 1).padStart(2, '0')
      const day = String(date.getDate()).padStart(2, '0')
      this.currentDate = `${year}-${month}-${day}`
    },
    getProgressIcon() {
      this.progressIcon = 'data:image/svg+xml,' + encodeURIComponent(`
        <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" viewBox="0 0 24 24" fill="#1677FF">
          <path d="M12 2C6.48 2 2 6.48 2 12s4.48 10 10 10 10-4.48 10-10S17.52 2 12 2Zm1 15h-2v-6h2v6Zm0-8h-2V7h2v2Z"/>
        </svg>
      `)
    },
    async loadTodoStats() {
      try {
        const response = await uniAPI.getEmployeeStats()
        if (response.success && response.data) {
          this.todoStats.pendingSamples = response.data.pendingSamples || response.data.pending_samples || 0
          this.todoStats.pendingReports = response.data.pendingReports || response.data.pending_reports || 0
          this.todoStats.newPatients = response.data.newPatients || response.data.new_patients || 0
        }
      } catch (error) {
        console.error('Load todo stats failed:', error)
      }
    },
    checkLogin() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      if (!userInfo.identity) {
        uni.navigateTo({ url: '/pages/login/index' })
        return false
      }
      return true
    },
    goToLogin() {
      uni.navigateTo({ url: '/pages/login/index' })
    },
    // 患者功能
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
    },
    // 员工功能
    新患录入() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/patient-create/index' })
    },
    我的患者() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/patients/index' })
    },
    样本接收() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/sample-receive/index' })
    },
    新增样本() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/patients/index?select=1' })
    },
    样本详情() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/samples/index' })
    },
    报告审核() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/report-review/index' })
    },
    我的邀请码() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/invite-code/index' })
    },
    采样记录() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/sample-receive/index' })
    },
    异常样本() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/sample-receive/index' })
    },
    消息通知() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/help-center/index' })
    },
    报告查询() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/reports/index' })
    },
    goToSampleReceive() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/sample-receive/index' })
    },
    goToReportReview() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/report-review/index' })
    },
    goToPatientList() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/patients/index' })
    }
  }
}
</script>

<style scoped>
.home-container {
  min-height: 100vh;
  background-color: #F5F7FA;
}

/* 患者首页样式 */
.patient-home {
  padding: 32rpx;
  box-sizing: border-box;
}

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

.function-content {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-width: 0;
}

.function-icon {
  width: 96rpx;
  height: 96rpx;
  margin-left: 16rpx;
  display: flex;
  align-items: center;
  justify-content: center;
  flex-shrink: 0;
}

.icon-img {
  width: 96rpx;
  height: 96rpx;
  display: block;
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
}

.circle-icon {
  width: 48rpx;
  height: 48rpx;
  display: block;
}

.sub-function-text {
  font-size: 22rpx;
  color: #1F2D3D;
  display: block;
}

/* 员工首页样式 */
.employee-home {
  padding: 32rpx;
  padding-bottom: 0;
  box-sizing: border-box;
}

.todo-card {
  background: linear-gradient(135deg, #1677FF 0%, #409EFF 100%);
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 32rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.15);
  color: #fff;
}

.todo-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24rpx;
}

.todo-title {
  font-size: 28rpx;
  font-weight: 600;
}

.todo-date {
  font-size: 20rpx;
  opacity: 0.9;
}

.todo-stats {
  display: flex;
  justify-content: space-around;
  align-items: center;
}

.stat-item {
  display: flex;
  flex-direction: column;
  align-items: center;
  flex: 1;
}

.stat-number {
  font-size: 36rpx;
  font-weight: bold;
  margin-bottom: 8rpx;
}

.stat-label {
  font-size: 20rpx;
  opacity: 0.9;
}

.stat-divider {
  width: 1rpx;
  height: 60rpx;
  background-color: rgba(255, 255, 255, 0.3);
}

.function-section {
  margin-bottom: 32rpx;
  background-color: #fff;
  border-radius: 20rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.08);
  overflow: hidden;
}

.function-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  padding: 24rpx;
}

.action-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12rpx;
  padding: 24rpx;
}

.employee-home .function-card {
  min-height: 156rpx;
  padding: 20rpx 8rpx;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 2rpx 10rpx rgba(22, 119, 255, 0.06);
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
}

.employee-home .function-card:active {
  background-color: #f9fafb;
}

.function-icon-bg {
  width: 88rpx;
  height: 88rpx;
  border-radius: 50%;
  background-color: #E6F4FF;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 16rpx;
  box-sizing: border-box;
}

.employee-home .function-icon-bg .function-icon {
  width: 48rpx;
  height: 48rpx;
  display: block;
  margin: 0;
}

.function-title {
  font-size: 24rpx;
  font-weight: 500;
  color: #1F2D3D;
}

.action-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 136rpx;
  padding: 18rpx 8rpx;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 2rpx 10rpx rgba(22, 119, 255, 0.05);
  transition: all 0.3s ease;
}

.action-card:active {
  background-color: #f9fafb;
}

.action-icon-bg {
  width: 80rpx;
  height: 80rpx;
  border-radius: 50%;
  background-color: #fff;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  margin-bottom: 12rpx;
}

.action-icon {
  width: 40rpx;
  height: 40rpx;
  display: block;
}

.action-title {
  font-size: 20rpx;
  color: #1F2D3D;
  text-align: center;
}

.guest-login-strip {
  background: #fff;
  border-radius: 20rpx;
  padding: 24rpx;
  margin-bottom: 32rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 24rpx;
}

.guest-login-copy {
  flex: 1;
  min-width: 0;
}

.guest-login-title {
  display: block;
  font-size: 28rpx;
  font-weight: 600;
  color: #1F2D3D;
  margin-bottom: 8rpx;
}

.guest-login-desc {
  display: block;
  font-size: 22rpx;
  color: #8C9AA8;
}

.guest-login-btn {
  width: 156rpx;
  height: 64rpx;
  border-radius: 32rpx;
  background-color: #1677FF;
  color: #fff;
  font-size: 26rpx;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  margin: 0;
}
</style>
