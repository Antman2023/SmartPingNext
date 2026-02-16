import { createApp } from 'vue'
import { createPinia } from 'pinia'
import ElementPlus from 'element-plus'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import en from 'element-plus/es/locale/lang/en'
import 'element-plus/dist/index.css'
import * as ElementPlusIconsVue from '@element-plus/icons-vue'

import App from './App.vue'
import router from './router'
import { useThemeStore } from './stores/theme'
import { useLocaleStore } from './stores/locale'
import i18n from './locales'

import './assets/styles/global.scss'

const app = createApp(App)

// 注册 Element Plus 图标
for (const [key, component] of Object.entries(ElementPlusIconsVue)) {
  app.component(key, component)
}

app.use(createPinia())
app.use(router)
app.use(i18n)

// Element Plus 语言映射
const elementLocales: Record<string, any> = {
  'zh-CN': zhCn,
  'en-US': en
}

// 初始化语言
const localeStore = useLocaleStore()
const elementLocale = elementLocales[localeStore.locale] || zhCn
app.use(ElementPlus, { locale: elementLocale })

// 初始化主题
const themeStore = useThemeStore()
themeStore.initTheme()

app.mount('#app')
