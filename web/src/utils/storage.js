/**
 * 现场控制台会话存储：本地草稿、SSH 密钥（仅 sessionStorage）、输入防抖。
 * 路由解析见 router/index.js。
 */

const DRAFT_KEY = 'wpgctl.draft.v1'
const SECRET_KEY = 'wpgctl.secrets.v1'

/**
 * debounce 延迟执行；flush 立即跑最后一次，cancel 丢掉待执行。
 * @param {Function} fn
 * @param {number} ms 间隔。路径/草稿用 280～900ms，避免每个按键写盘。
 * @returns {Function & { flush: Function, cancel: Function }}
 */
export function debounce(fn, ms) {
  let timer = 0
  const wrapped = (...args) => {
    clearTimeout(timer)
    timer = window.setTimeout(() => fn(...args), ms)
  }
  wrapped.flush = (...args) => {
    clearTimeout(timer)
    fn(...args)
  }
  wrapped.cancel = () => clearTimeout(timer)
  return wrapped
}

/**
 * loadDraft 读取 localStorage 里的表单草稿。损坏或空则返回 null。
 * @returns {object|null}
 */
export function loadDraft() {
  try {
    const raw = localStorage.getItem(DRAFT_KEY)
    if (!raw) return null
    const data = JSON.parse(raw)
    return data && typeof data === 'object' ? data : null
  } catch {
    return null
  }
}

/**
 * saveDraft 写入草稿并打 savedAt。配额满或隐私模式时静默失败。
 * @param {object} data
 */
export function saveDraft(data) {
  try {
    localStorage.setItem(DRAFT_KEY, JSON.stringify({ ...data, savedAt: Date.now() }))
  } catch {
    /* quota / private mode */
  }
}

/**
 * loadSecrets 读取本会话 SSH 密码/私钥路径，不落盘。
 * @returns {object|null}
 */
export function loadSecrets() {
  try {
    const raw = sessionStorage.getItem(SECRET_KEY)
    if (!raw) return null
    const data = JSON.parse(raw)
    return data && typeof data === 'object' ? data : null
  } catch {
    return null
  }
}

/**
 * saveSecrets 写入 sessionStorage。
 * @param {object} data
 */
export function saveSecrets(data) {
  try {
    sessionStorage.setItem(SECRET_KEY, JSON.stringify(data || {}))
  } catch {
    /* ignore */
  }
}
