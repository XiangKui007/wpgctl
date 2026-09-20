<script>
/** 本机联调向导：环境体检。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardLocalPrecheck',
  setup() {
    return useConsole()
  },
}
</script>

<template>
        <div>
          <div class="actions" style="margin-top: 0">
            <el-button type="primary" @click="runPrecheck" :disabled="busy">
              {{ busy ? '体检进行中…' : '执行 Precheck' }}
            </el-button>
            <el-button plain @click="goToStep(0)">返回</el-button>
          </div>
          <div v-if="precheckItems.length" class="check-list" style="margin-top: 1.25rem">
            <div v-for="(it, i) in precheckItems" :key="i" class="check-item">
              <span class="badge" :class="it.severity">{{ it.severity }}</span>
              <div>
                <strong>{{ it.name }}</strong>
                <div class="muted">{{ it.message }}</div>
                <div v-if="it.hint" class="hint-line">→ {{ it.hint }}</div>
              </div>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="precheckDone">
            <el-button type="primary" @click="goToStep(2)" :disabled="precheckBlocked || !precheckDone">进入初始化</el-button>
          </div>
        </div>
</template>
