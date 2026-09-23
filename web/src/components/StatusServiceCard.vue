<script>
/**
 * StatusServiceCard 状态页单条 Docker 服务卡片。
 * 主控机 / 从机共用同一套字段（空值显示 —），避免 compose 与 docker ps 数据源导致行数对不齐。
 */
import { useConsole } from '@/composables/useConsole.js'

export default {
  name: 'StatusServiceCard',
  props: {
    /** 状态接口里的单条服务（只读展示与操作） */
    service: { type: Object, required: true },
  },
  setup() {
    return useConsole()
  },
}
</script>

<template>
  <article
    class="status-card"
    :class="{ running: svcRunning(service), stopped: !svcRunning(service) }"
  >
    <div class="status-card-body">
      <div class="status-card-title">
        <span class="status-k">服务</span>
        <strong>{{ svcService(service) }}</strong>
        <el-tag
          size="small"
          :type="svcRunning(service) ? 'success' : 'danger'"
          effect="light"
          title="状态"
        >{{ svcState(service) }}</el-tag>
        <el-tag
          v-if="svcNode(service) || svcNodeIP(service)"
          size="small"
          :type="svcIsLocal(service) ? 'success' : 'info'"
          effect="plain"
          :title="svcNodeIP(service)"
        >{{ svcNode(service) || svcNodeIP(service) }}</el-tag>
      </div>
      <dl class="status-fields">
        <template v-if="svcNode(service) || svcNodeIP(service)">
          <dt title="所在机器">节点</dt>
          <dd>{{ svcNode(service) }}<span v-if="svcNodeIP(service)" class="muted"> · {{ svcNodeIP(service) }}</span></dd>
        </template>
        <template v-if="svcName(service)">
          <dt title="容器名">容器</dt>
          <dd>{{ svcName(service) }}</dd>
        </template>
        <template v-if="svcShortId(service)">
          <dt title="容器 ID">编号</dt>
          <dd class="status-mono" :title="svcId(service)">{{ svcShortId(service) }}</dd>
        </template>
        <template v-if="svcStatus(service)">
          <dt title="运行情况">运行</dt>
          <dd>{{ svcStatus(service) }}</dd>
        </template>
        <template v-if="svcHealth(service)">
          <dt title="健康检查">健康</dt>
          <dd :class="['status-health', { ok: svcHealth(service) === 'healthy', bad: svcHealth(service) === 'unhealthy' }]">{{ svcHealth(service) }}</dd>
        </template>
        <template v-if="svcCreated(service)">
          <dt title="创建时间">创建</dt>
          <dd>{{ svcCreated(service) }}</dd>
        </template>
        <dt title="Compose 项目">项目</dt>
        <dd>{{ svcProject(service) || '—' }}</dd>
        <dt class="status-wide-label" title="Docker 网络">网络</dt>
        <dd class="status-wide">{{ svcNetworks(service) || '—' }}</dd>
        <dt class="status-wide-label" title="端口映射">端口</dt>
        <dd class="status-wide">
          <div v-if="svcPortChips(service).visible.length" class="status-ports">
            <span
              v-for="p in svcPortChips(service).visible"
              :key="p"
              class="port-chip"
              :title="svcPorts(service)"
            >{{ p }}</span>
            <span
              v-if="svcPortChips(service).more"
              class="port-chip more"
              :title="svcPorts(service)"
            >+{{ svcPortChips(service).more }}</span>
          </div>
          <span
            v-else-if="svcIsHostNetwork(service)"
            class="muted"
            title="host 网络与宿主机共用端口，docker ps 通常不列映射"
          >host（与宿主机共用）</span>
          <span v-else-if="svcPorts(service)" class="muted" :title="svcPorts(service)">内部端口</span>
          <span v-else class="muted">—</span>
        </dd>
        <template v-if="!settings.privacyMode">
          <dt class="status-wide-label" title="镜像">镜像</dt>
          <dd class="status-wide status-image" :title="svcImage(service)">{{ svcImage(service) || '—' }}</dd>
        </template>
        <dt class="status-wide-label" title="Compose 工作目录">目录</dt>
        <dd class="status-wide status-image" :title="svcComposeDir(service)">{{ settings.privacyMode && svcComposeDir(service) ? '******' : (svcComposeDir(service) || '—') }}</dd>
        <template v-if="svcExitCode(service) !== null">
          <dt title="退出码">退出</dt>
          <dd class="status-health bad">{{ svcExitCode(service) }}</dd>
        </template>
      </dl>
    </div>
    <div class="status-ops">
      <el-button-group>
        <el-button
          v-if="!svcRunning(service)"
          type="success"
          plain
          size="small"
          :disabled="statusBusy"
          @click="runStatusAction('start', service)"
        >启动</el-button>
        <el-button
          v-else
          type="warning"
          plain
          size="small"
          :disabled="statusBusy"
          @click="runStatusAction('stop', service)"
        >停止</el-button>
        <el-button type="primary" plain size="small" :disabled="statusBusy" @click="runStatusAction('restart', service)">重启</el-button>
        <el-button type="danger" plain size="small" :disabled="statusBusy" @click="runStatusAction('down', service)">Down</el-button>
        <el-button
          type="info"
          plain
          size="small"
          :disabled="statusBusy"
          :title="svcIsLocal(service) ? '本机实时日志（docker logs -f）' : 'SSH 跟随该从机 docker logs -f'"
          @click="openLogsFor(service)"
        >日志</el-button>
      </el-button-group>
    </div>
  </article>
</template>
