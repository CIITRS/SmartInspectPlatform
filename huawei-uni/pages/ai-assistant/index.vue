<template>
  <view class="ai-assistant" :style="{ paddingBottom: paddingBottom }">
    
    <!-- 聊天区域 -->
    <scroll-view class="chat-container" scroll-y="true" :scroll-into-view="scrollToView" :scroll-with-animation="true" :style="{ paddingBottom: chatPaddingBottom }">
      <!-- 欢迎消息 -->
      <view class="welcome-message">
        <text class="welcome-text">您好，我是华微小智，有什么可以帮助您？</text>
      </view>
      
      <!-- 推荐问题 -->
      <view class="recommended-questions" v-if="messages.length === 0">
        <text class="recommended-title">常见问题</text>
        <view class="question-list">
          <view class="question-item" v-for="(question, index) in recommendedQuestions" :key="index" @click="askQuestion(question)">
            <text class="question-text">{{ question }}</text>
          </view>
        </view>
      </view>
      
      <!-- 聊天消息 -->
      <view v-for="(message, index) in messages" :key="index" class="message-wrapper" :id="'msg-' + index">
        <view class="message-item" :class="{ 'user-message': message.isUser, 'ai-message': !message.isUser }">
          <view class="message-content">{{ message.content }}</view>
          <view v-if="message.link" class="message-link" @click="openMessageLink(message.link)">
            <text class="message-link-text">{{ message.link.text }}</text>
          </view>
        </view>
      </view>
      
      <!-- 打字状态 -->
      <view v-if="isTyping" class="message-wrapper" id="msg-typing">
        <view class="message-item ai-message">
          <view class="message-content">
            <text class="typing">正在输入...</text>
          </view>
        </view>
      </view>

      <!-- 占位元素，用于滚动到底部 -->
      <view id="scroll-bottom-anchor"></view>
    </scroll-view>
    
    <!-- 输入区域 -->
    <view class="input-container" :class="{ 'keyboard-active': keyboardHeight > 0 }" :style="{ bottom: inputBottom }">
      <button @click="selectReport" class="add-btn">+</button>
      <view class="input-wrapper">
        <!-- 已选择的样本编号按钮 -->
        <view v-if="selectedReportsForAI.length > 0" class="selected-samples">
          <view v-for="report in selectedReportsForAI" :key="report.sampleCode || report.id" @click="handleSampleNumberClick(report)" class="sample-tag">
            <text class="sample-tag-text">【{{ getReportSampleCode(report) }}】</text>
          </view>
        </view>
        <textarea
          v-model="inputText"
          placeholder="请输入您的问题..."
          class="input"
          auto-height
          :maxheight="3*lineHeight"
          :adjust-position="false"
          :cursor-spacing="0"
          :show-confirm-bar="false"
          :hold-keyboard="true"
          @keyboardheightchange="handleKeyboardHeightChange"
          @blur="handleInputBlur"
          @confirm="sendMessage"
        ></textarea>
      </view>
      <button @click="sendMessage" class="send-btn" :disabled="isLoading">
        <image src="/static/send.png" class="send-icon" mode="aspectFit"></image>
      </button>
    </view>
    
    <!-- 报告选择弹窗 -->
    <view v-if="showReportSelect" class="report-select" @click="showReportSelect = false">
      <view class="report-select-inner" @click.stop>
        <view class="report-select-header">
          <text class="select-title">选择报告</text>
          <button @click="showReportSelect = false" class="close-btn">×</button>
        </view>
        <view class="upload-report-actions">
          <button class="upload-report-btn" @click="chooseReportImage">上传图片报告</button>
          <button class="upload-report-btn pdf" @click="chooseReportPDF">上传PDF报告</button>
        </view>
        <view class="report-list">
          <view v-for="report in reports" :key="getReportKey(report)" @click="toggleReportSelection(report)" class="report-item">
            <view class="report-checkbox">
              <view class="checkbox" :class="{ checked: isReportSelected(report) }">
                <text v-if="isReportSelected(report)" class="checkmark">✓</text>
              </view>
            </view>
            <view class="report-info">
              <text class="report-name">样本编号：{{ getReportSampleCode(report) }}</text>
              <text class="report-date">检测时间：{{ getReportDate(report) }}</text>
            </view>
          </view>
        </view>
        <view class="report-select-footer">
          <button @click="confirmReportSelection" class="confirm-btn">确定</button>
        </view>
      </view>
    </view>
    
    <!-- 确认删除对话框 -->
    <view v-if="showConfirmDialog" class="confirm-dialog" @click="cancelRemoveReport">
      <view class="confirm-dialog-inner" @click.stop>
        <text class="confirm-dialog-title">提示</text>
        <text class="confirm-dialog-content">是否不提供这个报告？</text>
        <view class="confirm-dialog-buttons">
          <button @click="confirmRemoveReport" class="confirm-dialog-btn confirm-dialog-btn-yes">是</button>
          <button @click="cancelRemoveReport" class="confirm-dialog-btn confirm-dialog-btn-no">否</button>
        </view>
      </view>
    </view>
  </view>
</template>

<script>
import { refreshTabBarFromStorage } from '../../utils/auth.js'
import { aiAPI, uniAPI } from '../../api/index.js'

export default {
  data() {
    return {
      messages: [],
      inputText: '',
      isTyping: false,
      isLoading: false,
      showReportSelect: false,
      selectedReports: [],
      lineHeight: 56,
      scrollToView: '',
      paddingBottom: '0px',
      keyboardHeight: 0,
      safeAreaBottom: 0,
      reports: [],
      showConfirmDialog: false,
      currentRemovingReport: null,
      selectedReportsForAI: [],
      recommendedQuestions: [
        '如何查看报告？',
        '如何邮寄样本？'
      ],
      faqAnswers: {
        '如何查看报告？': {
          content: '您可以点击首页-报告查询模块进行报告查询以及查看自己的未出报告样本。',
          link: {
            text: '前往报告查询模块',
            url: '/pages/patient/reports/index'
          }
        },
        '如何邮寄样本？': {
          content: '当您收到样本试剂盒并且采血完成后，请您使用顺丰下单【常温运输】到我司实验中心。并且可以将运单号填写到样本邮寄处方便我们接收。'
        }
      },
      userInfo: uni.getStorageSync('userInfo') || {}
    }
  },
  computed: {
    inputBottom() {
      if (this.keyboardHeight <= 0) return '0px'
      return `${Math.max(this.keyboardHeight - this.safeAreaBottom, 0)}px`
    },
    chatPaddingBottom() {
      return this.selectedReportsForAI.length > 0 ? '220rpx' : '156rpx'
    }
  },
  onLoad() {
    uni.setNavigationBarTitle({ title: '华微小智' })
    this.initSafeArea()
    this.calculatePaddingBottom()
  },
  onShow() {
    refreshTabBarFromStorage()
    this.calculatePaddingBottom()
  },
  mounted() {
    this.calculatePaddingBottom()
  },
  methods: {
    initSafeArea() {
      try {
        const info = uni.getSystemInfoSync()
        if (info && info.safeArea && info.screenHeight) {
          this.safeAreaBottom = Math.max(Number(info.screenHeight) - Number(info.safeArea.bottom), 0)
        } else {
          this.safeAreaBottom = 0
        }
      } catch (error) {
        this.safeAreaBottom = 0
      }
    },
    calculatePaddingBottom() {
      // #ifdef H5
      // H5端需要为 tabbar 留出空间
      this.paddingBottom = '100rpx'
      // #endif
      // #ifndef H5
      // 小程序端使用 env(safe-area-inset-bottom) 处理安全区域
      this.paddingBottom = '0px'
      // #endif
    },
    checkLogin() {
      const userInfo = uni.getStorageSync('userInfo') || {}
      if (!userInfo.identity) {
        uni.navigateTo({ url: '/pages/login/index' })
        return false
      }
      return true
    },
    handleKeyboardHeightChange(e) {
      const height = e && e.detail ? Number(e.detail.height || 0) : 0
      this.keyboardHeight = height > 0 ? height : 0
      this.scrollToBottom()
    },
    handleInputBlur() {
      this.keyboardHeight = 0
    },
    sendMessage() {
      if (!this.checkLogin()) return
      
      if (!this.inputText.trim()) return
      
      // 添加用户消息
      this.messages.push({ content: this.inputText, isUser: true })
      const userInput = this.inputText
      this.inputText = ''
      
      this.scrollToBottom()
      
      // 调用后端API
      this.sendToBackend(userInput)
    },
    askQuestion(question) {
      // 添加用户消息
      this.messages.push({ content: question, isUser: true })
      
      this.scrollToBottom()

      const faqAnswer = this.faqAnswers[question]
      if (faqAnswer) {
        this.messages.push({
          content: faqAnswer.content,
          isUser: false,
          link: faqAnswer.link || null
        })
        this.scrollToBottom()
        return
      }
      
      // 调用后端API
      if (!this.checkLogin()) return
      this.sendToBackend(question)
    },
    async sendToBackend(userInput) {
      this.isTyping = true;
      this.isLoading = true;
      let aiMessageIndex = -1;
      
      try {
        // 构建消息历史
        const messages = this.buildMessages(userInput);
        
        // 获取完整响应，然后模拟流式显示
        try {
          const response = await aiAPI.chat({
            messages: messages,
            stream: false
          });
          
          if (response.success && response.data) {
            const fullText = response.data.content;
            this.isTyping = false;
            aiMessageIndex = this.messages.length;
            this.messages.push({
              content: '',
              isUser: false
            });
            // 模拟流式显示
            await this.typeTextEffect(aiMessageIndex, fullText);
          } else {
            this.isTyping = false;
            this.messages.push({
              content: response.message || '抱歉，出现了一些问题，请稍后再试。',
              isUser: false
            });
          }
        } catch (error) {
          console.error('AI chat error:', error);
          this.isTyping = false;
          this.messages.push({
            content: error?.message || '抱歉，出现了一些问题，请稍后再试。',
            isUser: false
          });
        }
      } catch (error) {
        console.error('AI chat error:', error);
        this.messages.push({ 
          content: error?.message || '抱歉，出现了一些问题，请稍后再试。', 
          isUser: false 
        });
      } finally {
        this.isTyping = false;
        this.isLoading = false;
        this.scrollToBottom();
      }
    },
    // 模拟打字效果
    typeTextEffect(messageIndex, fullText) {
      return new Promise((resolve) => {
        let currentIndex = 0;
        const typeSpeed = 20; // 打字速度（毫秒）
        
        const typeChar = () => {
          if (currentIndex < fullText.length) {
            this.messages[messageIndex].content = fullText.substring(0, currentIndex + 1);
            currentIndex++;
            this.scrollToBottom();
            setTimeout(typeChar, typeSpeed);
          } else {
            // 打字完成
            resolve();
          }
        };
        
        typeChar();
      });
    },
    buildMessages(userInput) {
      // 构建消息历史，只保留最近的消息对
      const messages = []
      
      // 添加之前的对话历史（可选）
      const historyLength = this.messages.length > 0 && this.messages[this.messages.length - 1].isUser && this.messages[this.messages.length - 1].content === userInput
        ? this.messages.length - 1
        : this.messages.length

      for (let i = 0; i < historyLength; i++) {
        const msg = this.messages[i]
        messages.push({
          role: msg.isUser ? 'user' : 'assistant',
          content: msg.content
        })
      }
      
      // 构建包含报告数据的用户输入
      let fullUserInput = userInput
      
      // 如果有选中的报告，添加报告数据上下文
      if (this.selectedReportsForAI && this.selectedReportsForAI.length > 0) {
        const reportPrompts = this.selectedReportsForAI.map(report => {
          const date = this.getReportDate(report)
          const value = this.getReportValue(report)
          const explanation = this.getReportExplanation(report)
          return `这是我在${date}的报告，数值是${value}、报告说明是${explanation}。`
        }).join('\n')
        fullUserInput = `${reportPrompts}\n${userInput}`
      }
      
      // 添加当前用户输入
      messages.push({
        role: 'user',
        content: fullUserInput
      })
      
      return messages
    },
    scrollToBottom() {
      this.$nextTick(() => {
        this.scrollToView = 'scroll-bottom-anchor'
      })
    },
    selectReport() {
      if (!this.checkLogin()) return
      
      this.selectedReports = []
      this.showReportSelect = true
      
      // 调用API获取真实报告数据
      uniAPI.getReports().then(res => {
        if (res && res.data) {
          this.reports = (res.data.list || res.data || []).filter(report => report.status !== 'no_report')
        } else if (res && Array.isArray(res)) {
          this.reports = res.filter(report => report.status !== 'no_report')
        }
      }).catch(err => {
        console.error('获取报告列表失败:', err)
        uni.showToast({
          title: '获取报告列表失败',
          icon: 'none',
          duration: 1500
        })
      })
    },
    chooseReportImage() {
      if (this.isLoading) return
      uni.chooseImage({
        count: 1,
        sizeType: ['compressed'],
        sourceType: ['album', 'camera'],
        success: (result) => {
          const path = result.tempFilePaths && result.tempFilePaths[0]
          if (path) this.analyzeUploadedReport(path, '图片报告')
        }
      })
    },
    chooseReportPDF() {
      if (this.isLoading) return
      if (typeof wx === 'undefined' || !wx.chooseMessageFile) {
        uni.showToast({ title: '当前端不支持选择PDF，请使用微信小程序', icon: 'none' })
        return
      }
      wx.chooseMessageFile({
        count: 1,
        type: 'file',
        extension: ['pdf'],
        success: (result) => {
          const file = result.tempFiles && result.tempFiles[0]
          if (file && file.path) this.analyzeUploadedReport(file.path, file.name || 'PDF报告')
        }
      })
    },
    async analyzeUploadedReport(filePath, label) {
      this.showReportSelect = false
      this.isLoading = true
      this.isTyping = true
      this.messages.push({ content: `请分析我上传的${label}`, isUser: true })
      this.scrollToBottom()
      try {
        const response = await aiAPI.analyzeReport({ filePath })
        this.messages.push({
          content: response.data && response.data.content ? response.data.content : '未获得分析结果',
          isUser: false
        })
      } catch (error) {
        this.messages.push({ content: error.message || '报告分析失败，请稍后重试。', isUser: false })
      } finally {
        this.isLoading = false
        this.isTyping = false
        this.scrollToBottom()
      }
    },
    toggleReportSelection(report) {
      if (!this.checkLogin()) return
      
      const reportKey = this.getReportKey(report)
      const index = this.selectedReports.findIndex(r => this.getReportKey(r) === reportKey)
      if (index > -1) {
        this.selectedReports.splice(index, 1)
      } else {
        this.selectedReports.push(report)
      }
    },
    async confirmReportSelection() {
      if (!this.checkLogin()) return
      
      if (this.selectedReports.length === 0) {
        uni.showToast({
          title: '请至少选择一个报告',
          icon: 'none',
          duration: 1500
        })
        return
      }
      
      this.showReportSelect = false
      uni.showLoading({ title: '读取报告...' })
      try {
        const detailReports = await Promise.all(this.selectedReports.map(async report => {
          if (!report.id) return report
          try {
            const res = await uniAPI.getReportDetail(report.id)
            return res && res.success && res.data ? { ...report, ...res.data } : report
          } catch (error) {
            console.error('获取报告详情失败:', error)
            return report
          }
        }))
        this.selectedReportsForAI = detailReports
      } finally {
        uni.hideLoading()
      }
      
      // 样本编号现在通过sample-tag组件显示，不需要添加到inputText
    },
    getReportSampleCode(report) {
      return report.sample_code || report.sampleCode || report.name || report.id || '未知'
    },
    getReportKey(report) {
      return report.sample_code || report.sampleCode || report.id || report.name || ''
    },
    isReportSelected(report) {
      const reportKey = this.getReportKey(report)
      return this.selectedReports.some(item => this.getReportKey(item) === reportKey)
    },
    getReportDate(report) {
      const raw = report.test_time || report.testTime || report.detection_time || report.generated_time || report.report_time || report.reportTime || report.created_at || report.date || ''
      if (!raw) return '未知时间'
      return this.formatDate(raw)
    },
    getReportValue(report) {
      const value = report.calculation_result ?? report.reportValue ?? report.result ?? report.value ?? report.result_value ?? ''
      if (value === '' || value === undefined || value === null) return '未知'
      return value
    },
    getReportExplanation(report) {
      return report.result_explanation || report.signal_value_explanation || report.interpretation || report.resultDescription || report.description || '无'
    },
    formatDate(dateStr) {
      if (!dateStr) return ''
      const str = String(dateStr)
      const match = str.match(/\d{4}[-/]\d{1,2}[-/]\d{1,2}/)
      if (match) return match[0].replace(/\//g, '-')
      return str
    },
    openMessageLink(link) {
      if (!link || !link.url) return
      if (!this.checkLogin()) return
      uni.navigateTo({ url: link.url })
    },
    // 点击样本编号按钮，显示确认对话框
    handleSampleNumberClick(report) {
      this.currentRemovingReport = report
      this.showConfirmDialog = true
    },
    // 确认删除报告
    confirmRemoveReport() {
      if (this.currentRemovingReport) {
        // 从selectedReportsForAI中移除
        const currentKey = this.getReportKey(this.currentRemovingReport)
        const index = this.selectedReportsForAI.findIndex(r => this.getReportKey(r) === currentKey)
        if (index > -1) {
          this.selectedReportsForAI.splice(index, 1)
        }
        // 从selectedReports中移除
        const reportIndex = this.selectedReports.findIndex(r => this.getReportKey(r) === currentKey)
        if (reportIndex > -1) {
          this.selectedReports.splice(reportIndex, 1)
        }
        // 样本编号通过sample-tag组件显示，不需要更新inputText
      }
      this.showConfirmDialog = false
      this.currentRemovingReport = null
    },
    // 取消删除报告
    cancelRemoveReport() {
      this.showConfirmDialog = false
      this.currentRemovingReport = null
    }
  }
}
</script>

<style scoped>
.ai-assistant {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  display: flex;
  flex-direction: column;
  background-color: #F5F7FA;
  height: 100vh;
  box-sizing: border-box;
}

/* 顶部标题栏 */
.header {
  height: 88rpx;
  background-color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  border-bottom: 1rpx solid #E5E7EB;
  box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.05);
}

.header-title {
  font-size: 32rpx;
  font-weight: 600;
  color: #1F2D3D;
}

/* 聊天区域 */
.chat-container {
  flex: 1;
  padding: 32rpx;
  overflow-y: auto;
  height: 0;
  box-sizing: border-box;
}

/* 欢迎消息 */
.welcome-message {
  margin-bottom: 32rpx;
}

.welcome-text {
  font-size: 24rpx;
  color: #8C9AA8;
  line-height: 1.5;
}

/* 推荐问题 */
.recommended-questions {
  margin-bottom: 32rpx;
}

.recommended-title {
  font-size: 20rpx;
  color: #8C9AA8;
  margin-bottom: 16rpx;
  display: block;
}

.question-list {
  display: flex;
  flex-direction: column;
  gap: 12rpx;
}

.question-item {
  background-color: #fff;
  padding: 20rpx;
  border-radius: 16rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.06);
  transition: all 0.3s ease;
}

.question-item:hover {
  transform: translateY(-2rpx);
  box-shadow: 0 4rpx 12rpx rgba(22, 119, 255, 0.1);
}

.question-text {
  font-size: 22rpx;
  color: #1677FF;
}

/* 聊天消息容器 */
.message-wrapper {
  display: flex;
  margin-bottom: 24rpx;
  width: 100%;
}

/* 聊天消息 */
.message-item {
  max-width: 80%;
  animation: fadeIn 0.3s ease;
}

@keyframes fadeIn {
  from { opacity: 0; transform: translateY(10rpx); }
  to { opacity: 1; transform: translateY(0); }
}

.user-message {
  margin-left: auto;
  align-self: flex-end;
}

.ai-message {
  margin-right: auto;
  align-self: flex-start;
}

.message-content {
  padding: 16rpx 24rpx;
  border-radius: 20rpx;
  font-size: 24rpx;
  line-height: 1.5;
  word-break: break-word;
}

.user-message .message-content {
  background-color: #1677FF;
  color: #fff;
  border-bottom-right-radius: 5rpx;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.2);
}

.ai-message .message-content {
  background-color: #fff;
  color: #1F2D3D;
  border-bottom-left-radius: 5rpx;
  box-shadow: 0 2rpx 8rpx rgba(0, 0, 0, 0.05);
}

.message-link {
  display: inline-flex;
  margin-top: 12rpx;
  padding: 10rpx 18rpx;
  border-radius: 999rpx;
  background: #EAF3FF;
  border: 1rpx solid rgba(22, 119, 255, 0.2);
}

.message-link-text {
  font-size: 22rpx;
  color: #1677FF;
  font-weight: 500;
}

.typing {
  font-style: italic;
  color: #8C9AA8;
}

/* 输入区域 */
.input-container {
  position: fixed;
  left: 0;
  right: 0;
  display: flex;
  align-items: flex-end;
  gap: 16rpx;
  padding: 16rpx 32rpx;
  /* #ifdef H5 */
  padding-bottom: calc(16rpx + env(safe-area-inset-bottom));
  /* #endif */
  /* #ifndef H5 */
  padding-bottom: 16rpx;
  /* #endif */
  background-color: #fff;
  border-top: 1rpx solid #E5E7EB;
  box-shadow: 0 -2rpx 10rpx rgba(0, 0, 0, 0.05);
  z-index: 100;
  box-sizing: border-box;
  transition: bottom 0.18s ease;
}

.input-container.keyboard-active {
  padding-top: 12rpx;
  padding-bottom: 0;
}

.input-container.keyboard-active .add-btn,
.input-container.keyboard-active .send-btn {
  margin-bottom: 0;
}

.input-wrapper {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 8rpx;
}

.selected-samples {
  display: flex;
  flex-wrap: wrap;
  gap: 8rpx;
  padding: 8rpx 0;
}

.sample-tag {
  background-color: #EAF3FF;
  border: 1rpx solid #1677FF;
  border-radius: 8rpx;
  padding: 6rpx 12rpx;
  transition: all 0.2s ease;
}

.sample-tag:active {
  background-color: #BFDBFE;
}

.sample-tag-text {
  font-size: 22rpx;
  color: #1677FF;
}

.add-btn {
  width: 56rpx;
  height: 56rpx;
  min-width: 56rpx;
  flex: 0 0 56rpx;
  border-radius: 50%;
  border: 2rpx solid #1677FF;
  background-color: #fff;
  color: #1677FF;
  font-size: 28rpx;
  font-weight: bold;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
  margin-bottom: 4rpx;
}

.add-btn:hover {
  background-color: #EAF3FF;
}

.input {
  width: 100%;
  min-height: 56rpx;
  max-height: 168rpx;
  border: 2rpx solid #E5E7EB;
  border-radius: 28rpx;
  padding: 12rpx 24rpx;
  font-size: 24rpx;
  transition: all 0.3s ease;
  line-height: 1.5;
  box-sizing: border-box;
}

.input:focus {
  border-color: #1677FF;
  box-shadow: 0 0 0 3rpx rgba(22, 119, 255, 0.1);
}

.send-btn {
  width: 56rpx;
  height: 56rpx;
  min-width: 56rpx;
  flex: 0 0 56rpx;
  border-radius: 50%;
  background-color: #1677FF;
  border: none;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
  margin-bottom: 4rpx;
  padding: 0;
}

.send-btn:hover {
  background-color: #409EFF;
  box-shadow: 0 2rpx 8rpx rgba(22, 119, 255, 0.3);
}

.send-icon {
  width: 28rpx;
  height: 28rpx;
}

/* 报告选择弹窗 */
.report-select {
  position: fixed;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 999;
}

.report-select-inner {
  background-color: #fff;
  border-top-left-radius: 24rpx;
  border-top-right-radius: 24rpx;
  padding: 32rpx;
  width: 100%;
  box-shadow: 0 -4rpx 20rpx rgba(0, 0, 0, 0.1);
  animation: slideUp 0.3s ease;
}

@keyframes slideUp {
  from { transform: translateY(100%); }
  to { transform: translateY(0); }
}

.report-select-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 24rpx;
}
.upload-report-actions { display: flex; gap: 16rpx; padding: 18rpx 24rpx; border-bottom: 1rpx solid #eef2f6; }
.upload-report-btn { flex: 1; height: 68rpx; line-height: 68rpx; margin: 0; padding: 0; border-radius: 12rpx; border: none; background: #1677ff; color: #fff; font-size: 23rpx; }
.upload-report-btn.pdf { background: #13a8a8; }

.select-title {
  font-size: 28rpx;
  font-weight: 600;
  color: #1F2D3D;
}

.close-btn {
  width: 40rpx;
  height: 40rpx;
  border: none;
  background-color: transparent;
  font-size: 32rpx;
  color: #8C9AA8;
  display: flex;
  align-items: center;
  justify-content: center;
}

.report-list {
  display: flex;
  flex-direction: column;
  gap: 16rpx;
  max-height: 400rpx;
  overflow-y: auto;
}

.report-item {
  padding: 24rpx;
  border: 2rpx solid #E5E7EB;
  border-radius: 16rpx;
  background-color: #F9FAFB;
  transition: all 0.3s ease;
  display: flex;
  align-items: center;
  gap: 16rpx;
}

.report-item:hover {
  border-color: #1677FF;
  background-color: #EAF3FF;
}

.report-checkbox {
  flex-shrink: 0;
}

.checkbox {
  width: 40rpx;
  height: 40rpx;
  border: 2rpx solid #D1D5DB;
  border-radius: 8rpx;
  background-color: #fff;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
}

.checkbox.checked {
  background-color: #1677FF;
  border-color: #1677FF;
}

.checkmark {
  color: #fff;
  font-size: 24rpx;
  font-weight: bold;
}

.report-info {
  flex: 1;
}

.report-name {
  font-size: 24rpx;
  font-weight: 500;
  color: #1F2D3D;
  margin-bottom: 8rpx;
  display: block;
}

.report-date {
  font-size: 20rpx;
  color: #8C9AA8;
}

.report-select-footer {
  margin-top: 32rpx;
  padding-top: 24rpx;
  border-top: 1rpx solid #E5E7EB;
}

.confirm-btn {
  width: 100%;
  height: 88rpx;
  background-color: #1677FF;
  color: #fff;
  border: none;
  border-radius: 44rpx;
  font-size: 28rpx;
  font-weight: 600;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.3s ease;
}

.confirm-btn:hover {
  background-color: #409EFF;
  box-shadow: 0 4rpx 12rpx rgba(22, 119, 255, 0.3);
}

/* 响应式调整 */
@media (max-width: 375px) {
  .chat-container {
    padding: 24rpx;
  }
  
  .input-container {
    padding: 16rpx 24rpx;
  }

  .input-container.keyboard-active {
    padding-top: 12rpx;
    padding-bottom: 0;
  }
  
  .message-content {
    padding: 14rpx 20rpx;
    font-size: 22rpx;
  }
}

/* 确认删除对话框 */
.confirm-dialog {
  position: fixed;
  left: 0;
  right: 0;
  top: 0;
  bottom: 0;
  background-color: rgba(0, 0, 0, 0.5);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 1000;
}

.confirm-dialog-inner {
  background-color: #fff;
  border-radius: 24rpx;
  padding: 48rpx;
  width: 80%;
  max-width: 600rpx;
  box-shadow: 0 8rpx 32rpx rgba(0, 0, 0, 0.15);
  animation: fadeIn 0.2s ease;
}

.confirm-dialog-title {
  font-size: 32rpx;
  font-weight: 600;
  color: #1F2D3D;
  display: block;
  text-align: center;
  margin-bottom: 24rpx;
}

.confirm-dialog-content {
  font-size: 28rpx;
  color: #4B5563;
  display: block;
  text-align: center;
  margin-bottom: 48rpx;
  line-height: 1.5;
}

.confirm-dialog-buttons {
  display: flex;
  gap: 24rpx;
  justify-content: center;
}

.confirm-dialog-btn {
  flex: 1;
  height: 80rpx;
  border-radius: 40rpx;
  font-size: 28rpx;
  font-weight: 500;
  display: flex;
  align-items: center;
  justify-content: center;
  border: none;
}

.confirm-dialog-btn-yes {
  background-color: #FF4D4F;
  color: #fff;
}

.confirm-dialog-btn-yes:hover {
  background-color: #FF7875;
}

.confirm-dialog-btn-no {
  background-color: #F3F4F6;
  color: #4B5563;
}

.confirm-dialog-btn-no:hover {
  background-color: #E5E7EB;
}
</style>
