import { createRouter, createWebHistory } from 'vue-router'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { title: '正向监控' }
  },
  {
    path: '/reverse',
    name: 'Reverse',
    component: () => import('@/views/ReverseView.vue'),
    meta: { title: '反向监控' }
  },
  {
    path: '/topology',
    name: 'Topology',
    component: () => import('@/views/TopologyView.vue'),
    meta: { title: '拓扑图' }
  },
  {
    path: '/mapping',
    name: 'Mapping',
    component: () => import('@/views/MappingView.vue'),
    meta: { title: '延迟地图' }
  },
  {
    path: '/tools',
    name: 'Tools',
    component: () => import('@/views/ToolsView.vue'),
    meta: { title: '检测工具' }
  },
  {
    path: '/alerts',
    name: 'Alerts',
    component: () => import('@/views/AlertsView.vue'),
    meta: { title: '报警记录' }
  },
  {
    path: '/config',
    name: 'Config',
    component: () => import('@/views/ConfigView.vue'),
    meta: { title: '系统配置' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  document.title = `${to.meta.title || 'SmartPing'} - SmartPing`
  next()
})

export default router
