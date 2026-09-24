<script>
/** 页脚：产品名与版权，贴在各页底部。 */
export default {
  name: 'AppFooter',
  setup() {
    return { year: new Date().getFullYear() }
  },
}
</script>

<template>
  <footer class="app-footer">
    <div class="app-footer-inner">
      <span>WPGCTL · 水厂现场交付控制台</span>
      <span>© {{ year }} 威派格 WPGCTL</span>
    </div>
  </footer>
</template>
