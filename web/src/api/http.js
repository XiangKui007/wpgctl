/**
 * 控制台 JSON API 封装。页面不要直接 fetch('/api/...')，统一走这里以便错误文案一致。
 */

/**
 * api 请求 /api 并解析 JSON。HTTP 202 视为成功（任务已受理）。
 * @param {string} path 以 /api 开头的路径
 * @param {RequestInit} [opts] fetch 选项；默认带 JSON Content-Type
 * @returns {Promise<object>} 响应体；非 JSON 时为 {}
 * @throws {Error} 非 2xx（202 除外）时 message 为服务端 error 或 statusText
 */
export async function api(path, opts = {}) {
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
