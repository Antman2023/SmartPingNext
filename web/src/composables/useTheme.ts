import { useThemeStore } from '@/stores/theme'

export function useTheme() {
  const themeStore = useThemeStore()

  const toggleTheme = () => {
    themeStore.toggleTheme()
  }

  const setTheme = (theme: 'light' | 'dark') => {
    themeStore.setTheme(theme)
  }

  const initTheme = () => {
    themeStore.initTheme()
  }

  return {
    theme: themeStore.theme,
    toggleTheme,
    setTheme,
    initTheme
  }
}
