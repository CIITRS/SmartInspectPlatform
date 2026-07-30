<template>
  <view class="page">
    <view class="search-row">
      <input v-model="keyword" class="search-input" placeholder="搜索姓名/手机号/证件号/编号" confirm-type="search" @confirm="searchPatients" />
      <button class="search-btn" @click="searchPatients">搜索</button>
    </view>
    <view class="filter-row">
      <picker class="filter-picker" :range="groupFilterOptions" range-key="name" @change="onGroupFilterChange">
        <view class="filter-value">{{ selectedGroupFilterName }}</view>
      </picker>
      <picker class="filter-picker" :range="cancerFilterOptions" range-key="name" @change="onCancerFilterChange">
        <view class="filter-value">{{ selectedCancerFilterName }}</view>
      </picker>
    </view>
    <view class="group-toolbar">
      <input v-model="newGroupName" class="group-input" maxlength="30" placeholder="新分组名称" />
      <button class="group-add" @click="createGroup">新增分组</button>
    </view>

    <view v-if="!selectionMode" class="tabs">
      <view class="tab" :class="{ active: infoStatus === 'pending' }" @click="switchTab('pending')">未完善</view>
      <view class="tab" :class="{ active: infoStatus === 'completed' }" @click="switchTab('completed')">已完善</view>
    </view>

    <view class="toolbar">
      <button class="primary-btn" @click="goCreate">新患录入</button>
    </view>

    <view v-if="loading" class="state"><text>加载中...</text></view>
    <view v-else-if="patients.length === 0" class="state"><text>暂无患者</text></view>
    <view v-else class="list">
      <view v-for="item in patients" :key="item.id" class="patient-card" @click="goDetail(item.id)">
        <view class="patient-body" @click="goDetail(item.id)">
          <view class="patient-head">
            <text class="name">{{ item.name || '-' }}</text>
            <view class="head-right">
              <text class="status" :class="item.diagnosis_completed ? 'completed' : 'pending'">{{ item.diagnosis_completed ? '已完善' : '未完善' }}</text>
              <text class="code">{{ item.patient_code || '-' }}</text>
            </view>
          </view>
          <view class="info-row"><text class="lbl">手机号</text><text class="val">{{ item.phone || '-' }}</text></view>
          <view class="info-row"><text class="lbl">证件类型</text><text class="val">{{ item.id_document_type || '-' }}</text></view>
          <view class="info-row"><text class="lbl">证件号</text><text class="val">{{ item.id_document_no || item.id_card || '-' }}</text></view>
          <view class="info-row"><text class="lbl">诊断</text><text class="val">{{ item.diagnosis || '-' }}</text></view>
          <view class="info-row"><text class="lbl">检测癌型</text><text class="val">{{ item.cancer_types || '-' }}</text></view>
          <view class="info-row group-line" @click.stop>
            <text class="lbl">我的分组</text>
            <picker class="card-group-picker" :range="patientGroupOptions" range-key="name" @change="setPatientGroup(item, $event)">
              <view class="group-chip">{{ item.group_name || '未分组（点此设置）' }}</view>
            </picker>
          </view>
          <view class="actions">
            <button class="small-btn" @click.stop="goComplete(item.id)">{{ item.diagnosis_completed ? '修改' : '完善' }}</button>
            <button class="small-btn primary" @click.stop="goAddSample(item.id)">新增样本</button>
          </view>
        </view>
      </view>
    </view>
    <view v-if="loadingMore" class="load-more">加载更多...</view>
    <view v-else-if="patients.length > 0 && patients.length >= total" class="load-more">已显示全部</view>
  </view>
</template>

<script>
import { uniAPI } from '../../../api/index.js'

export default {
  data() {
    return {
      keyword: '',
      infoStatus: 'pending',
      loading: true,
      loadingMore: false,
      patients: [],
      selectionMode: false,
      page: 1,
      pageSize: 20,
      total: 0,
      groups: [],
      cancerTypes: [],
      groupId: 0,
      cancerTypeId: 0,
      newGroupName: ''
    }
  },
  computed: {
    groupFilterOptions() { return [{ id: 0, name: '全部分组' }].concat(this.groups) },
    cancerFilterOptions() { return [{ id: 0, name: '全部癌型' }].concat(this.cancerTypes) },
    patientGroupOptions() { return [{ id: 0, name: '未分组' }].concat(this.groups) },
    selectedGroupFilterName() {
      const item = this.groupFilterOptions.find(v => Number(v.id) === Number(this.groupId))
      return item ? item.name : '全部分组'
    },
    selectedCancerFilterName() {
      const item = this.cancerFilterOptions.find(v => Number(v.id) === Number(this.cancerTypeId))
      return item ? item.name : '全部癌型'
    }
  },
  onLoad(options) {
    this.selectionMode = String((options && options.select) || '') === '1'
    this.loadFilters()
    this.loadPatients()
  },
  onShow() {
    if (!this.loading) this.loadPatients(true)
  },
  onReachBottom() {
    if (!this.loading && !this.loadingMore && this.patients.length < this.total) {
      this.loadPatients(false)
    }
  },
  methods: {
    async loadPatients(reset = true) {
      if (reset) {
        this.page = 1
        this.loading = true
      } else {
        this.loadingMore = true
      }
      try {
        const params = {
          keyword: String(this.keyword || '').trim(),
          info_status: this.selectionMode ? '' : this.infoStatus,
          group_id: this.groupId || '',
          cancer_type_id: this.cancerTypeId || '',
          page: this.page,
          page_size: this.pageSize
        }
        const res = await uniAPI.getEmployeePatients(params)
        if (res.success && res.data) {
          const list = Array.isArray(res.data.list) ? res.data.list : []
          this.patients = reset ? list : this.patients.concat(list)
          this.total = Number(res.data.total || 0)
          this.page = reset ? 2 : this.page + (list.length > 0 ? 1 : 0)
        }
      } catch (error) {
        uni.showToast({ title: '加载失败', icon: 'none' })
      } finally {
        this.loading = false
        this.loadingMore = false
      }
    },
    searchPatients() {
      this.loadPatients(true)
    },
    async loadFilters() {
      try {
        const [groupRes, optionRes] = await Promise.all([
          uniAPI.getEmployeePatientGroups(),
          uniAPI.getEmployeeSampleOptions([])
        ])
        this.groups = (groupRes.data && groupRes.data.list) || []
        const cancerData = optionRes.data && optionRes.data.cancer_types
        this.cancerTypes = Array.isArray(cancerData) ? cancerData : ((cancerData && cancerData.list) || [])
      } catch (error) {
        console.error('加载筛选项失败', error)
      }
    },
    onGroupFilterChange(e) {
      const item = this.groupFilterOptions[Number(e.detail.value)]
      this.groupId = item ? Number(item.id) : 0
      this.loadPatients(true)
    },
    onCancerFilterChange(e) {
      const item = this.cancerFilterOptions[Number(e.detail.value)]
      this.cancerTypeId = item ? Number(item.id) : 0
      this.loadPatients(true)
    },
    async createGroup() {
      const name = String(this.newGroupName || '').trim()
      if (!name) { uni.showToast({ title: '请输入分组名称', icon: 'none' }); return }
      try {
        await uniAPI.createEmployeePatientGroup({ name })
        this.newGroupName = ''
        await this.loadFilters()
        uni.showToast({ title: '分组已创建', icon: 'success' })
      } catch (error) {
        uni.showToast({ title: error.message || '创建失败', icon: 'none' })
      }
    },
    async setPatientGroup(item, event) {
      const selected = this.patientGroupOptions[Number(event.detail.value)]
      try {
        await uniAPI.setEmployeePatientGroup(item.id, selected ? selected.id : 0)
        item.group_id = selected ? selected.id : 0
        item.group_name = selected && selected.id ? selected.name : ''
        await this.loadFilters()
      } catch (error) {
        uni.showToast({ title: error.message || '设置失败', icon: 'none' })
      }
    },
    switchTab(status) {
      this.infoStatus = status
      this.loadPatients(true)
    },
    goCreate() {
      uni.navigateTo({ url: '/pages/employee/patient-create/index' })
    },
    goDetail(id) {
      if (!id) return
      uni.navigateTo({ url: `/pages/employee/patient-detail/index?id=${id}` })
    },
    goComplete(id) {
      uni.navigateTo({ url: `/pages/employee/patient-complete/index?id=${id}` })
    },
    goAddSample(id) {
      uni.navigateTo({ url: `/pages/employee/sample-allocate/index?patient_ids=${id}` })
    }
  }
}
</script>

<style scoped>
.page { min-height: 100vh; padding: 32rpx; background: #f5f7fa; box-sizing: border-box; }
.search-row { display: flex; gap: 16rpx; margin-bottom: 20rpx; }
.search-input { flex: 1; height: 76rpx; padding: 0 20rpx; border-radius: 14rpx; background: #fff; border: 1rpx solid #e5e9f0; box-sizing: border-box; font-size: 26rpx; }
.search-btn { width: 136rpx; height: 76rpx; line-height: 76rpx; border-radius: 14rpx; background: #1677ff; color: #fff; font-size: 26rpx; border: none; }
.filter-row { display: flex; gap: 16rpx; margin-bottom: 16rpx; }
.filter-picker { flex: 1; }
.filter-value { height: 72rpx; line-height: 72rpx; padding: 0 18rpx; border-radius: 12rpx; background: #fff; color: #334155; font-size: 24rpx; border: 1rpx solid #e5e9f0; }
.group-toolbar { display: flex; gap: 14rpx; margin-bottom: 18rpx; }
.group-input { flex: 1; height: 68rpx; padding: 0 18rpx; border-radius: 12rpx; background: #fff; font-size: 24rpx; border: 1rpx solid #e5e9f0; }
.group-add { width: 170rpx; height: 68rpx; line-height: 68rpx; padding: 0; border-radius: 12rpx; background: #eaf2ff; color: #1677ff; font-size: 24rpx; border: none; }
.group-line { align-items: center; }
.card-group-picker { flex: 1; }
.group-chip { display: inline-block; padding: 6rpx 16rpx; border-radius: 999rpx; background: #eef6ff; color: #1677ff; font-size: 22rpx; }
.tabs { display: flex; padding: 8rpx; margin-bottom: 20rpx; border-radius: 16rpx; background: #eaf0f7; }
.tab { flex: 1; height: 64rpx; line-height: 64rpx; text-align: center; border-radius: 12rpx; color: #6b7785; font-size: 26rpx; }
.tab.active { background: #fff; color: #1677ff; font-weight: 700; box-shadow: 0 2rpx 8rpx rgba(22,119,255,0.08); }
.toolbar { display: flex; gap: 16rpx; margin-bottom: 24rpx; }
.primary-btn, .ghost-btn { flex: 1; height: 80rpx; line-height: 80rpx; border-radius: 14rpx; font-size: 28rpx; border: none; }
.primary-btn { background: #1677ff; color: #fff; }
.ghost-btn { background: #fff; color: #1677ff; border: 1rpx solid #c9dcff; }
.state { display: flex; justify-content: center; padding: 120rpx 0; color: #8c9aa8; font-size: 28rpx; }
.patient-card { display: flex; gap: 20rpx; padding: 24rpx; margin-bottom: 20rpx; border-radius: 18rpx; background: #fff; box-shadow: 0 2rpx 12rpx rgba(22,119,255,0.06); }
.patient-body { flex: 1; min-width: 0; }
.patient-head { display: flex; align-items: center; justify-content: space-between; gap: 16rpx; margin-bottom: 16rpx; }
.head-right { display: flex; flex-direction: column; align-items: flex-end; gap: 8rpx; }
.status { font-size: 21rpx; padding: 4rpx 12rpx; border-radius: 999rpx; }
.status.pending { color: #d46b08; background: #fff7e6; }
.status.completed { color: #237804; background: #f6ffed; }
.name { font-size: 30rpx; font-weight: 700; color: #1f2d3d; }
.code { font-size: 22rpx; color: #1677ff; }
.info-row { display: flex; gap: 16rpx; margin-top: 10rpx; font-size: 24rpx; }
.lbl { width: 92rpx; color: #8c9aa8; flex-shrink: 0; }
.val { flex: 1; color: #1f2d3d; word-break: break-all; }
.actions { display: flex; justify-content: flex-end; gap: 14rpx; margin-top: 18rpx; }
.small-btn { width: 164rpx; height: 60rpx; padding: 0 12rpx; line-height: 60rpx; border-radius: 12rpx; border: 1rpx solid #c9dcff; background: #fff; color: #1677ff; font-size: 24rpx; white-space: nowrap; box-sizing: border-box; }
.small-btn.primary { background: #1677ff; color: #fff; border-color: #1677ff; }
.load-more { padding: 24rpx 0 12rpx; text-align: center; color: #8c9aa8; font-size: 24rpx; }
</style>
