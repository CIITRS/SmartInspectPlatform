<template>
  <view class="page"><text>正在进入样本录入...</text></view>
</template>

<script>
import { captureSampleEntryLaunch, routePendingSampleEntry } from '../../utils/auth.js'

export default {
  onLoad(options) {
    captureSampleEntryLaunch({ ...options, sample_entry: '1' })
    setTimeout(routePendingSampleEntry, 0)
  }
}
</script>

<style scoped>
.page { min-height: 100vh; display: flex; align-items: center; justify-content: center; color: #64748b; }
</style>
