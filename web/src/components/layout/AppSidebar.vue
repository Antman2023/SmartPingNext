<template>
  <aside class="app-sidebar" :class="{ 'is-collapsed': sidebarStore.isCollapsed }">
    <el-menu
      :default-active="currentRoute"
      class="app-sidebar__menu"
      :router="true"
      :collapse="sidebarStore.isCollapsed"
    >
      <el-menu-item index="/">
        <el-icon><DataLine /></el-icon>
        <template #title>{{ $t('nav.dashboard') }}</template>
      </el-menu-item>
      <el-menu-item index="/reverse">
        <el-icon><DataAnalysis /></el-icon>
        <template #title>{{ $t('nav.reverse') }}</template>
      </el-menu-item>
      <el-menu-item index="/topology">
        <el-icon><Share /></el-icon>
        <template #title>{{ $t('nav.topology') }}</template>
      </el-menu-item>
      <el-menu-item index="/mapping">
        <el-icon><MapLocation /></el-icon>
        <template #title>{{ $t('nav.mapping') }}</template>
      </el-menu-item>
      <el-menu-item index="/tools">
        <el-icon><Tools /></el-icon>
        <template #title>{{ $t('nav.tools') }}</template>
      </el-menu-item>
      <el-menu-item index="/alerts">
        <el-icon><Bell /></el-icon>
        <template #title>{{ $t('nav.alerts') }}</template>
      </el-menu-item>
      <el-menu-item index="/config">
        <el-icon><Setting /></el-icon>
        <template #title>{{ $t('nav.config') }}</template>
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
