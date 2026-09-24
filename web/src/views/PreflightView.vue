<script>
/** 开始前：默认选 `/`，其下没有 workspace 则创建，已有则跳过并进入向导。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'PreflightView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>开始前</h2>
          <p>创建 /workspace（已有则跳过），然后进入向导。</p>
        </div>
        <el-button plain @click="view = 'home'">返回首页</el-button>
      </div>

      <div class="surface preflight" v-loading="panelBusy" :element-loading-text="busyText">
        <div class="field-grid">
          <div class="field full">
            <label class="req">初始化根目录</label>
            <div class="path-row">
              <el-input
                v-model="siteForm.paths.workspace"
                placeholder="/" />
              <el-button type="primary" plain size="small" @click="openPicker('pathsWorkspace', 'dir')">浏览</el-button>
            </div>
          </div>
        </div>

        <p class="hint-banner" style="margin-top:1rem">
          本步不安装 Docker，也不创建交付包目录。
        </p>

        <div class="actions" style="margin-top:1rem">
          <el-button type="primary"
            @click="initWorkspaceAndEnter"
            :disabled="busy || !(siteForm.paths.workspace || '').trim()"
          >
            {{ busy && workspaceBusy === 'init' ? '初始化中…' : '初始化目录并进入向导' }}
          </el-button>
        </div>

        <p v-if="workspaceMsg" class="choice-feedback" :class="workspaceProbe?.ready ? 'ok' : 'warn'" style="margin-top:1rem">
          {{ workspaceMsg }}
        </p>

        <ul v-if="workspaceProbe?.dirs?.length" class="workspace-dir-list">
          <li v-for="d in workspaceProbe.dirs" :key="d.path" :class="{ ok: d.exists, miss: !d.exists }">
            <strong>{{ d.label }}</strong>
            <span class="muted">{{ d.path }}</span>
            <span class="badge" :class="d.exists ? 'green' : 'yellow'">{{ d.exists ? '已有' : '缺失' }}</span>
          </li>
        </ul>
      </div>
    </section>
</template>
