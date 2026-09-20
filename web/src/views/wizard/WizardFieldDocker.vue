<script>
/** 现场向导 ②：离线安装 Docker。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldDocker',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 2 / 8</strong> — 在本机（主控）执行 Docker 离线安装；数据目录指向 <code>/workspace/docker_data/docker/lib</code>（与工作簿同盘）。
            <template v-if="isMultiNode"> 多机时 Init 还会通过 SSH 把 wpgctl 与 Docker 离线包上传到每台从机并安装 Docker、建目录、放行防火墙（凭据在上方「SSH 分发到从机」填写）。</template>
          </div>
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
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="initDone">
            <el-button type="primary" @click="completeFieldStep('docker', 2)">Docker 就绪，进入 ③ 数据库</el-button>
          </div>
        </div>
</template>
