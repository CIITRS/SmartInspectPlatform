<script>
export default {
  onLaunch() {
    console.log('App Launch')
  },
  onShow() {
    console.log('App Show')
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
