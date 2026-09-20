/**
 * 现场控制台路由：hash 模式，URL 形如 #/status、#/wizard/0/1。
 */
import { createRouter, createWebHashHistory } from 'vue-router'
import { routes } from './routes.js'

export { VIEWS, parseRoute, routePath, buildRoute, sameHash, samePath } from './helpers.js'
export { routes }

/**
 * createAppRouter 创建 hash 路由实例，交给 main.js app.use。
 * @returns {import('vue-router').Router}
 */
export function createAppRouter() {
  return createRouter({
    history: createWebHashHistory(),
    routes,
  })
}
