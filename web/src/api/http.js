/**
 * 控制台 JSON API 封装。页面不要直接 fetch('/api/...')，统一走这里以便错误文案一致。
 */

/**
 * API_PREFIX 实际请求前缀。代码里仍写 /api/xxx，发出去时替换为 /wpg-deploy-api/xxx：
 * 现场 Nginx 与业务前端共用 8877 时，根级 /api 太通用易与其他服务撞路径。
 * 后端两个前缀都认（见 internal/ui/server.go APIAlias），直连 9527 也不受影响。
 */
export const API_PREFIX = '/wpg-deploy-api'

/**
 * apiPath 把 /api 开头的路径换成带产品名的实际前缀；其他路径原样返回。
 * WebSocket、iframe、下载链接等绕过 api() 的地方也要用它。
 * @param {string} p 以 /api 开头的路径
 * @returns {string}
 */
export function apiPath(p) {
  if (p === '/api' || p.startsWith('/api/') || p.startsWith('/api?')) {
    return API_PREFIX + p.slice('/api'.length)
  }
  return p
}

/**
 * api 请求 /api 并解析 JSON。HTTP 202 视为成功（任务已受理）。
 * @param {string} path 以 /api 开头的路径
 * @param {RequestInit} [opts] fetch 选项；默认带 JSON Content-Type
 * @returns {Promise<object>} 响应体；非 JSON 时为 {}
 * @throws {Error} 非 2xx（202 除外）时 message 为服务端 error 或 statusText
 */
export async function api(path, opts = {}) {
  const res = await fetch(apiPath(path), {
    headers: { 'Content-Type': 'application/json' },
    ...opts,
  })
  const data = await res.json().catch(() => ({}))
  if (!res.ok && res.status !== 202) {
    throw new Error(data.error || res.statusText)
  }
  return data
}
