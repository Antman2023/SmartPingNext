<template>
  <nav class="app-navbar">
    <div class="app-navbar__left">
      <div
        class="app-navbar__toggle"
        :class="{ 'is-collapsed': sidebarStore.isCollapsed }"
        @click="sidebarStore.toggleCollapse"
      >
        <el-icon>
          <DArrowLeft v-if="!sidebarStore.isCollapsed" />
          <DArrowRight v-else />
        </el-icon>
      </div>
      <div class="app-navbar__brand">
        <el-icon class="app-navbar__logo"><Monitor /></el-icon>
        <span class="app-navbar__title">SmartPingNext</span>
        <span v-if="version" class="app-navbar__version">{{ version }}</span>
      </div>
    </div>
    <div class="app-navbar__actions">
      <ThemeToggle />
    </div>
  </nav>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { Monitor, DArrowLeft, DArrowRight } from '@element-plus/icons-vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
import { useSidebarStore } from '@/stores/sidebar'
import { useConfigStore } from '@/stores/config'

const sidebarStore = useSidebarStore()
const configStore = useConfigStore()

const version = computed(() => configStore.config?.Ver)
</script>

<style scoped lang="scss">
.app-navbar {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  height: 60px;
  background-color: var(--navbar-bg);
  color: var(--navbar-text);
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0 20px;
  z-index: 1000;
  box-shadow: var(--shadow-md);
}

.app-navbar__left {
  display: flex;
  align-items: center;
}

.app-navbar__toggle {
  width: 64px;
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  margin-left: -20px;
  margin-right: 4px;
  flex-shrink: 0;
  border-radius: 6px;

  &:hover {
    background-color: rgba(255, 255, 255, 0.1);
  }

  .el-icon {
    width: 18px;
    height: 18px;
    font-size: 18px;
  }
}

.app-navbar__brand {
  display: flex;
  align-items: center;
  gap: 12px;
}

.app-navbar__logo {
  font-size: 24px;
}

.app-navbar__title {
  font-size: 18px;
  font-weight: 600;
}

.app-navbar__version {
  font-size: 12px;
  font-weight: 400;
  opacity: 0.7;
  background-color: rgba(255, 255, 255, 0.1);
  padding: 2px 8px;
  border-radius: 4px;
}

.app-navbar__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}
</style>
