<template>
  <div class="mapping-view">
    <div class="mapping-header">
      <h2>{{ config?.Name || 'SmartPingNext' }} - {{ $t('mapping.title') }}</h2>
      <div class="mapping-actions">
        <el-date-picker
          v-model="selectedDate"
          type="datetime"
          :placeholder="$t('mapping.selectTime')"
          format="YYYY-MM-DD HH:mm"
          value-format="YYYY-MM-DD HH:mm"
          @change="loadMappingData"
        />
        <el-button @click="saveMapImage">{{ $t('common.saveImage') }}</el-button>
      </div>
    </div>

    <div class="mapping-content">
      <div ref="chartRef" class="map-container"></div>

      <div class="mapping-sidebar">
        <el-card>
          <template #header>
            <span>{{ $t('common.nodeList') }}</span>
          </template>
          <div class="agent-list">
            <div
              v-for="agent in agents"
              :key="agent.addr"
              class="agent-item"
              @click="switchAgent(agent)"
            >
              <el-icon v-if="agent.loading" class="is-loading"><Loading /></el-icon>
              <span>{{ agent.name }}</span>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import { getConfig } from '@/api/topology'
import { getMapping, getProxyMapping } from '@/api/mapping'
import type { Config, ChinaMapData } from '@/types'
import type { EChartsOption } from 'echarts'
import { useSidebarStore } from '@/stores/sidebar'

const { t } = useI18n()
const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
const selectedDate = ref('')
const currentBaseUrl = ref('')
const chartRef = ref<HTMLDivElement>()
const sidebarStore = useSidebarStore()
let chart: echarts.ECharts | null = null

const loadConfig = async () => {
  try {
    const cfg = await getConfig()
    config.value = cfg

    agents.value = Object.values(cfg.Network)
      .filter(n => n.Smartping)
      .map(n => ({ name: n.Name, addr: n.Addr, loading: false }))

    await loadMappingData()
  } catch (e) {
    console.error('加载配置失败', e)
    ElMessage.error('加载配置失败，请检查网络连接')
  }
}

const loadMappingData = async () => {
  try {
    let data: ChinaMapData
    if (currentBaseUrl.value) {
      data = await getProxyMapping(currentBaseUrl.value, selectedDate.value)
    } else {
      data = await getMapping(selectedDate.value)
    }

    updateChart(data)
  } catch (e) {
    console.error('加载地图数据失败', e)
    ElMessage.error('加载地图数据失败')
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  currentBaseUrl.value = `http://${agent.addr}:${config.value?.Port}`
  await loadMappingData()
  agent.loading = false
}

const updateChart = (data: ChinaMapData) => {
  if (!chart) return

  const option: EChartsOption = {
    backgroundColor: 'transparent',
    title: {
      text: data.text,
      subtext: data.subtext,
      left: 'center',
      textStyle: {
        color: 'var(--color-text-primary)'
      }
    },
    tooltip: {
      trigger: 'item',
      formatter: (params: any) => {
        return `${params.name}<br/>${params.seriesName}: ${parseFloat(params.value).toFixed(2)}ms`
      }
    },
    legend: {
      orient: 'vertical',
      left: 'left',
      data: [t('mapping.telecom'), t('mapping.unicom'), t('mapping.mobile')],
      textStyle: {
        color: 'var(--color-text-secondary)'
      }
    },
    visualMap: {
      min: 0,
      max: 200,
      left: 'left',
      bottom: 20,
      text: [t('common.high'), t('common.low')],
      pieces: [
        { gt: 200, color: '#E0022B' },
        { gt: 150, lte: 200, color: '#E09107' },
        { gt: 100, lte: 150, color: '#A3E00B' },
        { gt: 50, lte: 100, color: 'Green' },
        { lte: 50, color: 'DarkGreen' }
      ]
    },
    toolbox: {
      show: false
    },
    series: [
      {
        name: t('mapping.telecom'),
        type: 'map',
        map: 'china',
        data: data.avgdelay.ctcc
      },
      {
        name: t('mapping.unicom'),
        type: 'map',
        map: 'china',
        data: data.avgdelay.cucc
      },
      {
        name: t('mapping.mobile'),
        type: 'map',
        map: 'china',
        data: data.avgdelay.cmcc
      }
    ]
  }

  chart.setOption(option)
}

const MAP_URL = import.meta.env.VITE_MAP_URL || 'https://geo.datav.aliyun.com/areas_v3/bound/100000_full.json'

const initChart = async () => {
  if (!chartRef.value) return

  chart = echarts.init(chartRef.value)

  try {
    const chinaJson = await fetch(MAP_URL).then(res => res.json())
    echarts.registerMap('china', chinaJson)
  } catch (e) {
    console.error('加载地图失败', e)
    ElMessage.error('加载地图资源失败，请检查网络连接')
  }
}

const handleResize = () => {
  chart?.resize()
}

watch(() => sidebarStore.isCollapsed, () => {
  setTimeout(() => handleResize(), 500)
})

const saveMapImage = () => {
  if (chart) {
    const url = chart.getDataURL({
      type: 'png',
      pixelRatio: 2,
      backgroundColor: '#fff'
    })
    const link = document.createElement('a')
    link.download = `smartping-map-${Date.now()}.png`
    link.href = url
    link.click()
  }
}

onMounted(async () => {
  await initChart()
  await loadConfig()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  chart?.dispose()
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped lang="scss">
.mapping-view {
  height: 100%;
  overflow: hidden;
}

.mapping-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.mapping-actions {
  display: flex;
  align-items: center;
  gap: 12px;
}

.mapping-content {
  display: flex;
  gap: 20px;
  height: calc(100vh - 160px);
  overflow: hidden;
}

.map-container {
  flex: 1;
  min-width: 0;
  height: 100%;
  background-color: var(--color-bg-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
}

.mapping-sidebar {
  width: 200px;
  flex-shrink: 0;
}

.agent-list {
  .agent-item {
    padding: 10px 12px;
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: background-color 0.2s;
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--color-text-primary);

    &:hover {
      background-color: var(--color-bg-secondary);
    }
  }
}

@media (max-width: 768px) {
  .mapping-content {
    flex-direction: column;
  }

  .mapping-sidebar {
    width: 100%;
  }
}
</style>
