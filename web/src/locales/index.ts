import { createI18n } from 'vue-i18n'
import zhCN from './zh-CN'
import enUS from './en-US'

const messages = {
  'zh-CN': zhCN,
  'en-US': enUS
}

const i18n = createI18n({
  legacy: false,
  locale: 'zh-CN', // 初始值由 main.ts 从 localeStore 同步
  fallbackLocale: 'zh-CN',
  messages
})

export default i18n
