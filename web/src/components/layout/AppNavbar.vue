<template>
  <nav class="app-navbar" :aria-label="$t('nav.applicationHeader')">
    <div class="app-navbar__left">
      <button
        type="button"
        class="app-navbar__toggle"
        :class="{ 'is-collapsed': !isSidebarExpanded }"
        :aria-label="sidebarToggleLabel"
        :title="sidebarToggleLabel"
        aria-controls="app-sidebar"
        :aria-expanded="isSidebarExpanded"
        @click="handleSidebarToggle"
      >
        <Close
          v-if="isCompact && sidebarStore.isMobileOpen"
          class="app-navbar__toggle-icon"
          aria-hidden="true"
        />
        <Menu v-else-if="isCompact" class="app-navbar__toggle-icon" aria-hidden="true" />
        <DArrowLeft
          v-else-if="isSidebarExpanded"
          class="app-navbar__toggle-icon"
          aria-hidden="true"
        />
        <DArrowRight v-else class="app-navbar__toggle-icon" aria-hidden="true" />
      </button>
      <div class="app-navbar__brand">
        <div class="app-navbar__mark">
          <Monitor class="app-navbar__logo" aria-hidden="true" />
        </div>
        <div class="app-navbar__copy">
          <span class="app-navbar__title">SmartPingNext</span>
          <div class="app-navbar__meta-line">
            <span class="app-navbar__subtitle">{{ currentTitle }}</span>
            <span v-if="nodeName" class="app-navbar__chip">{{ nodeName }}</span>
          </div>
        </div>
      </div>
    </div>
    <div class="app-navbar__actions">
      <div v-if="nodeCount || version" class="app-navbar__metrics">
        <span v-if="nodeCount" class="app-navbar__metric"
          >{{ nodeCount }} {{ $t('common.node') }}</span
        >
        <span v-if="version" class="app-navbar__metric">v{{ version }}</span>
      </div>
      <ThemeToggle />
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useRoute } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { Monitor, DArrowLeft, DArrowRight, Menu, Close } from '@element-plus/icons-vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useSidebarStore } from '@/stores/sidebar'
import { useConfigStore } from '@/stores/config'
import { displayName } from '@/utils/format'
import { useViewportCompact } from '@/composables/useViewportCompact'

const sidebarStore = useSidebarStore()
const configStore = useConfigStore()
const route = useRoute()
const { t } = useI18n()
const { isCompact } = useViewportCompact()

const version = computed(() => configStore.config?.Ver)
const nodeName = computed(() => {
  const name = configStore.config?.Name
  return name ? displayName(name) : ''
})
const nodeCount = computed(() => Object.keys(configStore.config?.Network || {}).length)
const currentTitle = computed(() => {
  const titleKey = route.meta.titleKey
  return titleKey ? t(titleKey) : 'SmartPingNext'
})
const isSidebarExpanded = computed(() =>
  isCompact.value ? sidebarStore.isMobileOpen : !sidebarStore.isCollapsed
)
const sidebarToggleLabel = computed(() => {
  if (isCompact.value) {
    return t(sidebarStore.isMobileOpen ? 'nav.closeNavigation' : 'nav.openNavigation')
  }
  return t(sidebarStore.isCollapsed ? 'nav.expandNavigation' : 'nav.collapseNavigation')
})

const handleSidebarToggle = () => {
  if (isCompact.value) {
    sidebarStore.toggleMobile()
    return
  }
  sidebarStore.toggleCollapse()
}
</script>

<style scoped lang="scss">
.app-navbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: var(--app-navbar-height);
  background-color: var(--navbar-bg);
  color: var(--navbar-text);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 22px;
  z-index: 1000;
  border-bottom: 1px solid var(--color-border-light);
  box-shadow: var(--shadow-sm);
  backdrop-filter: blur(22px);
}

.app-navbar__left {
  display: flex;
  align-items: center;
  gap: 14px;
}

.app-navbar__toggle {
  width: 44px;
  height: 44px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  flex-shrink: 0;
  border-radius: 14px;
  border: 1px solid var(--color-border-light);
  background: color-mix(in srgb, var(--color-bg-primary) 82%, transparent);
  color: var(--navbar-text);
  box-shadow: inset 0 1px 0 rgba(255, 255, 255, 0.18);

  &:hover {
    transform: translateY(-1px);
    border-color: color-mix(in srgb, var(--color-primary) 20%, transparent);
    box-shadow: var(--shadow-sm);
  }
}

.app-navbar__toggle-icon {
  width: 18px;
  height: 18px;
}

.app-navbar__brand {
  display: flex;
  align-items: center;
  gap: 14px;
}

.app-navbar__mark {
  width: 46px;
  height: 46px;
  display: grid;
  place-items: center;
  border-radius: 16px;
  background:
    radial-gradient(
      circle at top left,
      color-mix(in srgb, var(--color-primary) 32%, transparent),
      transparent 58%
    ),
    linear-gradient(
      180deg,
      color-mix(in srgb, var(--color-bg-primary) 90%, #ffffff 10%),
      var(--color-bg-secondary)
    );
  border: 1px solid color-mix(in srgb, var(--color-primary) 16%, transparent);
  box-shadow: var(--shadow-sm);
}

.app-navbar__logo {
  width: 24px;
  height: 24px;
  color: var(--color-primary);
}

.app-navbar__copy {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.app-navbar__title {
  font-size: 20px;
  font-weight: 700;
  letter-spacing: -0.04em;
  color: var(--navbar-text);
}

.app-navbar__meta-line {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 20px;
}

.app-navbar__subtitle {
  font-size: 12px;
  font-weight: 600;
  letter-spacing: 0.12em;
  text-transform: uppercase;
  color: var(--color-text-secondary);
}

.app-navbar__chip,
.app-navbar__metric {
  display: inline-flex;
  align-items: center;
  padding: 6px 10px;
  border-radius: 999px;
  border: 1px solid var(--color-border-light);
  background: color-mix(in srgb, var(--color-bg-primary) 80%, transparent);
  color: var(--color-text-regular);
  font-size: 12px;
  font-weight: 500;
}

.app-navbar__actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-navbar__metrics {
  display: flex;
  align-items: center;
  gap: 8px;
}

@media (max-width: 900px) {
  .app-navbar {
    padding: 0 14px;
  }

  .app-navbar__metrics,
  .app-navbar__chip {
    display: none;
  }

  .app-navbar__subtitle {
    letter-spacing: 0.08em;
  }
}

@media (max-width: 640px) {
  .app-navbar__subtitle {
    display: none;
  }

  .app-navbar__title {
    font-size: 18px;
  }
}
</style>
