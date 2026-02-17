<template>
  <div ref="chartRef" class="ping-mini-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { PingLogData } from '@/types'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'
import { getPingMiniChartOption } from '@/utils/charts'

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

const getChartOption = (): EChartsOption => {
  const isDark = themeStore.theme === 'dark'
  return getPingMiniChartOption(props.data, isDark)
}

const initChart = () => {
  if (!chartRef.value) return
  chart = echarts.init(chartRef.value)
  chart.setOption(getChartOption())
}

const updateChart = () => {
  if (!chart) return
  chart.setOption(getChartOption())
}

const handleResize = () => {
  chart?.resize()
}

watch(() => props.data?.lastcheck, updateChart)
watch(() => themeStore.theme, updateChart)
watch(() => sidebarStore.isCollapsed, () => {
  safeSetTimeout(() => handleResize(), 500)
})

onMounted(() => {
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
.ping-mini-chart {
  width: 100%;
  min-height: 100px;
}
</style>
