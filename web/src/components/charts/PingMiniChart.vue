<template>
  <div ref="chartRef" class="ping-mini-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import type { PingLogData } from '@/types'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'

const props = defineProps<{
  data: PingLogData | null
  height?: number
}>()

const chartRef = ref<HTMLDivElement>()
const themeStore = useThemeStore()
const sidebarStore = useSidebarStore()
let chart: echarts.ECharts | null = null

const getChartOption = (): EChartsOption => {
  const isDark = themeStore.theme === 'dark'

  return {
    backgroundColor: 'transparent',
    grid: {
      left: 5,
      right: 5,
      top: 5,
      bottom: 20,
      containLabel: false
    },
    xAxis: {
      type: 'category',
      data: props.data?.lastcheck || [],
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        show: true,
        color: isDark ? '#6c6e72' : '#909399',
        fontSize: 10,
        interval: 'auto',
        formatter: (value: string) => {
          if (value && value.length >= 16) {
            return value.substring(11, 16)
          }
          return value
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        position: 'left',
        min: 0,
        max: function(value: { max: number }) {
          return Math.max(value.max * 1.1, 10)
        },
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { show: false },
        splitLine: { show: false }
      },
      {
        type: 'value',
        position: 'right',
        min: 0,
        max: 100,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: { show: false },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        type: 'line',
        data: props.data?.avgdelay || [],
        yAxisIndex: 0,
        smooth: false,
        symbol: 'none',
        lineStyle: {
          color: '#00CC66',
          width: 1.5
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(0, 204, 102, 0.3)' },
              { offset: 1, color: 'rgba(0, 204, 102, 0.05)' }
            ]
          }
        }
      },
      {
        type: 'line',
        data: props.data?.losspk || [],
        yAxisIndex: 1,
        smooth: false,
        symbol: 'none',
        lineStyle: {
          color: '#f56c6c',
          width: 1.5
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(245, 108, 108, 0.3)' },
              { offset: 1, color: 'rgba(245, 108, 108, 0.05)' }
            ]
          }
        }
      }
    ]
  }
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

watch(() => props.data, updateChart, { deep: true })
watch(() => themeStore.theme, updateChart)
watch(() => sidebarStore.isCollapsed, () => {
  // 等待 CSS 过渡完成 (0.3s)
  setTimeout(() => handleResize(), 350)
})

onMounted(() => {
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  chart?.dispose()
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped>
.ping-mini-chart {
  width: 100%;
  min-height: 100px;
}
</style>
