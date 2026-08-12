<template>
  <div class="app-shell">
    <header class="topbar">
      <div class="brand-mark">
        <strong>WPGCTL</strong>
        <span>水厂交付控制台</span>
      </div>
      <nav class="nav">
        <button :class="{ active: view === 'home' }" @click="view = 'home'">首页</button>
        <button :class="{ active: view === 'guide' }" @click="view = 'guide'">怎么用</button>
        <button :class="{ active: view === 'wizard' }" @click="goWizard">部署向导</button>
        <button :class="{ active: view === 'status' }" @click="openStatus">运行状态</button>
        <button :class="{ active: view === 'logs' }" @click="view = 'logs'">服务日志</button>
        <button :class="{ active: view === 'history' }" @click="openHistory">部署台账</button>
      </nav>
    </header>

    <!-- HOME -->
    <section v-if="view === 'home'" class="home">
      <div class="hero">
        <div class="hero-copy">
          <div class="hero-kicker">Plant Delivery Console</div>
          <h1>WPG<em>CTL</em></h1>
          <p class="lead">
            先改好一份 site.yaml，再点向导走完四步。下面是最短用法。
          </p>
          <div class="cta-row">
            <button class="btn btn-primary" @click="goWizard">打开部署向导</button>
            <button class="btn btn-ghost" @click="view = 'guide'">看使用说明</button>
          </div>
        </div>
        <div class="hero-visual">
          <div class="stat-float">
            <small>当前站点</small>
            <b>{{ siteCode }}</b>
          </div>
          <div class="wave-panel">
            <div class="meta">
              <strong>{{ siteName }}</strong>
              <span>{{ siteLoaded ? 'site.yaml 已加载，可开始向导' : '未读到 site.yaml，请先在项目根目录准备配置' }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="howto panel">
        <div class="panel-head">
          <div>
            <h2>三步上手</h2>
            <p>Windows 上先熟悉流程；真正部署请在 Linux 现场主控机执行。</p>
          </div>
        </div>
        <div class="howto-grid">
          <article class="howto-card">
            <span class="howto-num">01</span>
            <h3>改配置</h3>
            <p>编辑项目根目录 <code>site.yaml</code>：改 IP、密码、profiles、路径。这是现场唯一要手改的文件。</p>
          </article>
          <article class="howto-card">
            <span class="howto-num">02</span>
            <h3>开控制台</h3>
            <p>终端执行 <code>wpgctl ui --site site.yaml</code>，浏览器打开本页，点「部署向导」。</p>
          </article>
          <article class="howto-card">
            <span class="howto-num">03</span>
            <h3>跟向导走</h3>
            <p>体检 → 初始化 → 填 release 包路径 → 部署。升级以后用命令 <code>wpgctl upgrade</code>。</p>
          </article>
        </div>
      </div>
    </section>

    <!-- GUIDE -->
    <section v-else-if="view === 'guide'" class="panel">
      <div class="panel-head">
        <div>
          <h2>使用说明</h2>
          <p>按场景选命令；日常推荐用页面向导。</p>
        </div>
        <button class="btn btn-ghost" @click="view = 'home'">返回首页</button>
      </div>
      <div class="surface guide-blocks">
        <div>
          <h3>场景 A · 第一次部署（现场 Linux）</h3>
          <ol class="guide-ol">
            <li>把 <code>wpgctl</code> 二进制和 <code>site.yaml</code> 拷到主控机</li>
            <li>准备好 base / release 包目录（或 <code>wpgctl fetch</code> 拉取）</li>
            <li>执行 <code>wpgctl ui --site site.yaml</code>，打开浏览器走四步向导</li>
            <li>或纯命令：<code>precheck → init → deploy --package &lt;目录&gt;</code></li>
          </ol>
        </div>
        <div>
          <h3>场景 B · 日常升级 1~3 个服务</h3>
          <ol class="guide-ol">
            <li>拿到 patch 包目录</li>
            <li><code>wpgctl upgrade ./wpg-patch-4.0.3 --site site.yaml --yes</code></li>
            <li>失败会自动回滚镜像；SQL 需人工评估</li>
          </ol>
        </div>
        <div>
          <h3>场景 C · 你现在这台 Windows</h3>
          <ol class="guide-ol">
            <li>可改 <code>site.yaml</code>、打开本控制台熟悉界面</li>
            <li>可跑 Precheck 看体检报告（本机没有现场 Docker/包属正常）</li>
            <li>真正 <code>init/deploy</code> 请到 Linux 服务器做</li>
          </ol>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="goWizard">去部署向导试试</button>
        </div>
      </div>
    </section>

    <!-- WIZARD -->
    <section v-else-if="view === 'wizard'" class="panel">
      <div class="panel-head">
        <div>
          <h2>部署向导</h2>
          <p>按步骤完成首次交付。红色体检项会阻断后续动作。</p>
        </div>
      </div>

      <div class="steps">
        <div class="step" :class="stepClass(0)">
          <div class="idx">Step 01</div>
          <div class="title">确认站点</div>
        </div>
        <div class="step" :class="stepClass(1)">
          <div class="idx">Step 02</div>
          <div class="title">环境体检</div>
        </div>
        <div class="step" :class="stepClass(2)">
          <div class="idx">Step 03</div>
          <div class="title">初始化</div>
        </div>
        <div class="step" :class="stepClass(3)">
          <div class="idx">Step 04</div>
          <div class="title">部署启动</div>
        </div>
      </div>

      <div class="surface">
        <!-- step 0 -->
        <div v-if="wizardStep === 0">
          <p class="hint-banner">
            站点信息来自项目根目录的 <code>site.yaml</code>（只读预览）。要改 IP/密码，请先编辑该文件再点「重新加载」。
          </p>
          <div class="field-grid">
            <div class="field">
              <label>站点名称</label>
              <input :value="site?.site?.name || ''" readonly />
            </div>
            <div class="field">
              <label>站点编码</label>
              <input :value="site?.site?.code || ''" readonly />
            </div>
            <div class="field full">
              <label>启用 Profiles</label>
              <input :value="(site?.profiles || []).join(', ')" readonly />
            </div>
            <div class="field full">
              <label>Manifest 路径（可选，用于体检端口清单）</label>
              <input v-model="form.manifest" placeholder="configs/examples/manifest.release.example.yaml" />
            </div>
            <div class="field full">
              <label>Release 包目录（第 4 步部署时用，Windows 可先空着）</label>
              <input v-model="form.package" placeholder="例如 D:/packages/wpg-release-4.0.2 或 /opt/packages/..." />
            </div>
            <div class="field full">
              <label>Base 包目录（仅首次装 Docker 需要）</label>
              <input v-model="form.base" placeholder="例如 /opt/packages/wpg-base-1.0.0" />
            </div>
          </div>
          <p v-if="siteError" class="muted" style="color: var(--danger)">{{ siteError }}</p>
          <div class="actions">
            <button class="btn btn-primary" @click="nextFromSite" :disabled="!site">下一步：环境体检</button>
            <button class="btn btn-ghost" @click="loadSite">重新加载 site.yaml</button>
          </div>
        </div>

        <!-- step 1 -->
        <div v-else-if="wizardStep === 1">
          <div class="actions" style="margin-top: 0">
            <button class="btn btn-primary" @click="runPrecheck" :disabled="busy">
              {{ busy ? '体检进行中…' : '执行 Precheck' }}
            </button>
            <button class="btn btn-ghost" @click="wizardStep = 0">返回</button>
          </div>
          <div v-if="precheckItems.length" class="check-list" style="margin-top: 1.25rem">
            <div v-for="(it, i) in precheckItems" :key="i" class="check-item">
              <span class="badge" :class="it.severity">{{ it.severity }}</span>
              <div>
                <strong>{{ it.name }}</strong>
                <div class="muted">{{ it.message }}</div>
              </div>
            </div>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="precheckDone">
            <button class="btn btn-primary" @click="wizardStep = 2" :disabled="precheckBlocked">进入初始化</button>
          </div>
        </div>

        <!-- step 2 -->
        <div v-else-if="wizardStep === 2">
          <p class="muted">将创建目录、按需安装 Docker、放行端口。多机场景会 SSH 分发到各节点。</p>
          <div class="actions">
            <button class="btn btn-primary" @click="runInit" :disabled="busy">
              {{ busy ? '初始化中…' : '执行 Init' }}
            </button>
            <button class="btn btn-ghost" @click="wizardStep = 1">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div class="actions" v-if="initDone">
            <button class="btn btn-primary" @click="wizardStep = 3">进入部署</button>
          </div>
        </div>

        <!-- step 3 -->
        <div v-else-if="wizardStep === 3">
          <div class="field-grid">
            <div class="field full">
              <label>Release 包目录</label>
              <input v-model="form.package" placeholder="/path/to/wpg-release-4.0.2" />
            </div>
          </div>
          <div class="actions">
            <button class="btn btn-primary" @click="runDeploy(false)" :disabled="busy || !form.package">
              {{ busy ? '部署中…' : '开始 Deploy' }}
            </button>
            <button class="btn btn-ghost" @click="runDeploy(true)" :disabled="busy || !form.package">仅渲染（Dry-run）</button>
            <button class="btn btn-ghost" @click="wizardStep = 2">返回</button>
          </div>
          <div class="log-box" v-if="jobLogs.length">
            <div v-for="(l, i) in jobLogs" :key="i" :class="logClass(l)">{{ l }}</div>
          </div>
          <div v-if="deployDone" class="actions">
            <button class="btn btn-primary" @click="openStatus">查看运行状态</button>
          </div>
        </div>
      </div>
    </section>

    <!-- STATUS -->
    <section v-else-if="view === 'status'" class="panel">
      <div class="panel-head">
        <div>
          <h2>运行状态</h2>
          <p><span class="pulse"></span>基于 docker compose ps 实时汇总</p>
        </div>
        <button class="btn btn-ghost" @click="loadStatus">刷新</button>
      </div>
      <div class="surface">
        <p v-if="statusWarning" class="muted">{{ statusWarning }}</p>
        <table class="table" v-if="services.length">
          <thead>
            <tr>
              <th>服务</th>
              <th>状态</th>
              <th>健康</th>
              <th>镜像</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="s in services" :key="s.Name || s.Service">
              <td>{{ s.Service || s.Name }}</td>
              <td>{{ s.State || '—' }}</td>
              <td>{{ s.Health || s.Status || '—' }}</td>
              <td class="muted">{{ s.Image || '—' }}</td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">暂无服务数据。完成部署后在此查看。</p>
      </div>
    </section>

    <!-- LOGS -->
    <section v-else-if="view === 'logs'" class="panel">
      <div class="panel-head">
        <div>
          <h2>服务日志</h2>
          <p>WebSocket 拉取容器最近日志，适配远程桌面排障。</p>
        </div>
      </div>
      <div class="surface">
        <div class="field-grid">
          <div class="field">
            <label>服务 / 容器名</label>
            <input v-model="logService" placeholder="waterwork-center" />
          </div>
        </div>
        <div class="actions">
          <button class="btn btn-primary" @click="startLogs" :disabled="!logService">开始监听</button>
          <button class="btn btn-ghost" @click="stopLogs">停止</button>
        </div>
        <div class="log-box" style="max-height: 480px">
          <pre style="margin:0;white-space:pre-wrap;font:inherit">{{ liveLogs || '等待日志…' }}</pre>
        </div>
      </div>
    </section>

    <!-- HISTORY -->
    <section v-else-if="view === 'history'" class="panel">
      <div class="panel-head">
        <div>
          <h2>部署台账</h2>
          <p>本地 ~/.wpgctl/state 中的版本真相来源。</p>
        </div>
        <button class="btn btn-ghost" @click="loadHistory">刷新</button>
      </div>
      <div class="surface">
        <table class="table" v-if="deployments.length">
          <thead>
            <tr>
              <th>时间</th>
              <th>动作</th>
              <th>版本</th>
              <th>站点</th>
              <th>结果</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="d in deployments.slice().reverse()" :key="d.id">
              <td>{{ formatTime(d.time) }}</td>
              <td>{{ d.action }}</td>
              <td>{{ d.packageVersion }}</td>
              <td>{{ d.siteCode }}</td>
              <td>
                <span class="badge" :class="d.success ? 'green' : 'red'">
                  {{ d.success ? 'ok' : 'fail' }}
                </span>
              </td>
            </tr>
          </tbody>
        </table>
        <p v-else class="muted">尚无部署记录。</p>
      </div>
    </section>
  </div>
</template>

<script setup>
import { computed, onMounted, onUnmounted, reactive, ref } from 'vue'

const view = ref('home')
const wizardStep = ref(0)
const site = ref(null)
const siteError = ref('')
const busy = ref(false)
const jobLogs = ref([])
const precheckItems = ref([])
const precheckDone = ref(false)
const precheckBlocked = ref(false)
const initDone = ref(false)
const deployDone = ref(false)
const services = ref([])
const statusWarning = ref('')
const deployments = ref([])
const logService = ref('')
const liveLogs = ref('')
let logWs = null
let jobWs = null

const form = reactive({
  manifest: 'configs/examples/manifest.release.example.yaml',
  package: '',
  base: '',
})

const siteName = computed(() => site.value?.site?.name || '未加载站点')
const siteCode = computed(() => site.value?.site?.code || '—')
const siteLoaded = computed(() => !!site.value)

function stepClass(i) {
  return {
    active: wizardStep.value === i,
    done: wizardStep.value > i,
  }
}

function logClass(line) {
  if (String(line).includes('ERROR')) return 'err'
  if (String(line).includes('完成') || String(line).includes('ok')) return 'ok'
  return ''
}

async function api(path, opts = {}) {
  const res = await fetch(path, {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok && res.status !== 202) {
    throw new Error(data.error || res.statusText)
  }
  return data
}

async function loadSite() {
  siteError.value = ''
  try {
    site.value = await api('/api/site')
  } catch (e) {
    site.value = null
    siteError.value = e.message
  }
}

function goWizard() {
  view.value = 'wizard'
  wizardStep.value = 0
  loadSite()
}

function nextFromSite() {
  wizardStep.value = 1
  precheckDone.value = false
  precheckItems.value = []
  jobLogs.value = []
}

function watchJob(id) {
  return new Promise((resolve) => {
    if (jobWs) jobWs.close()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    jobWs = new WebSocket(`${proto}://${location.host}/api/ws/job?id=${id}`)
    jobWs.onmessage = (ev) => {
      const job = JSON.parse(ev.data)
      jobLogs.value = job.logs || []
      if (job.status === 'ok' || job.status === 'fail') {
        jobWs.close()
        resolve(job)
      }
    }
    jobWs.onerror = () => resolve({ status: 'fail', message: 'websocket error', result: null })
  })
}

async function runPrecheck() {
  busy.value = true
  jobLogs.value = []
  precheckDone.value = false
  try {
    const job = await api('/api/precheck', {
      method: 'POST',
      body: JSON.stringify({ manifest: form.manifest }),
    })
    const done = await watchJob(job.id)
    const rep = done.result
    precheckItems.value = rep?.items || []
    precheckBlocked.value = !!rep?.hasRed || done.status === 'fail'
    precheckDone.value = true
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function runInit() {
  busy.value = true
  jobLogs.value = []
  initDone.value = false
  try {
    const job = await api('/api/init', {
      method: 'POST',
      body: JSON.stringify({ base: form.base, manifest: form.manifest, local: true }),
    })
    const done = await watchJob(job.id)
    initDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function runDeploy(dryRun) {
  busy.value = true
  jobLogs.value = []
  deployDone.value = false
  try {
    const job = await api('/api/deploy', {
      method: 'POST',
      body: JSON.stringify({ package: form.package, dryRun }),
    })
    const done = await watchJob(job.id)
    deployDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function openStatus() {
  view.value = 'status'
  await loadStatus()
}

async function loadStatus() {
  try {
    const data = await api('/api/status')
    services.value = data.services || []
    statusWarning.value = data.warning || ''
  } catch (e) {
    statusWarning.value = e.message
    services.value = []
  }
}

async function openHistory() {
  view.value = 'history'
  await loadHistory()
}

async function loadHistory() {
  try {
    deployments.value = await api('/api/deployments')
  } catch {
    deployments.value = []
  }
}

function startLogs() {
  stopLogs()
  const proto = location.protocol === 'https:' ? 'wss' : 'ws'
  logWs = new WebSocket(`${proto}://${location.host}/api/ws/logs?service=${encodeURIComponent(logService.value)}`)
  logWs.onmessage = (ev) => {
    const data = JSON.parse(ev.data)
    if (data.error) liveLogs.value = data.error
    else liveLogs.value = data.logs || ''
  }
}

function stopLogs() {
  if (logWs) {
    logWs.close()
    logWs = null
  }
}

function formatTime(t) {
  if (!t) return '—'
  try {
    return new Date(t).toLocaleString()
  } catch {
    return t
  }
}

onMounted(loadSite)
onUnmounted(() => {
  stopLogs()
  if (jobWs) jobWs.close()
})
</script>
