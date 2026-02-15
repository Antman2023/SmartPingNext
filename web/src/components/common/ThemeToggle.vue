<template>
  <el-tooltip :content="theme === 'light' ? '切换深色模式' : '切换浅色模式'">
    <el-button
      circle
      :icon="theme === 'light' ? Moon : Sunny"
      @click="toggleTheme"
    />
  </el-tooltip>
</template>

<script setup lang="ts">
import { Moon, Sunny } from '@element-plus/icons-vue'
import { useTheme } from '@/composables/useTheme'

const { theme, toggleTheme } = useTheme()
</script>

<style scoped lang="scss">
.el-button {
  background-color: rgba(255, 255, 255, 0.1);
  border-color: rgba(255, 255, 255, 0.2);
  color: var(--navbar-text);

  &:hover {
    background-color: rgba(255, 255, 255, 0.2);
  }
}
</style>
