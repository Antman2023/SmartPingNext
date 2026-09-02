import { createApp } from 'vue'
import { createPinia } from 'pinia'
import './plugins/elementPlusStyles'

import App from './App.vue'
import router from './router'
import { installElementPlus } from './plugins/elementPlus'
import { useThemeStore } from './stores/theme'
import { useLocaleStore } from './stores/locale'
import i18n from './locales'

import './assets/styles/global.scss'

const app = createApp(App)
installElementPlus(app)

const pinia = createPinia()
app.use(pinia)
app.use(router)

// 初始化语言
const localeStore = useLocaleStore()
const initialLocale = localeStore.locale
document.documentElement.lang = initialLocale

// 设置 i18n 初始语言
i18n.global.locale.value = initialLocale

app.use(i18n)

// 监听语言变化，同步更新 vue-i18n
localeStore.$subscribe((_mutation, state) => {
  i18n.global.locale.value = state.locale
  document.documentElement.lang = state.locale
})

// 初始化主题
const themeStore = useThemeStore()
themeStore.initTheme()

app.mount('#app')
