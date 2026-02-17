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

const pinia = createPinia()
app.use(pinia)
app.use(router)

// 初始化语言
const localeStore = useLocaleStore()
const initialLocale = localeStore.locale

// 设置 i18n 初始语言
i18n.global.locale.value = initialLocale

app.use(i18n)
app.use(ElementPlus, { locale: initialLocale === 'en-US' ? en : zhCn })

// 监听语言变化，同步更新 vue-i18n
localeStore.$subscribe((_mutation, state) => {
  i18n.global.locale.value = state.locale
})

// 初始化主题
const themeStore = useThemeStore()
themeStore.initTheme()

app.mount('#app')
