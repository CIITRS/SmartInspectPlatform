<script>
import { captureSampleEntryLaunch, routePendingSampleEntry } from './utils/auth.js'

export default {
  onLaunch(options) {
    captureSampleEntryLaunch(options)
    setTimeout(routePendingSampleEntry, 0)
  },
  onShow(options) {
    captureSampleEntryLaunch(options)
    setTimeout(routePendingSampleEntry, 50)
  },
  onHide() {
    console.log('App Hide')
  }
}
</script>

<style>
body {
  background-color: #f5f7fa;
}

button {
  display: flex;
  align-items: center;
  justify-content: center;
  line-height: normal !important;
  box-sizing: border-box;
}

button::after {
  border: none;
}
</style>
