<script>
/** 现场向导 ⑧：容器与端口验收。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFieldVerify',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <p class="wizard-step-purpose">{{ currentStepPurpose }}</p>
          <div class="hint-banner">
            <strong>步骤 8 / 8</strong> — 检查前面步骤部署的容器是否运行，以及 MySQL / Redis / Nacos / Kafka / Nginx 等关键端口是否可连通（多机时通过 SSH 汇总从机容器）。
          </div>
          <p v-if="verifyReport" class="muted" style="margin-top:0.75rem">
            {{ verifyReport.summary }}
            <span
              class="badge"
              :class="verifyReport.ok ? 'green' : 'yellow'"
              style="margin-left:0.35rem"
            >{{ verifyReport.ok ? '通过' : '未完全通过' }}</span>
          </p>
          <div v-if="verifyReport?.warnings?.length" class="wizard-alert muted" style="margin-top:0.5rem">
            <div v-for="(w, i) in verifyReport.warnings" :key="'vw'+i">WARN: {{ w }}</div>
          </div>
          <div v-if="verifyReport?.ports?.length" style="margin-top:1rem">
            <h4 style="margin:0 0 0.5rem;font-size:0.95rem">端口连通性</h4>
            <table class="table">
              <thead>
                <tr><th>服务</th><th>地址</th><th>结果</th><th>说明</th></tr>
              </thead>
              <tbody>
                <tr v-for="(p, i) in verifyReport.ports" :key="'vp'+i">
                  <td>{{ p.name }}</td>
                  <td><code>{{ p.host }}:{{ p.port }}</code></td>
                  <td><span class="badge" :class="p.ok ? 'green' : ''">{{ p.ok ? 'OK' : 'FAIL' }}</span></td>
                  <td class="muted">{{ p.message }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div v-if="verifyReport?.services?.length" style="margin-top:1rem">
            <h4 style="margin:0 0 0.5rem;font-size:0.95rem">容器状态（{{ verifyReport.serviceOk }}/{{ verifyReport.serviceAll }}）</h4>
            <table class="table">
              <thead>
                <tr><th>容器</th><th>节点</th><th>状态</th><th>说明</th></tr>
              </thead>
              <tbody>
                <tr v-for="(s, i) in verifyReport.services" :key="'vs'+i">
                  <td>{{ s.name }}</td>
                  <td>{{ s.node }}</td>
                  <td><span class="badge" :class="s.ok ? 'green' : ''">{{ s.state || (s.ok ? 'running' : 'down') }}</span></td>
                  <td class="muted">{{ s.message || s.status }}</td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="actions" :class="{ busy }" style="margin-top:1rem">
            <el-button type="primary"
              :class="{ 'btn-active': activeJobKey === 'verify' }"
              @click="runVerify"
              :disabled="busy"
            >
              {{ activeJobKey === 'verify' ? '检查中…' : '开始验收' }}
            </el-button>
            <el-button plain @click="goToStep(6)">返回</el-button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="'vf'+i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="fieldStepDone.verify || verifyDone">
            <el-button type="primary" @click="goStatus">完成，查看服务状态</el-button>
          </div>
        </div>
</template>
