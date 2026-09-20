<script>
/** 本机联调向导：Deploy。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardLocalDeploy',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <div class="field-grid">
            <div class="field full">
              <label class="req">Release 包目录</label>
              <div class="path-row">
                <el-input v-model="form.package" placeholder="选择含 manifest.yaml 的包目录" />
                <el-button type="primary" plain size="small" @click="openPicker('package', 'dir')">浏览</el-button>
              </div>
            </div>
          </div>
          <div class="actions">
            <el-button type="primary" plain @click="loadPreview" :disabled="busy || !form.package">预览将部署的服务</el-button>
            <el-button type="primary" @click="runDeploy(false)" :disabled="busy || !form.package">
              {{ busy ? '部署中…' : '开始 Deploy' }}
            </el-button>
            <el-button type="warning" plain @click="runDeploy(true)" :disabled="busy || !form.package">仅渲染（Dry-run）</el-button>
            <el-button plain @click="goToStep(2)">返回</el-button>
          </div>
          <div v-if="preview" class="surface" style="margin-top:1rem;padding:1rem">
            <p class="muted">版本 {{ preview.version }} · {{ preview.count }} 个服务 · profiles: {{ (preview.profiles || []).join(', ') }}</p>
            <p class="muted">端口: {{ (preview.ports || []).join(', ') }}</p>
            <table class="table" v-if="preview?.services?.length">
              <thead><tr><th>服务</th><th>层</th><th>端口</th><th>镜像</th></tr></thead>
              <tbody>
                <tr v-for="s in preview.services" :key="s.name">
                  <td>{{ s.name }}</td>
                  <td>L{{ s.layer }}</td>
                  <td>{{ s.port || '—' }}</td>
                  <td class="muted">{{ s.image }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div v-if="smokeRows.length" class="surface" style="margin-top:1rem;padding:1rem">
            <p class="muted">健康矩阵</p>
            <table class="table">
              <thead><tr><th>服务</th><th>端口</th><th>状态</th><th>说明</th></tr></thead>
              <tbody>
                <tr v-for="e in smokeRows" :key="e.name">
                  <td>{{ e.name }}</td>
                  <td>{{ e.port || '—' }}</td>
                  <td><span class="badge" :class="e.ok ? 'green' : 'red'">{{ e.ok ? 'ok' : 'fail' }}</span></td>
                  <td class="muted">{{ e.message }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="deployDone" class="actions">
            <el-button type="primary" @click="openStatus">查看运行状态</el-button>
            <el-button type="primary" plain @click="openReport()">打开验收报告</el-button>
          </div>
        </div>
</template>
