<script>
/** 服务日志：WebSocket 跟随 docker logs -f；多机可选从机，经 SSH 执行同一条命令。 */
import { nextTick, ref, watch } from 'vue'
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'LogsView',
  setup() {
    const ctx = useConsole()
    const logBox = ref(null)
    const pinBottom = ref(true)
    function onLogScroll() {
      const el = logBox.value
      if (!el) return
      pinBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48
    }
    watch(
      () => ctx.liveLogs.value,
      async () => {
        if (!pinBottom.value) return
        await nextTick()
        const el = logBox.value
        if (el) el.scrollTop = el.scrollHeight
      },
    )
    return { ...ctx, logBox, onLogScroll }
  },
}
</script>

<template>
    <section class="panel">
      <div class="panel-head">
        <div>
          <h2>服务日志</h2>
          <p>实时跟随 docker logs -f。从机走 SSH，凭据与状态页共用。</p>
        </div>
      </div>
      <div class="surface">
        <div class="field-grid">
          <div class="field">
            <label class="req">服务 / 容器名</label>
            <el-input v-model="logService" placeholder="waterwork-center" />
          </div>
          <div class="field" v-if="logNodeOptions.length > 1">
            <label>节点</label>
            <el-select v-model="logNodeIP" placeholder="本机 Docker" style="width:100%">
              <el-option
                v-for="n in logNodeOptions"
                :key="n.value"
                :label="n.label"
                :value="n.value"
              />
            </el-select>
          </div>
        </div>

        <details v-if="isRemoteLogNode" class="ssh-panel" :open="true">
          <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
            <strong>SSH 凭据（从机 docker logs -f）</strong>
            <span class="muted">· 与向导/状态页共用，不落盘</span>
            <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
              {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
            </span>
          </summary>
          <div class="field-grid" style="margin-top:0.6rem">
            <div class="field">
              <label>从机 SSH 密码（仅本次会话）</label>
              <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
            </div>
            <div class="field">
              <label>或 SSH 私钥路径（主控机上）</label>
              <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
            </div>
          </div>
        </details>

        <div class="actions">
          <el-button type="primary" @click="startLogs" :disabled="!logService">开始监听</el-button>
          <el-button type="warning" plain @click="stopLogs">停止</el-button>
        </div>
        <div ref="logBox" class="log-box tall" @scroll="onLogScroll">
          <pre>{{ liveLogs || '等待日志…' }}</pre>
        </div>
      </div>
    </section>
</template>
