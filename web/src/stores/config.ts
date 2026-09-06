import { defineStore } from 'pinia'
import { ref } from 'vue'
import type { Config } from '@/types'
import { fetchConfig, saveConfig as saveConfigApi } from '@/api/config'
import { isRequestCanceled } from '@/api'
import i18n from '@/locales'

export const useConfigStore = defineStore('config', () => {
  const config = ref<Config | null>(null)
  const loading = ref(false)
  const error = ref<string | null>(null)
  let requestId = 0
  let loadPromise: Promise<Config | null> | null = null
  let savePromise: Promise<void> | null = null

  const cloneConfig = (value: Config): Config => {
    return JSON.parse(JSON.stringify(value)) as Config
  }

  const loadConfig = (): Promise<Config | null> => {
    if (savePromise) {
      return savePromise.then(loadConfig, loadConfig)
    }
    if (loadPromise) {
      return loadPromise
    }

    const currentRequestId = ++requestId
    loading.value = true
    error.value = null

    const promise = (async () => {
      try {
        const loadedConfig = await fetchConfig()
        if (currentRequestId !== requestId) {
          return loadConfig()
        }
        if (currentRequestId === requestId) {
          config.value = cloneConfig(loadedConfig)
        }
        return loadedConfig
      } catch (e) {
        if (currentRequestId !== requestId) {
          return loadConfig()
        }
        if (currentRequestId === requestId) {
          error.value = i18n.global.t('common.configLoadFailed')
        }
        console.error(e)
        return null
      } finally {
        if (currentRequestId === requestId) {
          loading.value = false
        }
      }
    })()

    loadPromise = promise
    void promise.finally(() => {
      if (loadPromise === promise) {
        loadPromise = null
      }
    })
    return promise
  }

  const saveConfig = (newConfig: Config, password: string, signal?: AbortSignal): Promise<void> => {
    const configSnapshot = cloneConfig(newConfig)
    const execute = async () => {
      const currentRequestId = ++requestId
      loadPromise = null
      loading.value = true
      error.value = null
      try {
        signal?.throwIfAborted()
        await saveConfigApi(configSnapshot, password, signal)
        if (currentRequestId === requestId) {
          config.value = configSnapshot
        }
      } catch (e) {
        if (currentRequestId === requestId && !isRequestCanceled(e)) {
          error.value = i18n.global.t('common.configSaveFailed')
        }
        if (!isRequestCanceled(e)) {
          console.error(e)
        }
        throw e
      } finally {
        if (currentRequestId === requestId) {
          loading.value = false
        }
      }
    }
    const promise = savePromise ? savePromise.then(execute, execute) : execute()
    savePromise = promise
    const clearSavePromise = () => {
      if (savePromise === promise) savePromise = null
    }
    void promise.then(clearSavePromise, clearSavePromise)
    return promise
  }

  return {
    config,
    loading,
    error,
    loadConfig,
    saveConfig
  }
})
