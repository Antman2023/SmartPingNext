<template>
  <Transition name="sidebar-backdrop">
    <button
      v-if="isCompact && sidebarStore.isMobileOpen"
      type="button"
      class="app-sidebar__backdrop"
      :aria-label="$t('nav.closeNavigation')"
      @click="sidebarStore.closeMobile"
    />
  </Transition>
  <Transition name="sidebar-drawer">
    <aside
      v-if="!isCompact || sidebarStore.isMobileOpen"
      id="app-sidebar"
      class="app-sidebar"
      :class="{ 'is-collapsed': isDesktopCollapsed, 'is-mobile': isCompact }"
      :aria-label="$t('nav.primaryNavigation')"
    >
      <div v-if="!isDesktopCollapsed" class="app-sidebar__summary">
        <span class="app-sidebar__eyebrow">{{ $t('nav.consoleLabel') }}</span>
        <strong class="app-sidebar__node">{{ currentNode }}</strong>
        <p class="app-sidebar__meta">{{ smartpingCount }} {{ $t('common.probes') }}</p>
      </div>
      <el-menu
        :default-active="currentRoute"
        class="app-sidebar__menu"
        :router="true"
        :collapse="isDesktopCollapsed"
        :collapse-transition="false"
        :aria-label="$t('nav.primaryNavigation')"
        @select="handleSelect"
      >
        <el-menu-item
          v-for="item in menuItems"
          :key="item.index"
          :index="item.index"
          :aria-label="$t(item.label)"
        >
          <el-icon><component :is="item.icon" /></el-icon>
          <template #title>{{ $t(item.label) }}</template>
        </el-menu-item>
      </el-menu>
    </aside>
  </Transition>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, watch } from 'vue'
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

const isDesktopCollapsed = computed(() => !isCompact.value && sidebarStore.isCollapsed)

const handleSelect = () => {
  if (isCompact.value) {
    sidebarStore.closeMobile()
  }
}

const handleEscape = (event: KeyboardEvent) => {
  if (event.key === 'Escape' && sidebarStore.isMobileOpen) {
    sidebarStore.closeMobile()
  }
}

watch(
  () => route.path,
  () => sidebarStore.closeMobile()
)

watch(
  [isCompact, () => sidebarStore.isMobileOpen],
  ([compact, open]) => {
    document.documentElement.classList.toggle('sidebar-open', compact && open)
    if (!compact && open) {
      sidebarStore.closeMobile()
    }
  },
  { immediate: true }
)

onMounted(() => window.addEventListener('keydown', handleEscape))

onBeforeUnmount(() => {
  window.removeEventListener('keydown', handleEscape)
  document.documentElement.classList.remove('sidebar-open')
})
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
  z-index: 950;

  &.is-collapsed {
    width: var(--app-sidebar-collapsed-width);
    padding-inline: 10px;
  }
}

.app-sidebar__backdrop {
  position: fixed;
  inset: var(--app-navbar-height) 0 0;
  z-index: 900;
  border: 0;
  padding: 0;
  background: rgba(2, 8, 18, 0.54);
  backdrop-filter: blur(3px);
  cursor: pointer;
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

@media (max-width: 960px) {
  .app-sidebar.is-mobile {
    width: min(296px, calc(100vw - 56px));
    padding-top: 14px;
    box-shadow: var(--shadow-lg);
  }
}

.sidebar-drawer-enter-active,
.sidebar-drawer-leave-active,
.sidebar-backdrop-enter-active,
.sidebar-backdrop-leave-active {
  transition:
    transform 0.24s ease,
    opacity 0.24s ease;
}

.sidebar-drawer-enter-from,
.sidebar-drawer-leave-to {
  transform: translateX(-100%);
}

.sidebar-backdrop-enter-from,
.sidebar-backdrop-leave-to {
  opacity: 0;
}
</style>
