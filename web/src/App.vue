<template>
  <div class="app-shell" :class="isLocalDocker ? 'mode-local' : 'mode-field'">
    <!-- 任务期间不再全屏遮罩：顶部细进度条 + 顶栏标签，日志框留给现场看。 -->
    <div v-if="busy" class="app-progress" role="progressbar" aria-busy="true" />
    <AppHeader />
    <router-view />
    <PathPickerDialog />
    <FileEditorDialog />
  </div>
</template>

<script setup>
/**
 * 现场控制台壳：只放顶栏、路由视图、全局对话框与 provide。
 *
 * 业务代码不要写在这里，按职责落到对应目录：
 * - composables/createConsole.js  会话状态与动作（站点表单、向导、部署、状态、日志）
 * - router/                       hash 路由表与 path/hash 纯函数
 * - api/http.js                   /api 请求封装
 * - constants/catalog.js          服务目录、步骤定义、默认账号
 * - utils/                        格式化、状态卡片字段、本地草稿
 * - views/ 与 views/wizard/       页面模板，通过 useConsole() 取状态
 * - components/                   顶栏、路径选择器、文件编辑器等可复用组件
 */
import { onMounted, onUnmounted, provide } from 'vue'
import { WPGCTL_KEY } from '@/composables/key.js'
import { createConsole } from '@/composables/createConsole.js'
import AppHeader from '@/components/AppHeader.vue'
import PathPickerDialog from '@/components/PathPickerDialog.vue'
import FileEditorDialog from '@/components/FileEditorDialog.vue'

const ctx = createConsole()
provide(WPGCTL_KEY, ctx)
const { isLocalDocker, busy } = ctx

onMounted(() => ctx.mount())
onUnmounted(() => ctx.unmount())
</script>
