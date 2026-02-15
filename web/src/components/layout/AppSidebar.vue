<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': sidebarStore.isCollapsed }">
    <div class="app-sidebar__toggle" @click="sidebarStore.toggleCollapse">
      <el-icon>
        <Fold v-if="!sidebarStore.isCollapsed" />
        <Expand v-else />
      </el-icon>
      <span v-if="sidebarStore.isCollapsed" class="toggle-text">展开</span>
    </div>
    <el-menu
      :default-active="currentRoute"
      class="app-sidebar__menu"
      :router="true"
      :collapse="sidebarStore.isCollapsed"
    >
      <el-menu-item index="/">
        <el-icon><DataLine /></el-icon>
        <template #title>正向监控</template>
      </el-menu-item>
      <el-menu-item index="/reverse">
        <el-icon><DataAnalysis /></el-icon>
        <template #title>反向监控</template>
      </el-menu-item>
      <el-menu-item index="/topology">
        <el-icon><Share /></el-icon>
        <template #title>拓扑图</template>
      </el-menu-item>
      <el-menu-item index="/mapping">
        <el-icon><MapLocation /></el-icon>
        <template #title>延迟地图</template>
      </el-menu-item>
      <el-menu-item index="/tools">
        <el-icon><Tools /></el-icon>
        <template #title>检测工具</template>
      </el-menu-item>
      <el-menu-item index="/alerts">
        <el-icon><Bell /></el-icon>
        <template #title>报警记录</template>
      </el-menu-item>
      <el-menu-item index="/config">
        <el-icon><Setting /></el-icon>
        <template #title>系统配置</template>
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
  Setting,
  Fold,
  Expand
} from '@element-plus/icons-vue'
import { useSidebarStore } from '@/stores/sidebar'

const route = useRoute()
const currentRoute = computed(() => route.path)
const sidebarStore = useSidebarStore()
</script>

<style scoped lang="scss">
.app-sidebar {
  position: fixed;
  top: 60px;
  left: 0;
  bottom: 0;
  width: 200px;
  background-color: var(--sidebar-bg);
  overflow-y: auto;
  overflow-x: hidden;
  transition: width 0.3s ease;

  &.is-collapsed {
    width: 64px;
  }
}

.app-sidebar__toggle {
  height: 48px;
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 8px;
  cursor: pointer;
  color: var(--sidebar-text);
  margin: 8px;
  padding: 0 12px;
  border-radius: 8px;
  transition: all 0.2s ease;

  &:hover {
    background-color: rgba(255, 255, 255, 0.08);
    color: var(--sidebar-active-text);
  }

  .el-icon {
    width: 18px;
    height: 18px;
    font-size: 18px;
    flex-shrink: 0;
  }

  .toggle-text {
    font-size: 12px;
    white-space: nowrap;
  }
}

.is-collapsed .app-sidebar__toggle {
  flex-direction: column;
  gap: 4px;
  padding: 8px 0;
}

.app-sidebar__menu {
  border-right: none;
  background-color: transparent;

  :deep(.el-menu-item) {
    color: var(--sidebar-text);

    &:hover {
      background-color: rgba(255, 255, 255, 0.05);
    }

    &.is-active {
      color: var(--sidebar-active-text);
      background-color: rgba(64, 158, 255, 0.1);
    }
  }

  &:not(.el-menu--collapse) {
    width: 200px;
  }
}
</style>
