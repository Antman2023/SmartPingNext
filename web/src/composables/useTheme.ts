import { useThemeStore, type ThemePreference } from '@/stores/theme'

export function useTheme() {
  const themeStore = useThemeStore()

  return {
    theme: themeStore.theme,
    preference: themeStore.preference,
    toggleTheme: () => themeStore.toggleTheme(),
    setTheme: (theme: ThemePreference) => themeStore.setTheme(theme)
  }
}
