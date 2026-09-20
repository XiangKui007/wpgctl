<script>
/** 本机联调向导：Init。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardLocalInit',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="muted">将创建目录、按需安装 Docker、放行端口。多机场景会 SSH 分发到各节点。</p>
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
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="initDone">
            <el-button type="primary" @click="goToStep(3)">进入部署</el-button>
          </div>
        </div>
</template>
