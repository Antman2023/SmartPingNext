<template>
  <nav class="app-navbar">
    <div class="app-navbar__brand">
      <el-icon class="app-navbar__logo"><Monitor /></el-icon>
      <span class="app-navbar__title">SmartPing</span>
    </div>
    <div class="app-navbar__actions">
      <ThemeToggle />
    </div>
  </nav>
</template>

<script setup lang="ts">
import { Monitor } from '@element-plus/icons-vue'
import ThemeToggle from '@/components/common/ThemeToggle.vue'
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

.app-navbar__actions {
  display: flex;
  align-items: center;
  gap: 16px;
}
</style>
