<script>
/**
 * StatusServiceCard 状态页单条 Docker 服务卡片。
 * 字段在宽卡片里两栏排列，操作条固定在底部，便于分栏网格等高。
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
        <template v-if="svcProject(service) && svcProject(service) !== svcService(service) && svcProject(service) !== svcName(service)">
          <dt title="Compose 项目">项目</dt>
          <dd>{{ svcProject(service) }}</dd>
        </template>
        <template v-if="svcNetworks(service)">
          <dt class="status-wide-label" title="Docker 网络">网络</dt>
          <dd class="status-wide">{{ svcNetworks(service) }}</dd>
        </template>
        <template v-if="svcPortChips(service).visible.length">
          <dt class="status-wide-label" title="端口映射">端口</dt>
          <dd class="status-wide">
            <div class="status-ports">
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
          </dd>
        </template>
        <template v-else-if="svcPorts(service)">
          <dt class="status-wide-label" title="端口映射">端口</dt>
          <dd class="status-wide" :title="svcPorts(service)">内部端口</dd>
        </template>
        <template v-if="!settings.privacyMode && svcImage(service)">
          <dt class="status-wide-label" title="镜像">镜像</dt>
          <dd class="status-wide status-image" :title="svcImage(service)">{{ svcImage(service) }}</dd>
        </template>
        <template v-if="svcComposeDir(service)">
          <dt class="status-wide-label" title="Compose 工作目录">目录</dt>
          <dd class="status-wide status-image" :title="svcComposeDir(service)">{{ settings.privacyMode ? '******' : svcComposeDir(service) }}</dd>
        </template>
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
          v-if="svcComposeDir(service)"
          type="danger"
          plain
          size="small"
          :disabled="statusBusy"
          @click="runStatusAction('stack-down', service)"
        >Down 栈</el-button>
        <el-button
          type="info"
          plain
          size="small"
          :disabled="statusBusy || !svcIsLocal(service)"
          :title="svcIsLocal(service) ? '本机容器日志' : '从机日志请到该机查看'"
          @click="openLogsFor(service)"
        >日志</el-button>
      </el-button-group>
    </div>
  </article>
</template>
