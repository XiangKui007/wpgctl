/**
 * 控制台展示用纯函数：路径拼接、日志着色、文件图标。
 * 不读 Vue 状态；需要默认工作簿路径的函数由调用方传入 fallback。
 */

/**
 * joinPath 按根路径的分隔符拼接子路径。
 * @param {string} root 根目录
 * @param {string} name 子目录或文件名
 * @returns {string}
 */
export function joinPath(root, name) {
  if (!root) return ''
  const sep = root.includes('\\') ? '\\' : '/'
  return root.replace(/[/\\]+$/, '') + sep + name
}

/**
 * formatBytes 把字节数写成 KB/MB/GB，给解压进度条用。
 * @param {number} n 字节数
 * @returns {string}
 */
export function formatBytes(n) {
  const v = Number(n)
  if (!Number.isFinite(v) || v < 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let x = v
  let i = 0
  while (x >= 1024 && i < units.length - 1) {
    x /= 1024
    i++
  }
  const digits = i === 0 ? 0 : x >= 10 ? 1 : 2
  return `${x.toFixed(digits)} ${units[i]}`
}

/**
 * zipBaseName 取路径最后一段，用于 zip 摘要展示。
 * @param {string} p 绝对路径
 * @returns {string}
 */
export function zipBaseName(p) {
  if (!p) return ''
  const parts = String(p).replace(/\\/g, '/').split('/')
  return parts[parts.length - 1] || p
}

/**
 * pathBaseName 取路径最后一段（去掉末尾分隔符）。
 * @param {string} p 目录或文件路径
 * @returns {string}
 */
function pathBaseName(p) {
  const parts = String(p || '')
    .replace(/[/\\]+$/, '')
    .split(/[/\\]/)
    .filter(Boolean)
  return parts[parts.length - 1] || ''
}

/**
 * isWorkbookRoot 判断路径是否已经是名为 workspace 的工作簿根。
 * @param {string} p 目录路径
 * @returns {boolean}
 */
export function isWorkbookRoot(p) {
  return pathBaseName(p).toLowerCase() === 'workspace'
}

/**
 * resolveWorkbookFromParent 把「开始前」所选父目录解析成工作簿根。
 * 选 `/` → `/workspace`；路径中已有 workspace 祖先则用那一层，避免把交付包目录当工作簿。
 * 相对路径（只填了包名）回落 fallback。
 * @param {string} parent 用户选择或填写的父目录
 * @param {string} fallback 空输入或相对包名时的工作簿路径
 * @returns {string}
 */
export function resolveWorkbookFromParent(parent, fallback) {
  const raw = String(parent || '').trim()
  if (!raw) return fallback
  const sep = raw.includes('\\') ? '\\' : '/'
  const isAbs = raw.startsWith('/') || /^[A-Za-z]:/.test(raw)
  if (!isAbs) return fallback
  const norm = raw.replace(/[/\\]+$/, '')
  if (!norm || /^[A-Za-z]:$/.test(norm)) {
    return (norm ? norm + sep : sep) + 'workspace'
  }
  const parts = norm.split(/[/\\]/).filter(Boolean)
  const idx = parts.findIndex((p) => String(p).toLowerCase() === 'workspace')
  if (idx >= 0) {
    if (raw.startsWith('/')) return '/' + parts.slice(0, idx + 1).join('/')
    const drive = /^[A-Za-z]:$/.test(parts[0]) ? parts[0] : ''
    const rest = drive ? parts.slice(1, idx + 1) : parts.slice(0, idx + 1)
    return drive ? drive + sep + rest.join(sep) : rest.join(sep)
  }
  if (pathBaseName(norm).toLowerCase() === 'workspace') return norm
  return norm + sep + 'workspace'
}

/**
 * isEditableConfigFile 判断路径是否允许在对话框里改（.env / .conf）。
 * @param {string} path 文件路径
 * @returns {boolean}
 */
export function isEditableConfigFile(path) {
  const p = String(path || '').toLowerCase()
  return p.endsWith('.env') || p.endsWith('.conf')
}

/**
 * stripAnsi 去掉 docker logs 里的终端颜色码，避免日志页出现看不见的 ESC 只剩 [33m。
 * @param {string} s 原始日志
 * @returns {string} 纯文本
 */
export function stripAnsi(s) {
  return String(s || '')
    .replace(/\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)/g, '')
    .replace(/\x1b\[[0-9;:=?]*[A-Za-z]?/g, '')
    .replace(/\x1b[()][0-9A-B]/g, '')
    .replace(/\[[0-9]{1,3}(?:;[0-9]{1,3}){0,6}m/g, '')
}

/**
 * logClass 按日志行内容返回样式类名。
 * @param {string} line 日志一行
 * @returns {string} err / ok / 空
 */
export function logClass(line) {
  const s = String(line)
  if (s.includes('ERROR')) return 'err'
  if (s.includes('WARN')) return 'warn'
  if (s.includes('完成') || s.includes('成功') || s.includes('ok')) return 'ok'
  return ''
}

/**
 * formatTime 把时间戳转成本地可读字符串。
 * @param {string|number|Date} t
 * @returns {string}
 */
export function formatTime(t) {
  if (!t) return '—'
  try {
    return new Date(t).toLocaleString()
  } catch {
    return t
  }
}

/**
 * elTagType 把体检/状态色映射到 Element Plus Tag 的 type。
 * @param {boolean|string} v
 * @returns {string} success / danger / warning / info
 */
export function elTagType(v) {
  const s = String(v || '').toLowerCase()
  if (v === true || s === 'green' || s === 'ok' || s === 'success') return 'success'
  if (v === false || s === 'red' || s === 'fail' || s === 'error' || s === 'danger') return 'danger'
  if (s === 'yellow' || s === 'warn' || s === 'warning') return 'warning'
  return 'info'
}

/**
 * fsEntryIcon 返回路径选择器条目的短标签。
 * @param {object} e 文件系统条目
 * @returns {string}
 */
export function fsEntryIcon(e) {
  if (e.isDir) {
    if (e.hasManifest) return 'PKG'
    if (e.hasDockerInstall) return 'DKR'
    return 'DIR'
  }
  if (e.isArchive) return (e.archiveKind || 'zip').toUpperCase()
  return 'FILE'
}
