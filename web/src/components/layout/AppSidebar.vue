<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': isSidebarCondensed }">
    <div v-if="!isSidebarCondensed" class="app-sidebar__summary">
      <span class="app-sidebar__eyebrow">{{ $t('nav.consoleLabel') }}</span>
      <strong class="app-sidebar__node">{{ currentNode }}</strong>
      <p class="app-sidebar__meta">{{ smartpingCount }} {{ $t('common.probes') }}</p>
    </div>
    <el-menu
      :default-active="currentRoute"
      class="app-sidebar__menu"
      :router="true"
      :collapse="isSidebarCondensed"
      :collapse-transition="false"
    >
      <el-menu-item v-for="item in menuItems" :key="item.index" :index="item.index">
        <el-icon><component :is="item.icon" /></el-icon>
        <template #title>{{ $t(item.label) }}</template>
      </el-menu-item>
    </el-menu>
  </aside>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import {
  DataLine,
  DataAnalysis,
  Share,
  MapLocation,
  Tools,
  Bell,
  Setting
} from '@element-plus/icons-vue'
import { useSidebarStore } from '@/stores/sidebar'
import { useConfigStore } from '@/stores/config'
import { displayName } from '@/utils/format'
import { useViewportCompact } from '@/composables/useViewportCompact'

const route = useRoute()
const currentRoute = computed(() => route.path)
const sidebarStore = useSidebarStore()
const configStore = useConfigStore()
const { isCompact } = useViewportCompact()

const menuItems = [
  { index: '/', label: 'nav.dashboard', icon: DataLine },
  { index: '/reverse', label: 'nav.reverse', icon: DataAnalysis },
  { index: '/topology', label: 'nav.topology', icon: Share },
  { index: '/mapping', label: 'nav.mapping', icon: MapLocation },
  { index: '/tools', label: 'nav.tools', icon: Tools },
  { index: '/alerts', label: 'nav.alerts', icon: Bell },
  { index: '/config', label: 'nav.config', icon: Setting }
]

const currentNode = computed(() => {
  const nodeName = configStore.config?.Name
  return nodeName ? displayName(nodeName) : 'SmartPingNext'
})

const smartpingCount = computed(() => {
  return Object.values(configStore.config?.Network || {}).filter((node) => node.Smartping).length
})

const isSidebarCondensed = computed(() => sidebarStore.isCollapsed || isCompact.value)
</script>

<style scoped lang="scss">
.app-sidebar {
  position: fixed;
  top: var(--app-navbar-height);
  left: 0;
  bottom: 0;
  width: var(--app-sidebar-width);
  padding: 18px 14px 18px;
  background: var(--sidebar-bg);
  overflow-y: auto;
  overflow-x: hidden;
  transition:
    width 0.3s ease,
    background-color 0.3s ease;
  border-right: 1px solid var(--color-border-light);
  backdrop-filter: blur(22px);

  &.is-collapsed {
    width: var(--app-sidebar-collapsed-width);
    padding-inline: 10px;
  }
}

.app-sidebar__summary {
  padding: 8px 8px 16px;
}

.app-sidebar__eyebrow {
  display: inline-block;
  margin-bottom: 8px;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  color: var(--color-primary);
}

.app-sidebar__node {
  display: block;
  font-size: 18px;
  font-weight: 700;
  letter-spacing: -0.03em;
  color: var(--color-text-primary);
}

.app-sidebar__meta {
  margin: 8px 0 0;
  font-size: 13px;
  color: var(--color-text-secondary);
}

.app-sidebar__menu {
  background-color: transparent;
  width: 100%;

  :deep(.el-menu-item) {
    height: 52px;
    margin-bottom: 6px;
    border-radius: 16px;
    color: var(--sidebar-text);
    font-weight: 500;
    transition:
      transform 0.22s ease,
      background-color 0.22s ease,
      color 0.22s ease,
      border-color 0.22s ease;

    &:hover {
      background-color: var(--sidebar-hover-bg);
      transform: translateX(2px);
    }

    &.is-active {
      color: var(--sidebar-active-text);
      background-color: var(--sidebar-active-bg);
      box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--color-primary) 18%, transparent);
    }
  }

  :deep(.el-menu-item .el-icon) {
    width: 18px;
    margin-right: 12px;
    font-size: 18px;
  }

  :deep(.el-menu-item span) {
    letter-spacing: -0.01em;
  }

  &.el-menu--collapse {
    width: calc(var(--app-sidebar-collapsed-width) - 20px);

    :deep(.el-menu-item) {
      justify-content: center;
      padding: 0 !important;
    }

    :deep(.el-menu-item .el-icon) {
      margin-right: 0;
    }
  }
}

@media (max-width: 900px) {
  .app-sidebar {
    padding-top: 14px;
  }
}
</style>
