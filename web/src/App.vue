<template>
  <div
    class="app-shell"
    :class="isLocalDocker ? 'mode-local' : 'mode-field'"
    v-loading.fullscreen.lock="busy"
    :element-loading-text="busyText || '处理中…'"
  >
    <AppHeader />
    <router-view />
    <PathPickerDialog />
    <FileEditorDialog />
  </div>
</template>

<script setup>
/**
 * 现场控制台壳：顶栏、hash 路由视图、对话框。
 * 会话状态与动作在 createConsole()，子页面用 useConsole()。
 */
import { onMounted, onUnmounted, provide } from 'vue'
import { WPGCTL_KEY } from '@/composables/key.js'
import { createConsole } from '@/composables/createConsole.js'
import AppHeader from '@/components/AppHeader.vue'
import PathPickerDialog from '@/components/PathPickerDialog.vue'
import FileEditorDialog from '@/components/FileEditorDialog.vue'

const ctx = createConsole()
provide(WPGCTL_KEY, ctx)
const { isLocalDocker, busy, busyText } = ctx

onMounted(() => ctx.mount())
onUnmounted(() => ctx.unmount())
</script>
