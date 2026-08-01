<template>
  <div class="page-shell mapping-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('mapping.title') }}</span>
        <h1 class="page-title">{{ displayName(config?.Name || 'SmartPingNext') }}</h1>
        <p class="page-subtitle">{{ $t('mapping.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.sources') }}</span>
            <strong class="page-kpi__value">{{ agents.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.selectedDate') }}</span>
            <strong class="page-kpi__value mapping-view__date-kpi">{{
              selectedDate || '--'
            }}</strong>
          </div>
        </div>

        <section class="surface-panel surface-panel--tight">
          <div class="control-row">
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
        </section>
      </div>
    </div>

    <div class="page-frame">
      <div class="page-main">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('mapping.title') }}</h2>
              <p class="surface-panel__description">
                {{ currentAgentName }} · {{ selectedDate || $t('common.status') }}
              </p>
            </div>
            <div class="metric-inline">
              <strong>{{ agents.length }}</strong>
              <span>{{ $t('common.sources') }}</span>
            </div>
          </div>

          <div ref="chartRef" class="mapping-view__map"></div>
        </section>
      </div>

      <aside class="page-aside page-aside--narrow">
        <section class="surface-panel surface-panel--soft">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('common.nodeList') }}</h2>
              <p class="surface-panel__description">
                {{ $t('common.sources') }} · {{ agents.length }}
              </p>
            </div>
          </div>

          <div class="list-stack">
            <button
              v-for="agent in agents"
              :key="agent.addr"
              type="button"
              class="list-row mapping-view__agent"
              :class="{ 'is-active': currentAgent === agent.addr }"
              @click="switchAgent(agent)"
            >
              <div class="list-row__meta">
                <el-icon v-if="agent.loading" class="is-loading"><Loading /></el-icon>
                <div v-else class="mapping-view__agent-dot"></div>
                <div class="list-row__text">
                  <span class="list-row__title">{{ displayName(agent.name) }}</span>
                  <span class="list-row__caption">{{ agent.addr }}</span>
                </div>
              </div>
            </button>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import * as echarts from 'echarts'
import type { EChartsOption } from 'echarts'
import { fetchConfig } from '@/api/config'
import { getMapping, getProxyMapping } from '@/api/mapping'
import { useSidebarStore } from '@/stores/sidebar'
import { displayName } from '@/utils/format'
import type { ChinaMapData, Config } from '@/types'

const { t } = useI18n()
const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
const selectedDate = ref('')
const currentBaseUrl = ref('')
const currentAgent = ref('')
const chartRef = ref<HTMLDivElement>()
const sidebarStore = useSidebarStore()
const isMapReady = ref(false)
let chart: echarts.ECharts | null = null
let isUnmounted = false
const mapAbortController = new AbortController()

const currentAgentName = computed(() => {
  if (!currentAgent.value) {
    return displayName(config.value?.Name || 'SmartPingNext')
  }

  const matched = agents.value.find((agent) => agent.addr === currentAgent.value)
  return matched ? displayName(matched.name) : currentAgent.value
})

const loadConfig = async () => {
  try {
    const cfg = await fetchConfig()
    config.value = cfg
    currentAgent.value = cfg.Addr

    agents.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    await loadMappingData()
  } catch (error) {
    console.error('加载配置失败', error)
    ElMessage.error('加载配置失败，请检查网络连接')
  }
}

const loadMappingData = async () => {
  if (isUnmounted) {
    return
  }

  try {
    const data = currentBaseUrl.value
      ? await getProxyMapping(currentBaseUrl.value, selectedDate.value)
      : await getMapping(selectedDate.value)

    if (!isMapReady.value) {
      return
    }

    updateChart(data)
  } catch (error) {
    console.error('加载地图数据失败', error)
    ElMessage.error('加载地图数据失败')
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  currentAgent.value = agent.addr
  currentBaseUrl.value = `http://${agent.addr}:${config.value?.Port}`
  await loadMappingData()
  agent.loading = false
}

const updateChart = (data: ChinaMapData) => {
  if (!chart || !isMapReady.value) {
    return
  }

  const option: EChartsOption = {
    backgroundColor: 'transparent',
    title: {
      text: data.text,
      subtext: data.subtext,
      left: 'center',
      textStyle: {
        color: 'var(--color-text-primary)',
        fontWeight: 700,
        fontSize: 20
      },
      subtextStyle: {
        color: 'var(--color-text-secondary)'
      }
    },
    tooltip: {
      trigger: 'item',
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      formatter: (params: any) => {
        const delay = Number(params.value)
        const delayText = Number.isFinite(delay) ? `${delay.toFixed(2)}ms` : '--'
        return `${params.name}<br/>${params.seriesName}: ${delayText}`
      }
    },
    legend: {
      orient: 'vertical',
      right: 24,
      top: 80,
      data: [t('mapping.telecom'), t('mapping.unicom'), t('mapping.mobile')],
      textStyle: {
        color: 'var(--color-text-secondary)'
      }
    },
    visualMap: {
      min: 0,
      max: 200,
      left: 24,
      bottom: 24,
      text: [t('common.high'), t('common.low')],
      textStyle: {
        color: 'var(--color-text-secondary)'
      },
      pieces: [
        { gt: 200, color: '#dc5c65' },
        { gt: 150, lte: 200, color: '#de9b19' },
        { gt: 100, lte: 150, color: '#99c33c' },
        { gt: 50, lte: 100, color: '#2f7df6' },
        { lte: 50, color: '#1f9f6a' }
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
        data: data.avgdelay.ctcc,
        emphasis: {
          label: {
            color: '#ffffff'
          }
        }
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

const mapUrlCandidates = [
  import.meta.env.VITE_MAP_URL?.trim(),
  'https://cdn.jsdelivr.net/npm/echarts-map@3.0.1/json/china.json',
  'https://fastly.jsdelivr.net/npm/echarts-map@3.0.1/json/china.json',
  'https://unpkg.com/echarts-map@3.0.1/json/china.json',
  'https://geo.datav.aliyun.com/areas_v3/bound/100000_full.json'
].filter((url, index, arr): url is string => !!url && arr.indexOf(url) === index)

const initChart = async () => {
  if (!chartRef.value) {
    return
  }

  chart = echarts.init(chartRef.value)

  const loadErrors: string[] = []
  for (const mapUrl of mapUrlCandidates) {
    try {
      const response = await fetch(mapUrl, { signal: mapAbortController.signal })
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`)
      }
      const chinaJson = await response.json()
      if (isUnmounted) {
        return
      }
      echarts.registerMap('china', chinaJson)
      isMapReady.value = true
      return
    } catch (error) {
      if (mapAbortController.signal.aborted) {
        return
      }
      loadErrors.push(`${mapUrl} -> ${String(error)}`)
    }
  }

  console.error('加载地图失败', loadErrors)
  ElMessage.error('加载地图资源失败，请配置可访问的 VITE_MAP_URL')
}

const handleResize = () => {
  chart?.resize()
}

watch(
  () => sidebarStore.isCollapsed,
  () => {
    setTimeout(() => handleResize(), 400)
  }
)

const saveMapImage = () => {
  if (!chart) {
    return
  }

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

onMounted(async () => {
  window.addEventListener('resize', handleResize)
  await initChart()
  if (isUnmounted) {
    return
  }
  await loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  mapAbortController.abort()
  isMapReady.value = false
  chart?.dispose()
  chart = null
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped lang="scss">
.mapping-view__date-kpi {
  font-size: 14px;
}

.mapping-view__map {
  min-height: 520px;
  height: min(72vh, 760px);
}

.mapping-view__agent {
  width: 100%;
  border: none;
  cursor: pointer;
  font: inherit;
}

.mapping-view__agent-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-primary);
  animation: subtle-pulse 2.4s ease infinite;
}

@media (max-width: 900px) {
  .mapping-view__map {
    min-height: 420px;
    height: 58vh;
  }
}
</style>
