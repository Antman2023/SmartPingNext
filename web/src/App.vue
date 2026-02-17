<template>
  <el-config-provider :locale="elementLocale">
    <AppLayout>
      <router-view />
    </AppLayout>
  </el-config-provider>
</template>

<script setup lang="ts">
import { onErrorCaptured } from 'vue'
import { ElMessage } from 'element-plus'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useLocale } from '@/composables/useLocale'

const { elementLocale } = useLocale()

// 全局错误边界
onErrorCaptured((error: Error, instance, info) => {
  console.error('Vue Error:', error)
  console.error('Component:', instance)
  console.error('Error Info:', info)

  ElMessage.error(`组件错误: ${error.message}`)

  // 返回 false 阻止错误继续向上传播
  return false
})
</script>
