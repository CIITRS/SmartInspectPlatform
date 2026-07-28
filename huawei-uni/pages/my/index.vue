<template>
  <view class="my-page">
    <!-- 未登录状态 -->
    <view v-if="!isLoggedIn" class="identity-card not-logged-in">
      <view class="user-info">
        <image src="../../static/logo.svg" class="avatar"></image>
        <view class="user-details">
          <text class="user-name">未登录</text>
          <text class="user-phone">请先登录后使用</text>
        </view>
      </view>
      <button @click="goToLogin" class="login-btn-small">
        <text class="login-btn-text">去登录</text>
      </button>
    </view>

    <!-- 已登录状态 -->
    <view v-else class="identity-card">
      <view class="user-info">
        <image src="../../static/logo.svg" class="avatar"></image>
        <view class="user-details">
          <text class="user-name">{{ displayName }}</text>
          <text class="user-phone">{{ userInfo.phone || '' }}</text>
          <text class="user-identity">{{ identityLabel }}</text>
        </view>
      </view>
      <button v-if="canSwitchIdentity" @click="switchUser" class="switch-btn">
        <text class="switch-icon">⇄</text>
        <text class="switch-text">切换</text>
      </button>
    </view>
    
    <!-- 功能列表 -->
    <view class="function-section">
      <view class="function-card">
        <view class="function-item" @click="我的套餐">
          <image src="../../static/sale.png" class="item-icon"></image>
		<text class="item-text">我的套餐</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
        <view class="function-item" @click="我的样本">
          <image src="../../static/Sample.png" class="item-icon"></image>
          <text class="item-text">我的样本</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
        <view v-if="isEmployee" class="function-item" @click="客户填写二维码">
          <image src="../../static/QRcode.png" class="item-icon"></image>
          <text class="item-text">客户填写二维码</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
      </view>
    </view>
    
    <view class="function-section">
      <text class="section-title">关于我们</text>
      <view class="function-card">
        <view class="function-item" @click="联系我们">
          <image src="../../static/phone.png" class="item-icon"></image>
          <text class="item-text">联系我们</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
        <view class="function-item" @click="关于">
          <image src="../../static/about.png" class="item-icon"></image>
          <text class="item-text">关于</text>
          <text class="item-arrow">{{ '>' }}</text>
        </view>
      </view>
    </view>
    
    <!-- 退出登录按钮 - 仅登录后显示 -->
    <button v-if="isLoggedIn" @click="logout" class="logout-btn">退出登录</button>
  </view>
</template>

<script>
import { authAPI } from '../../api/index.js'
import { parseLoginPayload, saveLoginState } from '../../utils/auth.js'

export default {
  data() {
    return {
      userInfo: uni.getStorageSync('userInfo') || {}
    }
  },
  onShow() {
    // 每次页面显示时刷新登录状态
    this.userInfo = uni.getStorageSync('userInfo') || {}
  },
  computed: {
    isLoggedIn() {
      return !!this.userInfo.identity
    },
    displayName() {
      if (this.userInfo.identity === 'patient' && this.userInfo.patient) {
        return this.userInfo.patient.name || '患者'
      }
      if (this.userInfo.identity === 'employee' && this.userInfo.employee) {
        return this.userInfo.employee.real_name || this.userInfo.employee.username || '员工'
      }
      return '用户'
    },
    identityLabel() {
      if (this.userInfo.identity === 'patient') return '患者'
      if (this.userInfo.identity === 'employee') return '员工'
      return ''
    },
    isEmployee() {
      return this.userInfo.identity === 'employee'
    },
    identityList() {
      return Array.isArray(this.userInfo.identityList) ? this.userInfo.identityList : []
    },
    canSwitchIdentity() {
      return this.identityList.length > 1
    }
  },
  methods: {
    goToLogin() {
      uni.navigateTo({ url: '/pages/login/index' })
    },
    checkLogin() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      if (!userInfo.identity) {
        uni.navigateTo({ url: '/pages/login/index' })
        return false
      }
      return true
    },
    我的套餐() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/packages/index' })
    },
    我的样本() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/patient/samples/index' })
    },
    客户填写二维码() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/employee/invite-code/index' })
    },
    联系我们() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/contact/index' })
    },
    关于() {
      if (!this.checkLogin()) return
      uni.navigateTo({ url: '/pages/about/index' })
    },
    switchUser() {
      if (!this.checkLogin()) return

      if (!this.canSwitchIdentity) {
        uni.showToast({ title: '当前手机号暂无可切换身份', icon: 'none' })
        return
      }

      const labels = this.identityList.map((identity) => {
        const type = identity.identity_type === 'employee' ? '员工' : '患者'
        return `${type}：${identity.title || identity.info?.real_name || identity.info?.name || '-'}`
      })
      uni.showActionSheet({
        itemList: labels,
        success: async (res) => {
          const identity = this.identityList[res.tapIndex]
          if (!identity) return
          try {
            uni.showLoading({ title: '切换中...' })
            const response = await authAPI.switchIdentity({
              identity_type: identity.identity_type,
              user_id: identity.info?.user_id || identity.user_id || 0,
              patient_id: identity.info?.patient_id || identity.patient_id || 0
            })
            uni.hideLoading()
            if (!response.success || !response.data) {
              uni.showToast({ title: response.message || '切换失败', icon: 'none' })
              return
            }
            const result = parseLoginPayload({
              session_id: response.data.session_id || this.userInfo.sessionId || '',
              user_info: response.data.user_info
            })
            const nextIdentity = result.identity || identity
            this.userInfo = saveLoginState({
              phone: this.userInfo.phone || nextIdentity.info?.phone || '',
              sessionId: response.data.session_id || this.userInfo.sessionId || '',
              identity: nextIdentity,
              identityList: response.data.identity_list || this.identityList
            })
            uni.showToast({ title: '切换成功', icon: 'success' })
          } catch (error) {
            uni.hideLoading()
            uni.showToast({ title: '切换失败', icon: 'none' })
          }
        }
      })
    },
    logout() {
      uni.showModal({
        title: '退出登录',
        content: '确定要退出登录吗？',
        success: (res) => {
          if (res.confirm) {
            uni.removeStorageSync('userInfo')
            uni.reLaunch({ url: '/pages/login/index' })
          }
        }
      })
    }
  }
}
</script>

<style scoped>
.my-page {
  padding: 32rpx;
  min-height: 100vh;
  background-color: #F5F7FA;
}

/* 顶部蓝色身份卡 */
.identity-card {
  background: linear-gradient(135deg, #1677FF 0%, #409EFF 100%);
  border-radius: 24rpx;
  padding: 32rpx;
  margin-bottom: 32rpx;
  box-shadow: 0 4rpx 16rpx rgba(22, 119, 255, 0.15);
  display: flex;
  justify-content: space-between;
  align-items: center;
  color: #fff;
}

.identity-card.not-logged-in {
  background: linear-gradient(135deg, #8c9aa8 0%, #a0b0c0 100%);
}

.login-btn-small {
  background-color: rgba(255, 255, 255, 0.25);
  color: #fff;
  border: 2rpx solid rgba(255, 255, 255, 0.5);
  border-radius: 32rpx;
  padding: 12rpx 32rpx;
  font-size: 24rpx;
  display: flex;
  align-items: center;
  justify-content: center;
}

.login-btn-text {
  font-size: 24rpx;
  font-weight: 500;
}

.user-info {
  display: flex;
  align-items: center;
  flex: 1;
}

.avatar {
  width: 100rpx;
  height: 100rpx;
  border-radius: 50%;
  border: 4rpx solid rgba(255, 255, 255, 0.3);
  margin-right: 24rpx;
}

.user-details {
  flex: 1;
}

.user-name {
  font-size: 32rpx;
  font-weight: 600;
  margin-bottom: 8rpx;
  display: block;
}

.user-phone {
  font-size: 24rpx;
  opacity: 0.9;
  margin-bottom: 8rpx;
  display: block;
}

.user-identity {
  font-size: 20rpx;
  opacity: 0.8;
  background-color: rgba(255, 255, 255, 0.2);
  padding: 4rpx 12rpx;
  border-radius: 12rpx;
  display: inline-block;
}

.switch-btn {
  background-color: rgba(255, 255, 255, 0.2);
  color: #fff;
  border: none;
  border-radius: 16rpx;
  padding: 16rpx 20rpx;
  font-size: 20rpx;
  display: flex;
  flex-direction: column;
  align-items: center;
  transition: all 0.3s ease;
}

.switch-btn:active {
  background-color: rgba(255, 255, 255, 0.3);
}

.switch-icon {
  font-size: 24rpx;
  margin-bottom: 4rpx;
}

.switch-text {
  font-size: 18rpx;
}

/* 功能列表 */
.function-section {
  margin-bottom: 32rpx;
}

.section-title {
  font-size: 20rpx;
  color: #8C9AA8;
  margin-bottom: 16rpx;
  display: block;
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

.function-item:active {
  background-color: #F9FAFB;
}

.item-icon {
  width: 40rpx;
  height: 40rpx;
  margin-right: 20rpx;
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

/* 退出登录按钮 */
.logout-btn {
  width: 100%;
  height: 88rpx;
  background-color: #fff;
  border: 2rpx solid #F56C6C;
  color: #F56C6C;
  border-radius: 16rpx;
  font-size: 24rpx;
  font-weight: 500;
  margin-top: 16rpx;
  transition: all 0.3s ease;
  box-shadow: 0 2rpx 8rpx rgba(245, 108, 108, 0.1);
}

.logout-btn:active {
  background-color: #FEF0F0;
  box-shadow: 0 4rpx 12rpx rgba(245, 108, 108, 0.15);
}

/* 响应式调整 */
@media (max-width: 375px) {
  .my-page {
    padding: 24rpx 24rpx 140rpx;
  }
  
  .identity-card {
    padding: 24rpx;
  }
  
  .user-name {
    font-size: 28rpx;
  }
  
  .user-phone {
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
