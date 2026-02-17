import { defineStore } from 'pinia'
import { ref } from 'vue'

export type LocaleCode = 'zh-CN' | 'en-US'

export const useLocaleStore = defineStore('locale', () => {
  const locale = ref<LocaleCode>(
    (localStorage.getItem('locale') as LocaleCode) || 'zh-CN'
  )

  const setLocale = (newLocale: LocaleCode) => {
    locale.value = newLocale
    localStorage.setItem('locale', newLocale)
  }

  return {
    locale,
    setLocale
  }
})
