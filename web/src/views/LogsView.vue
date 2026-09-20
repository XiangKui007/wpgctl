<script>
/** 服务日志：按容器名 WebSocket 拉取。 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'LogsView',
  setup() {
    return useConsole()
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>服务日志</h2>
          <p>WebSocket 拉取容器最近日志，适配远程桌面排障。</p>
        </div>
      </div>
      <div class="surface">
        <div class="field-grid">
          <div class="field">
            <label class="req">服务 / 容器名</label>
            <el-input v-model="logService" placeholder="waterwork-center" />
          </div>
        </div>
        <div class="actions">
          <el-button type="primary" @click="startLogs" :disabled="!logService">开始监听</el-button>
          <el-button type="warning" plain @click="stopLogs">停止</el-button>
        </div>
        <div class="log-box tall">
          <pre>{{ liveLogs || '等待日志…' }}</pre>
        </div>
      </div>
    </section>
</template>
