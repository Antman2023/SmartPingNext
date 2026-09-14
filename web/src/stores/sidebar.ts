import { defineStore } from 'pinia'
import { ref } from 'vue'

function getStoredCollapse(): boolean {
  try {
    return localStorage.getItem('sidebar-collapsed') === 'true'
  } catch {
    return false
  }
}

export const useSidebarStore = defineStore('sidebar', () => {
  const isCollapsed = ref(getStoredCollapse())
  const isMobileOpen = ref(false)

  const toggleCollapse = () => {
    isCollapsed.value = !isCollapsed.value
    try {
      localStorage.setItem('sidebar-collapsed', String(isCollapsed.value))
    } catch {
      // Keep the control usable when browser storage is unavailable.
    }
  }

  const toggleMobile = () => {
    isMobileOpen.value = !isMobileOpen.value
  }

  const closeMobile = () => {
    isMobileOpen.value = false
  }

  return {
    isCollapsed,
    isMobileOpen,
    toggleCollapse,
    toggleMobile,
    closeMobile
  }
})
