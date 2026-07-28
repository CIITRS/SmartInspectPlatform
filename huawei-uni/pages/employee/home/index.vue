<template>
  <view class="employee-home">
    <!-- 顶部图片区 -->
    <view class="top-banner">
      <image src="../../../static/logo.svg" class="banner-image"></image>
      <view class="banner-content">
        <text class="banner-title">华微智检</text>
        <text class="banner-subtitle">员工工作台</text>
      </view>
    </view>
    
    <!-- 待办任务卡片 -->
    <view class="todo-card">
      <view class="todo-header">
        <text class="todo-title">今日待办</text>
        <text class="todo-date">{{ currentDate }}</text>
      </view>
      <view class="todo-stats">
        <block v-for="(item, index) in todoItems" :key="item.key">
          <view class="stat-item" @click="runAction(item.action)">
            <text class="stat-number">{{ item.value }}</text>
            <text class="stat-label">{{ item.label }}</text>
          </view>
          <view v-if="index < todoItems.length - 1" class="stat-divider"></view>
        </block>
        <view v-if="todoItems.length === 0" class="stat-empty">
          <text>暂无待办</text>
        </view>
      </view>
    </view>
    
    <!-- 员工功能宫格 -->
    <view class="function-section">
      <view class="section-header">
        <text class="section-title">{{ isSalesRole ? '销售工作台' : '工作台' }}</text>
      </view>
      <view class="function-grid" :class="{ sales: isSalesRole }">
        <view class="function-card" v-for="item in primaryFunctions" :key="item.title" @click="runAction(item.action)">
          <view class="function-icon-bg">
            <image :src="item.icon" class="function-icon"></image>
          </view>
          <text class="function-title">{{ item.title }}</text>
        </view>
      </view>
    </view>
    
    <!-- 快捷操作 -->
    <view class="function-section">
      <view class="section-header">
        <text class="section-title">快捷操作</text>
      </view>
      <view class="action-grid" :class="{ sales: isSalesRole }">
        <view class="action-card" v-for="item in quickActions" :key="item.title" @click="runAction(item.action)">
          <view class="action-icon-bg">
            <image :src="item.icon" class="action-icon"></image>
          </view>
          <text class="action-title">{{ item.title }}</text>
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
      todoStats: {
        pendingSamples: 0,
        pendingReports: 0,
        newPatients: 0
      },
      currentDate: ''
    }
  },
  computed: {
    employeeInfo() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      return userInfo.employee || userInfo.userInfo || {}
    },
    roleName() {
      const role = this.employeeInfo.role || {}
      return String(role.name || this.employeeInfo.role_name || this.employeeInfo.roleName || this.employeeInfo.position || '').trim()
    },
    isSalesRole() {
      return /销售|客户经理|sale|sales/i.test(this.roleName)
    },
    todoItems() {
      if (this.isSalesRole) {
        return [
          { key: 'patients', label: '我的患者', value: this.todoStats.newPatients, action: '我的患者' },
          { key: 'allocate', label: '新增样本', value: '去处理', action: '新增样本' },
          { key: 'invite', label: '邀请患者', value: '邀请', action: '我的邀请码' }
        ]
      }
      return [
        { key: 'samples', label: '待接收样本', value: this.todoStats.pendingSamples, action: '样本接收' },
        { key: 'reports', label: '待审核报告', value: this.todoStats.pendingReports, action: '报告审核' },
        { key: 'patients', label: '新患录入', value: this.todoStats.newPatients, action: '我的患者' }
      ]
    },
    primaryFunctions() {
      const salesFunctions = [
        { title: '新患录入', icon: '../../../static/infor.png', action: '新患录入' },
        { title: '我的患者', icon: '../../../static/patient.png', action: '我的患者' },
        { title: '新增样本', icon: '../../../static/simple-add.png', action: '新增样本' },
        { title: '样本详情', icon: '../../../static/simple-list.png', action: '样本详情' },
        { title: '我的邀请码', icon: '../../../static/QRcode.png', action: '我的邀请码' }
      ]
      if (this.isSalesRole) return salesFunctions
      return [
        ...salesFunctions.slice(0, 2),
        { title: '样本接收', icon: '../../../static/simple-receive.png', action: '样本接收' },
        salesFunctions[2],
        salesFunctions[3],
        { title: '报告审核', icon: '../../../static/report.png', action: '报告审核' },
        salesFunctions[4]
      ]
    },
    quickActions() {
      if (this.isSalesRole) {
        return [
          { title: '报告查询', icon: '../../../static/report.png', action: '报告查询' },
          { title: '消息通知', icon: '../../../static/follow-up.png', action: '消息通知' }
        ]
      }
      return [
        { title: '采样记录', icon: '../../../static/Blood.png', action: '采样记录' },
        { title: '异常样本', icon: '../../../static/heart.png', action: '异常样本' },
        { title: '消息通知', icon: '../../../static/follow-up.png', action: '消息通知' },
        { title: '报告查询', icon: '../../../static/report.png', action: '报告查询' }
      ]
    }
  },
  onLoad() {
    this.getCurrentDate()
    this.loadTodoStats()
  },
  onShow() {
    refreshTabBarFromStorage()
    this.loadTodoStats()
  },
  methods: {
    getCurrentDate() {
      const date = new Date()
      const year = date.getFullYear()
      const month = String(date.getMonth() + 1).padStart(2, '0')
      const day = String(date.getDate()).padStart(2, '0')
      this.currentDate = `${year}-${month}-${day}`
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
    runAction(action) {
      if (!action || typeof this[action] !== 'function') return
      this[action]()
    },
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
      if (this.isSalesRole) {
        uni.showToast({ title: '销售账号无审核权限', icon: 'none' })
        return
      }
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
      if (this.isSalesRole) {
        uni.showToast({ title: '销售账号无审核权限', icon: 'none' })
        return
      }
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
.employee-home {
  padding: 32rpx;
  padding-bottom: 0;
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

/* 待办任务卡片 */
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
  cursor: pointer;
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

.stat-empty {
  width: 100%;
  text-align: center;
  font-size: 24rpx;
  opacity: 0.9;
}

/* 功能区域 */
.function-section {
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

.function-grid {
  display: grid;
  grid-template-columns: repeat(3, minmax(0, 1fr));
  gap: 16rpx;
  padding: 24rpx;
}

.function-grid.sales {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.action-grid {
  display: grid;
  grid-template-columns: repeat(4, minmax(0, 1fr));
  gap: 12rpx;
  padding: 24rpx;
}

.action-grid.sales {
  grid-template-columns: repeat(2, minmax(0, 1fr));
}

.function-card {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  min-height: 156rpx;
  padding: 20rpx 8rpx;
  border-radius: 16rpx;
  background: #fff;
  box-shadow: 0 2rpx 10rpx rgba(22, 119, 255, 0.06);
  transition: all 0.3s ease;
}

.function-card:active {
  background-color: #f9fafb;
}

.function-icon-bg {
  width: 88rpx;
  height: 88rpx;
  min-width: 88rpx;
  min-height: 88rpx;
  border-radius: 50%;
  background-color: #E6F4FF;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.12);
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  margin-bottom: 16rpx;
  transition: all 0.3s ease;
  flex-shrink: 0;
}

.function-card:active .function-icon-bg {
  transform: scale(0.95);
}

.function-icon {
  width: 42rpx;
  height: 42rpx;
  min-width: 42rpx;
  min-height: 42rpx;
  flex-shrink: 0;
  display: block;
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
  min-width: 80rpx;
  min-height: 80rpx;
  border-radius: 50%;
  background-color: #fff;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.08);
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: 1;
  margin-bottom: 12rpx;
  flex-shrink: 0;
}

.action-icon {
  width: 34rpx;
  height: 34rpx;
  min-width: 34rpx;
  min-height: 34rpx;
  flex-shrink: 0;
  display: block;
}

.action-title {
  font-size: 20rpx;
  color: #1F2D3D;
  text-align: center;
}

/* 响应式调整 */
@media (max-width: 375px) {
  .employee-home {
    padding: 24rpx 24rpx 0;
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
  
  .todo-card {
    padding: 24rpx;
  }
  
  .stat-number {
    font-size: 32rpx;
  }
  
  .function-icon-bg {
    width: 80rpx;
    height: 80rpx;
    min-width: 80rpx;
    min-height: 80rpx;
    flex-shrink: 0;
  }
  
  .function-icon {
    width: 36rpx;
    height: 36rpx;
    min-width: 36rpx;
    min-height: 36rpx;
    flex-shrink: 0;
  }
  
  .function-title {
    font-size: 22rpx;
  }
  
  .action-icon-bg {
    width: 64rpx;
    height: 64rpx;
    min-width: 64rpx;
    min-height: 64rpx;
    flex-shrink: 0;
  }
  
  .action-icon {
    width: 28rpx;
    height: 28rpx;
    min-width: 28rpx;
    min-height: 28rpx;
    flex-shrink: 0;
  }
  
  .action-title {
    font-size: 18rpx;
  }
}
</style>
