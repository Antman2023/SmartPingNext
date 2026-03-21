import { onMounted, onUnmounted, ref } from 'vue'

export function useViewportCompact(breakpoint = 960) {
  const isCompact = ref(typeof window !== 'undefined' ? window.innerWidth < breakpoint : false)

  const updateViewport = () => {
    isCompact.value = window.innerWidth < breakpoint
  }

  onMounted(() => {
    updateViewport()
    window.addEventListener('resize', updateViewport)
  })

  onUnmounted(() => {
    window.removeEventListener('resize', updateViewport)
  })

  return {
    isCompact
  }
}
