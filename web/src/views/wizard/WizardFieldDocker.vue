<script>
/** 现场向导 ②：离线安装 Docker。 */
import { useConsole } from '@/composables/useConsole.js'
import WizardFirewallPanel from './WizardFirewallPanel.vue'

export default {
  name: 'WizardFieldDocker',
  components: { WizardFirewallPanel },
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="node-docker-list">
            <div class="node-section-head">
              <div>
                <strong>各机器 Docker</strong>
                <p class="muted">状态挂在对应机器后面。</p>
              </div>
              <el-button type="primary" plain size="small" :loading="nodeDockerBusy" @click="refreshNodeDocker">
                {{ nodeDockerBusy ? '检查中…' : '重新检查' }}
              </el-button>
            </div>
            <ul>
              <li v-for="(n, idx) in siteForm.nodes" :key="idx">
                <span class="node-number">{{ idx + 1 }}</span>
                <strong>{{ n.name || `机器 ${idx + 1}` }}</strong>
                <code>{{ n.ip || '未填 IP' }}</code>
                <span v-if="idx === 0" class="badge green">主控</span>
                <span
                  class="badge"
                  :class="nodeDockerOf(n).ok ? 'green' : 'yellow'"
                  :title="nodeDockerOf(n).message || ''"
                >{{ nodeDockerLabel(n) || (nodeDockerBusy ? 'Docker 检查中' : '未检查') }}</span>
              </li>
            </ul>
          </div>
          <WizardFirewallPanel />
          <div class="field-grid" style="margin-top:0.75rem">
            <div class="field full">
              <label class="req">Docker 离线包目录</label>
              <div class="path-row">
                <el-input v-model="form.dockerPackage" placeholder="middleware/docker_package/docker_package" />
                <el-button type="primary" plain size="small" @click="openPicker('dockerPackage', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>
          <div class="actions" :class="{ busy }">
            <el-button type="primary" :class="{ 'btn-active': activeJobKey === 'init' }" @click="runInit" :disabled="busy || !form.dockerPackage">
              {{ busy && activeJobKey === 'init' ? '安装中…' : '执行 Docker 安装 (Init)' }}
            </el-button>
            <el-button plain @click="goToStep(0)">返回</el-button>
          </div>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions" v-if="initDone">
            <el-button type="primary" @click="completeFieldStep('docker', 2)">Docker 就绪，进入 ③ 数据库</el-button>
          </div>
        </div>
</template>
