import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export type ThemeMode = 'light' | 'dark'

const VALID_THEMES: ThemeMode[] = ['light', 'dark']

function getStoredTheme(): ThemeMode {
  const stored = localStorage.getItem('theme')
  return VALID_THEMES.includes(stored as ThemeMode) ? (stored as ThemeMode) : 'light'
}

function applyTheme(theme: ThemeMode): void {
  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.classList.toggle('dark', theme === 'dark')
}

export const useThemeStore = defineStore('theme', () => {
  const theme = ref<ThemeMode>(getStoredTheme())

  const setTheme = (newTheme: ThemeMode) => {
    theme.value = newTheme
    localStorage.setItem('theme', newTheme)
  }

  const toggleTheme = () => {
    setTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  const initTheme = () => {
    applyTheme(theme.value)
  }

  // 监听主题变化
  watch(theme, (newTheme) => {
    applyTheme(newTheme)
  })

  return {
    theme,
    setTheme,
    toggleTheme,
    initTheme
  }
})
