import { createRouter, createWebHistory } from 'vue-router'
import i18n from '@/locales'

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { titleKey: 'pageTitle.dashboard' }
  },
  {
    path: '/reverse',
    name: 'Reverse',
    component: () => import('@/views/ReverseView.vue'),
    meta: { titleKey: 'pageTitle.reverse' }
  },
  {
    path: '/topology',
    name: 'Topology',
    component: () => import('@/views/TopologyView.vue'),
    meta: { titleKey: 'pageTitle.topology' }
  },
  {
    path: '/mapping',
    name: 'Mapping',
    component: () => import('@/views/MappingView.vue'),
    meta: { titleKey: 'pageTitle.mapping' }
  },
  {
    path: '/tools',
    name: 'Tools',
    component: () => import('@/views/ToolsView.vue'),
    meta: { titleKey: 'pageTitle.tools' }
  },
  {
    path: '/alerts',
    name: 'Alerts',
    component: () => import('@/views/AlertsView.vue'),
    meta: { titleKey: 'pageTitle.alerts' }
  },
  {
    path: '/config',
    name: 'Config',
    component: () => import('@/views/ConfigView.vue'),
    meta: { titleKey: 'pageTitle.config' }
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach((to, _from, next) => {
  const titleKey = to.meta.titleKey as string
  const title = titleKey ? i18n.global.t(titleKey) : 'SmartPingNext'
  document.title = `${title} - SmartPingNext`
  next()
})

export default router
