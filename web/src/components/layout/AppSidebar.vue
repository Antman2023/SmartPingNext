<template>
  <aside class="app-sidebar">
    <el-menu
      :default-active="currentRoute"
      class="app-sidebar__menu"
      :router="true"
    >
      <el-menu-item index="/">
        <el-icon><DataLine /></el-icon>
        <span>正向监控</span>
      </el-menu-item>
      <el-menu-item index="/reverse">
        <el-icon><DataAnalysis /></el-icon>
        <span>反向监控</span>
      </el-menu-item>
      <el-menu-item index="/topology">
        <el-icon><Share /></el-icon>
        <span>拓扑图</span>
      </el-menu-item>
      <el-menu-item index="/mapping">
        <el-icon><MapLocation /></el-icon>
        <span>延迟地图</span>
      </el-menu-item>
      <el-menu-item index="/tools">
        <el-icon><Tools /></el-icon>
        <span>检测工具</span>
      </el-menu-item>
      <el-menu-item index="/alerts">
        <el-icon><Bell /></el-icon>
        <span>报警记录</span>
      </el-menu-item>
      <el-menu-item index="/config">
        <el-icon><Setting /></el-icon>
        <span>系统配置</span>
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

const route = useRoute()
const currentRoute = computed(() => route.path)
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
}
</style>
