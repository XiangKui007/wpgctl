<script>
/** 开始前：初始化 workspace 目录后进入部署向导。 */
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
          <p>初始化工作簿目录（没有就建、已有就跳过），然后进入部署向导。Docker 仍在向导第 ② 步安装。</p>
        </div>
        <el-button plain @click="view = 'home'">返回首页</el-button>
      </div>

      <div class="surface preflight">
        <div class="field-grid">
          <div class="field full">
            <label class="req">workspace 路径</label>
            <div class="path-row">
              <el-input
                v-model="siteForm.paths.workspace"
                :placeholder="isLocalDocker ? 'D:/workspace' : '/workspace'" />
              <el-button type="primary" plain size="small" @click="openPicker('pathsWorkspace', 'dir')">浏览</el-button>
            </div>
          </div>
        </div>

        <p class="hint-banner" style="margin-top:1rem">
          默认工作簿根为 <code>/workspace</code>（其下常见 <code>platform</code>、<code>middleware</code>/<code>middle</code>、<code>sz-waterwork</code>）。
          将创建（已存在则跳过）：workspace、<code>rendered</code>、<code>bak</code>；Linux 下还会建 <code>/workspace/docker_data</code>（dockerd --graph）。
        </p>

        <div class="actions" style="margin-top:1rem">
          <el-button type="primary"
            @click="initWorkspaceAndEnter"
            :disabled="busy || !siteForm.paths.workspace"
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
