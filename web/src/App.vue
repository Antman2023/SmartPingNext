<template>
  <el-config-provider :locale="elementLocale">
    <AppLayout>
      <router-view />
    </AppLayout>
  </el-config-provider>
</template>

<script setup lang="ts">
import { onErrorCaptured, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useRoute } from 'vue-router'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useLocale } from '@/composables/useLocale'
import { useI18n } from 'vue-i18n'

const { elementLocale, locale } = useLocale()
const { t } = useI18n()
const route = useRoute()

watch(
  [locale, () => route.meta.titleKey],
  () => {
    const titleKey = route.meta.titleKey
    const title = titleKey ? t(titleKey) : 'SmartPingNext'
    document.title = `${title} - SmartPingNext`
  },
  { immediate: true }
)

// 全局错误边界
onErrorCaptured((error: Error, instance, info) => {
  console.error('Vue Error:', error)
  console.error('Component:', instance)
  console.error('Error Info:', info)

  ElMessage.error(t('common.componentError', { message: error.message }))

  // 返回 false 阻止错误继续向上传播
  return false
})
</script>
