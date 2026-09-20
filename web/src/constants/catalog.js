/**
 * 向导与节点规划的静态目录：服务分组、步骤文案、默认凭据。
 * 不含响应式状态；createConsole() 只引用这里的常量。
 */

/** 多机表单展示具体服务；保存时自动转换为兼容后端的 roles。 */
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
      { id: 'nginx', label: 'Nginx（含前端）' },
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

/** 前端静态随 Nginx 只部署在中间件机；分配时钉在下列服务所在节点（优先 Nacos）。 */
export const MIDDLEWARE_ANCHOR_IDS = ['nacos', 'kafka', 'minio', 'emqx', 'influxdb']

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
  { key: 'site', idx: '01', title: '节点配置', purpose: '填写本机联调节点信息、中间件 IP 与业务模块，保存到 site.yaml。' },
  { key: 'precheck', idx: '02', title: '环境体检', purpose: '检查 Docker、端口、磁盘等，红色项会阻断后续部署。' },
  { key: 'init', idx: '03', title: '初始化', purpose: '创建目录、安装 Docker（如需要）、放行端口。' },
  { key: 'deploy', idx: '04', title: '部署启动', purpose: '按 manifest 拉包并启动全部服务。' },
]

/** Linux 现场 8 步 SOP（Tab 可回看，前进软锁定） */
export const fieldStepDefs = [
  { key: 'site', idx: '01', title: '节点配置', purpose: '分 5 小步填写：项目信息 → 机器规划（单机 / 多机、服务分配）→ 中间件连接 → 目录与安装包 → 确认保存；保存后写入 site.yaml。' },
  { key: 'docker', idx: '02', title: '安装 Docker', purpose: '离线安装 Docker；docker.service 的 --graph 指向 /workspace/docker_data/docker/lib（与工作簿同盘）。安装成功会输出 docker -v。多机时同时 SSH 到各从机上传离线包并安装。' },
  { key: 'database', idx: '03', title: '数据库', purpose: 'MySQL / PostgreSQL / PostGIS / MongoDB：无 .env，仅 docker-compose。一键完成解压 → load → compose up；多机时按分配自动 SSH 分发。部署后可选用本机 .sql 对 MySQL 或 PostgreSQL 执行（非必做）。' },
  { key: 'middleware', idx: '04', title: '中间件', purpose: '先部署 Redis / Kafka / Nacos。Nacos 起来后必须导入 nacos*.zip 配置，否则后续平台、市政水厂会拉不到配置报错。' },
  { key: 'business', idx: '05', title: '平台业务', purpose: 'platform 各服务：可编辑 .env 后部署（展开 tar.zip → load → compose up）。' },
  { key: 'standalone', idx: '06', title: '市政/模型', purpose: '独立交付包：市政水厂目录下 waterwork-center 与 waterwork-device 都会部署。PgSQL 版 .env 仍用 MYSQL_* 键名，部署时写入 PostgreSQL 的 IP/端口/账号/密码。先 load java8.tar，再 compose up --build。' },
  { key: 'nginx', idx: '07', title: 'Nginx', purpose: '前端静态只部署到中间件所在机器，由 Nginx 转发；先编辑并保存 http-web-8877.conf，再部署（load / 解压 html / compose up）。' },
  { key: 'verify', idx: '08', title: '验收', purpose: '检查已部署容器是否运行，并探测 MySQL / Redis / Nacos / Kafka / Nginx 等关键端口是否可连通。' },
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
  ],
  business: [
    { name: 'public', label: '平台 public' },
    { name: 'device', label: '设备 device' },
    { name: 'alarm', label: '报警 alarm' },
    { name: 'graph', label: '组态 graph' },
    { name: 'gis', label: 'GIS' },
    { name: 'monitor', label: '监控 monitor' },
    { name: 'report-center', label: '报表' },
    { name: 'out-work', label: 'out-work' },
  ],
  standalone: [
    { name: 'waterwork', label: '市政水厂', pathKey: 'waterwork', hint: 'waterwork-4.1.1' },
    { name: 'intelligent-model', label: '模型服务', pathKey: 'intelligentModel', hint: 'wpg-intelligent-model-4.1.2' },
  ],
}

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
