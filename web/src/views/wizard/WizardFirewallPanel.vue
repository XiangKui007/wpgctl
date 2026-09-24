<script>
/** 向导内防火墙：按机器切换检查、启动、reload；已运行时列出当前放行端口。 */
import { onMounted } from 'vue'
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'WizardFirewallPanel',
  setup() {
    const ctx = useConsole()
    onMounted(() => {
      ctx.firewallNodeIndex.value = 0
      ctx.loadFirewallStatus()
    })
    return ctx
  },
}
</script>

<template>
  <div class="fw-panel" :class="{ ok: firewallStatus?.running, warn: firewallStatus && !firewallStatus.running }">
    <div class="fw-panel-head">
      <strong>防火墙</strong>
      <span class="badge" :class="firewallBadge.cls">{{ firewallBadge.text }}</span>
      <span class="muted">{{ firewallToolLabel }}</span>
    </div>
    <div v-if="siteForm.nodes.length" class="fw-node-switch">
      <span class="muted">检查机器</span>
      <el-button
        v-for="(n, i) in siteForm.nodes"
        :key="n.ip + i"
        size="small"
        :type="firewallNodeIndex === i ? 'primary' : 'default'"
        :plain="firewallNodeIndex !== i"
        :disabled="firewallChecking || busy"
        @click="selectFirewallNode(i)"
      >
        {{ n.name || `机器 ${i + 1}` }}
        <span class="muted">{{ n.ip || '未填 IP' }}{{ i === 0 ? ' · 主控' : '' }}</span>
      </el-button>
    </div>
    <p class="muted" style="margin:0.4rem 0 0.7rem">
      {{ firewallStatus?.hint || '启动前会放行 SSH 22 和控制台 9527。' }}
    </p>
    <div v-if="firewallStatus?.running" class="fw-ports">
      <strong>当前放行</strong>
      <template v-if="(firewallStatus.openPorts || []).length">
        <span v-for="(p, pi) in firewallStatus.openPorts" :key="(p.kind || '') + p.spec + pi" class="fw-port-chip">{{ p.spec }}</span>
      </template>
      <span v-else class="muted">无额外端口</span>
    </div>
    <div class="actions" style="margin:0.65rem 0 0" :class="{ busy }">
      <el-button
        type="primary"
        plain
        :class="{ 'btn-active': firewallChecking }"
        :disabled="firewallChecking || busy"
        @click="loadFirewallStatus"
      >
        {{ firewallChecking ? '检查中…' : '检查状态' }}
      </el-button>
      <el-button
        type="success"
        plain
        :class="{ 'btn-active': activeJobKey === 'firewall-start' }"
        :disabled="busy || !firewallStatus?.canStart"
        :title="firewallStatus?.canStart ? '' : '从机请执行本步 Init，或切回主控机再启动'"
        @click="startFirewall"
      >
        {{ activeJobKey === 'firewall-start' ? '启动中…' : '启动防火墙' }}
      </el-button>
      <el-button
        plain
        :class="{ 'btn-active': activeJobKey === 'firewall-reload' }"
        :disabled="busy || !firewallStatus?.canReload"
        :title="firewallStatus?.canReload ? '让 firewalld permanent 规则生效' : '仅主控机 firewalld 已运行时需要 reload'"
        @click="reloadFirewall"
      >
        {{ activeJobKey === 'firewall-reload' ? 'reload 中…' : 'reload 防火墙' }}
      </el-button>
    </div>
    <p v-if="firewallStatusError" class="muted" style="margin:0.5rem 0 0;color:var(--danger)">{{ firewallStatusError }}</p>
  </div>
</template>
