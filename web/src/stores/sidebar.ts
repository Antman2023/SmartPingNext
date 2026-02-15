import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export const useSidebarStore = defineStore('sidebar', () => {
  const isCollapsed = ref(
    localStorage.getItem('sidebar-collapsed') === 'true'
  )

  const toggleCollapse = () => {
    isCollapsed.value = !isCollapsed.value
  }

  watch(isCollapsed, (val) => {
    localStorage.setItem('sidebar-collapsed', String(val))
  })

  return {
    isCollapsed,
    toggleCollapse
  }
})
