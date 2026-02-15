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

// 暴露保存图片方法
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
      top: 0,
      textStyle: {
        color: isDark ? '#a3a6ad' : '#606266'
      }
    },
    grid: {
      left: '3%',
      right: '3%',
      top: 30,
      bottom: 50,
      containLabel: true
    },
    dataZoom: [{
      type: 'slider',
      bottom: 10,
      borderColor: isDark ? '#4c4d4f' : '#dcdfe6',
      backgroundColor: isDark ? 'rgba(30,30,30,0.9)' : 'rgba(248,248,248,0.9)',
      fillerColor: isDark ? 'rgba(64,158,255,0.2)' : 'rgba(64,158,255,0.15)',
      handleStyle: {
        color: isDark ? '#4c4d4f' : '#fff',
        borderColor: isDark ? '#6c6e72' : '#909399'
      },
      moveHandleStyle: {
        color: isDark ? '#4c4d4f' : '#ddd'
      },
      emphasis: {
        handleStyle: {
          borderColor: '#409eff',
          color: isDark ? '#4c4d4f' : '#fff'
        },
        moveHandleStyle: {
          color: '#409eff'
        },
        handleLabel: {
          show: true
        }
      },
      handleLabel: {
        show: true
      },
      selectedDataBackground: {
        lineStyle: { color: '#409eff' },
        areaStyle: { color: 'rgba(64,158,255,0.2)' }
      },
      dataBackground: {
        lineStyle: { color: isDark ? '#6c6e72' : '#909399' },
        areaStyle: { color: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(64,158,255,0.1)' }
      },
      textStyle: {
        color: isDark ? '#a3a6ad' : '#606266'
      }
    }],
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
          const time = value.length >= 16 ? value.substring(11, 16) : value
          const date = value.length >= 10 ? value.substring(5, 10) : '' // MM-DD

          // 检查是否是日期变化点（与前一个数据点日期不同）
          const lastcheck = props.data?.lastcheck || []
          const prevValue = index > 0 ? lastcheck[index - 1] : ''
          const prevDate = prevValue && prevValue.length >= 10 ? prevValue.substring(5, 10) : ''

          // 如果是第一个点或者日期变化了，显示日期
          if (date && (index === 0 || date !== prevDate)) {
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
