<template>
  <el-config-provider :locale="elementLocale">
    <AppLayout>
      <router-view />
    </AppLayout>
  </el-config-provider>
</template>

<script setup lang="ts">
import AppLayout from '@/components/layout/AppLayout.vue'
import { useLocale } from '@/composables/useLocale'

const { elementLocale } = useLocale()
</script>
