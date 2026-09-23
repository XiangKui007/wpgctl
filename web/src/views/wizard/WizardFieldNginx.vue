<script>
/** 现场向导 ⑦：Nginx conf 与部署；已在运行可跳过。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldNginx',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="field full" style="margin-bottom:0.75rem">
            <label class="req">nginx 模块目录</label>
            <div class="path-row">
              <el-input v-model="fieldPaths.nginxDir" :placeholder="moduleDir('middleware', 'nginx') || '/workspace/middle/middle/nginx'" />
              <el-button type="primary" plain size="small" @click="openPicker('nginxDir', 'dir')">浏览</el-button>
            </div>
            <p class="muted" style="margin:0.35rem 0 0;font-size:0.85rem">
              <code>{{ nginxWebConfPath }}</code>
              · 部署时会按 conf 里的 listen 自动放行防火墙端口（8877 固定放行）
            </p>
            <p v-if="nginxRuntime" class="hint-line" style="margin-top:0.45rem">
              <span class="badge" :class="nginxRuntime.deployed ? 'green' : 'yellow'">{{ nginxRuntimeLabel }}</span>
              <span v-if="nginxRuntime.reason && !nginxRuntime.deployed" class="muted"> {{ nginxRuntime.reason }}</span>
            </p>
          </div>
          <div class="actions" :class="{ busy }" style="margin-bottom:0.75rem">
            <el-button
              type="primary"
              plain
              size="small"
              @click="openNginxConfEditor"
              :disabled="busy || !fieldPaths.nginxDir"
            >编辑 conf</el-button>
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'nginx-patch' && !nginxRuntime?.deployed }"
              @click="runNginxDeploy()"
              :disabled="busy || !fieldPaths.nginxDir || !isServiceEnabled('nginx')"
            >
              {{ activeJobKey === 'nginx-patch' ? '部署中…' : (nginxRuntime?.deployed ? '已部署，跳过' : '部署') }}
            </el-button>
            <el-button
              v-if="nginxRuntime?.deployed"
              type="warning"
              plain
              size="small"
              @click="runNginxDeploy({ force: true })"
              :disabled="busy || !fieldPaths.nginxDir || !isServiceEnabled('nginx')"
            >强制重新部署</el-button>
            <el-button plain @click="goToStep(5)">返回</el-button>
          </div>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions" v-if="fieldStepDone.nginx || nginxPatchDone">
            <el-button type="primary" @click="completeFieldStep('nginx', 7)">Nginx 就绪，进入 ⑧ 验收</el-button>
          </div>
        </div>
</template>
