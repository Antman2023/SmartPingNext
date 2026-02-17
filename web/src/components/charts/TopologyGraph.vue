<template>
  <div ref="chartRef" class="topology-chart" :style="{ height: height + 'px' }"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { useThemeStore } from '@/stores/theme'
import { useSidebarStore } from '@/stores/sidebar'

interface Node {
  name: string
  color: string
}

interface Link {
  source: string
  target: string
  color: string
  curveness: number
}

const props = defineProps<{
  nodes: Node[]
  links: Link[]
  symbolSize?: number
  lineWidth?: number
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
    tooltip: { show: true },
    animationDurationUpdate: 1500,
    animationEasingUpdate: 'quinticInOut',
    series: [
      {
        type: 'graph',
        layout: 'circular',
        symbolSize: props.symbolSize || 50,
        focusNodeAdjacency: true,
        roam: true,
        label: {
          show: true,
          color: isDark ? '#e5eaf3' : '#303133'
        },
        edgeSymbol: ['circle', 'arrow'],
        edgeSymbolSize: [3, 15],
        edgeLabel: {
          fontSize: 12
        },
        data: props.nodes.map(node => ({
          name: node.name,
          draggable: true,
          itemStyle: {
            color: node.color
          }
        })),
        links: props.links.map(link => ({
          source: link.source,
          target: link.target,
          lineStyle: {
            curveness: link.curveness,
            color: link.color,
            width: props.lineWidth || 2
          }
        })),
        lineStyle: {
          opacity: 0.9,
          width: props.lineWidth || 2,
          curveness: 0
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

watch(() => [props.nodes, props.links], updateChart, { deep: true })
watch(() => themeStore.theme, updateChart)
watch(() => sidebarStore.isCollapsed, () => {
  setTimeout(() => handleResize(), 500)
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
.topology-chart {
  width: 100%;
}
</style>
