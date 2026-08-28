<template>
  <div class="app-layout">
    <AppNavbar />
    <div class="app-layout__body">
      <AppSidebar />
      <main class="app-main" :class="{ 'is-collapsed': isDesktopSidebarCollapsed }">
        <div class="app-main__inner">
          <slot />
        </div>
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted } from 'vue'
import AppNavbar from './AppNavbar.vue'
import AppSidebar from './AppSidebar.vue'
import { useSidebarStore } from '@/stores/sidebar'
import { useConfigStore } from '@/stores/config'
import { useViewportCompact } from '@/composables/useViewportCompact'

const sidebarStore = useSidebarStore()
const configStore = useConfigStore()
const { isCompact } = useViewportCompact()
const isDesktopSidebarCollapsed = computed(() => !isCompact.value && sidebarStore.isCollapsed)

onMounted(() => {
  if (!configStore.config) {
    configStore.loadConfig()
  }
})
</script>

<style scoped lang="scss">
.app-layout {
  min-height: 100vh;
  display: flex;
  flex-direction: column;
  position: relative;
}

.app-layout__body {
  display: block;
  flex: 1;
  padding-top: var(--app-navbar-height);
  min-height: 100vh;
}

.app-main {
  margin-left: var(--app-sidebar-width);
  width: calc(100% - var(--app-sidebar-width));
  min-width: 0;
  min-height: calc(100vh - var(--app-navbar-height));
  transition: margin-left 0.3s ease;
  padding: 28px 28px 36px;
  overflow: auto;

  &.is-collapsed {
    margin-left: var(--app-sidebar-collapsed-width);
    width: calc(100% - var(--app-sidebar-collapsed-width));
  }
}

.app-main__inner {
  width: min(100%, var(--app-content-max-width));
  margin: 0 auto;
  min-height: 100%;
}

@media (max-width: 960px) {
  .app-main {
    width: 100%;
    margin-left: 0;
    padding: 20px 16px 28px;
  }
}
</style>
