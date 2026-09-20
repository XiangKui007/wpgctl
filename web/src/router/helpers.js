/**
 * 路由纯函数：页面名、hash 解析、path 生成。
 * 不引用 Vue 页面，避免 createConsole 把全部视图打进同一分包。
 */

/** 顶栏与会话识别的页面名（与 hash 第一段一致）。 */
export const VIEWS = [
  'home',
  'preflight',
  'wizard',
  'status',
  'logs',
  'fetch',
  'upgrade',
  'history',
  'report',
]

/**
 * parseRoute 解析 location.hash。
 * @param {string} hash
 * @returns {{ view: string, wizardStep: number|null, siteSubStep: number|null, extra: string }}
 */
export function parseRoute(hash) {
  const raw = String(hash || '')
    .replace(/^#\/?/, '')
    .trim()
  if (!raw) {
    return { view: '', wizardStep: null, siteSubStep: null, extra: '' }
  }
  const parts = raw.split('/').filter(Boolean)
  const name = parts[0] === 'guide' ? 'home' : parts[0] || ''
  const a = parts[1] || ''
  const b = parts[2] || ''
  const view = VIEWS.includes(name) ? name : 'home'
  const out = { view, wizardStep: null, siteSubStep: null, extra: '' }
  if (name === 'wizard') {
    const step = Number(a)
    if (Number.isInteger(step) && step >= 0) out.wizardStep = step
    const sub = Number(b)
    if (Number.isInteger(sub) && sub >= 0) out.siteSubStep = sub
  } else if ((name === 'logs' || name === 'report') && a) {
    try {
      out.extra = decodeURIComponent(a)
    } catch {
      out.extra = a
    }
  }
  return out
}

/**
 * routePath 由会话状态生成 vue-router path（不含 #）。
 * @param {{ view: string, wizardStep?: number, siteSubStep?: number, extra?: string }} args
 * @returns {string}
 */
export function routePath({ view, wizardStep = 0, siteSubStep = 0, extra = '' }) {
  const name = VIEWS.includes(view) ? view : 'home'
  let path = '/' + name
  if (name === 'wizard') {
    path += '/' + Number(wizardStep || 0)
    if (Number(wizardStep || 0) === 0 && Number(siteSubStep || 0) > 0) {
      path += '/' + Number(siteSubStep)
    }
  } else if ((name === 'logs' || name === 'report') && extra) {
    path += '/' + encodeURIComponent(extra)
  }
  return path
}

/**
 * buildRoute 生成带 # 的完整 hash。
 * @param {{ view: string, wizardStep?: number, siteSubStep?: number, extra?: string }} args
 * @returns {string}
 */
export function buildRoute(args) {
  return '#' + routePath(args)
}

/**
 * sameHash 比较两个 hash，忽略前导 #/ 与末尾斜杠。
 * @param {string} a
 * @param {string} b
 * @returns {boolean}
 */
export function sameHash(a, b) {
  const norm = (h) =>
    String(h || '')
      .replace(/^#\/?/, '')
      .replace(/\/+$/, '')
  return norm(a) === norm(b)
}

/**
 * samePath 比较 vue-router path。
 * @param {string} a
 * @param {string} b
 * @returns {boolean}
 */
export function samePath(a, b) {
  const norm = (p) =>
    String(p || '')
      .replace(/\/+$/, '')
      .replace(/^\/?/, '/')
  return norm(a) === norm(b)
}
