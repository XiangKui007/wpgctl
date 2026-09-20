/**
 * 状态页容器字段归一：兼容后端大小写不一的 JSON。
 * 不读 Vue 状态；搜索文案需要隐私模式时由调用方传入。
 */

/**
 * svcName 返回容器名。
 * @param {object} s 状态接口里的单条服务
 * @returns {string}
 */
export function svcName(s) {
  return s.Name || s.name || s.Service || s.service || ''
}

/**
 * svcService 返回 Compose 服务名。
 * @param {object} s
 * @returns {string}
 */
export function svcService(s) {
  return s.Service || s.service || s.Name || s.name || '—'
}

/**
 * svcId 返回容器 ID 或名称。
 * @param {object} s
 * @returns {string}
 */
export function svcId(s) {
  return s.ID || s.id || svcName(s)
}

/**
 * svcShortId 返回容器 ID 短号（12 位），方便对照 docker inspect / logs。
 * @param {object} s 状态接口里的单条服务
 * @returns {string}
 */
export function svcShortId(s) {
  const id = String(s.ID || s.id || '').trim()
  if (!id) return ''
  return id.length > 12 ? id.slice(0, 12) : id
}

/**
 * svcState 返回容器状态字（running / exited）。
 * @param {object} s
 * @returns {string}
 */
export function svcState(s) {
  return s.State || s.state || '—'
}

/**
 * svcHealth 返回 docker 健康检查结果。
 * @param {object} s 状态接口里的单条服务
 * @returns {string} healthy / unhealthy 等；没有健康检查时为空字符串
 */
export function svcHealth(s) {
  return s.Health || s.health || ''
}

/**
 * svcStatus 返回状态说明；健康信息单独走 svcHealth，这里去掉重复括号。
 * @param {object} s
 * @returns {string}
 */
export function svcStatus(s) {
  let raw = String(s.Status || s.status || '')
  if (!raw) raw = String(s.Health || s.health || '')
  const health = svcHealth(s)
  if (health) {
    raw = raw.replace(/\s*\((?:healthy|unhealthy|health:\s*starting)\)/gi, '').trim()
  }
  return raw
}

/**
 * svcPorts 返回端口映射原文。
 * @param {object} s
 * @returns {string}
 */
export function svcPorts(s) {
  return s.Ports || s.ports || ''
}

/**
 * svcProject 返回 Compose 项目名。
 * @param {object} s
 * @returns {string}
 */
export function svcProject(s) {
  return s.Project || s.project || ''
}

/**
 * svcImage 返回镜像名。
 * @param {object} s
 * @returns {string}
 */
export function svcImage(s) {
  return s.Image || s.image || ''
}

/**
 * svcCreated 返回容器创建时间（已去掉时区后缀）。
 * @param {object} s 状态接口里的单条服务
 * @returns {string}
 */
export function svcCreated(s) {
  const raw = String(s.Created || s.created || s.CreatedAt || '').trim()
  if (!raw) return ''
  const m = raw.match(/^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}(?::\d{2})?)/)
  return m ? m[1].replace('T', ' ') : raw
}

/**
 * svcNetworks 返回容器加入的网络名。
 * @param {object} s 状态接口里的单条服务
 * @returns {string}
 */
export function svcNetworks(s) {
  return String(s.Networks || s.networks || '').trim()
}

/**
 * svcRunning 判断容器是否在跑。
 * @param {object} s
 * @returns {boolean}
 */
export function svcRunning(s) {
  const st = String(svcState(s)).toLowerCase()
  if (st === 'running' || st === 'up') return true
  return /^up\b/i.test(String(svcStatus(s)))
}

/**
 * svcExitCode 非运行中才返回退出码；running 时为 null。
 * @param {object} s 状态接口里的单条服务
 * @returns {number|null}
 */
export function svcExitCode(s) {
  if (svcRunning(s)) return null
  if (s.ExitCode === 0 || s.ExitCode === '0') return 0
  if (s.ExitCode == null || s.ExitCode === '') return null
  const n = Number(s.ExitCode)
  return Number.isFinite(n) ? n : null
}

/**
 * svcComposeDir 返回 compose 目录。
 * @param {object} s
 * @returns {string}
 */
export function svcComposeDir(s) {
  return s.ComposeDir || s.composeDir || ''
}

/**
 * svcNode 返回容器所在节点名。
 * @param {object} s
 * @returns {string}
 */
export function svcNode(s) {
  return String(s.Node || s.node || '').trim()
}

/**
 * svcNodeIP 返回容器所在节点 IP。
 * @param {object} s
 * @returns {string}
 */
export function svcNodeIP(s) {
  return String(s.NodeIP || s.nodeIP || s.nodeIp || '').trim()
}

/**
 * svcIsLocal 判断容器是否在本机 Docker。
 * @param {object} s
 * @returns {boolean}
 */
export function svcIsLocal(s) {
  if (s.Local === true || s.local === true) return true
  if (s.Local === false || s.local === false) return false
  return !svcNodeIP(s)
}

/**
 * svcPortChips 从端口原文抽出卡片展示用的短标签。
 * @param {object} s
 * @returns {{ visible: string[], more: number }}
 */
export function svcPortChips(s) {
  const raw = String(svcPorts(s) || '')
  const seen = new Set()
  const ports = []
  const re = /(?:0\.0\.0\.0|127\.\d+\.\d+\.\d+|\[::\]):(\d+)(?:->(\d+(?:-\d+)?)\/(\w+))?/g
  let m
  while ((m = re.exec(raw))) {
    const host = m[1]
    const dest = m[2] || host
    const proto = m[3] || 'tcp'
    const label = dest === host ? `${host}/${proto}` : `${host}→${dest}`
    if (seen.has(host)) continue
    seen.add(host)
    ports.push(label)
  }
  const max = 6
  return {
    visible: ports.slice(0, max),
    more: Math.max(0, ports.length - max),
  }
}

/**
 * svcSearchText 拼出状态页过滤用的小写全文。
 * @param {object} s
 * @param {boolean} [privacyMode] 开隐私时不搜镜像和 compose 目录
 * @returns {string}
 */
export function svcSearchText(s, privacyMode) {
  return [
    svcService(s),
    svcName(s),
    svcShortId(s),
    svcProject(s),
    svcPorts(s),
    svcNetworks(s),
    privacyMode ? '' : svcImage(s),
    privacyMode ? '' : svcComposeDir(s),
    svcState(s),
    svcStatus(s),
    svcCreated(s),
    svcNode(s),
    svcNodeIP(s),
  ].join(' ').toLowerCase()
}
