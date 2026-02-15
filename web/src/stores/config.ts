import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Config } from '@/types'
import { getConfig as fetchConfig, saveConfig as saveConfigApi } from '@/api/config'

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
      error.value = '加载配置失败'
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
      error.value = '保存配置失败'
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
