<template>
  <div ref="chartRef" class="ping-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { PingLogData } from '@/types'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'
import { getPingChartOption } from '@/utils/charts'

const props = defineProps<{
  data: PingLogData | null
  height?: number
}>()

const chartRef = ref<HTMLDivElement>()
const themeStore = useThemeStore()
const sidebarStore = useSidebarStore()
let chart: echarts.ECharts | null = null
let isUnmounted = false
const resizeTimers: number[] = []

const saveAsImage = () => {
  if (chart) {
    const url = chart.getDataURL({
      type: 'png',
      pixelRatio: 2,
      backgroundColor: themeStore.theme === 'dark' ? '#1a1a1a' : '#fff'
    })
    const link = document.createElement('a')
    link.download = `smartping-chart-${Date.now()}.png`
    link.href = url
    link.click()
  }
}

defineExpose({
  saveAsImage
})

const getChartOption = (): EChartsOption => {
  const isDark = themeStore.theme === 'dark'
  return getPingChartOption(props.data, isDark, true)
}

const safeSetTimeout = (callback: () => void, delay: number) => {
  const timer = window.setTimeout(() => {
    if (!isUnmounted) {
      callback()
    }
  }, delay)
  resizeTimers.push(timer)
  return timer
}

const clearAllTimers = () => {
  resizeTimers.forEach(timer => window.clearTimeout(timer))
  resizeTimers.length = 0
}

const initChart = () => {
  if (!chartRef.value) return
  if (chart) {
    chart.dispose()
  }
  chart = echarts.init(chartRef.value)
  chart.setOption(getChartOption())
  safeSetTimeout(() => chart?.resize(), 0)
}

const updateChart = () => {
  if (!chart) {
    initChart()
    return
  }
  chart.setOption(getChartOption(), true)
  safeSetTimeout(() => chart?.resize(), 0)
}

const handleResize = () => {
  chart?.resize()
}

watch(() => props.data?.lastcheck, async () => {
  await nextTick()
  updateChart()
})

watch(() => themeStore.theme, async () => {
  await nextTick()
  updateChart()
})

watch(() => sidebarStore.isCollapsed, () => {
  safeSetTimeout(() => handleResize(), 500)
})

onMounted(async () => {
  await nextTick()
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  isUnmounted = true
  clearAllTimers()
  chart?.dispose()
  chart = null
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.ping-chart {
  width: 100%;
  min-height: 200px;
}
</style>
