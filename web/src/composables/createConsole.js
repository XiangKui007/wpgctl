/**
 * 现场控制台会话工厂：状态、API 与动作。
 * App.vue 只负责壳、provide 与生命周期，视图通过 useConsole() 取这里的绑定。
 */
import { computed, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage, ElMessageBox } from 'element-plus'
import { api } from '@/api/http.js'
import {
  ALL_SERVICE_IDS,
  ALL_SINGLE_ROLES,
  DEFAULT_CREDS,
  defaultNode,
  emptyFieldStepDone,
  fieldModules,
  fieldStepDefs,
  localStepDefs,
  moduleDefs,
  NODE_SERVICE_GROUPS,
  PERSIST_FIELD_KEYS,
  PERSIST_FORM_KEYS,
  SERVICE_PROFILE,
  MIDDLEWARE_ANCHOR_IDS,
} from '@/constants/catalog.js'
import {
  elTagType,
  formatTime,
  fsEntryIcon,
  inferWorkspaceFromMiddleware as inferWorkspaceRoot,
  isEditableConfigFile,
  joinPath,
  logClass,
  zipBaseName,
} from '@/utils/format.js'
import {
  svcComposeDir,
  svcCreated,
  svcExitCode,
  svcHealth,
  svcId,
  svcImage,
  svcName,
  svcNetworks,
  svcPortChips,
  svcPorts,
  svcProject,
  svcRunning,
  svcSearchText as formatSvcSearchText,
  svcService,
  svcShortId,
  svcState,
  svcStatus,
  svcNode,
  svcNodeIP,
  svcIsLocal,
} from '@/utils/statusFormat.js'
import {
  debounce,
  loadDraft,
  loadSecrets,
  saveDraft,
  saveSecrets,
} from '@/utils/storage.js'
import { parseRoute, routePath, samePath, VIEWS } from '@/router/helpers.js'

/**
 * createConsole 组装现场控制台会话。
 * @returns {object} 交给 App.vue provide 的绑定；含 mount / unmount
 */
export function createConsole() {
  const router = useRouter()
  const route = useRoute()
  const view = ref('home')
const navActive = computed(() =>
  view.value === 'preflight' || view.value === 'wizard' ? 'wizard' : view.value,
)
const wizardStep = ref(0)
const maxReachedStep = ref(0)
const site = ref(null)
const siteYaml = ref('')
const sitePath = ref('')
const siteError = ref('')
const siteSaveMsg = ref('')
const siteEditMode = ref('form')
/** 磁盘上是否真有 site.yaml。向导「已完成」状态必须跟这份文件走，不能只靠浏览器草稿。 */
const siteFileExists = ref(false)

const deployTopology = ref('multi')
const sshCreds = reactive({ password: '', keyPath: '' })
const sshCredsReady = computed(() => !!(sshCreds.password || sshCreds.keyPath))
const sshPanelOpen = ref(false)
/** 多机分发：目标机缺目录时是否自动上传 / 是否强制覆盖。 */
const remoteSync = reactive({ syncFiles: true, forceSync: false })

/** 开始前：工作簿目录探测结果。 */
const workspaceProbe = ref(null)
const workspaceMsg = ref('')
const workspaceBusy = ref('')

/** 现场工作簿根：Linux 为 /workspace（其下 platform、middleware/middle、sz-waterwork、docker_data）；本机 Docker 为 D:/workspace。 */
function defaultWorkspacePath() {
  return isLocalDocker.value ? 'D:/workspace' : '/workspace'
}

function goDeployEntry() {
  if (isLocalDocker.value) {
    goWizard()
    return
  }
  loadSite().finally(() => {
    if (siteFileExists.value && (maxReachedStep.value > 0 || fieldStepDone.site)) {
      view.value = 'wizard'
      return
    }
    view.value = 'preflight'
    workspaceMsg.value = ''
    workspaceProbe.value = null
    if (!siteForm.paths.workspace) siteForm.paths.workspace = defaultWorkspacePath()
  })
}

/** 开始前：缺目录就建、已有就跳过，然后进入向导（不再让人在检查/创建/进入之间选择）。 */
async function initWorkspaceAndEnter() {
  if (!siteForm.paths.workspace) siteForm.paths.workspace = defaultWorkspacePath()
  const ws = (siteForm.paths.workspace || '').trim()
  if (!ws) {
    workspaceMsg.value = '请填写 workspace 路径'
    return
  }
  workspaceBusy.value = 'init'
  busy.value = true
  try {
    const res = await api('/api/workspace', {
      method: 'POST',
      body: JSON.stringify({
        workspace: ws,
        nginxHtml: (siteForm.paths.nginxHtml || '').trim() || undefined,
      }),
    })
    workspaceProbe.value = res.probe
    const n = (res.created || []).length
    workspaceMsg.value = n ? `已创建 ${n} 个目录。` : '目录已存在，跳过创建。'
    // 开始前只建目录；site.yaml 要等①填完节点/中间件后再保存，这里 PUT 会因空 IP、空密码被校验拒绝。
    applyDefaultCreds(false)
    persistDraftSoon()
    goWizard()
  } catch (e) {
    workspaceMsg.value = e.message
  } finally {
    busy.value = false
    workspaceBusy.value = ''
  }
}

/**
 * 多机模式下返回服务分配的目标机器（非主控机时才返回，主控机/单机返回 null，表示本地执行）。
 */
function remoteTargetFor(serviceId) {
  if (!isMultiNode.value) return null
  const idx = serviceAssignments[serviceId]
  if (idx == null || idx <= 0) return null
  const n = siteForm.nodes[idx]
  return n?.ip ? n : null
}

/** 组装模块部署 / nginx 请求中的 SSH 分发字段；本地执行时返回空对象。 */
function remoteFields(serviceId) {
  const n = remoteTargetFor(serviceId)
  if (!n) return {}
  return {
    node: n.name || n.ip,
    sshPassword: sshCreds.password || undefined,
    sshKeyPath: sshCreds.keyPath || undefined,
    syncFiles: remoteSync.syncFiles,
    forceSync: remoteSync.forceSync,
  }
}

function logRemoteHint(serviceId) {
  const n = remoteTargetFor(serviceId)
  if (!n) return
  jobLogs.value.push(`→ 目标机器 ${n.name} (${n.ip})，通过 SSH 分发执行`)
  if (!sshCredsReady.value) {
    siteError.value = `目标机器 ${n.name} 需要 SSH 凭据：未填写时将尝试主控机环境变量 WPGCTL_SSH_PASSWORD / WPGCTL_SSH_KEY，失败请展开「SSH 分发到从机」填写后重试`
    sshPanelOpen.value = true
  } else {
    siteError.value = ''
  }
}
const serviceAssignments = reactive(
  Object.fromEntries(ALL_SERVICE_IDS.map((service) => [service, 0])),
)

const siteForm = reactive({
  site: { name: '', code: '' },
  nodes: [defaultNode('app-node', '127.0.0.1', ALL_SINGLE_ROLES, ALL_SERVICE_IDS)],
  middleware: {
    nacos: {
      host: '127.0.0.1',
      port: 8848,
      namespace: DEFAULT_CREDS.nacosNamespace,
      username: DEFAULT_CREDS.nacosUser,
      password: DEFAULT_CREDS.nacosPassword,
    },
    mysql: {
      disabled: false,
      host: '127.0.0.1',
      port: 3306,
      user: DEFAULT_CREDS.mysqlUser,
      password: DEFAULT_CREDS.mysqlPassword,
    },
    pgsql: {
      host: '127.0.0.1',
      port: DEFAULT_CREDS.pgsqlPort,
      user: DEFAULT_CREDS.pgsqlUser,
      password: DEFAULT_CREDS.pgsqlPassword,
    },
    redis: {
      host: '127.0.0.1',
      port: 6377,
      password: DEFAULT_CREDS.redisPassword,
    },
    kafka: { host: '127.0.0.1', port: 9092 },
  },
  paths: { workspace: '', nginxHtml: '', waterwork: '', intelligentModel: '' },
})

const selectedModules = reactive({
  platform: true,
  device: false,
  alarm: false,
  gis: false,
  monitor: true,
  graph: false,
})

const passwordVisible = reactive({
  nacos: false,
  mysql: false,
  redis: false,
  pgsql: false,
})

const fieldStepDone = reactive({
  site: false,
  docker: false,
  database: false,
  middleware: false,
  business: false,
  standalone: false,
  nginx: false,
  verify: false,
})

function hasWizardProgress() {
  return (
    maxReachedStep.value > 0 ||
    nacosImportDone.value ||
    nginxPatchDone.value ||
    verifyDone.value ||
    Object.values(fieldStepDone).some(Boolean)
  )
}

/** 落盘用的向导进度。无 site.yaml 时 ②～⑧ 完成态必须写空；① 的 5 小步仍记下，避免改机器规划就被打回「项目信息」。 */
function wizardProgressSnapshot() {
  if (!siteFileExists.value) {
    return {
      wizardStep: 0,
      maxReachedStep: 0,
      siteSubStep: siteSubStep.value,
      siteSubStepReached: siteSubStepReached.value,
      fieldStepDone: emptyFieldStepDone(),
      nacosImportDone: false,
      nginxPatchDone: false,
      verifyDone: false,
    }
  }
  return {
    wizardStep: wizardStep.value,
    maxReachedStep: maxReachedStep.value,
    siteSubStep: siteSubStep.value,
    siteSubStepReached: siteSubStepReached.value,
    fieldStepDone: { ...fieldStepDone },
    nacosImportDone: nacosImportDone.value,
    nginxPatchDone: nginxPatchDone.value,
    verifyDone: verifyDone.value,
  }
}

/** 清空向导部署进度。路径草稿可留；步骤完成态不能在 site.yaml 消失后继续显示。 */
function resetWizardProgress({ announce = false } = {}) {
  const had = hasWizardProgress()
  Object.assign(fieldStepDone, emptyFieldStepDone())
  maxReachedStep.value = 0
  wizardStep.value = 0
  siteSubStep.value = 0
  siteSubStepReached.value = 0
  nacosImportDone.value = false
  nginxPatchDone.value = false
  verifyDone.value = false
  verifyReport.value = null
  initDone.value = false
  deployDone.value = false
  precheckDone.value = false
  precheckBlocked.value = false
  jobLogs.value = []
  for (const key of Object.keys(fieldModuleStatus)) delete fieldModuleStatus[key]
  if (announce && had) {
    notify('site.yaml 已不存在，向导进度已重置，请从①重新保存配置。', 'warn')
  }
}
const verifyReport = ref(null)
const verifyDone = ref(false)
const fieldPaths = reactive({
  middlewareRoot: '',
  platformRoot: '',
  nginxDir: '',
  nacosConfigZips: '', // 多行：每行一个 nacos*.zip 绝对路径
  gatewayIP: '',
  appIP: '',
  graphIP: '',
})
const fieldModuleStatus = reactive({})
const nginxPatchDone = ref(false)
const nacosImportDone = ref(false)
const standaloneModules = computed(() =>
  fieldModules.standalone.filter((m) => (siteForm.paths[m.pathKey] || '').trim()),
)

const fieldDatabaseModules = computed(() => {
  if (siteForm.middleware.mysql.disabled) {
    return fieldModules.database.filter((m) => m.name !== 'mysql')
  }
  return fieldModules.database
})

const deployableDatabaseModules = computed(() =>
  fieldDatabaseModules.value.filter((m) => isServiceEnabled(m.name)),
)

const deployableMiddlewareModules = computed(() =>
  fieldModules.middleware.filter((m) => isServiceEnabled(m.name)),
)

const busy = ref(false)
const activeJobKey = ref('')
const fileEditor = reactive({ path: '', text: '', loading: false, msg: '' })
const jobLogs = ref([])
const precheckItems = ref([])
const precheckDone = ref(false)
const precheckBlocked = ref(false)
const initDone = ref(false)
const deployDone = ref(false)
const preview = ref(null)
const smokeRows = ref([])
const services = ref([])
const statusWarning = ref('')
const statusSource = ref('')
const statusRunning = ref(0)
const statusStopped = ref(0)
const statusTotal = ref(0)
const statusBusy = ref(false)
const statusActionMsg = ref('')
const statusActionOk = ref(true)
const statusQuery = ref('')
const statusQueryInput = ref('')
const statusFilter = ref('all')
const statusNodes = ref([])
const statusNodeFilter = ref('all')
const applyStatusQuery = debounce((q) => {
  statusQuery.value = String(q || '')
}, 180)
watch(statusQueryInput, (q) => applyStatusQuery(q))
function clearStatusQuery() {
  statusQueryInput.value = ''
  statusQuery.value = ''
  applyStatusQuery.cancel()
}
const deployments = ref([])
const logService = ref('')
const liveLogs = ref('')
let logWs = null
let jobWs = null

const form = reactive({
  manifest: 'configs/examples/manifest.release.example.yaml',
  package: '',
  base: '',
  dockerPackage: '',
})

const fetchForm = reactive({
  name: '',
  from: '',
  local: '',
})
const fetchResultDir = ref('')

const scanForm = reactive({
  dir: '',
  kind: 'base',
  version: '',
})
const scanYaml = ref('')
const scanImages = ref([])
const scanError = ref('')
const scanMsg = ref('')

const hostCandidates = ref([])
const selectedHost = ref('')
const hostFillMsg = ref('')
const hostOptions = computed(() => (Array.isArray(hostCandidates.value) ? hostCandidates.value : []))

const upgradeForm = reactive({
  patchDir: '',
  rollbackTo: '',
})
const latestVersion = ref('')
const packagesList = ref([])
const baseReady = ref(false)
const packagesCount = ref(0)
const reportId = ref('')

const settings = reactive({
  operator: '',
  privacyMode: false,
  scenario: 'linux',
  advancedMode: false,
})

const busyText = computed(() => {
  if (workspaceBusy.value === 'init') return '正在初始化目录…'
  if (activeJobKey.value === 'nacos-import') return '正在导入 Nacos 配置…'
  if (activeJobKey.value) return '任务执行中，请稍候…'
  return '处理中…'
})

const picker = reactive({
  open: false,
  target: '',
  mode: 'dir',
  current: '',
  parent: '',
  entries: [],
  selected: [],
  error: '',
  expandMsg: '',
  expanding: false,
})

const nacosConfigZipList = computed(() =>
  (fieldPaths.nacosConfigZips || '')
    .split(/\n+/)
    .map((s) => s.trim())
    .filter(Boolean),
)

const nacosConfigZipSummary = computed(() => {
  const list = nacosConfigZipList.value
  if (!list.length) return ''
  if (list.length === 1) return list[0]
  return `已选 ${list.length} 个 zip：` + list.map((z) => zipBaseName(z)).join('、')
})

function setNacosConfigZips(paths) {
  const uniq = []
  const seen = new Set()
  for (const p of paths || []) {
    const t = String(p || '').trim()
    if (!t || seen.has(t)) continue
    seen.add(t)
    uniq.push(t)
  }
  fieldPaths.nacosConfigZips = uniq.join('\n')
}

function removeNacosConfigZip(index) {
  const next = [...nacosConfigZipList.value]
  next.splice(index, 1)
  setNacosConfigZips(next)
}

function clearNacosConfigZips() {
  fieldPaths.nacosConfigZips = ''
}

function togglePickerSelect(path) {
  const i = picker.selected.indexOf(path)
  if (i >= 0) picker.selected.splice(i, 1)
  else picker.selected.push(path)
}

function isPickerSelected(path) {
  return picker.selected.includes(path)
}

function confirmNacosZipPicker() {
  setNacosConfigZips(picker.selected)
  picker.open = false
}

/** 数据库步骤可选执行的 .sql（本机路径，不进 site.yaml）。 */
const sqlApplyFiles = ref([])
const sqlApplyDriver = ref('pgsql')
const sqlApplyDatabase = ref('')
const sqlApplySummary = computed(() => {
  const list = sqlApplyFiles.value
  if (!list.length) return ''
  if (list.length === 1) return list[0]
  return `已选 ${list.length} 个：` + list.map((z) => zipBaseName(z)).join('、')
})

/**
 * confirmSqlPicker 把路径选择器里勾选的 .sql 写入 sqlApplyFiles。
 * @returns {void}
 */
function confirmSqlPicker() {
  const uniq = []
  const seen = new Set()
  for (const p of picker.selected || []) {
    const t = String(p || '').trim()
    if (!t || seen.has(t)) continue
    seen.add(t)
    uniq.push(t)
  }
  sqlApplyFiles.value = uniq
  picker.open = false
}

/**
 * removeSqlApplyFile 从待执行列表去掉一项。
 * @param {number} index 下标
 * @returns {void}
 */
function removeSqlApplyFile(index) {
  const next = [...sqlApplyFiles.value]
  next.splice(index, 1)
  sqlApplyFiles.value = next
}

/** clearSqlApplyFiles 清空待执行的 .sql。 */
function clearSqlApplyFiles() {
  sqlApplyFiles.value = []
}

const runtimeOS = ref('')
const runtimeArch = ref('')
const dockerOk = ref(null)
const dockerMsg = ref('')
const deployHint = ref('')

const siteName = computed(() => {
  const p = site.value
  if (!p) return '未加载站点'
  return p.site?.name || p.Site?.Name || p.Site?.name || '未加载站点'
})
const siteCode = computed(() => {
  const p = site.value
  if (!p) return '—'
  return p.site?.code || p.Site?.Code || p.Site?.code || '—'
})
const siteLoaded = computed(() => !!site.value)
const reportUrl = computed(() => (reportId.value ? `/api/report?id=${encodeURIComponent(reportId.value)}` : '/api/report'))
const isLocalDocker = computed(() => settings.scenario === 'windows')

const nginxHtmlPath = computed(() => {
  const base = fieldPaths.nginxDir || (fieldPaths.middlewareRoot ? joinPath(fieldPaths.middlewareRoot, 'nginx') : '')
  if (!base) return ''
  return joinPath(base, 'html')
})

const nginxWebConfPath = computed(() => {
  const base = fieldPaths.nginxDir || (fieldPaths.middlewareRoot ? joinPath(fieldPaths.middlewareRoot, 'nginx') : '')
  if (!base) return 'middleware/nginx/conf/conf.d/http-web-8877.conf'
  return joinPath(joinPath(base, 'conf'), 'conf.d/http-web-8877.conf').replace(/\\/g, '/')
})
/** 首页主文案：突出快速部署、少命令、好上手，而不是审计/回滚 SOP。 */
const heroLead = computed(() =>
  isLocalDocker.value
    ? '本机 Docker 联调：按向导快速跑通部署，少配环境、少记命令。'
    : 'Linux 现场交付：跟着向导完成安装与部署，简单好用，把现场效率提上来。',
)
const modeHint = computed(() =>
  isLocalDocker.value
    ? '已开启「本机 Docker」。需要回到现场交付时，关闭顶栏或首页开关即可。'
    : '默认现场 Linux 交付。需要在本机用 Docker Desktop 联调时，打开「本机 Docker」开关。',
)
const platformHint = computed(() => {
  if (isLocalDocker.value) {
    return runtimeOS.value === 'windows'
      ? '当前机器是 Windows，适合 Docker Desktop 本机联调。'
      : '本机 Docker 联调已开启；请确保本机 Docker 可用。'
  }
  if (runtimeOS.value === 'linux') {
    return '当前是 Linux，可直接作为现场主控机执行完整交付。'
  }
  if (runtimeOS.value === 'windows') {
    return '默认按现场 Linux 交付引导。若要在本机联调，请打开「本机 Docker」。'
  }
  return '默认 Linux 现场交付；可用开关切换为本机 Docker 联调。'
})
const envChipText = computed(() => {
  const modeTag = isLocalDocker.value ? '本机' : '现场'
  const osTag = runtimeOS.value === 'windows' ? 'Windows' : runtimeOS.value ? 'Linux' : ''
  let dockerTag = ''
  if (dockerOk.value === true) dockerTag = runtimeOS.value === 'windows' ? 'Desktop 就绪' : 'Docker 就绪'
  else if (dockerOk.value === false) dockerTag = runtimeOS.value === 'windows' ? 'Desktop 未就绪' : 'Docker 未就绪'
  return [modeTag, osTag, dockerTag].filter(Boolean).join(' · ')
})

const wizardStepDefs = computed(() => (isLocalDocker.value ? localStepDefs : fieldStepDefs))

const currentStepPurpose = computed(() => wizardStepDefs.value[wizardStep.value]?.purpose || '')

const primaryNodeIP = computed(() => siteForm.nodes[0]?.ip || '127.0.0.1')

const isMultiNode = computed(
  () => !isLocalDocker.value && deployTopology.value === 'multi' && siteForm.nodes.length > 1,
)

function isStepDone(key) {
  if (key === 'site') return fieldStepDone.site
  if (isLocalDocker.value) {
    if (key === 'precheck') return precheckDone.value && !precheckBlocked.value
    if (key === 'init') return initDone.value
    if (key === 'deploy') return deployDone.value
    return false
  }
  return !!fieldStepDone[key]
}

function canGoToStep(i) {
  if (busy.value) return i === wizardStep.value
  if (i === wizardStep.value) return true
  if (i === 0) return true
  if (i <= maxReachedStep.value) return true
  if (i === wizardStep.value + 1) {
    const prev = wizardStepDefs.value[i - 1]?.key
    return prev ? isStepDone(prev) : false
  }
  return false
}

function stepTabClass(i) {
  const key = wizardStepDefs.value[i]?.key
  return {
    active: wizardStep.value === i,
    done: isStepDone(key) || wizardStep.value > i,
    locked: !canGoToStep(i),
  }
}

async function prepareSiteStep({ requireDocker = false } = {}) {
  if (siteEditMode.value === 'form') {
    await saveSiteFormOnly()
  } else {
    await api('/api/site', {
      method: 'PUT',
      body: JSON.stringify({ yaml: siteYaml.value }),
    })
  }
  if (!fieldPaths.middlewareRoot) {
    siteError.value = '请填写 middleware 根目录'
    return false
  }
  if (requireDocker && !form.dockerPackage) {
    siteError.value = '请填写 Docker 离线安装目录'
    return false
  }
  fieldStepDone.site = true
  fieldPaths.gatewayIP = fieldPaths.gatewayIP || primaryNodeIP.value
  if (!fieldPaths.nginxDir && fieldPaths.middlewareRoot) {
    fieldPaths.nginxDir = await resolveNestedModulePath(fieldPaths.middlewareRoot, 'nginx')
  }
  return true
}

/** 向导底部按钮等可信跳转：不做 Tab 锁定校验 */
function applyWizardStep(i) {
  siteError.value = ''
  wizardStep.value = i
  maxReachedStep.value = Math.max(maxReachedStep.value, i)
  if (i === 7 && !isLocalDocker.value) runVerify()
}

async function goToStep(i, opts = {}) {
  if (i === wizardStep.value) return
  if (!isLocalDocker.value && siteHydrated) {
    await refreshSitePresence()
    if (!siteFileExists.value && i > 0) {
      siteError.value = 'site.yaml 已不存在，请从①重新保存配置。'
      return
    }
  }
  if (opts.internal === true) {
    applyWizardStep(i)
    return
  }

  // 从站点配置 Tab 点前进：等同保存并解锁（不必先点底部按钮）
  if (
    i > wizardStep.value &&
    wizardStep.value === 0 &&
    !fieldStepDone.site &&
    !isLocalDocker.value
  ) {
    busy.value = true
    try {
      const ok = await prepareSiteStep()
      if (!ok) return
    } catch (e) {
      siteError.value = e.message
      return
    } finally {
      busy.value = false
    }
  }

  if (!canGoToStep(i)) {
    if (i > wizardStep.value) {
      if (busy.value) {
        siteError.value = '任务进行中，请稍后再切换步骤'
      } else {
        const prev = wizardStepDefs.value[i - 1]
        if (wizardStep.value === 0 && !fieldStepDone.site && isLocalDocker.value) {
          siteError.value = '请先点击下方「下一步：环境体检」保存站点配置'
        } else if (prev) {
          siteError.value = `请先完成「${prev.title}」后再进入（或点击该步底部「就绪 / 下一步」）`
        } else {
          siteError.value = '请先完成上一步后再进入'
        }
      }
    }
    return
  }
  applyWizardStep(i)
}

let fieldPathsHydrated = false
let fieldPathsSaveTimer = null

function collectFieldPaths() {
  const out = {}
  for (const k of PERSIST_FIELD_KEYS) if (fieldPaths[k]) out[k] = fieldPaths[k]
  for (const k of PERSIST_FORM_KEYS) if (form[k]) out['form.' + k] = form[k]
  if (deployTopology.value === 'multi') out.deployTopology = 'multi'
  return out
}

function applyFieldPaths(saved) {
  if (!saved || typeof saved !== 'object') return
  for (const k of PERSIST_FIELD_KEYS) if (saved[k]) fieldPaths[k] = saved[k]
  for (const k of PERSIST_FORM_KEYS) if (saved['form.' + k]) form[k] = saved['form.' + k]
}

async function loadSettings() {
  try {
    const st = await api('/api/settings')
    settings.operator = st.operator || ''
    settings.privacyMode = !!st.privacyMode
    settings.scenario = st.scenario === 'windows' ? 'windows' : 'linux'
    settings.advancedMode = !!st.advancedMode
    applyFieldPaths(st.fieldPaths)
    applyUISession(st.uiSession, { includeProgress: false })
  } catch {
    /* keep defaults */
  } finally {
    fieldPathsHydrated = true
  }
}

/** 路径类输入变更后延迟保存，避免每个按键都写盘。 */
function scheduleFieldPathsSave() {
  if (!fieldPathsHydrated) return
  clearTimeout(fieldPathsSaveTimer)
  fieldPathsSaveTimer = setTimeout(() => {
    saveSettings().catch(() => {})
  }, 600)
}

watch(
  () => [
    ...PERSIST_FIELD_KEYS.map((k) => fieldPaths[k]),
    ...PERSIST_FORM_KEYS.map((k) => form[k]),
  ],
  scheduleFieldPathsSave,
)

async function saveSettings() {
  try {
    await api('/api/settings', {
      method: 'PUT',
      body: JSON.stringify({
        mode: 'implementer',
        operator: settings.operator,
        privacyMode: settings.privacyMode,
        scenario: settings.scenario,
        advancedMode: settings.advancedMode,
        fieldPaths: collectFieldPaths(),
        uiSession: collectUISession(),
      }),
    })
    if (!settings.advancedMode && ['fetch', 'upgrade', 'history', 'report'].includes(view.value)) {
      view.value = 'home'
    }
  } catch (e) {
    console.warn('save settings failed', e)
  }
}

async function setScenario(scenario) {
  settings.scenario = scenario === 'windows' ? 'windows' : 'linux'
  await saveSettings()
}

async function onLocalDockerToggle(checked) {
  await setScenario(checked ? 'windows' : 'linux')
}

async function loadPackages() {
  try {
    const data = await api('/api/packages')
    packagesList.value = data.packages || []
    baseReady.value = !!data.baseReady
    packagesCount.value = data.count ?? packagesList.value.length
  } catch {
    packagesList.value = []
    baseReady.value = false
    packagesCount.value = 0
  }
}

async function loadLatest() {
  try {
    const data = await api('/api/latest')
    latestVersion.value = data.latest?.packageVersion || ''
  } catch {
    latestVersion.value = ''
  }
}

async function askConfirm(title, text, opts = {}) {
  try {
    await ElMessageBox.confirm(String(text || ''), title, {
      type: opts.danger ? 'warning' : 'info',
      confirmButtonText: opts.danger ? '确认执行' : '确认继续',
      cancelButtonText: '取消',
      distinguishCancelAndClose: true,
    })
    return true
  } catch {
    return false
  }
}

async function loadSite() {
  siteError.value = ''
  siteSaveMsg.value = ''
  try {
    const data = await api('/api/site')
    siteYaml.value = data.yaml || ''
    sitePath.value = data.path || ''
    site.value = data.parsed
    const exists = !!data.exists
    // 只在「曾经有文件、现在没了」时清进度。首次填写尚无 site.yaml 时不能清，否则改一台/多台会跳回项目信息。
    if (!exists) {
      if (siteFileExists.value) resetWizardProgress({ announce: true })
      siteFileExists.value = false
    } else {
      siteFileExists.value = true
    }
    if (exists && data.parsed) {
      fillSiteForm(data.parsed)
      fieldStepDone.site = true
      maxReachedStep.value = Math.max(maxReachedStep.value, 1)
      // 已有配置：各小步均可直接点选查看/修改
      siteSubStepReached.value = siteSubSteps.value.length - 1
    } else if (!siteHydrated) {
      applyDefaultCreds(true)
    }
    if (data.error) {
      siteError.value = data.error
    }
    if (data.message) {
      siteSaveMsg.value = data.message
    }
  } catch (e) {
    site.value = null
    const disappeared = siteFileExists.value
    siteFileExists.value = false
    siteError.value = e.message
    if (disappeared) resetWizardProgress({ announce: true })
    if (!siteHydrated) applyDefaultCreds(true)
  }
}

/** 离开页面再回来时核对 site.yaml 是否还在，避免删文件后向导仍显示旧进度。 */
async function refreshSitePresence() {
  if (!siteHydrated) return
  try {
    const data = await api('/api/site')
    const exists = !!data.exists
    if (exists === siteFileExists.value) return
    if (!exists) {
      siteFileExists.value = false
      resetWizardProgress({ announce: true })
      persistDraftSoon()
      persistSessionSoon()
      return
    }
    await loadSite()
  } catch {
    /* 探测失败不打断当前页 */
  }
}

function onSiteVisibility() {
  if (document.visibilityState === 'visible') refreshSitePresence()
}

function fillSiteForm(parsed) {
  if (!parsed) return
  // API JSON 可能是小写 yaml 风格，也可能是 Go 默认导出的 PascalCase
  const siteMeta = parsed.site || parsed.Site || {}
  const profiles = parsed.profiles || parsed.Profiles || []
  const nodes = parsed.nodes || parsed.Nodes || []
  const m = parsed.middleware || parsed.Middleware || {}
  const paths = parsed.paths || parsed.Paths || {}
  const nacos = m.nacos || m.Nacos || {}
  const mysql = m.mysql || m.MySQL || m.Mysql || {}
  const redis = m.redis || m.Redis || {}
  const kafka = m.kafka || m.Kafka || {}
  const pgsql = m.pgsql || m.PgSQL || m.Pgsql || {}

  siteForm.site.name = siteMeta.name || siteMeta.Name || ''
  siteForm.site.code = siteMeta.code || siteMeta.Code || ''
  for (const mod of moduleDefs) {
    selectedModules[mod.id] = profiles.includes(mod.id)
  }
  if (!profiles.length) {
    selectedModules.platform = true
    selectedModules.monitor = true
  }
  if (nodes.length) {
    siteForm.nodes = nodes.map((n, i) => {
      const ssh = n.ssh || n.SSH || {}
      const roles = n.roles || n.Roles || []
      const services = n.services || n.Services || []
      const row = defaultNode(
        n.name || n.Name || `node${i + 1}`,
        n.ip || n.IP || '127.0.0.1',
        roles.length ? roles : ALL_SINGLE_ROLES,
        services,
      )
      row.sshUser = ssh.user || ssh.User || 'root'
      row.sshPort = ssh.port || ssh.Port || 22
      return row
    })
    deployTopology.value = nodes.length > 1 ? 'multi' : 'single'
    hydrateServiceAssignments()
  } else {
    siteForm.nodes = [defaultNode('app-node', '127.0.0.1', ALL_SINGLE_ROLES, ALL_SERVICE_IDS)]
    deployTopology.value = 'single'
    assignAllToPrimary(false)
  }
  siteForm.middleware.nacos.host = nacos.host || nacos.Host || siteForm.middleware.nacos.host
  siteForm.middleware.nacos.port = nacos.port || nacos.Port || 8848
  siteForm.middleware.nacos.namespace =
    nacos.namespace || nacos.Namespace || siteForm.site.code || DEFAULT_CREDS.nacosNamespace
  siteForm.middleware.nacos.username = nacos.username || nacos.Username || DEFAULT_CREDS.nacosUser
  siteForm.middleware.nacos.password = resolveLoadedPassword(
    nacos.password || nacos.Password,
    DEFAULT_CREDS.nacosPassword,
  )
  siteForm.middleware.mysql.disabled = !!(mysql.disabled || mysql.Disabled)
  siteForm.middleware.mysql.host = mysql.host || mysql.Host || siteForm.middleware.mysql.host
  siteForm.middleware.mysql.port = mysql.port || mysql.Port || 3306
  siteForm.middleware.mysql.user = mysql.user || mysql.User || DEFAULT_CREDS.mysqlUser
  siteForm.middleware.mysql.password = resolveLoadedPassword(
    mysql.password || mysql.Password,
    DEFAULT_CREDS.mysqlPassword,
  )
  siteForm.middleware.pgsql.host =
    pgsql.host ||
    pgsql.Host ||
    siteForm.middleware.pgsql.host ||
    siteForm.middleware.mysql.host
  siteForm.middleware.pgsql.port = pgsql.port || pgsql.Port || DEFAULT_CREDS.pgsqlPort
  siteForm.middleware.pgsql.user = pgsql.user || pgsql.User || DEFAULT_CREDS.pgsqlUser
  siteForm.middleware.pgsql.password = resolveLoadedPassword(
    pgsql.password || pgsql.Password,
    DEFAULT_CREDS.pgsqlPassword,
  )
  siteForm.middleware.redis.host = redis.host || redis.Host || siteForm.middleware.redis.host
  siteForm.middleware.redis.port = redis.port || redis.Port || 6377
  siteForm.middleware.redis.password = resolveLoadedPassword(
    redis.password || redis.Password,
    DEFAULT_CREDS.redisPassword,
  )
  siteForm.middleware.kafka.host = kafka.host || kafka.Host || siteForm.middleware.kafka.host
  siteForm.middleware.kafka.port = kafka.port || kafka.Port || 9092
  siteForm.paths.workspace = paths.workspace || paths.Workspace || ''
  siteForm.paths.nginxHtml = paths.nginxHtml || paths.NginxHTML || paths.NginxHtml || ''
  siteForm.paths.waterwork = paths.waterwork || paths.Waterwork || ''
  siteForm.paths.intelligentModel = paths.intelligentModel || paths.IntelligentModel || ''
}

/** 脱敏回显保持空串（保存时沿用文件原值）；缺省则填交付默认密码 */
function resolveLoadedPassword(raw, fallback) {
  if (raw === '******') return ''
  if (raw) return raw
  return fallback
}

function applyDefaultCreds(force = false) {
  const put = (obj, key, val) => {
    if (force || !obj[key]) obj[key] = val
  }
  put(siteForm.middleware.nacos, 'username', DEFAULT_CREDS.nacosUser)
  put(siteForm.middleware.nacos, 'namespace', DEFAULT_CREDS.nacosNamespace)
  put(siteForm.middleware.nacos, 'password', DEFAULT_CREDS.nacosPassword)
  if (!siteForm.middleware.mysql.disabled) {
    put(siteForm.middleware.mysql, 'user', DEFAULT_CREDS.mysqlUser)
    put(siteForm.middleware.mysql, 'password', DEFAULT_CREDS.mysqlPassword)
  }
  put(siteForm.middleware.pgsql, 'user', DEFAULT_CREDS.pgsqlUser)
  put(siteForm.middleware.pgsql, 'password', DEFAULT_CREDS.pgsqlPassword)
  if (force || !siteForm.middleware.pgsql.port) {
    siteForm.middleware.pgsql.port = DEFAULT_CREDS.pgsqlPort
  }
  put(siteForm.middleware.redis, 'password', DEFAULT_CREDS.redisPassword)
  hostFillMsg.value = force
    ? '已填充公司标准账号密码'
    : '已补全空缺的默认账号密码'
}

function switchSiteMode(mode) {
  siteEditMode.value = mode
}

function currentProfiles() {
  const out = []
  if (isLocalDocker.value) {
    // 本机联调仍走 manifest 一键流程，沿用勾选框
    out.push(...moduleDefs.map((m) => m.id).filter((id) => selectedModules[id]))
  } else {
    // 现场向导：由「分配服务」推导；单机默认全部
    for (const [svc, profile] of Object.entries(SERVICE_PROFILE)) {
      if (svc === 'waterwork' || svc === 'intelligent-model') continue
      if (isServiceEnabled(svc)) out.push(profile)
    }
    if (!out.includes('platform')) out.push('platform')
  }
  if ((siteForm.paths.waterwork || '').trim() && isServiceEnabled('waterwork')) {
    out.push('waterwork')
  }
  if ((siteForm.paths.intelligentModel || '').trim() && isServiceEnabled('intelligent-model')) {
    out.push('intelligent-model')
  }
  return [...new Set(out)]
}

function buildFormNodes() {
  if (deployTopology.value === 'single' || isLocalDocker.value) {
    const n =
      siteForm.nodes[0] ||
      defaultNode('app-node', primaryNodeIP.value, ALL_SINGLE_ROLES, ALL_SERVICE_IDS)
    return [
      {
        name: n.name || 'app-node',
        ip: n.ip || primaryNodeIP.value,
        ssh: { user: n.sshUser || 'root', port: Number(n.sshPort) || 22 },
        roles: ALL_SINGLE_ROLES,
        services: ALL_SERVICE_IDS,
      },
    ]
  }
  return siteForm.nodes.map((n, i) => ({
    name: (n.name || '').trim() || `node${i + 1}`,
    ip: n.ip,
    ssh: { user: n.sshUser || 'root', port: Number(n.sshPort) || 22 },
    roles: rolesForServices(assignedServicesForNode(i)),
    services: assignedServicesForNode(i),
  }))
}

function serviceGroupFor(serviceId) {
  return NODE_SERVICE_GROUPS.find((group) =>
    group.services.some((service) => service.id === serviceId),
  )
}

function rolesForServices(services) {
  const roles = new Set()
  for (const serviceId of services) {
    const group = serviceGroupFor(serviceId)
    if (!group) continue
    if (group.id === 'database') roles.add('database')
    if (group.id === 'middleware') roles.add('middleware')
    if (group.id === 'platform') {
      roles.add('platform')
      if (serviceId === 'monitor') roles.add('monitor')
    }
    if (serviceId === 'waterwork') roles.add('waterwork')
    if (serviceId === 'intelligent-model') roles.add('intelligent-model')
  }
  return [...roles]
}

function assignedServicesForNode(nodeIndex) {
  return ALL_SERVICE_IDS.filter((serviceId) => serviceAssignments[serviceId] === nodeIndex)
}

function isServiceEnabled(serviceId) {
  return !isMultiNode.value || serviceAssignments[serviceId] !== -1
}

function hydrateServiceAssignments() {
  for (const serviceId of ALL_SERVICE_IDS) {
    let index = siteForm.nodes.findIndex((node) => node.services?.includes(serviceId))
    if (index < 0) {
      const group = serviceGroupFor(serviceId)
      const fallbackRole =
        group?.id === 'standalone'
          ? serviceId
          : group?.id
      index = siteForm.nodes.findIndex((node) => node.roles?.includes(fallbackRole))
    }
    serviceAssignments[serviceId] = index >= 0 ? index : 0
  }
  syncServiceAssignments(false)
}

function serviceNode(serviceId) {
  const index = serviceAssignments[serviceId]
  if (index == null || index < 0) return null
  return siteForm.nodes[index] || null
}

/**
 * middlewareAnchorIndex 中间件锚点机器：优先 Nacos，其次 Kafka 等。
 * Nginx 与前端静态钉在这台，不随业务服务分发到其它节点。
 * @returns {number}
 */
function middlewareAnchorIndex() {
  for (const id of MIDDLEWARE_ANCHOR_IDS) {
    const idx = serviceAssignments[id]
    if (idx != null && idx >= 0 && siteForm.nodes[idx]) return idx
  }
  return 0
}

const middlewareAnchorLabel = computed(() => {
  const n = siteForm.nodes[middlewareAnchorIndex()]
  if (!n) return '中间件所在机器（前端同机）'
  return `${n.name || '中间件机'} · ${n.ip || '未填 IP'}（前端同机）`
})

/** pinNginxToMiddleware 把 Nginx/前端固定到中间件机；「不部署」保持不变。 */
function pinNginxToMiddleware() {
  if (!isMultiNode.value) return
  if (serviceAssignments.nginx === -1) return
  serviceAssignments.nginx = middlewareAnchorIndex()
}

function syncServiceAssignments(showMessage = true) {
  pinNginxToMiddleware()
  const mysqlNode = serviceNode('mysql')
  const pgsqlNode = serviceNode('pgsql')
  const redisNode = serviceNode('redis')
  const nacosNode = serviceNode('nacos')
  const kafkaNode = serviceNode('kafka')
  if (!siteForm.middleware.mysql.disabled && mysqlNode?.ip) {
    siteForm.middleware.mysql.host = mysqlNode.ip
  }
  if (pgsqlNode?.ip) siteForm.middleware.pgsql.host = pgsqlNode.ip
  if (redisNode?.ip) siteForm.middleware.redis.host = redisNode.ip
  if (nacosNode?.ip) siteForm.middleware.nacos.host = nacosNode.ip
  if (kafkaNode?.ip) siteForm.middleware.kafka.host = kafkaNode.ip
  if (showMessage) hostFillMsg.value = '服务分配已同步到中间件连接地址'
}

function assignAllToPrimary(showMessage = true) {
  for (const serviceId of ALL_SERVICE_IDS) serviceAssignments[serviceId] = 0
  syncServiceAssignments(showMessage)
}

function setDeployTopology(mode) {
  deployTopology.value = mode
  if (mode === 'single' && siteForm.nodes.length > 1) {
    const first = siteForm.nodes[0]
    siteForm.nodes = [
      defaultNode(first.name || 'app-node', first.ip, ALL_SINGLE_ROLES, ALL_SERVICE_IDS),
    ]
    assignAllToPrimary(false)
  }
  if (mode === 'multi' && siteForm.nodes.length === 1) {
    applyDualNodeTemplate()
  }
}

/** 常用双机：第一台 db-node 跑库，第二台默认名 node2（不要再用 app-node）。 */
function applyDualNodeTemplate() {
  deployTopology.value = 'multi'
  const masterIP = siteForm.nodes[0]?.ip || primaryNodeIP.value
  siteForm.nodes = [
    defaultNode('db-node', masterIP),
    defaultNode('node2', ''),
  ]
  for (const serviceId of ALL_SERVICE_IDS) {
    serviceAssignments[serviceId] =
      serviceGroupFor(serviceId)?.id === 'database' || serviceId === 'redis' ? 0 : 1
  }
  syncServiceAssignments(false)
  hostFillMsg.value = '已套用 db-node + node2 双机模板，请修改各机 IP'
}

function addNode() {
  siteForm.nodes.push(defaultNode(`node${siteForm.nodes.length + 1}`, ''))
}

function removeNode(idx) {
  if (siteForm.nodes.length <= 1) return
  siteForm.nodes.splice(idx, 1)
  for (const serviceId of ALL_SERVICE_IDS) {
    const assigned = serviceAssignments[serviceId]
    if (assigned === idx) serviceAssignments[serviceId] = 0
    else if (assigned > idx) serviceAssignments[serviceId] = assigned - 1
  }
  syncServiceAssignments(false)
}

function moduleTargetLabel(serviceId, fallbackPhase) {
  const directIndex = serviceAssignments[serviceId]
  if (directIndex === -1) return '不部署'
  let n = siteForm.nodes[directIndex]
  if (!n && fallbackPhase) {
    const group = NODE_SERVICE_GROUPS.find((item) => item.id === fallbackPhase)
    n = serviceNode(group?.services[0]?.id)
  }
  if (!n) return '未分配角色'
  return `${n.name} (${n.ip})`
}

function buildFormPayload() {
  const profiles = currentProfiles()
  const ns = siteForm.middleware.nacos.namespace || siteForm.site.code
  return {
    site: { name: siteForm.site.name, code: siteForm.site.code },
    nodes: buildFormNodes(),
    profiles,
    middleware: {
      nacos: {
        host: siteForm.middleware.nacos.host,
        port: Number(siteForm.middleware.nacos.port) || 8848,
        namespace: ns,
        username: siteForm.middleware.nacos.username || DEFAULT_CREDS.nacosUser,
        password: siteForm.middleware.nacos.password,
      },
      mysql: siteForm.middleware.mysql.disabled
        ? { disabled: true }
        : {
            host: siteForm.middleware.mysql.host,
            port: Number(siteForm.middleware.mysql.port) || 3306,
            user: siteForm.middleware.mysql.user || DEFAULT_CREDS.mysqlUser,
            password: siteForm.middleware.mysql.password,
          },
      pgsql: {
        host:
          siteForm.middleware.pgsql.host ||
          (siteForm.middleware.mysql.disabled
            ? primaryNodeIP.value
            : siteForm.middleware.mysql.host),
        port: Number(siteForm.middleware.pgsql.port) || DEFAULT_CREDS.pgsqlPort,
        user: siteForm.middleware.pgsql.user || DEFAULT_CREDS.pgsqlUser,
        password: siteForm.middleware.pgsql.password,
      },
      redis: {
        host: siteForm.middleware.redis.host,
        port: Number(siteForm.middleware.redis.port) || 6377,
        password: siteForm.middleware.redis.password,
      },
      kafka: {
        host: siteForm.middleware.kafka.host,
        port: Number(siteForm.middleware.kafka.port) || 9092,
      },
    },
    paths: {
      workspace: siteForm.paths.workspace,
      nginxHtml: siteForm.paths.nginxHtml,
      waterwork: siteForm.paths.waterwork || undefined,
      intelligentModel: siteForm.paths.intelligentModel || undefined,
    },
  }
}

function validateNodePlan() {
  if (!isMultiNode.value) return
  const names = new Set()
  const ips = new Set()
  for (let i = 0; i < siteForm.nodes.length; i++) {
    const node = siteForm.nodes[i]
    const label = `机器 ${i + 1}`
    if (!node.name?.trim()) throw new Error(`${label}：请填写机器名称`)
    if (!node.ip?.trim()) throw new Error(`${label}：请填写 IP 地址`)
    if (!node.sshUser?.trim()) throw new Error(`${label}：请填写 SSH 用户`)
    if (names.has(node.name.trim())) throw new Error(`机器名称重复：${node.name}`)
    if (ips.has(node.ip.trim())) throw new Error(`机器 IP 重复：${node.ip}`)
    names.add(node.name.trim())
    ips.add(node.ip.trim())
  }
}

/** 自动写入 site.yaml 前先看表单是否过得了后端 Validate，避免开始前/填一半时打出 400。 */
function siteFormReadyToPersist() {
  if (!siteForm.site.name?.trim() || !siteForm.site.code?.trim()) return false
  try {
    validateNodePlan()
  } catch {
    return false
  }
  if (isLocalDocker.value || deployTopology.value === 'single') {
    if (!siteForm.nodes[0]?.ip?.trim()) return false
  }
  const mw = siteForm.middleware
  if (!(mw.nacos.password || '').trim()) return false
  if (!mw.mysql.disabled && (!(mw.mysql.user || '').trim() || !(mw.mysql.password || '').trim())) return false
  if (!(mw.pgsql.user || '').trim() || !(mw.pgsql.password || '').trim()) return false
  if (!(mw.redis.password || '').trim()) return false
  return true
}

/* ---------- 节点配置分小步（表单模式） ---------- */
const siteSubStep = ref(0)
const siteSubStepReached = ref(0)

const siteSubSteps = computed(() => {
  const local = isLocalDocker.value
  const multi = !local && deployTopology.value === 'multi'
  return [
    {
      key: 'project',
      title: '项目信息',
      desc: local
        ? '填写项目名称、编码，并勾选本次联调要启用的业务模块（对应 manifest profiles）。'
        : '填写项目名称与编码。编码会写入 site.yaml 作为本节点标识，建议使用英文或数字。',
    },
    {
      key: 'machines',
      title: local ? '本机节点' : '机器规划',
      desc: local
        ? '确认本机 Docker 的访问 IP，一般为 127.0.0.1；点击「自动获取本机 IP」可自动填写。'
        : multi
          ? '先添加参与部署的机器（第一台为当前主控机），再为每项服务选择要跑在哪台机器上；从机需填写 SSH 账号供 ② 步远程初始化。'
          : '联调 / 极小规模才用单机；真实现场请切回「多台机器（推荐）」。',
    },
    {
      key: 'middleware',
      title: '中间件连接',
      desc: multi
        ? 'Nacos / MySQL / Redis / PgSQL / Kafka 的 Host 已按上一步服务分配自动同步，账号密码默认公司标准值；一般只需核对，不一致时再修改。'
        : '核对 Nacos / MySQL / Redis / PgSQL / Kafka 的 Host 与账号密码。默认公司标准值，与标准包一致时无需修改；本版本不依赖 MySQL 可勾选跳过。',
    },
    {
      key: 'paths',
      title: '目录与安装包',
      desc: local
        ? '指定 Release 包目录 / Manifest（体检用）以及 workspace、nginxHtml 路径。'
        : '指定交付包解压后的 middleware、platform 根目录与 Docker 离线包目录，后续 ②～⑥ 步据此定位各模块；workspace 为挂载盘工作簿根（默认 /workspace），可按 middleware 根目录一键推算。',
    },
    {
      key: 'confirm',
      title: '确认保存',
      desc: '核对以上各项汇总，点「修改」可回到对应小步；确认无误后保存写入 site.yaml 并进入下一步。',
    },
  ]
})

const currentSiteSubStep = computed(() => siteSubSteps.value[siteSubStep.value] || siteSubSteps.value[0])

function assignedServiceLabels(nodeIndex) {
  const ids = assignedServicesForNode(nodeIndex)
  return NODE_SERVICE_GROUPS.flatMap((g) => g.services)
    .filter((svc) => ids.includes(svc.id))
    .map((svc) => svc.label)
}

/** 校验第 i 小步是否可离开；不通过时抛出带提示的 Error。 */
function validateSiteSubStep(i) {
  const key = siteSubSteps.value[i]?.key
  if (key === 'project') {
    if (!siteForm.site.name?.trim()) throw new Error('第 1 小步：请填写项目名称')
    if (!siteForm.site.code?.trim()) throw new Error('第 1 小步：请填写项目编码')
    return
  }
  if (key === 'machines') {
    if (isLocalDocker.value || deployTopology.value === 'single') {
      if (!siteForm.nodes[0]?.ip?.trim()) throw new Error('第 2 小步：请填写机器 IP（可点「自动获取本机 IP」）')
      return
    }
    validateNodePlan()
    const unassigned = siteForm.nodes.map((_, idx) => idx).filter((idx) => assignedServicesForNode(idx).length === 0)
    if (unassigned.length) {
      throw new Error(`第 2 小步：机器 ${unassigned.map((i) => i + 1).join('、')} 未分配任何服务，请分配或删除该机器`)
    }
    return
  }
  if (key === 'middleware') {
    const mw = siteForm.middleware
    if (!mw.nacos.host?.trim()) throw new Error('第 3 小步：请填写 Nacos Host')
    if (!mw.mysql.disabled && !mw.mysql.host?.trim()) throw new Error('第 3 小步：请填写 MySQL Host，或勾选「本版本不依赖 MySQL」')
    if (!mw.redis.host?.trim()) throw new Error('第 3 小步：请填写 Redis Host')
    if (!mw.pgsql.host?.trim()) throw new Error('第 3 小步：请填写 PgSQL Host')
    return
  }
  if (key === 'paths') {
    if (!isLocalDocker.value && !fieldPaths.middlewareRoot?.trim()) {
      throw new Error('第 4 小步：请填写 middleware 根目录')
    }
    if (!siteForm.paths.workspace?.trim()) {
      throw new Error('第 4 小步：请填写 workspace 路径（可点「按 middleware 根目录自动填充」）')
    }
  }
}

function goSiteSubStep(i) {
  if (i < 0 || i >= siteSubSteps.value.length) return
  if (i > siteSubStepReached.value) {
    // 向前跳转需依次通过中间各小步校验
    try {
      for (let k = siteSubStep.value; k < i; k++) validateSiteSubStep(k)
    } catch (e) {
      siteError.value = e.message
      return
    }
  }
  siteError.value = ''
  siteSubStep.value = i
  siteSubStepReached.value = Math.max(siteSubStepReached.value, i)
}

function nextSiteSubStep() {
  const key = siteSubSteps.value[siteSubStep.value]?.key
  if (
    key === 'paths' &&
    !isLocalDocker.value &&
    fieldPaths.middlewareRoot?.trim() &&
    (!siteForm.paths.workspace?.trim() || !siteForm.paths.nginxHtml?.trim())
  ) {
    // workspace / nginxHtml 未填时按 middleware 根目录自动推算（只补空项），减少手工操作
    applyPathsFromMiddleware().then(() => {
      try {
        validateSiteSubStep(siteSubStep.value)
      } catch (e) {
        siteError.value = e.message
        return
      }
      goSiteSubStep(siteSubStep.value + 1)
    })
    return
  }
  try {
    validateSiteSubStep(siteSubStep.value)
  } catch (e) {
    siteError.value = e.message
    return
  }
  if (siteSubSteps.value[siteSubStep.value]?.key === 'machines' && !isLocalDocker.value) {
    // 进入中间件小步前，按机器规划同步各中间件 Host
    if (deployTopology.value === 'multi') syncServiceAssignments()
    else syncSingleNodeHosts()
  }
  goSiteSubStep(siteSubStep.value + 1)
}

function prevSiteSubStep() {
  siteError.value = ''
  siteSubStep.value = Math.max(0, siteSubStep.value - 1)
}

/** 单机模式：机器 IP 变更后同步到各中间件 Host（仅覆盖为空或仍是旧值/回环的项）。 */
function syncSingleNodeHosts() {
  const ip = siteForm.nodes[0]?.ip?.trim()
  if (!ip) return
  const mw = siteForm.middleware
  for (const svc of [mw.nacos, mw.mysql, mw.redis, mw.pgsql, mw.kafka]) {
    if (!svc) continue
    const cur = (svc.host || '').trim()
    if (!cur || cur === '127.0.0.1' || cur === 'localhost' || cur === lastSyncedSingleIP.value) svc.host = ip
  }
  lastSyncedSingleIP.value = ip
}
const lastSyncedSingleIP = ref('')

async function saveSite() {
  busy.value = true
  siteError.value = ''
  siteSaveMsg.value = ''
  try {
    const useForm = siteEditMode.value === 'form'
    if (useForm) {
      validateNodePlan()
      syncServiceAssignments(false)
      await api('/api/site/form', {
        method: 'PUT',
        body: JSON.stringify(buildFormPayload()),
      })
    } else {
      await api('/api/site', {
        method: 'PUT',
        body: JSON.stringify({ yaml: siteYaml.value }),
      })
    }
    siteSaveMsg.value = '已保存并校验通过'
    await loadSite()
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

async function expandPickerDir() {
  if (!picker.current) return
  picker.expanding = true
  picker.error = ''
  picker.expandMsg = ''
  try {
    const res = await api('/api/fs/expand', {
      method: 'POST',
      body: JSON.stringify({ dir: picker.current }),
    })
    const nZip = res.zipExtracted || 0
    const nTar = res.tarUnwrapped || 0
    const nTarFiles = (res.tarFiles || []).length
    picker.expandMsg = `完成：解压 ${nZip} 个 zip，展开 ${nTar} 个 tar.zip，发现 ${nTarFiles} 个 .tar`
    await browseFS(picker.current)
  } catch (e) {
    picker.error = e.message
  } finally {
    picker.expanding = false
  }
}

async function openPicker(target, mode) {
  picker.open = true
  picker.target = target
  picker.mode = mode
  picker.error = ''
  picker.expandMsg = ''
  picker.selected =
    target === 'nacosConfigZips'
      ? [...nacosConfigZipList.value]
      : target === 'sqlApplyFiles'
        ? [...sqlApplyFiles.value]
        : []
  const start =
    (target === 'manifest' && form.manifest) ||
    (target === 'package' && form.package) ||
    (target === 'base' && form.base) ||
    (target === 'dockerPackage' && form.dockerPackage) ||
    (target === 'middlewareRoot' && fieldPaths.middlewareRoot) ||
    (target === 'platformRoot' && fieldPaths.platformRoot) ||
    (target === 'nginxDir' && fieldPaths.nginxDir) ||
    (target === 'pathsWorkspace' && siteForm.paths.workspace) ||
    (target === 'pathsNginxHtml' && siteForm.paths.nginxHtml) ||
    (target === 'waterworkDir' && siteForm.paths.waterwork) ||
    (target === 'intelligentModelDir' && siteForm.paths.intelligentModel) ||
    (target === 'nacosConfigZips' && (nacosConfigZipList.value[0] || fieldPaths.middlewareRoot)) ||
    (target === 'sqlApplyFiles' &&
      (sqlApplyFiles.value[0] || siteForm.paths.workspace || fieldPaths.middlewareRoot)) ||
    (target === 'fetchLocal' && fetchForm.local) ||
    (target === 'scanDir' && scanForm.dir) ||
    (target === 'patchDir' && upgradeForm.patchDir) ||
    ''
  await browseFS(start)
}

async function browseFS(path) {
  picker.error = ''
  try {
    const q = new URLSearchParams({ mode: picker.mode })
    if (path) q.set('path', path)
    const data = await api('/api/fs?' + q.toString())
    picker.current = data.path || ''
    picker.parent = data.parent || ''
    picker.entries = data.entries || []
  } catch (e) {
    picker.error = e.message
    if (path) {
      await browseFS('')
    }
  }
}

function confirmPicker(path) {
  if (picker.target === 'manifest') form.manifest = path
  if (picker.target === 'package') form.package = path
  if (picker.target === 'base') form.base = path
  if (picker.target === 'dockerPackage') form.dockerPackage = path
  if (picker.target === 'middlewareRoot') {
    fieldPaths.middlewareRoot = path
    applyPathsFromMiddleware()
  }
  if (picker.target === 'platformRoot') fieldPaths.platformRoot = path
  if (picker.target === 'nginxDir') fieldPaths.nginxDir = path
  if (picker.target === 'pathsWorkspace') siteForm.paths.workspace = path
  if (picker.target === 'pathsNginxHtml') siteForm.paths.nginxHtml = path
  if (picker.target === 'waterworkDir') siteForm.paths.waterwork = path
  if (picker.target === 'intelligentModelDir') siteForm.paths.intelligentModel = path
  if (picker.target === 'fetchLocal') fetchForm.local = path
  if (picker.target === 'scanDir') scanForm.dir = path
  if (picker.target === 'patchDir') upgradeForm.patchDir = path
  picker.open = false
}

function onFsClick(e) {
  if (e.isDir) {
    browseFS(e.path)
    return
  }
  if (picker.mode === 'nacos-zip' || picker.mode === 'sql') {
    togglePickerSelect(e.path)
    return
  }
  if (e.isArchive && picker.mode === 'dir') {
    expandPickerDir()
    return
  }
  confirmPicker(e.path)
}

function onFsDblClick(e) {
  if (e.isDir && picker.mode === 'dir') confirmPicker(e.path)
}

async function goWizard() {
  await loadSite()
  view.value = 'wizard'
}

async function nextFromSite() {
  busy.value = true
  try {
    if (isLocalDocker.value) {
      if (siteEditMode.value === 'form') {
        await saveSiteFormOnly()
      }
      const summary = await api('/api/confirm-summary', {
        method: 'POST',
        body: JSON.stringify({ package: form.package }),
      })
      const ok = await askConfirm('部署前确认', summary.text || '确认进入环境体检？')
      if (!ok) return
      fieldStepDone.site = true
      applyWizardStep(1)
      precheckDone.value = false
      precheckItems.value = []
    } else {
      const ok = await prepareSiteStep()
      if (!ok) return
      applyWizardStep(1)
    }
    jobLogs.value = []
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

/**
 * 解析套层包内模块目录（如 …/middleware/middleware/nginx）。
 * @param {string} root
 * @param {string} name
 * @returns {Promise<string>}
 */
async function resolveNestedModulePath(root, name) {
  if (!root || !name) return joinPath(root, name)
  try {
    const res = await api(
      '/api/module/path?root=' + encodeURIComponent(root) + '&name=' + encodeURIComponent(name),
    )
    if (res.path) return res.path
  } catch (_) {
    /* 回落到直接拼接 */
  }
  return joinPath(root, name)
}

function moduleDir(phase, name) {
  const root =
    phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  if (!root) return '（未配置根目录）'
  const direct = joinPath(root, name)
  const base = root.split(/[/\\]/).filter(Boolean).pop()
  const nested = joinPath(joinPath(root, base), name)
  return direct === nested ? direct : `${direct}（或 ${nested}）`
}

function completeFieldStep(key, nextStep) {
  if (!fieldStepDone[key === 'docker' ? 'site' : prevKey(key)] && key !== 'docker') {
    /* allow manual confirm */
  }
  if (key === 'docker' && !initDone.value) {
    siteError.value = '请先成功执行 Docker 安装'
    return
  }
  if (key === 'middleware' && isServiceEnabled('nacos') && !nacosImportDone.value) {
    siteError.value = '请先导入 Nacos 配置（部署 Nacos 后选择 nacos*.zip 点「导入配置」），否则平台/水厂服务会注册失败'
    return
  }
  siteError.value = ''
  fieldStepDone[key] = true
  jobLogs.value = []
  applyWizardStep(nextStep)
}

function prevKey(key) {
  const order = ['site', 'docker', 'database', 'middleware', 'business', 'standalone', 'nginx', 'verify']
  const i = order.indexOf(key)
  return i > 0 ? order[i - 1] : 'site'
}

async function saveSiteFormOnly() {
  const payload = buildFormPayload()
  await api('/api/site/form', { method: 'PUT', body: JSON.stringify(payload) })
}

/**
 * 按阶段解析模块目录，兼容多层 middleware / platform 套层。
 * @param {string} phase
 * @param {string} name
 * @returns {Promise<string>}
 */
async function resolveModuleDir(phase, name) {
  const root = phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  if (!root) return ''
  return resolveNestedModulePath(root, name)
}

async function loadTextFile(path, { find = false } = {}) {
  if (!path) return
  fileEditor.path = path
  fileEditor.loading = true
  fileEditor.msg = ''
  try {
    let url = '/api/fs/text?path=' + encodeURIComponent(path)
    if (find) url += '&find=1'
    const data = await api(url)
    fileEditor.path = data.path || path
    fileEditor.text = data.text || ''
    if (data.exists === false) {
      fileEditor.msg = data.hint || '文件尚不存在，保存后会新建'
    } else {
      fileEditor.msg = data.hint || `已读取 ${data.size || 0} 字节`
    }
  } catch (e) {
    fileEditor.text = ''
    fileEditor.msg = '读取失败: ' + e.message
  } finally {
    fileEditor.loading = false
  }
}

async function saveTextFile(path) {
  const p = path || fileEditor.path
  if (!p) return
  fileEditor.loading = true
  fileEditor.msg = ''
  try {
    await api('/api/fs/text?path=' + encodeURIComponent(p), {
      method: 'PUT',
      body: JSON.stringify({ text: fileEditor.text }),
    })
    fileEditor.msg = '已保存: ' + p
    notify('已保存 ' + p.replace(/^.*[\\/]/, ''), 'ok')
  } catch (e) {
    fileEditor.msg = '保存失败: ' + e.message
  } finally {
    fileEditor.loading = false
  }
}

async function openEnvEditor(phase, name) {
  const dir = await resolveModuleDir(phase, name)
  if (!dir) {
    siteError.value = '请先填写根目录'
    return
  }
  const envPath = joinPath(dir, '.env')
  await loadTextFile(envPath, { find: true })
  siteSaveMsg.value = `编辑 ${fileEditor.path || envPath}（保存后生效；部署时会按需再改 IP）`
}

async function openStandaloneEnvEditor(pathKey) {
  const dir = (siteForm.paths[pathKey] || '').trim()
  if (!dir) {
    siteError.value = '请先填写该独立包目录'
    return
  }
  await loadTextFile(joinPath(dir, '.env'), { find: true })
}

function closeFileEditor() {
  fileEditor.path = ''
  fileEditor.text = ''
  fileEditor.msg = ''
}

async function openNginxConfEditor() {
  if (!fieldPaths.nginxDir) {
    siteError.value = '请先填写 nginx 模块目录'
    return
  }
  await loadTextFile(nginxWebConfPath.value, { find: true })
}

/** Nginx 部署：使用已保存的 conf，不再自动同步 proxy_pass IP。 */
async function runNginxDeploy() {
  activeJobKey.value = 'nginx-patch'
  busy.value = true
  jobLogs.value = []
  nginxPatchDone.value = false
  logRemoteHint('nginx')
  try {
    // 多机时仍上传模块目录（含已编辑的 conf），但不做 IP 同步
    const job = await api('/api/nginx/patch', {
      method: 'POST',
      body: JSON.stringify({
        nginxDir: fieldPaths.nginxDir,
        expandHtml: true,
        composeUp: true,
        skipProxyPatch: true,
        ...remoteFields('nginx'),
        syncFiles: true,
      }),
    })
    const done = await watchJob(job.id)
    nginxPatchDone.value = done.status === 'ok'
    if (done.status === 'ok') fieldStepDone.nginx = true
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runModuleDeploy(phase, name, patchEnv, opts = {}) {
  const root =
    phase === 'business' ? fieldPaths.platformRoot : fieldPaths.middlewareRoot
  const jobKey = `${phase}-${name}`
  const quiet = !!opts.quiet
  if (!quiet) {
    activeJobKey.value = jobKey
    busy.value = true
    jobLogs.value = []
  } else {
    activeJobKey.value = jobKey
  }
  // Kafka：先把广告 IP 写回 site.yaml，部署时后端据此改 compose
  if (name === 'kafka' && !opts.skipKafkaPersist) {
    await persistKafkaAdvertiseHost()
  }
  logRemoteHint(name)
  const logPrefix = quiet ? [...jobLogs.value] : null
  try {
    const job = await api('/api/module/deploy', {
      method: 'POST',
      body: JSON.stringify({
        moduleRoot: root,
        moduleName: name,
        expand: true,
        load: true,
        patchEnv: !!patchEnv, // 数据库等无 .env 的模块传 false
        composeUp: true,
        autoNacosImport: name === 'nacos' && nacosConfigZipList.value.length > 0,
        nacosConfigZips: name === 'nacos' ? nacosConfigZipList.value : undefined,
        ...remoteFields(name),
      }),
    })
    const done = await watchJob(job.id, logPrefix ? { logPrefix } : {})
    if (done.status === 'ok') {
      fieldModuleStatus[patchEnv ? 'biz-' + name : name] = 'OK'
      if (name === 'nacos') {
        const logs = done.logs || []
        nacosImportDone.value = logs.some((l) => /Nacos 配置导入完成|已导入:/.test(String(l)))
      }
      if (quiet) jobLogs.value = [...(logPrefix || []), ...(done.logs || []), `${name} 完成`]
      if (name === 'nacos' && !nacosImportDone.value) {
        jobLogs.value.push('下一步：选择 nacos*.zip 并点「导入配置」。未导入则平台/水厂会因拉不到配置而报错。')
      }
      return true
    }
    jobLogs.value.push('ERROR: ' + done.message)
    return false
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
    return false
  } finally {
    if (!quiet) {
      busy.value = false
      activeJobKey.value = ''
    }
  }
}

/** 把 Kafka 所在机器 IP 同步进表单并写入 site.yaml，供 compose 补丁使用。 */
async function persistKafkaAdvertiseHost() {
  if (isMultiNode.value) syncServiceAssignments(false)
  else {
    const ip = siteForm.nodes[0]?.ip?.trim()
    if (ip) siteForm.middleware.kafka.host = ip
  }
  const host = (siteForm.middleware.kafka.host || '').trim()
  if (!host) return
  try {
    await saveSiteFormOnly()
    jobLogs.value.push(`已写入 Kafka 广告 IP → site.yaml middleware.kafka.host=${host}`)
  } catch (e) {
    jobLogs.value.push('WARN: 保存 site.yaml 失败，Kafka IP 可能仍用旧值: ' + e.message)
  }
}

/** 数据库步骤：依次一键部署全部可部署库（无 .env，仅 compose）。 */
async function runAllDatabaseDeploy() {
  const list = deployableDatabaseModules.value
  if (!list.length) {
    jobLogs.value = ['ERROR: 没有可部署的数据库模块']
    return
  }
  activeJobKey.value = 'database-all'
  busy.value = true
  jobLogs.value = [`开始一键部署数据库（共 ${list.length} 个，无 .env，仅 docker-compose）…`]
  let ok = 0
  try {
    for (const m of list) {
      jobLogs.value.push(`—— ${m.label} ——`)
      const success = await runModuleDeploy('database', m.name, false, { quiet: true })
      if (!success) {
        jobLogs.value.push(`ERROR: ${m.label} 失败，已停止后续库`)
        return
      }
      ok++
    }
    jobLogs.value.push(`数据库一键部署完成：${ok}/${list.length}`)
    fieldStepDone.database = true
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

/**
 * runApplySQL 把已选的本机 .sql 打到 site.yaml 里的 MySQL / PostgreSQL。
 * 可选步骤，失败不阻断进入中间件。
 */
async function runApplySQL() {
  if (!sqlApplyFiles.value.length) {
    siteError.value = '请先浏览选择至少一个 .sql 文件'
    return
  }
  siteError.value = ''
  const ok = await askConfirm(
    '执行 SQL',
    `将按顺序对 ${sqlApplyDriver.value === 'mysql' ? 'MySQL' : 'PostgreSQL'} 执行 ${sqlApplyFiles.value.length} 个文件。失败不会回滚已执行语句。确认？`,
    { danger: true },
  )
  if (!ok) return
  activeJobKey.value = 'db-apply'
  busy.value = true
  jobLogs.value = ['开始执行 SQL…']
  try {
    const job = await api('/api/db/apply', {
      method: 'POST',
      body: JSON.stringify({
        files: sqlApplyFiles.value,
        driver: sqlApplyDriver.value,
        database: (sqlApplyDatabase.value || '').trim() || undefined,
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      notify('SQL 执行完成', 'ok')
    } else {
      siteError.value = done.message || 'SQL 执行失败'
    }
  } catch (e) {
    siteError.value = e.message
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

/** 中间件步骤：依次一键部署；Kafka 由后端自动改 KAFKA_ADVERTISED_LISTENERS。 */
async function runAllMiddlewareDeploy() {
  const list = deployableMiddlewareModules.value
  if (!list.length) {
    jobLogs.value = ['ERROR: 没有可部署的中间件模块']
    return
  }
  activeJobKey.value = 'middleware-all'
  busy.value = true
  jobLogs.value = [`开始一键部署中间件（共 ${list.length} 个）…`]
  await persistKafkaAdvertiseHost()
  const host = (siteForm.middleware.kafka.host || '').trim()
  if (host) {
    jobLogs.value.push(
      `Kafka 广告地址将写入 compose: PLAINTEXT://${host}:${siteForm.middleware.kafka.port || 9092}`,
    )
  } else {
    jobLogs.value.push('WARN: 未配置 Kafka Host，将尝试从节点 services 推断')
  }
  let ok = 0
  try {
    // 建议顺序：redis → kafka → nacos → 其余
    const order = ['redis', 'kafka', 'nacos', 'minio', 'influxdb', 'emqx']
    const sorted = [...list].sort((a, b) => {
      const ia = order.indexOf(a.name)
      const ib = order.indexOf(b.name)
      return (ia < 0 ? 99 : ia) - (ib < 0 ? 99 : ib)
    })
    for (const m of sorted) {
      jobLogs.value.push(`—— ${m.label} ——`)
      const success = await runModuleDeploy('middleware', m.name, false, {
        quiet: true,
        skipKafkaPersist: true,
      })
      if (!success) {
        jobLogs.value.push(`ERROR: ${m.label} 失败，已停止后续中间件`)
        return
      }
      ok++
    }
    if (isServiceEnabled('nacos') && !nacosImportDone.value) {
      jobLogs.value.push('Nacos 已部署。请在下方选择 nacos*.zip 并点「导入配置」，完成前不要进入平台部署。')
      fieldStepDone.middleware = false
    } else {
      jobLogs.value.push(`中间件一键部署完成：${ok}/${sorted.length}`)
      fieldStepDone.middleware = true
    }
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runStandaloneDeploy(pathKey, name) {
  const moduleDir = (siteForm.paths[pathKey] || '').trim()
  if (!moduleDir) {
    jobLogs.value.push('ERROR: 未配置包目录')
    return
  }
  activeJobKey.value = 'std-' + name
  busy.value = true
  jobLogs.value = []
  logRemoteHint(name)
  try {
    const job = await api('/api/module/deploy', {
      method: 'POST',
      body: JSON.stringify({
        moduleDir,
        expand: false,
        load: false,
        patchEnv: true,
        composeUp: true,
        composeBuild: true,
        ...remoteFields(name),
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      fieldModuleStatus['std-' + name] = 'OK'
      fieldStepDone.standalone = true
    } else if (done.message && !(jobLogs.value || []).some((l) => String(l).includes(done.message))) {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function expandPlatformArchives(showLogs = true) {
  if (!fieldPaths.platformRoot) {
    siteError.value = '请先填写 platform 根目录'
    return false
  }
  if (showLogs) {
    busy.value = true
    jobLogs.value = []
  }
  try {
    const res = await api('/api/fs/expand', {
      method: 'POST',
      body: JSON.stringify({ dir: fieldPaths.platformRoot }),
    })
    const nZip = res.zipExtracted ?? 0
    const nTar = res.tarUnwrapped ?? 0
    const nFiles = (res.tarFiles || []).length
    const msg = `platform 解压完成：zip=${nZip} tar.zip=${nTar} 可用 .tar=${nFiles}`
    if (showLogs) jobLogs.value.push(msg)
    siteSaveMsg.value = msg
    return true
  } catch (e) {
    const partial = e.message || String(e)
    if (showLogs) jobLogs.value.push('WARN: ' + partial)
    siteSaveMsg.value = partial
    return false
  } finally {
    if (showLogs) busy.value = false
  }
}

async function patchAllBusinessEnv() {
  busy.value = true
  try {
    const res = await api('/api/module/patch-env', {
      method: 'POST',
      body: JSON.stringify({ root: fieldPaths.platformRoot }),
    })
    siteSaveMsg.value = `已更新 ${res.count || 0} 处 .env IP`
  } catch (e) {
    siteError.value = e.message
  } finally {
    busy.value = false
  }
}

/**
 * 从 middleware / middle 目录推出工作簿根（其父目录）。
 * 兼容双层 …/middleware/middleware；对不上则回落默认 /workspace。
 * @param {string} root
 * @returns {string}
 */
function inferWorkspaceFromMiddleware(root) {
  return inferWorkspaceRoot(root, defaultWorkspacePath())
}

async function applyPathsFromMiddleware() {
  const root = fieldPaths.middlewareRoot
  if (!root) {
    siteError.value = '请先填写 middleware 根目录'
    return
  }
  siteForm.paths.workspace = siteForm.paths.workspace || inferWorkspaceFromMiddleware(root)
  const nginxDir = await resolveNestedModulePath(root, 'nginx')
  fieldPaths.nginxDir = fieldPaths.nginxDir || nginxDir
  siteForm.paths.nginxHtml = siteForm.paths.nginxHtml || joinPath(fieldPaths.nginxDir, 'html')
  siteSaveMsg.value = '已按 middleware 目录填充：workspace 为工作簿根，nginxHtml = nginx/html'
}

async function runNacosImport() {
  const zips = nacosConfigZipList.value
  if (!zips.length) {
    openPicker('nacosConfigZips', 'nacos-zip')
    siteError.value = '请先勾选至少一个 nacos*.zip，确认后再点「导入配置」'
    return
  }
  activeJobKey.value = 'nacos-import'
  busy.value = true
  jobLogs.value = []
  nacosImportDone.value = false
  siteError.value = ''
  try {
    const job = await api('/api/nacos/import', {
      method: 'POST',
      body: JSON.stringify({ configZips: zips }),
    })
    const done = await watchJob(job.id)
    nacosImportDone.value = done.status === 'ok'
    if (done.status === 'ok') {
      fieldStepDone.middleware = true
      jobLogs.value.push('Nacos 配置已导入，可以进入 ⑤ 部署平台业务。')
    } else if (done.message && !(jobLogs.value || []).some((l) => String(l).includes(done.message))) {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runVerify() {
  activeJobKey.value = 'verify'
  busy.value = true
  jobLogs.value = []
  verifyDone.value = false
  verifyReport.value = null
  try {
    const job = await api('/api/verify', {
      method: 'POST',
      body: JSON.stringify({
        sshPassword: sshCreds.password || undefined,
        sshKeyPath: sshCreds.keyPath || undefined,
      }),
    })
    const done = await watchJob(job.id)
    const rep = done.result || null
    verifyReport.value = rep
    verifyDone.value = done.status === 'ok' || !!rep
    if (done.status === 'ok' || (rep && (rep.serviceOk > 0 || rep.portOk > 0))) {
      fieldStepDone.verify = true
    }
    if (done.status !== 'ok' && done.message) {
      jobLogs.value.push('验收结果: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

function goStatus() {
  view.value = 'status'
  loadStatus()
}

function watchJob(id, opts = {}) {
  const prefix = Array.isArray(opts.logPrefix) ? opts.logPrefix : null
  return new Promise((resolve) => {
    if (jobWs) jobWs.close()
    const proto = location.protocol === 'https:' ? 'wss' : 'ws'
    jobWs = new WebSocket(`${proto}://${location.host}/api/ws/job?id=${id}`)
    jobWs.onmessage = (ev) => {
      const job = JSON.parse(ev.data)
      const lines = job.logs || []
      jobLogs.value = prefix ? [...prefix, ...lines] : lines
      const smoke = job.result?.smoke || job.result?.Smoke
      if (Array.isArray(smoke)) {
        smokeRows.value = smoke.map((s) => ({
          name: s.name || s.Name,
          port: s.port || s.Port,
          ok: s.ok ?? s.OK,
          message: s.message || s.Message,
        }))
      }
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
  activeJobKey.value = 'init'
  busy.value = true
  jobLogs.value = []
  initDone.value = false
  try {
    const job = await api('/api/init', {
      method: 'POST',
      body: JSON.stringify({
        base: form.base,
        dockerPackage: form.dockerPackage,
        manifest: form.manifest,
        local: !isMultiNode.value,
        sshPassword: sshCreds.password || undefined,
        sshKeyPath: sshCreds.keyPath || undefined,
      }),
    })
    const done = await watchJob(job.id)
    initDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
    activeJobKey.value = ''
  }
}

async function runDeploy(dryRun) {
  busy.value = true
  jobLogs.value = []
  smokeRows.value = []
  deployDone.value = false
  try {
    const job = await api('/api/deploy', {
      method: 'POST',
      body: JSON.stringify({
        package: form.package,
        dryRun,
        operator: settings.operator,
        scenario: settings.scenario,
      }),
    })
    const done = await watchJob(job.id)
    deployDone.value = done.status === 'ok'
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await loadLatest()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function loadPreview() {
  preview.value = null
  try {
    preview.value = await api('/api/preview', {
      method: 'POST',
      body: JSON.stringify({ package: form.package }),
    })
  } catch (e) {
    jobLogs.value = ['ERROR: ' + e.message]
  }
}

async function openStatus() {
  view.value = 'status'
  await loadStatus()
}

async function loadStatus() {
  statusBusy.value = true
  try {
    const data = await api('/api/status', {
      method: 'POST',
      body: JSON.stringify({
        sshPassword: sshCreds.password || undefined,
        sshKeyPath: sshCreds.keyPath || undefined,
      }),
    })
    applyStatusPayload(data)
  } catch (e) {
    statusWarning.value = e.message
    services.value = []
    statusNodes.value = []
    statusRunning.value = 0
    statusStopped.value = 0
    statusTotal.value = 0
  } finally {
    statusBusy.value = false
  }
}

function applyStatusPayload(data) {
  services.value = data.services || []
  statusNodes.value = data.nodes || []
  statusWarning.value = data.warning || ''
  statusSource.value = data.source || ''
  statusRunning.value = data.running || 0
  statusStopped.value = data.stopped || 0
  statusTotal.value = data.total || services.value.length
  const keys = new Set(statusNodes.value.map((n) => (n.local ? 'local' : n.ip)).filter(Boolean))
  if (statusNodeFilter.value !== 'all' && !keys.has(statusNodeFilter.value)) {
    statusNodeFilter.value = 'all'
  }
}

function svcSearchText(s) {
  return formatSvcSearchText(s, settings.privacyMode)
}

const filteredServices = computed(() => {
  const q = statusQuery.value.trim().toLowerCase()
  const nodeIP = statusNodeFilter.value
  return services.value.filter((s) => {
    if (statusFilter.value === 'running' && !svcRunning(s)) return false
    if (statusFilter.value === 'stopped' && svcRunning(s)) return false
    if (nodeIP !== 'all') {
      if (nodeIP === 'local') {
        if (!svcIsLocal(s)) return false
      } else if (svcNodeIP(s) !== nodeIP) {
        return false
      }
    }
    if (!q) return true
    return svcSearchText(s).includes(q)
  })
})

async function runStatusAction(action, s) {
  const name = svcName(s)
  if (!name) return
  if (action === 'down') {
    const ok = await askConfirm('Down 容器', `将强制删除容器 ${name}（docker rm -f）。确认？`, { danger: true })
    if (!ok) return
  }
  if (action === 'stack-down') {
    const dir = svcComposeDir(s)
    const ok = await askConfirm(
      'Down 整个栈',
      `将在 ${dir || 'compose 目录'} 执行 docker compose down，停止并删除该项目全部容器。确认？`,
      { danger: true },
    )
    if (!ok) return
  }
  statusBusy.value = true
  statusActionMsg.value = ''
  try {
    const data = await api('/api/status/action', {
      method: 'POST',
      body: JSON.stringify({
        action,
        name,
        composeDir: svcComposeDir(s) || undefined,
        nodeIP: svcNodeIP(s) || undefined,
        sshPassword: sshCreds.password || undefined,
        sshKeyPath: sshCreds.keyPath || undefined,
      }),
    })
    applyStatusPayload(data)
    statusActionOk.value = true
    const labels = { start: '已启动', stop: '已停止', restart: '已重启', down: '已 Down', 'stack-down': '栈已 Down' }
    statusActionMsg.value = (labels[action] || '完成') + '：' + name
  } catch (e) {
    statusActionOk.value = false
    statusActionMsg.value = e.message
  } finally {
    statusBusy.value = false
  }
}

/**
 * openLogsFor 打开本机容器日志流。从机容器目前不能跟日志，只提示到该机查看。
 * @param {object|string} s 状态卡片或容器名
 * @returns {void}
 */
function openLogsFor(s) {
  const name = typeof s === 'string' ? s : svcName(s)
  if (!name) return
  if (s && typeof s === 'object' && !svcIsLocal(s)) {
    statusActionOk.value = false
    statusActionMsg.value = `「${name}」在 ${svcNode(s) || svcNodeIP(s)} 上，日志请到该机查看（状态页日志流目前只接本机 Docker）`
    return
  }
  logService.value = name
  view.value = 'logs'
  startLogs()
}

async function openHistory() {
  view.value = 'history'
  await loadHistory()
}

async function openFetch() {
  view.value = 'fetch'
  jobLogs.value = []
  fetchResultDir.value = ''
  await loadPackages()
}

function openReport(id) {
  reportId.value = id || ''
  view.value = 'report'
}

function exportDeliveries() {
  window.location = '/api/deliveries/export'
}

async function downloadDiag() {
  busy.value = true
  try {
    const res = await fetch('/api/diag')
    if (!res.ok) {
      const data = await res.json().catch(() => ({}))
      throw new Error(data.error || res.statusText)
    }
    const blob = await res.blob()
    const cd = res.headers.get('Content-Disposition') || ''
    const match = /filename=([^;]+)/i.exec(cd)
    const name = match ? match[1].trim().replace(/"/g, '') : 'wpgctl-diag.tar.gz'
    const url = URL.createObjectURL(blob)
    const a = document.createElement('a')
    a.href = url
    a.download = name
    a.click()
    URL.revokeObjectURL(url)
  } catch (e) {
    notify('诊断包下载失败: ' + e.message, 'warn')
  } finally {
    busy.value = false
  }
}

async function openUpgrade() {
  if (!latestVersion.value) return
  view.value = 'upgrade'
  jobLogs.value = []
  await loadLatest()
}

async function runFetch() {
  busy.value = true
  jobLogs.value = []
  fetchResultDir.value = ''
  try {
    const job = await api('/api/fetch', {
      method: 'POST',
      body: JSON.stringify({
        name: fetchForm.name,
        from: fetchForm.from,
        local: fetchForm.local,
      }),
    })
    const done = await watchJob(job.id)
    if (done.status === 'ok') {
      fetchResultDir.value = done.result?.packageDir || done.result?.PackageDir || ''
      await loadPackages()
    } else {
      jobLogs.value.push('ERROR: ' + done.message)
    }
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

function applyHostToForm(host) {
  if (!host) return
  if (siteForm.nodes[0]) siteForm.nodes[0].ip = host
  if (deployTopology.value === 'multi' && siteForm.nodes.length > 1) {
    syncServiceAssignments(false)
    return
  }
  siteForm.middleware.nacos.host = host
  if (!siteForm.middleware.mysql.disabled) {
    siteForm.middleware.mysql.host = host
  }
  siteForm.middleware.pgsql.host = host
  siteForm.middleware.redis.host = host
  siteForm.middleware.kafka.host = host
}

async function fillHostsAuto() {
  hostFillMsg.value = ''
  try {
    const info = await api('/api/hostinfo')
    const loopback = info.loopback || '127.0.0.1'
    const preferred = info.preferred || loopback
    const addrs = Array.isArray(info.addresses) ? info.addresses.slice() : []
    if (!addrs.includes(loopback)) addrs.unshift(loopback)
    if (preferred && !addrs.includes(preferred)) addrs.unshift(preferred)

    // 本机 Docker 默认 127.0.0.1；现场交付用探测到的网卡 IP
    const pick = isLocalDocker.value ? loopback : preferred
    hostCandidates.value = addrs
    selectedHost.value = pick
    applyHostToForm(pick)

    const hostHint = info.hostname ? `（${info.hostname}）` : ''
    if (isLocalDocker.value) {
      hostFillMsg.value = `已填入 ${pick}${hostHint}；如需局域网 IP 可在下拉框切换`
    } else {
      const sync = siteForm.middleware.mysql.disabled
        ? 'Nacos/PgSQL/Redis/Kafka Host'
        : 'Nacos/MySQL/PgSQL/Redis/Kafka Host'
      hostFillMsg.value = `已填入 ${pick}${hostHint}；已同步到 ${sync}`
    }
  } catch (e) {
    hostFillMsg.value = '获取失败: ' + e.message
  }
}

function applySelectedHost() {
  applyHostToForm(selectedHost.value)
  hostFillMsg.value = `已切换为 ${selectedHost.value}`
}

async function runScan(write) {
  busy.value = true
  scanError.value = ''
  scanMsg.value = ''
  try {
    const data = await api('/api/packages/scan', {
      method: 'POST',
      body: JSON.stringify({
        dir: scanForm.dir,
        kind: scanForm.kind || 'base',
        version: scanForm.version,
        write: !!write,
      }),
    })
    scanImages.value = data.images || []
    scanYaml.value = data.yaml || ''
    if (data.warnings?.length) {
      scanMsg.value = `完成，有 ${data.warnings.length} 条提示` + (write && data.manifestPath ? `；已写入 ${data.manifestPath}` : '')
    } else if (write && data.manifestPath) {
      scanMsg.value = `已写入 ${data.manifestPath}`
    } else {
      scanMsg.value = `预览完成，共 ${data.count || scanImages.value.length} 个服务`
    }
    if (write) await loadPackages()
  } catch (e) {
    scanError.value = e.message
  } finally {
    busy.value = false
  }
}

function useFetchedPackage() {
  if (fetchResultDir.value) {
    usePackagePath(fetchResultDir.value)
  }
}

function usePackagePath(path) {
  form.package = path
  goWizard()
}

async function runUpgrade() {
  const ok = await askConfirm('确认升级', `确认由 ${settings.operator || 'operator'} 执行？`)
  if (!ok) return
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/upgrade', {
      method: 'POST',
      body: JSON.stringify({ patchDir: upgradeForm.patchDir, yes: true }),
    })
    const done = await watchJob(job.id)
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await openUpgrade()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
}

async function runRollback() {
  const ok = await askConfirm('确认回滚', `确认由 ${settings.operator || 'operator'} 执行？`)
  if (!ok) return
  busy.value = true
  jobLogs.value = []
  try {
    const job = await api('/api/rollback', {
      method: 'POST',
      body: JSON.stringify({ to: upgradeForm.rollbackTo }),
    })
    const done = await watchJob(job.id)
    if (done.status !== 'ok') jobLogs.value.push('ERROR: ' + done.message)
    else await openUpgrade()
  } catch (e) {
    jobLogs.value.push('ERROR: ' + e.message)
  } finally {
    busy.value = false
  }
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

watch(wizardStep, (step, prev) => {
  if (step === 4 && prev !== 4 && !isLocalDocker.value && fieldPaths.platformRoot) {
    expandPlatformArchives(true)
  }
})

function notify(text, kind = 'info') {
  const type = kind === 'warn' ? 'warning' : kind === 'ok' ? 'success' : 'info'
  ElMessage({ message: text, type, duration: 3600 })
}

function collectUISession() {
  return {
    view: view.value,
    ...wizardProgressSnapshot(),
    deployTopology: deployTopology.value,
    logService: logService.value,
    siteEditMode: siteEditMode.value,
    sshKeyPath: sshCreds.keyPath,
    sshPanelOpen: sshPanelOpen.value,
    remoteSync: { ...remoteSync },
    reportId: reportId.value,
    fetchForm: { ...fetchForm },
    scanForm: { ...scanForm },
    upgradeForm: { ...upgradeForm },
    serviceAssignments: { ...serviceAssignments },
  }
}

function applyUISession(sess, { includeView = true, includeProgress = true } = {}) {
  if (!sess || typeof sess !== 'object') return
  if (includeView && VIEWS.includes(sess.view)) view.value = sess.view
  // ① 的小步与是否已有 site.yaml 无关，设置/草稿都要恢复，否则切一台/多台后刷新会回到项目信息。
  const sub = Number(sess.siteSubStep)
  if (Number.isInteger(sub) && sub >= 0) siteSubStep.value = sub
  const subReached = Number(sess.siteSubStepReached)
  if (Number.isInteger(subReached) && subReached >= 0) {
    siteSubStepReached.value = Math.max(siteSubStepReached.value, subReached)
  }
  if (includeProgress) {
    const step = Number(sess.wizardStep)
    if (Number.isInteger(step) && step >= 0) wizardStep.value = step
    const maxStep = Number(sess.maxReachedStep)
    if (Number.isInteger(maxStep) && maxStep >= 0) maxReachedStep.value = Math.max(maxReachedStep.value, maxStep)
    if (sess.fieldStepDone && typeof sess.fieldStepDone === 'object') {
      Object.assign(fieldStepDone, sess.fieldStepDone)
    }
    if (typeof sess.nacosImportDone === 'boolean') nacosImportDone.value = sess.nacosImportDone
    if (typeof sess.nginxPatchDone === 'boolean') nginxPatchDone.value = sess.nginxPatchDone
    if (typeof sess.verifyDone === 'boolean') verifyDone.value = sess.verifyDone
  }
  if (sess.deployTopology === 'single' || sess.deployTopology === 'multi') {
    deployTopology.value = sess.deployTopology
  }
  if (sess.logService) logService.value = sess.logService
  if (sess.siteEditMode === 'form' || sess.siteEditMode === 'yaml') siteEditMode.value = sess.siteEditMode
  if (sess.sshKeyPath) sshCreds.keyPath = sess.sshKeyPath
  if (typeof sess.sshPanelOpen === 'boolean') sshPanelOpen.value = sess.sshPanelOpen
  if (sess.remoteSync && typeof sess.remoteSync === 'object') Object.assign(remoteSync, sess.remoteSync)
  if (sess.reportId) reportId.value = sess.reportId
  if (sess.fetchForm) Object.assign(fetchForm, sess.fetchForm)
  if (sess.scanForm) Object.assign(scanForm, sess.scanForm)
  if (sess.upgradeForm) Object.assign(upgradeForm, sess.upgradeForm)
  if (sess.serviceAssignments) Object.assign(serviceAssignments, sess.serviceAssignments)
}

function collectDraftPayload() {
  return {
    siteForm: JSON.parse(JSON.stringify(siteForm)),
    fieldPaths: { ...fieldPaths },
    form: { ...form },
    fetchForm: { ...fetchForm },
    scanForm: { ...scanForm },
    upgradeForm: { ...upgradeForm },
    remoteSync: { ...remoteSync },
    serviceAssignments: { ...serviceAssignments },
    deployTopology: deployTopology.value,
    sqlApplyFiles: [...sqlApplyFiles.value],
    sqlApplyDriver: sqlApplyDriver.value,
    sqlApplyDatabase: sqlApplyDatabase.value,
    ...wizardProgressSnapshot(),
    logService: logService.value,
    siteEditMode: siteEditMode.value,
    sshPanelOpen: sshPanelOpen.value,
    reportId: reportId.value,
    siteYaml: siteEditMode.value === 'yaml' ? siteYaml.value : '',
  }
}

function applyDraft(d, { includeProgress = true } = {}) {
  if (!d || typeof d !== 'object') return
  applyUISession(d, { includeView: false, includeProgress })
  if (d.siteForm) {
    if (d.siteForm.site) Object.assign(siteForm.site, d.siteForm.site)
    if (d.siteForm.paths) Object.assign(siteForm.paths, d.siteForm.paths)
    if (d.siteForm.middleware) {
      for (const key of ['nacos', 'mysql', 'pgsql', 'redis', 'kafka']) {
        if (d.siteForm.middleware[key]) Object.assign(siteForm.middleware[key], d.siteForm.middleware[key])
      }
    }
    if (Array.isArray(d.siteForm.nodes) && d.siteForm.nodes.length) {
      siteForm.nodes.splice(0, siteForm.nodes.length, ...d.siteForm.nodes)
    }
  }
  if (d.fieldPaths) Object.assign(fieldPaths, d.fieldPaths)
  if (d.form) Object.assign(form, d.form)
  if (Array.isArray(d.sqlApplyFiles)) {
    sqlApplyFiles.value = d.sqlApplyFiles.map((p) => String(p || '').trim()).filter(Boolean)
  }
  if (d.sqlApplyDriver === 'mysql' || d.sqlApplyDriver === 'pgsql') {
    sqlApplyDriver.value = d.sqlApplyDriver
  }
  if (typeof d.sqlApplyDatabase === 'string') sqlApplyDatabase.value = d.sqlApplyDatabase
  if (d.siteYaml && siteEditMode.value === 'yaml') siteYaml.value = d.siteYaml
  pinNginxToMiddleware()
}

function persistSecrets() {
  saveSecrets({ password: sshCreds.password || '', keyPath: sshCreds.keyPath || '' })
}

function restoreSecrets() {
  const s = loadSecrets()
  if (!s) return
  if (s.password) sshCreds.password = s.password
  if (s.keyPath && !sshCreds.keyPath) sshCreds.keyPath = s.keyPath
}

let routeReady = false
let syncingFromHash = false
let siteHydrated = false
let siteFormDirty = false

function currentPath() {
  return routePath({
    view: view.value,
    wizardStep: wizardStep.value,
    siteSubStep: siteSubStep.value,
    extra: view.value === 'logs' ? logService.value : view.value === 'report' ? reportId.value : '',
  })
}

function currentHash() {
  return '#' + currentPath()
}

function syncHash({ replace = false } = {}) {
  const next = currentPath()
  if (samePath(route.path, next)) return
  if (replace) router.replace(next)
  else router.push(next)
}

function applyHashToState() {
  const parsed = parseRoute(location.hash)
  if (!parsed.view) return false
  syncingFromHash = true
  // 无 site.yaml 时禁止从 hash 进入 ②～⑧；① 的小步（#/wizard/0/1 机器规划）必须能停留。
  if (
    parsed.view === 'wizard' &&
    !siteFileExists.value &&
    !isLocalDocker.value &&
    parsed.wizardStep != null &&
    parsed.wizardStep > 0
  ) {
    view.value = 'preflight'
  } else {
    view.value = parsed.view
  }
  if (parsed.wizardStep != null) {
    if (siteFileExists.value) {
      wizardStep.value = parsed.wizardStep
      maxReachedStep.value = Math.max(maxReachedStep.value, parsed.wizardStep)
    } else {
      wizardStep.value = 0
    }
  }
  if (parsed.siteSubStep != null) {
    siteSubStep.value = parsed.siteSubStep
    siteSubStepReached.value = Math.max(siteSubStepReached.value, parsed.siteSubStep)
  }
  if (parsed.view === 'logs' && parsed.extra) logService.value = parsed.extra
  if (parsed.view === 'report' && parsed.extra) reportId.value = parsed.extra
  if (/^#\/?guide(\/|$)/i.test(location.hash)) {
    router.replace('/home')
  }
  syncingFromHash = false
  return true
}

async function hydrateRouteData(name) {
  if (name === 'status') await loadStatus()
  else if (name === 'history') await loadHistory()
  else if (name === 'fetch') await loadPackages()
  else if (name === 'upgrade') await loadLatest()
  else if (name === 'logs' && logService.value) startLogs()
}

function onRouteChange() {
  applyHashToState()
  hydrateRouteData(view.value)
}

function onNavSelect(index) {
  if (index === 'home') view.value = 'home'
  else if (index === 'wizard') goDeployEntry()
  else if (index === 'status') openStatus()
  else if (index === 'logs') view.value = 'logs'
  else if (index === 'fetch') openFetch()
  else if (index === 'upgrade') openUpgrade()
  else if (index === 'history') openHistory()
  else if (index === 'report') openReport()
}

function onGlobalKeydown(e) {
  if (e.key === 'Escape' && picker.open) picker.open = false
}

const persistDraftSoon = debounce(() => {
  if (!siteHydrated) return
  saveDraft(collectDraftPayload())
  persistSecrets()
}, 280)

const persistSessionSoon = debounce(() => {
  if (!fieldPathsHydrated) return
  saveSettings().catch(() => {})
}, 700)

const persistSiteFormSoon = debounce(async () => {
  if (!siteHydrated || !siteFormDirty || siteEditMode.value !== 'form') return
  if (!siteFormReadyToPersist()) {
    persistDraftSoon()
    return
  }
  try {
    const data = await api('/api/site')
    if (!data.exists) {
      // 首次填写本来就没有 site.yaml：只跳过自动保存，不能 reset 小步（切一台/多台会改 nodes）。
      if (siteFileExists.value) {
        siteFileExists.value = false
        resetWizardProgress({ announce: true })
      }
      persistDraftSoon()
      persistSessionSoon()
      return
    }
    await saveSiteFormOnly()
    siteFormDirty = false
    siteFileExists.value = true
    if (view.value === 'wizard' && wizardStep.value === 0) {
      siteSaveMsg.value = '已自动保存'
    }
  } catch {
    /* 表单未填完整时只留本地草稿 */
  }
}, 900)

watch(
  () => [
    view.value,
    wizardStep.value,
    siteSubStep.value,
    logService.value,
    reportId.value,
  ],
  () => {
    if (!routeReady || syncingFromHash) return
    syncHash()
    persistSessionSoon()
    persistDraftSoon()
  },
)

watch(
  () => [
    maxReachedStep.value,
    siteSubStepReached.value,
    deployTopology.value,
    siteEditMode.value,
    nacosImportDone.value,
    nginxPatchDone.value,
    verifyDone.value,
    sshCreds.password,
    sshCreds.keyPath,
    ...Object.values(fieldStepDone),
  ],
  () => {
    if (!siteHydrated) return
    persistDraftSoon()
    persistSessionSoon()
  },
)

watch(
  siteForm,
  () => {
    if (!siteHydrated) return
    siteFormDirty = true
    persistDraftSoon()
    persistSiteFormSoon()
  },
  { deep: true },
)

watch(sshCreds, persistDraftSoon, { deep: true })
watch(
  [sqlApplyFiles, sqlApplyDriver, sqlApplyDatabase],
  () => {
    if (!siteHydrated) return
    persistDraftSoon()
  },
)
watch(
  () => siteForm.middleware.mysql.disabled,
  (disabled) => {
    if (disabled && sqlApplyDriver.value === 'mysql') sqlApplyDriver.value = 'pgsql'
  },
)
watch(
  [fetchForm, scanForm, upgradeForm],
  () => {
    if (!siteHydrated) return
    persistDraftSoon()
    persistSessionSoon()
  },
  { deep: true },
)

watch(
  () => route.fullPath,
  () => {
    if (!routeReady) return
    onRouteChange()
  },
)

async function mount() {
  try {
    const h = await api('/api/health')
    runtimeOS.value = h.os || ''
    runtimeArch.value = h.arch || ''
    dockerOk.value = h.dockerOk
    dockerMsg.value = h.dockerMsg || ''
    deployHint.value = h.deployHint || ''
  } catch {
    /* ignore */
  }
  await loadSettings()
  await loadSite()
  const draft = loadDraft()
  if (draft) {
    applyDraft(draft, { includeProgress: siteFileExists.value })
    siteFormDirty = true
  }
  restoreSecrets()
  siteHydrated = true
  persistDraftSoon()
  persistSiteFormSoon()
  const fromHash = applyHashToState()
  if (!fromHash) {
    const target = currentPath()
    await router.replace('/home')
    if (!samePath(target, '/home')) {
      await router.push(target)
    }
  } else {
    syncHash({ replace: true })
  }
  await Promise.all([loadPackages(), loadLatest(), hydrateRouteData(view.value)])
  routeReady = true
  persistSessionSoon()
  window.addEventListener('keydown', onGlobalKeydown)
  document.addEventListener('visibilitychange', onSiteVisibility)
}
function unmount() {
  stopLogs()
  if (jobWs) jobWs.close()
  persistDraftSoon.flush()
  persistSessionSoon.flush()
  persistSiteFormSoon.flush()
  window.removeEventListener('keydown', onGlobalKeydown)
  document.removeEventListener('visibilitychange', onSiteVisibility)
}

return {
    mount,
    unmount,
    view,
  navActive,
  wizardStep,
  maxReachedStep,
  site,
  siteYaml,
  sitePath,
  siteError,
  siteSaveMsg,
  siteEditMode,
  NODE_SERVICE_GROUPS,
  ALL_SINGLE_ROLES,
  ALL_SERVICE_IDS,
  defaultNode,
  DEFAULT_CREDS,
  deployTopology,
  sshCreds,
  sshCredsReady,
  sshPanelOpen,
  remoteSync,
  workspaceProbe,
  workspaceMsg,
  workspaceBusy,
  defaultWorkspacePath,
  goDeployEntry,
  initWorkspaceAndEnter,
  remoteTargetFor,
  remoteFields,
  logRemoteHint,
  serviceAssignments,
  siteForm,
  moduleDefs,
  selectedModules,
  passwordVisible,
  localStepDefs,
  fieldStepDefs,
  fieldStepDone,
  verifyReport,
  verifyDone,
  fieldPaths,
  fieldModuleStatus,
  nginxPatchDone,
  nacosImportDone,
  fieldModules,
  standaloneModules,
  fieldDatabaseModules,
  deployableDatabaseModules,
  deployableMiddlewareModules,
  busy,
  activeJobKey,
  fileEditor,
  jobLogs,
  precheckItems,
  precheckDone,
  precheckBlocked,
  initDone,
  deployDone,
  preview,
  smokeRows,
  services,
  statusWarning,
  statusSource,
  statusRunning,
  statusStopped,
  statusTotal,
  statusBusy,
  statusActionMsg,
  statusActionOk,
  statusQuery,
  statusQueryInput,
  statusFilter,
  statusNodes,
  statusNodeFilter,
  applyStatusQuery,
  clearStatusQuery,
  deployments,
  logService,
  liveLogs,
  logWs,
  jobWs,
  form,
  fetchForm,
  fetchResultDir,
  scanForm,
  scanYaml,
  scanImages,
  scanError,
  scanMsg,
  hostCandidates,
  selectedHost,
  hostFillMsg,
  hostOptions,
  upgradeForm,
  latestVersion,
  packagesList,
  baseReady,
  packagesCount,
  reportId,
  settings,
  busyText,
  picker,
  nacosConfigZipList,
  nacosConfigZipSummary,
  zipBaseName,
  setNacosConfigZips,
  removeNacosConfigZip,
  clearNacosConfigZips,
  togglePickerSelect,
  isPickerSelected,
  confirmNacosZipPicker,
  sqlApplyFiles,
  sqlApplyDriver,
  sqlApplyDatabase,
  sqlApplySummary,
  confirmSqlPicker,
  removeSqlApplyFile,
  clearSqlApplyFiles,
  runtimeOS,
  runtimeArch,
  dockerOk,
  dockerMsg,
  deployHint,
  siteName,
  siteCode,
  siteLoaded,
  reportUrl,
  isLocalDocker,
  nginxHtmlPath,
  nginxWebConfPath,
  heroLead,
  modeHint,
  platformHint,
  envChipText,
  wizardStepDefs,
  currentStepPurpose,
  primaryNodeIP,
  isMultiNode,
  isStepDone,
  canGoToStep,
  stepTabClass,
  prepareSiteStep,
  applyWizardStep,
  goToStep,
  logClass,
  api,
  PERSIST_FIELD_KEYS,
  PERSIST_FORM_KEYS,
  fieldPathsHydrated,
  fieldPathsSaveTimer,
  collectFieldPaths,
  applyFieldPaths,
  loadSettings,
  scheduleFieldPathsSave,
  saveSettings,
  setScenario,
  onLocalDockerToggle,
  loadPackages,
  loadLatest,
  askConfirm,
  loadSite,
  fillSiteForm,
  resolveLoadedPassword,
  applyDefaultCreds,
  switchSiteMode,
  SERVICE_PROFILE,
  currentProfiles,
  buildFormNodes,
  serviceGroupFor,
  rolesForServices,
  assignedServicesForNode,
  isServiceEnabled,
  hydrateServiceAssignments,
  serviceNode,
  middlewareAnchorIndex,
  middlewareAnchorLabel,
  pinNginxToMiddleware,
  syncServiceAssignments,
  assignAllToPrimary,
  setDeployTopology,
  applyDualNodeTemplate,
  addNode,
  removeNode,
  moduleTargetLabel,
  buildFormPayload,
  validateNodePlan,
  siteSubStep,
  siteSubStepReached,
  siteSubSteps,
  currentSiteSubStep,
  assignedServiceLabels,
  validateSiteSubStep,
  goSiteSubStep,
  nextSiteSubStep,
  prevSiteSubStep,
  syncSingleNodeHosts,
  lastSyncedSingleIP,
  saveSite,
  fsEntryIcon,
  expandPickerDir,
  openPicker,
  browseFS,
  confirmPicker,
  onFsClick,
  onFsDblClick,
  goWizard,
  nextFromSite,
  joinPath,
  moduleDir,
  completeFieldStep,
  prevKey,
  saveSiteFormOnly,
  resolveModuleDir,
  loadTextFile,
  saveTextFile,
  openEnvEditor,
  openStandaloneEnvEditor,
  isEditableConfigFile,
  closeFileEditor,
  openNginxConfEditor,
  runNginxDeploy,
  runModuleDeploy,
  persistKafkaAdvertiseHost,
  runAllDatabaseDeploy,
  runApplySQL,
  runAllMiddlewareDeploy,
  runStandaloneDeploy,
  expandPlatformArchives,
  patchAllBusinessEnv,
  applyPathsFromMiddleware,
  runNacosImport,
  runVerify,
  goStatus,
  watchJob,
  runPrecheck,
  runInit,
  runDeploy,
  loadPreview,
  openStatus,
  loadStatus,
  applyStatusPayload,
  svcName,
  svcService,
  svcId,
  svcShortId,
  svcState,
  svcStatus,
  svcPorts,
  svcProject,
  svcImage,
  svcHealth,
  svcCreated,
  svcNetworks,
  svcExitCode,
  svcComposeDir,
  svcRunning,
  svcPortChips,
  svcSearchText,
  svcNode,
  svcNodeIP,
  svcIsLocal,
  filteredServices,
  runStatusAction,
  openLogsFor,
  openHistory,
  openFetch,
  openReport,
  exportDeliveries,
  downloadDiag,
  openUpgrade,
  runFetch,
  applyHostToForm,
  fillHostsAuto,
  applySelectedHost,
  runScan,
  useFetchedPackage,
  usePackagePath,
  runUpgrade,
  runRollback,
  loadHistory,
  startLogs,
  stopLogs,
  formatTime,
  elTagType,
  notify,
  collectUISession,
  applyUISession,
  collectDraftPayload,
  applyDraft,
  persistSecrets,
  restoreSecrets,
  routeReady,
  syncingFromHash,
  siteHydrated,
  siteFormDirty,
  currentHash,
  syncHash,
  applyHashToState,
  hydrateRouteData,
  onRouteChange,
  onNavSelect,
  onGlobalKeydown,
  persistDraftSoon,
  persistSessionSoon,
  persistSiteFormSoon,
  }
}