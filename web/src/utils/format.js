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
 * inferWorkspaceFromMiddleware 从 middleware 根推工作簿根。
 * 兼容双层 …/middleware/middleware；对不上则回落 fallback。
 * @param {string} root middleware 根目录
 * @param {string} fallback 默认工作簿路径
 * @returns {string}
 */
export function inferWorkspaceFromMiddleware(root, fallback) {
  const norm = String(root || '').replace(/[/\\]+$/, '')
  const stripped = norm.replace(/[/\\](middleware|middle)([/\\](middleware|middle))?$/i, '')
  if (stripped && stripped !== norm) return stripped
  return fallback
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
 * logClass 按日志行内容返回样式类名。
 * @param {string} line 日志一行
 * @returns {string} err / ok / 空
 */
export function logClass(line) {
  if (String(line).includes('ERROR')) return 'err'
  if (String(line).includes('完成') || String(line).includes('ok')) return 'ok'
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
