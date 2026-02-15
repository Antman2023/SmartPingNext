<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': sidebarStore.isCollapsed }">
    <div class="app-sidebar__toggle" @click="sidebarStore.toggleCollapse">
      <el-icon>
        <Fold v-if="!sidebarStore.isCollapsed" />
        <Expand v-else />
      </el-icon>
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
  height: 40px;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  color: var(--sidebar-text);
  border-bottom: 1px solid rgba(255, 255, 255, 0.05);

  &:hover {
    background-color: rgba(255, 255, 255, 0.05);
  }
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
