<template>
  <web-view :src="albumUrl"></web-view>
</template>

<script>
export default {
  data() {
    return {
      albumUrl: 'https://mp.weixin.qq.com/mp/appmsgalbum?__biz=MzY5NDI3Mjc3NA==&action=getalbum&album_id=4548004175791816705#wechat_redirect'
    }
  }
}
</script>
