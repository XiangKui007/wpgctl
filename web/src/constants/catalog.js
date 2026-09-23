/**
 * 向导与节点规划的静态目录：服务分组、步骤文案、默认凭据。
 * 不含响应式状态；createConsole() 只引用这里的常量。
 */

/** 多机表单展示具体服务；保存时自动转换为兼容后端的 roles。
 * id 必须与包目录名、site.yaml nodes[].services、Go moduledeploy.Modules 一致。
 * 新增可选中间件的 .env 键族在 internal/moduledeploy/envbind.go 的 envBinders 加一行。
 */
export const NODE_SERVICE_GROUPS = [
  {
    id: 'database',
    label: '数据库',
    hint: '数据存储',
    services: [
      { id: 'mysql', label: 'MySQL' },
      { id: 'pgsql', label: 'PostgreSQL' },
      { id: 'postgis', label: 'PostGIS' },
      { id: 'mongodb', label: 'MongoDB' },
    ],
  },
  {
    id: 'middleware',
    label: '中间件',
    hint: '基础组件',
    services: [
      { id: 'redis', label: 'Redis' },
      { id: 'kafka', label: 'Kafka' },
      { id: 'nacos', label: 'Nacos' },
      { id: 'minio', label: 'MinIO' },
      { id: 'influxdb', label: 'InfluxDB' },
      { id: 'emqx', label: 'EMQX' },
      { id: 'waterjob', label: 'WaterJob（XXL-JOB）' },
      { id: 'nginx', label: 'Nginx（含前端，主控机）' },
    ],
  },
  {
    id: 'platform',
    label: '平台服务',
    hint: '业务应用',
    services: [
      { id: 'public', label: '平台基础' },
      { id: 'device', label: '设备管理' },
      { id: 'alarm', label: '报警' },
      { id: 'graph', label: '组态' },
      { id: 'gis', label: 'GIS' },
      { id: 'monitor', label: '监控' },
      { id: 'report-center', label: '报表' },
      { id: 'out-work', label: 'Out Work' },
    ],
  },
  {
    id: 'standalone',
    label: '独立服务',
    hint: '独立交付包',
    services: [
      { id: 'waterwork', label: '市政水厂' },
      { id: 'intelligent-model', label: '模型服务' },
    ],
  },
]

export const ALL_SINGLE_ROLES = ['database', 'middleware', 'platform', 'waterwork', 'monitor']
export const ALL_SERVICE_IDS = NODE_SERVICE_GROUPS.flatMap((group) => group.services.map((svc) => svc.id))

/**
 * defaultNode 生成节点规划表的一行默认值。
 * @param {string} name 节点名
 * @param {string} ip 节点 IP
 * @param {string[]} [roles] 兼容后端的角色
 * @param {string[]} [services] 分配到该机的服务 id
 * @returns {object}
 */
export function defaultNode(name, ip, roles = [], services = []) {
  return {
    name,
    ip,
    sshUser: 'root',
    sshPort: 22,
    roles: roles.slice(),
    services: services.slice(),
  }
}

export const DEFAULT_CREDS = {
  nacosUser: 'nacos',
  nacosPassword: 'wpg@nice#LKsalk98',
  nacosNamespace: 'intergrate',
  mysqlUser: 'wpg',
  mysqlPassword: 'DmJme(ZFl9txW@2P',
  pgsqlUser: 'wpg',
  pgsqlPassword: 't7u!m0Wpyu7EfbVN',
  pgsqlPort: 5433,
  redisPassword: 'SJ(Qu%(kfXQBjxyT',
}

export const moduleDefs = [
  { id: 'platform', label: '平台基础' },
  { id: 'device', label: '设备管理' },
  { id: 'alarm', label: '报警' },
  { id: 'gis', label: 'GIS' },
  { id: 'monitor', label: '监控' },
  { id: 'graph', label: '组态' },
]

export const localStepDefs = [
  { key: 'site', idx: '01', title: '节点配置', purpose: '填写本机节点与业务模块。' },
  { key: 'precheck', idx: '02', title: '环境体检', purpose: '检查 Docker、端口、磁盘。红色项会阻断。' },
  { key: 'init', idx: '03', title: '初始化', purpose: '创建目录、按需安装 Docker、启动防火墙（SSH 22、控制台 9527）。' },
  { key: 'deploy', idx: '04', title: '部署启动', purpose: '按 manifest 启动全部服务。' },
]

/** Linux 现场 8 步 SOP（Tab 可回看，前进软锁定） */
export const fieldStepDefs = [
	{ key: 'site', idx: '01', title: '节点配置', purpose: '填写项目、机器、中间件连接与安装包目录。' },
	{ key: 'docker', idx: '02', title: '安装 Docker', purpose: '离线安装 Docker；启动防火墙时放行 SSH 22 与控制台 9527。' },
  { key: 'database', idx: '03', title: '数据库', purpose: 'compose 部署数据库，可选执行 .sql。' },
  { key: 'middleware', idx: '04', title: '中间件', purpose: '部署 Redis / Kafka / Nacos 等；Nacos 起来后须导入配置。' },
  { key: 'business', idx: '05', title: '平台业务', purpose: '部署 platform 服务。可先批量更新 .env。' },
  { key: 'standalone', idx: '06', title: '市政/模型', purpose: '一键或分项部署市政水厂 / 模型；未填目录可跳过。' },
  { key: 'nginx', idx: '07', title: 'Nginx', purpose: '在主控机部署 Nginx 与前端静态；容器已在跑可跳过。' },
  { key: 'verify', idx: '08', title: '验收', purpose: '检查容器与关键端口。' },
]

export const fieldModules = {
  database: [
    { name: 'mysql', label: 'MySQL', skippable: true },
    { name: 'pgsql', label: 'PostgreSQL' },
    { name: 'postgis', label: 'PostGIS' },
    { name: 'mongodb', label: 'MongoDB' },
  ],
  middleware: [
    { name: 'redis', label: 'Redis' },
    { name: 'kafka', label: 'Kafka' },
    { name: 'nacos', label: 'Nacos' },
    { name: 'minio', label: 'MinIO' },
    { name: 'influxdb', label: 'InfluxDB' },
    { name: 'emqx', label: 'EMQX' },
    { name: 'waterjob', label: 'WaterJob（XXL-JOB）' },
  ],
  business: [
    { name: 'public', label: '平台 public' },
    { name: 'device', label: '设备 device' },
    { name: 'alarm', label: '报警 alarm' },
    { name: 'graph', label: '组态 graph' },
    { name: 'gis', label: 'GIS（giscenter + gisdefault）' },
    { name: 'monitor', label: '监控 monitor' },
    { name: 'report-center', label: '报表' },
    { name: 'out-work', label: 'out-work' },
  ],
  standalone: [
    { name: 'waterwork', label: '市政水厂', pathKey: 'waterwork', hint: '浏览选择包目录' },
    { name: 'intelligent-model', label: '模型服务', pathKey: 'intelligentModel', hint: '浏览选择包目录' },
  ],
}

/** 市政水厂夹层下两套服务；.env 内容相同，部署任一都会按站点同时改写两份。 */
export const WATERWORK_SUBSERVICES = [
  { id: 'center', dirName: 'waterwork-center', label: 'waterwork-center' },
  { id: 'device', dirName: 'waterwork-device', label: 'waterwork-device' },
]

/** 服务 id → site.yaml profile（后端 validProfiles）。无映射的服务不产生 profile。 */
export const SERVICE_PROFILE = {
  public: 'platform',
  device: 'device',
  alarm: 'alarm',
  graph: 'graph',
  gis: 'gis',
  monitor: 'monitor',
  waterwork: 'waterwork',
  'intelligent-model': 'intelligent-model',
}

/** 需要持久化到 settings.json 的向导本机路径（不属于 site.yaml）。 */
export const PERSIST_FIELD_KEYS = ['middlewareRoot', 'platformRoot', 'nginxDir', 'nacosConfigZips', 'gatewayIP', 'appIP', 'graphIP']
export const PERSIST_FORM_KEYS = ['manifest', 'package', 'base', 'dockerPackage']

/**
 * emptyFieldStepDone 返回现场 8 步全部未完成的快照。
 * @returns {Record<string, boolean>}
 */
export function emptyFieldStepDone() {
  return {
    site: false,
    docker: false,
    database: false,
    middleware: false,
    business: false,
    standalone: false,
    nginx: false,
    verify: false,
  }
}
