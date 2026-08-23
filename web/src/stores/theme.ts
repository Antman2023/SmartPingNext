import { defineStore } from 'pinia'
import { computed, ref, watch } from 'vue'

export type ThemeMode = 'light' | 'dark'
export type ThemePreference = ThemeMode | 'system'

const VALID_PREFERENCES: ThemePreference[] = ['system', 'light', 'dark']

function getStoredPreference(): ThemePreference {
  try {
    const stored = localStorage.getItem('theme')
    return VALID_PREFERENCES.includes(stored as ThemePreference)
      ? (stored as ThemePreference)
      : 'system'
  } catch {
    return 'system'
  }
}

function storePreference(preference: ThemePreference): void {
  try {
    localStorage.setItem('theme', preference)
  } catch {
    // The resolved theme still works when browser storage is unavailable.
  }
}

function getSystemTheme(mediaQuery?: MediaQueryList): ThemeMode {
  const query = mediaQuery || window.matchMedia('(prefers-color-scheme: dark)')
  return query.matches ? 'dark' : 'light'
}

function applyTheme(theme: ThemeMode): void {
  document.documentElement.setAttribute('data-theme', theme)
  document.documentElement.classList.toggle('dark', theme === 'dark')
  document.documentElement.style.colorScheme = theme
}

export const useThemeStore = defineStore('theme', () => {
  const preference = ref<ThemePreference>(getStoredPreference())
  const systemTheme = ref<ThemeMode>(getSystemTheme())
  const theme = computed<ThemeMode>(() =>
    preference.value === 'system' ? systemTheme.value : preference.value
  )
  let initialized = false

  const setTheme = (newPreference: ThemePreference) => {
    preference.value = newPreference
  }

  const toggleTheme = () => {
    setTheme(theme.value === 'light' ? 'dark' : 'light')
  }

  const initTheme = () => {
    if (!initialized) {
      const mediaQuery = window.matchMedia('(prefers-color-scheme: dark)')
      systemTheme.value = getSystemTheme(mediaQuery)
      mediaQuery.addEventListener('change', (event) => {
        systemTheme.value = event.matches ? 'dark' : 'light'
      })
      initialized = true
    }
    applyTheme(theme.value)
  }

  watch(theme, (newTheme) => {
    applyTheme(newTheme)
  })

  watch(preference, (newPreference) => {
    storePreference(newPreference)
  })

  return {
    preference,
    theme,
    setTheme,
    toggleTheme,
    initTheme
  }
})
