import { computed } from 'vue'
import { useLocaleStore } from '@/stores/locale'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'

const elementLocales: Record<string, any> = {
  'zh-CN': zhCn,
  'en-US': en
}

export function useLocale() {
  const localeStore = useLocaleStore()

  const elementLocale = computed(() => elementLocales[localeStore.locale] || zhCn)

  const setLocale = (locale: typeof localeStore.locale) => {
    localeStore.setLocale(locale)
  }

  return {
    locale: localeStore.locale,
    elementLocale,
    setLocale
  }
}
