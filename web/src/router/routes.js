/**
 * 页面路由表：按视图懒加载，构建时打成独立分包。
 */
export const routes = [
  { path: '/', redirect: '/home' },
  {
    path: '/home',
    name: 'home',
    component: () => import('@/views/HomeView.vue'),
    meta: { nav: 'home' },
  },
  {
    path: '/preflight',
    name: 'preflight',
    component: () => import('@/views/PreflightView.vue'),
    meta: { nav: 'wizard' },
  },
  {
    path: '/wizard/:step(\\d+)?/:sub(\\d+)?',
    name: 'wizard',
    component: () => import('@/views/WizardView.vue'),
    meta: { nav: 'wizard' },
  },
  {
    path: '/status',
    name: 'status',
    component: () => import('@/views/StatusView.vue'),
    meta: { nav: 'status' },
  },
  {
    path: '/logs/:service?',
    name: 'logs',
    component: () => import('@/views/LogsView.vue'),
    meta: { nav: 'logs' },
  },
  {
    path: '/fetch',
    name: 'fetch',
    component: () => import('@/views/FetchView.vue'),
    meta: { nav: 'fetch' },
  },
  {
    path: '/upgrade',
    name: 'upgrade',
    component: () => import('@/views/UpgradeView.vue'),
    meta: { nav: 'upgrade' },
  },
  {
    path: '/history',
    name: 'history',
    component: () => import('@/views/HistoryView.vue'),
    meta: { nav: 'history' },
  },
  {
    path: '/report/:id?',
    name: 'report',
    component: () => import('@/views/ReportView.vue'),
    meta: { nav: 'report' },
  },
  { path: '/guide', redirect: '/home' },
  { path: '/guide/:pathMatch(.*)*', redirect: '/home' },
  { path: '/:pathMatch(.*)*', redirect: '/home' },
]
