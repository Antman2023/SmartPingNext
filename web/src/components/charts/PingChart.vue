<template>
  <div ref="chartRef" class="ping-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import type { EChartsOption } from 'echarts'
import type { EChartsType } from 'echarts/core'
import type { PingLogData } from '@/types'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'
import { getPingChartOption } from '@/utils/charts'
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
const resizeTimers = new Set<number>()

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
  const compact = (chartRef.value?.clientWidth || Number.POSITIVE_INFINITY) < 480
  return getPingChartOption(
    props.data,
    isDark,
    {
      maxDelay: t('charts.maxDelay'),
      averageDelay: t('charts.averageDelay'),
      minDelay: t('charts.minDelay'),
      lossRate: t('charts.lossRate'),
      latency: t('charts.latency'),
      loss: t('charts.loss')
    },
    true,
    compact
  )
}

const safeSetTimeout = (callback: () => void, delay: number) => {
  const timer = window.setTimeout(() => {
    resizeTimers.delete(timer)
    if (!isUnmounted) {
      callback()
    }
  }, delay)
  resizeTimers.add(timer)
  return timer
}

const clearAllTimers = () => {
  resizeTimers.forEach(timer => window.clearTimeout(timer))
  resizeTimers.clear()
}

const initChart = () => {
  if (isUnmounted || !chartRef.value) return
  if (chart) {
    chart.dispose()
  }
  chart = echarts.init(chartRef.value)
  chart.setOption(getChartOption())
  safeSetTimeout(() => chart?.resize(), 0)
}

const updateChart = () => {
  if (isUnmounted) return
  if (!chart) {
    initChart()
    return
  }
  chart.setOption(getChartOption(), true)
  safeSetTimeout(() => chart?.resize(), 0)
}

const handleResize = debounce(() => {
  chart?.resize()
  chart?.setOption(getChartOption(), true)
}, 200)

watch(() => props.data?.lastcheck, async () => {
  await nextTick()
  updateChart()
})

watch(() => themeStore.theme, async () => {
  await nextTick()
  updateChart()
})

watch(locale, async () => {
  await nextTick()
  updateChart()
})

watch(() => sidebarStore.isCollapsed, () => {
  safeSetTimeout(() => handleResize(), 500)
})

onMounted(async () => {
  await nextTick()
  if (isUnmounted) return
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  isUnmounted = true
  clearAllTimers()
  handleResize.cancel()
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
