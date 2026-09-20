<script>
/** 顶栏：品牌、主导航、现场开关与诊断包。 */
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
            <span>{{ isLocalDocker ? '本机 Docker 联调' : 'Linux 现场交付' }}</span>
          </div>
        </div>
        <div class="topbar-tools">
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
              :model-value="isLocalDocker"
              active-text="本机 Docker"
              @change="onLocalDockerToggle"
            />
            <el-switch size="small" v-model="settings.privacyMode" active-text="投屏" @change="saveSettings" />
            <el-switch
              size="small"
              v-model="settings.advancedMode"
              active-text="一键部署"
              @change="saveSettings"
            />
          </div>
          <el-button
            type="primary"
            plain
            size="small"
            :disabled="busy"
            title="打包日志和配置，发给公司排查"
            @click="downloadDiag"
          >
            <el-icon><Download /></el-icon>
            诊断包
          </el-button>
          <a
            v-if="settings.advancedMode"
            class="el-button el-button--small el-button--primary is-plain"
            href="/api/deliveries/export"
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
