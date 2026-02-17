import { createRouter, createWebHistory, type RouteLocationNormalized } from 'vue-router'
import i18n from '@/locales'

// 扩展路由元信息类型
declare module 'vue-router' {
  interface RouteMeta {
    titleKey?: string
    requiresAuth?: boolean
  }
}

const routes = [
  {
    path: '/',
    name: 'Dashboard',
    component: () => import('@/views/DashboardView.vue'),
    meta: { titleKey: 'dashboard.title' }
  },
  {
    path: '/reverse',
    name: 'Reverse',
    component: () => import('@/views/ReverseView.vue'),
    meta: { titleKey: 'reverse.title' }
  },
  {
    path: '/topology',
    name: 'Topology',
    component: () => import('@/views/TopologyView.vue'),
    meta: { titleKey: 'topology.title' }
  },
  {
    path: '/mapping',
    name: 'Mapping',
    component: () => import('@/views/MappingView.vue'),
    meta: { titleKey: 'mapping.title' }
  },
  {
    path: '/tools',
    name: 'Tools',
    component: () => import('@/views/ToolsView.vue'),
    meta: { titleKey: 'tools.title' }
  },
  {
    path: '/alerts',
    name: 'Alerts',
    component: () => import('@/views/AlertsView.vue'),
    meta: { titleKey: 'alerts.title' }
  },
  {
    path: '/config',
    name: 'Config',
    component: () => import('@/views/ConfigView.vue'),
    meta: { titleKey: 'config.title' }
  },
  {
    path: '/:pathMatch(.*)*',
    name: 'NotFound',
    redirect: '/'
  }
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

function setPageTitle(to: RouteLocationNormalized): void {
  const titleKey = to.meta.titleKey
  const title = titleKey ? i18n.global.t(titleKey) : 'SmartPingNext'
  document.title = `${title} - SmartPingNext`
}

function checkPermission(to: RouteLocationNormalized): boolean {
  // 如果路由需要认证，检查权限
  // 当前项目无登录系统，暂时跳过权限检查
  if (to.meta.requiresAuth) {
    // TODO: 添加权限检查逻辑
    // const authStore = useAuthStore()
    // if (!authStore.isAuthenticated) return false
  }
  return true
}

router.beforeEach((to, _from, next) => {
  setPageTitle(to)

  if (!checkPermission(to)) {
    // 权限不足时跳转到首页
    next({ path: '/', replace: true })
    return
  }

  next()
})

export default router
