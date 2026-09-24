<script>
/** 顶栏：品牌、主导航与操作者。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'AppHeader',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <header class="topbar">
      <div class="chrome">
        <div class="brand-mark">
          <span class="mark" aria-hidden="true"></span>
          <div class="brand-text">
            <strong>WPGCTL</strong>
            <span>Linux 现场交付</span>
          </div>
        </div>
        <div class="topbar-tools">
          <el-tag
            v-if="activeJobKey"
            class="job-pill"
            size="small"
            type="primary"
            effect="dark"
            round
            :title="'点击回到任务页面看日志（' + activeJobView + '）'"
            @click="goActiveJob"
          >
            <el-icon class="is-loading"><Loading /></el-icon>
            {{ activeJobLabel }} · {{ jobElapsedText }}
          </el-tag>
          <el-tag
            v-if="runtimeOS"
            class="env-tag"
            size="small"
            :type="dockerOk === false ? 'warning' : 'success'"
            effect="light"
            round
          >{{ envChipText }}</el-tag>
          <el-input
            v-model="settings.operator"
            class="operator-input"
            size="small"
            placeholder="操作者"
            title="操作者署名，写入交付记录"
            @change="saveSettings"
          />
          <div class="tool-switches">
            <el-switch
              size="small"
              v-model="settings.advancedMode"
              active-text="一键部署"
              @change="saveSettings"
            />
          </div>
          <a
            v-if="settings.advancedMode"
            class="el-button el-button--small el-button--primary is-plain"
            href="/wpg-deploy-api/deliveries/export"
          >导出摘要</a>
        </div>
      </div>
      <div class="nav-row">
        <el-menu
          class="app-nav"
          mode="horizontal"
          :ellipsis="false"
          :default-active="navActive"
          :key="navActive + String(settings.advancedMode)"
          @select="onNavSelect"
        >
          <el-menu-item index="home">
            <el-icon><HomeFilled /></el-icon>
            <span>首页</span>
          </el-menu-item>
          <el-menu-item index="wizard">
            <el-icon><Guide /></el-icon>
            <span>部署向导</span>
          </el-menu-item>
          <el-menu-item index="status">
            <el-icon><Monitor /></el-icon>
            <span>状态</span>
          </el-menu-item>
          <el-menu-item index="logs">
            <el-icon><Document /></el-icon>
            <span>日志</span>
          </el-menu-item>
          <template v-if="settings.advancedMode">
            <el-menu-item index="fetch">
              <el-icon><Box /></el-icon>
              <span>包中心</span>
            </el-menu-item>
            <el-menu-item index="upgrade" :disabled="!latestVersion">
              <el-icon><Upload /></el-icon>
              <span>升级</span>
            </el-menu-item>
            <el-menu-item index="history">
              <el-icon><Tickets /></el-icon>
              <span>交付单</span>
            </el-menu-item>
            <el-menu-item index="report">
              <el-icon><Checked /></el-icon>
              <span>验收报告</span>
            </el-menu-item>
          </template>
        </el-menu>
      </div>
    </header>
</template>
