import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type ThemeMode = 'light' | 'dark'

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<ThemeMode>(
    (localStorage.getItem('theme') as ThemeMode) || 'light'
  )

  const setTheme = (newTheme: ThemeMode) => {
    theme.value = newTheme
    document.documentElement.setAttribute('data-theme', newTheme)
    localStorage.setItem('theme', newTheme)
  }

  const toggleTheme = () => {
    setTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  const initTheme = () => {
    document.documentElement.setAttribute('data-theme', theme.value)
  }

  // 监听主题变化
  watch(theme, (newTheme) => {
    document.documentElement.setAttribute('data-theme', newTheme)
  })

  return {
    theme,
    setTheme,
    toggleTheme,
    initTheme
  }
})
