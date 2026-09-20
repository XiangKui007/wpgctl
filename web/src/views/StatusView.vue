<script>
/** 运行状态：本机与从机 Docker 容器卡片。多机按节点分栏，单机两列网格。 */
import { computed } from 'vue'
import { useConsole } from '@/composables/useConsole.js'
import StatusServiceCard from '../components/StatusServiceCard.vue'

/**
 * groupStatusColumns 多机且看「全部节点」时按机器分栏。
 * 只有一栏时返回空数组，交给两列卡片网格。
 * @param {object[]} services
 * @param {object[]} nodes
 * @param {string} nodeFilter
 * @param {(s: object) => boolean} isLocal
 * @param {(s: object) => string} nodeIPOf
 * @param {(s: object) => string} nodeNameOf
 * @returns {{ key: string, name: string, ip: string, error?: string, local: boolean, services: object[] }[]}
 */
function groupStatusColumns(services, nodes, nodeFilter, isLocal, nodeIPOf, nodeNameOf) {
  if (!Array.isArray(nodes) || nodes.length <= 1 || nodeFilter !== 'all') return []
  const map = new Map()
  for (const n of nodes) {
    const key = n.local ? 'local' : String(n.ip || '')
    if (!key) continue
    map.set(key, {
      key,
      name: n.name || (n.local ? '本机' : key),
      ip: n.ip || '',
      error: n.error || '',
      local: !!n.local,
      services: [],
    })
  }
  for (const s of services) {
    const key = isLocal(s) ? 'local' : String(nodeIPOf(s) || 'unknown')
    if (!map.has(key)) {
      map.set(key, {
        key,
        name: nodeNameOf(s) || key,
        ip: nodeIPOf(s) || '',
        error: '',
        local: isLocal(s),
        services: [],
      })
    }
    map.get(key).services.push(s)
  }
  const cols = [...map.values()].filter((c) => c.services.length > 0 || c.error)
  return cols.length > 1 ? cols : []
}

export default {
  name: 'StatusView',
  components: { StatusServiceCard },
  setup() {
    const ctx = useConsole()
    const statusColumns = computed(() =>
      groupStatusColumns(
        ctx.filteredServices.value,
        ctx.statusNodes.value,
        ctx.statusNodeFilter.value,
        ctx.svcIsLocal,
        ctx.svcNodeIP,
        ctx.svcNode,
      ),
    )
    return { ...ctx, statusColumns }
  },
}
</script>

<template>
    <section class="panel panel-status">
      <div class="panel-head">
        <div>
          <h2>运行状态</h2>
          <p>
            <span class="pulse"></span>
            {{ statusSource === 'cluster' ? '本机 + SSH 汇总各节点 docker ps' : statusSource === 'rendered' ? 'rendered compose 项目' : 'docker ps 汇总（现场逐步部署）' }}
          </p>
        </div>
        <el-button type="primary" plain @click="loadStatus" :disabled="statusBusy">
          <el-icon><Refresh /></el-icon>
          刷新
        </el-button>
      </div>

      <details v-if="isMultiNode" class="ssh-panel" :open="sshPanelOpen">
        <summary @click.prevent="sshPanelOpen = !sshPanelOpen">
          <strong>SSH 凭据（查询从机 Docker）</strong>
          <span class="muted">· 与向导共用，刷新时 SSH 到其他节点执行 docker ps</span>
          <span class="badge" :class="sshCredsReady ? 'green' : 'yellow'" style="margin-left:auto">
            {{ sshCredsReady ? '凭据已填' : '未填凭据' }}
          </span>
        </summary>
        <div class="field-grid" style="margin-top:0.6rem">
          <div class="field">
            <label>从机 SSH 密码（仅本次会话，不落盘）</label>
            <el-input v-model="sshCreds.password" type="password" autocomplete="new-password" placeholder="各从机相同密码时填写" show-password />
          </div>
          <div class="field">
            <label>或 SSH 私钥路径（主控机上）</label>
            <el-input v-model="sshCreds.keyPath" placeholder="/root/.ssh/id_rsa" />
          </div>
        </div>
      </details>

      <div class="status-stats" v-if="statusTotal">
        <button type="button" class="status-stat" :class="{ active: statusFilter === 'all' }" @click="statusFilter = 'all'">
          <span class="status-stat-n">{{ statusTotal }}</span>
          <span class="status-stat-l">全部</span>
        </button>
        <button type="button" class="status-stat ok" :class="{ active: statusFilter === 'running' }" @click="statusFilter = 'running'">
          <span class="status-stat-n">{{ statusRunning }}</span>
          <span class="status-stat-l">运行中</span>
        </button>
        <button type="button" class="status-stat off" :class="{ active: statusFilter === 'stopped' }" @click="statusFilter = 'stopped'">
          <span class="status-stat-n">{{ statusStopped }}</span>
          <span class="status-stat-l">已停止</span>
        </button>
      </div>

      <div class="status-nodes" v-if="statusNodes.length > 1">
        <button
          type="button"
          class="status-node"
          :class="{ active: statusNodeFilter === 'all' }"
          @click="statusNodeFilter = 'all'"
        >全部节点</button>
        <button
          v-for="n in statusNodes"
          :key="n.local ? 'local' : n.ip"
          type="button"
          class="status-node"
          :class="{ active: statusNodeFilter === (n.local ? 'local' : n.ip), bad: !!n.error }"
          :title="n.error || n.ip"
          @click="statusNodeFilter = n.local ? 'local' : n.ip"
        >
          <strong>{{ n.name }}</strong>
          <span class="muted">{{ n.error ? '失败' : n.total }}</span>
        </button>
      </div>

      <div class="status-toolbar" v-if="services.length">
        <el-input
          v-model="statusQueryInput"
          class="status-search"
          clearable
          placeholder="搜索服务、容器、编号、端口、目录…"
        >
          <template #prefix>
            <el-icon><Search /></el-icon>
          </template>
        </el-input>
        <span class="status-count muted">显示 {{ filteredServices.length }} / {{ services.length }}</span>
      </div>

      <el-alert
        v-if="statusWarning"
        class="status-note"
        type="warning"
        show-icon
        :closable="false"
        :title="statusWarning"
      />
      <el-alert
        v-if="statusActionMsg"
        class="status-note"
        :type="statusActionOk ? 'success' : 'warning'"
        show-icon
        :closable="false"
        :title="statusActionMsg"
      />

      <div v-if="statusColumns.length > 1" class="status-split">
        <section v-for="col in statusColumns" :key="col.key" class="status-split-pane">
          <header class="status-split-head">
            <strong>{{ col.name }}</strong>
            <span v-if="col.ip" class="muted">{{ col.ip }}</span>
            <span class="status-count muted">{{ col.services.length }}</span>
          </header>
          <el-alert
            v-if="col.error"
            class="status-note"
            type="warning"
            show-icon
            :closable="false"
            :title="col.error"
          />
          <div class="status-list status-list-stack">
            <StatusServiceCard
              v-for="s in col.services"
              :key="(svcIsLocal(s) ? 'local' : svcNodeIP(s)) + ':' + svcId(s)"
              :service="s"
            />
          </div>
          <el-empty v-if="!col.services.length && !col.error" description="该节点当前筛选下没有容器。" />
        </section>
      </div>
      <el-row v-else-if="filteredServices.length" :gutter="16" class="status-list">
        <el-col
          v-for="s in filteredServices"
          :key="(svcIsLocal(s) ? 'local' : svcNodeIP(s)) + ':' + svcId(s)"
          :xs="24"
          :md="12"
        >
          <StatusServiceCard :service="s" />
        </el-col>
      </el-row>
      <el-empty
        v-else-if="services.length"
        class="status-empty"
        :description="statusQueryInput.trim() ? `没有匹配「${statusQueryInput}」的服务。` : '当前筛选下没有服务。'"
      />
      <el-empty v-else description="暂无服务数据。完成部署后可在此启停、重启或 Down 容器。" />
    </section>
</template>
