<template>
  <div class="app-layout">
    <AppNavbar />
    <div class="app-layout__body">
      <AppSidebar />
      <main class="app-main" :class="{ 'is-collapsed': sidebarStore.isCollapsed }">
        <slot />
      </main>
    </div>
  </div>
</template>

<script setup lang="ts">
import { onMounted } from 'vue'
import AppNavbar from './AppNavbar.vue'
import AppSidebar from './AppSidebar.vue'
import { useSidebarStore } from '@/stores/sidebar'
import { useConfigStore } from '@/stores/config'

const sidebarStore = useSidebarStore()
const configStore = useConfigStore()

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
  overflow: hidden;
}

.app-layout__body {
  display: flex;
  flex: 1;
  padding-top: 60px;
  overflow: hidden;
}

.app-main {
  flex: 1;
  margin-left: 200px;
  padding: 20px;
  background-color: var(--color-bg-secondary);
  min-height: calc(100vh - 60px);
  box-sizing: border-box;
  transition: margin-left 0.3s ease;
  overflow: hidden;

  &.is-collapsed {
    margin-left: 64px;
  }
}
</style>
