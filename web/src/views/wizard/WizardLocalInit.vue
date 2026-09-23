<script>
/** 本机联调向导：Init。 */
import { useConsole } from '@/composables/useConsole.js'
import WizardFirewallPanel from './WizardFirewallPanel.vue'

export default {
  name: 'WizardLocalInit',
  components: { WizardFirewallPanel },
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="muted">创建目录、按需安装 Docker、启动防火墙（SSH 22、控制台 9527）。</p>
          <WizardFirewallPanel />
          <div class="field-grid" style="margin-top:0.75rem">
            <div class="field full">
              <label class="req">Docker 离线安装目录</label>
              <div class="path-row">
                <el-input v-model="form.dockerPackage" placeholder="含 offline_install_docker.sh 的目录" />
                <el-button type="primary" plain size="small" @click="openPicker('dockerPackage', 'dir')">浏览</el-button>
              </div>
            </div>
            <div class="field full">
              <label>Base 包目录（可选）</label>
              <div class="path-row">
                <el-input v-model="form.base" placeholder="middleware 或标准 base" />
                <el-button type="primary" plain size="small" @click="openPicker('base', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>
          <div class="actions">
            <el-button type="primary" @click="runInit" :disabled="busy">
              {{ busy ? '初始化中…' : '执行 Init' }}
            </el-button>
            <el-button plain @click="goToStep(1)">返回</el-button>
          </div>
          <JobLogBox :lines="jobLogs" :running="!!activeJobKey" />
          <div class="actions" v-if="initDone">
            <el-button type="primary" @click="goToStep(3)">进入部署</el-button>
          </div>
        </div>
</template>
