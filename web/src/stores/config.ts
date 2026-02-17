import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Config } from '@/types'
import { fetchConfig, saveConfig as saveConfigApi } from '@/api/config'
import i18n from '@/locales'

export const useConfigStore = defineStore('config', () => {
  const config = ref<Config | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)

  const loadConfig = async () => {
    loading.value = true
    error.value = null
    try {
      config.value = await fetchConfig()
    } catch (e) {
      error.value = i18n.global.t('common.configLoadFailed')
      console.error(e)
    } finally {
      loading.value = false
    }
  }

  const saveConfig = async (newConfig: Config, password: string) => {
    loading.value = true
    error.value = null
    try {
      await saveConfigApi(newConfig, password)
      config.value = newConfig
    } catch (e) {
      error.value = i18n.global.t('common.configSaveFailed')
      console.error(e)
      throw e
    } finally {
      loading.value = false
    }
  }

  return {
    config,
    loading,
    error,
    loadConfig,
    saveConfig
  }
})
