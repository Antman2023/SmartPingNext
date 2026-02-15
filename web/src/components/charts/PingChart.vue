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
    title: {
      text: '',
      left: 'center',
      textStyle: {
        color: isDark ? '#e5eaf3' : '#303133'
      }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: isDark ? '#252525' : '#fff',
      borderColor: isDark ? '#4c4d4f' : '#dcdfe6',
      textStyle: {
        color: isDark ? '#e5eaf3' : '#303133'
      },
      formatter: (params: any) => {
        let result = params[0].name + '<br/>'
        params.forEach((item: any) => {
          let value = item.value
          if (item.seriesName === '丢包率') {
            value = parseFloat(value).toFixed(0) + '%'
          } else {
            value = parseFloat(value).toFixed(2) + 'ms'
          }
          result += item.marker + item.seriesName + ': ' + value + '<br/>'
        })
        return result
      }
    },
    legend: {
      data: ['最大延迟', '平均延迟', '最小延迟', '丢包率'],
      selected: {
        '最大延迟': false,
        '最小延迟': false
      },
      textStyle: {
        color: isDark ? '#a3a6ad' : '#606266'
      }
    },
    toolbox: {
      feature: {
        saveAsImage: { pixelRatio: 2 }
      }
    },
    grid: {
      left: '3%',
      right: '4%',
      bottom: '12%',
      containLabel: true
    },
    dataZoom: [{}],
    xAxis: {
      type: 'category',
      data: props.data?.lastcheck || [],
      axisLine: {
        lineStyle: { color: isDark ? '#4c4d4f' : '#dcdfe6' }
      },
      axisLabel: {
        color: isDark ? '#a3a6ad' : '#606266',
        rotate: 0,
        interval: 'auto',
        formatter: (value: string, index: number) => {
          if (!value) return ''
          // 显示时间 HH:mm
          const time = value.length >= 16 ? value.substring(11, 16) : value
          // 每隔一定间隔显示日期（每小时第一个点显示日期）
          const dataLen = props.data?.lastcheck?.length || 0
          const interval = Math.max(Math.floor(dataLen / 8), 1)
          if (index % interval === 0 && value.length >= 10) {
            const date = value.substring(5, 10) // MM-DD
            return `{date|${date}}\n{time|${time}}`
          }
          return `{time|${time}}`
        },
        rich: {
          date: {
            color: isDark ? '#a3a6ad' : '#909399',
            fontSize: 11,
            padding: [0, 0, 2, 0]
          },
          time: {
            color: isDark ? '#cfd3dc' : '#606266',
            fontSize: 12
          }
        }
      },
      axisTick: {
        alignWithLabel: true
      }
    },
    yAxis: [
      {
        type: 'value',
        name: '延迟(ms)',
        position: 'left',
        nameTextStyle: { color: isDark ? '#a3a6ad' : '#606266' },
        axisLine: {
          lineStyle: { color: isDark ? '#4c4d4f' : '#dcdfe6' }
        },
        axisLabel: {
          color: isDark ? '#a3a6ad' : '#606266'
        },
        splitLine: {
          lineStyle: { color: isDark ? '#363637' : '#ebeef5' }
        }
      },
      {
        type: 'value',
        name: '丢包率(%)',
        min: 0,
        max: 100,
        position: 'right',
        nameTextStyle: { color: isDark ? '#a3a6ad' : '#606266' },
        axisLine: {
          lineStyle: { color: isDark ? '#4c4d4f' : '#dcdfe6' }
        },
        axisLabel: {
          color: isDark ? '#a3a6ad' : '#606266',
          formatter: '{value}%'
        },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: '最大延迟',
        type: 'line',
        data: props.data?.maxdelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#e6a23c' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '平均延迟',
        type: 'line',
        data: props.data?.avgdelay || [],
        animation: false,
        lineStyle: { width: 2 },
        itemStyle: { color: '#00CC66' },
        areaStyle: { opacity: 0.2 }
      },
      {
        name: '最小延迟',
        type: 'line',
        data: props.data?.mindelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#409eff' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '丢包率',
        type: 'line',
        yAxisIndex: 1,
        data: props.data?.losspk || [],
        animation: false,
        lineStyle: { width: 2 },
        itemStyle: { color: '#f56c6c' },
        areaStyle: { opacity: 0.2 }
      }
    ]
  }
}

const initChart = () => {
  if (!chartRef.value) return
  if (chart) {
    chart.dispose()
  }
  chart = echarts.init(chartRef.value)
  chart.setOption(getChartOption())
  // 初始化后立即 resize 确保尺寸正确
  setTimeout(() => chart?.resize(), 0)
}

const updateChart = () => {
  if (!chart) {
    initChart()
    return
  }
  chart.setOption(getChartOption(), true)
  setTimeout(() => chart?.resize(), 0)
}

const handleResize = () => {
  chart?.resize()
}

watch(() => props.data, async () => {
  await nextTick()
  updateChart()
}, { deep: true })

watch(() => themeStore.theme, async () => {
  await nextTick()
  updateChart()
})

watch(() => sidebarStore.isCollapsed, () => {
  // 等待 CSS 过渡完成 (0.3s)
  setTimeout(() => handleResize(), 350)
})

onMounted(async () => {
  await nextTick()
  initChart()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
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
