<template>
  <div ref="chartRef" class="ping-mini-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EChartsOption } from 'echarts'
import type { EChartsType } from 'echarts/core'
import type { PingLogData } from '@/types'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'
import { getPingMiniChartOption } from '@/utils/charts'
import { debounce } from '@/utils/debounce'
import { echarts } from '@/utils/echartsLine'

const props = defineProps<{
  data: PingLogData | null
  height?: number
}>()

const chartRef = ref<HTMLDivElement>()
const { t, locale } = useI18n()
const themeStore = useThemeStore()
const sidebarStore = useSidebarStore()
let chart: EChartsType | null = null
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
  return getPingMiniChartOption(props.data, isDark, {
    maxDelay: t('charts.maxDelay'),
    averageDelay: t('charts.averageDelay'),
    minDelay: t('charts.minDelay'),
    lossRate: t('charts.lossRate'),
    latency: t('charts.latency'),
    loss: t('charts.loss')
  })
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

const handleResize = debounce(() => {
  chart?.resize()
}, 200)

watch(() => props.data?.lastcheck, updateChart)
watch(() => themeStore.theme, updateChart)
watch(locale, updateChart)
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
