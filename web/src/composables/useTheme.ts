import { storeToRefs } from 'pinia'
import { useThemeStore, type ThemePreference } from '@/stores/theme'

export function useTheme() {
  const themeStore = useThemeStore()
  const { theme, preference } = storeToRefs(themeStore)

  return {
    theme,
    preference,
    toggleTheme: () => themeStore.toggleTheme(),
    setTheme: (theme: ThemePreference) => themeStore.setTheme(theme)
  }
}
