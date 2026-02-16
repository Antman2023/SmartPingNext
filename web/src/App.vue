<template>
  <el-config-provider :locale="elementLocale">
    <AppLayout>
      <router-view />
    </AppLayout>
  </el-config-provider>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import AppLayout from '@/components/layout/AppLayout.vue'
import { useLocaleStore } from '@/stores/locale'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const localeStore = useLocaleStore()

const elementLocales: Record<string, any> = {
  'zh-CN': zhCn,
  'en-US': en
}

const elementLocale = computed(() => elementLocales[localeStore.locale] || zhCn)
</script>
