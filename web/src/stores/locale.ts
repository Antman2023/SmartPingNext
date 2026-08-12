import { defineStore } from 'pinia'
import { ref } from 'vue'

export type LocaleCode = 'zh-CN' | 'en-US'

const VALID_LOCALES: LocaleCode[] = ['zh-CN', 'en-US']

function isLocaleCode(locale: string | null): locale is LocaleCode {
  return VALID_LOCALES.includes(locale as LocaleCode)
}

function getInitialLocale(): LocaleCode {
  const storedLocale = localStorage.getItem('locale')
  if (isLocaleCode(storedLocale)) {
    return storedLocale
  }

  const systemLocale = navigator.languages[0] || navigator.language
  return systemLocale.toLowerCase().startsWith('zh') ? 'zh-CN' : 'en-US'
}

export const useLocaleStore = defineStore('locale', () => {
  const locale = ref<LocaleCode>(getInitialLocale())

  const setLocale = (newLocale: LocaleCode) => {
    locale.value = newLocale
    localStorage.setItem('locale', newLocale)
  }

  return {
    locale,
    setLocale
  }
})
