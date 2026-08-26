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
import { ElDatePicker, ElMessage } from 'element-plus'
import { Loading } from '@element-plus/icons-vue'
import type { EChartsOption } from 'echarts'
import type { EChartsType } from 'echarts/core'
import chinaMap from 'china-map-geojson/lib/china'
import { fetchConfig } from '@/api/config'
import { getMapping, getProxyMapping } from '@/api/mapping'
import { useSidebarStore } from '@/stores/sidebar'
import { useThemeStore } from '@/stores/theme'
import { displayName } from '@/utils/format'
import { echarts } from '@/utils/echartsMap'
import type { ChinaMapData, Config } from '@/types'

const { t, locale } = useI18n()
const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
const selectedDate = ref('')
const currentBaseUrl = ref('')
const currentAgent = ref('')
const chartRef = ref<HTMLDivElement>()
const sidebarStore = useSidebarStore()
const themeStore = useThemeStore()
const isMapReady = ref(false)
let chart: EChartsType | null = null
let isUnmounted = false
let latestData: ChinaMapData | null = null
let configRequestId = 0
let mappingRequestId = 0
let resizeTimer: number | null = null

const currentAgentName = computed(() => {
  if (!currentAgent.value) {
    return displayName(config.value?.Name || 'SmartPingNext')
  }

  const matched = agents.value.find((agent) => agent.addr === currentAgent.value)
  return matched ? displayName(matched.name) : currentAgent.value
})

const loadConfig = async () => {
  const requestId = ++configRequestId
  try {
    const cfg = await fetchConfig()
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    config.value = cfg
    currentAgent.value = cfg.Addr

    agents.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    await loadMappingData()
  } catch (error) {
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
    ElMessage.error(t('common.configLoadFailedNetwork'))
  }
}

const loadMappingData = async () => {
  if (isUnmounted) {
    return
  }

  const requestId = ++mappingRequestId
  const baseUrl = currentBaseUrl.value
  const date = selectedDate.value
  try {
    const data = baseUrl ? await getProxyMapping(baseUrl, date) : await getMapping(date)

    if (isUnmounted || requestId !== mappingRequestId || !isMapReady.value) {
      return
    }

    latestData = data
    updateChart(data)
  } catch (error) {
    if (isUnmounted || requestId !== mappingRequestId) {
      return
    }
    console.error('加载地图数据失败', error)
    ElMessage.error(t('common.dataLoadFailed'))
  } finally {
    if (!isUnmounted && requestId === mappingRequestId) {
      agents.value.forEach((agent) => {
        agent.loading = false
      })
    }
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agents.value.forEach((item) => {
    item.loading = false
  })
  agent.loading = true
  currentAgent.value = agent.addr
  currentBaseUrl.value = `http://${agent.addr}:${config.value?.Port}`
  await loadMappingData()
}

const updateChart = (data: ChinaMapData) => {
  if (!chart || !isMapReady.value) {
    return
  }

  const styles = window.getComputedStyle(document.documentElement)
  const textColor = styles.getPropertyValue('--color-text-primary').trim() || '#303133'
  const secondaryTextColor =
    styles.getPropertyValue('--color-text-secondary').trim() || '#606266'

  const option: EChartsOption = {
    backgroundColor: 'transparent',
    title: {
      text: data.text,
      subtext: data.subtext,
      left: 'center',
      textStyle: {
        color: textColor,
        fontWeight: 700,
        fontSize: 20
      },
      subtextStyle: {
        color: secondaryTextColor
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
        color: secondaryTextColor
      }
    },
    visualMap: {
      min: 0,
      max: 200,
      left: 24,
      bottom: 24,
      text: [t('common.high'), t('common.low')],
      textStyle: {
        color: secondaryTextColor
      },
      pieces: [
        { gt: 200, color: '#dc5c65' },
        { gt: 150, lte: 200, color: '#de9b19' },
        { gt: 100, lte: 150, color: '#99c33c' },
        { gt: 50, lte: 100, color: '#2f7df6' },
        { lte: 50, color: '#1f9f6a' }
      ]
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

const initChart = () => {
  if (!chartRef.value) {
    return
  }

  chart = echarts.init(chartRef.value)
  echarts.registerMap('china', chinaMap)
  isMapReady.value = true
}

const handleResize = () => {
  chart?.resize()
}

watch(
  () => sidebarStore.isCollapsed,
  () => {
    if (resizeTimer !== null) {
      window.clearTimeout(resizeTimer)
    }
    resizeTimer = window.setTimeout(() => {
      resizeTimer = null
      if (!isUnmounted) {
        handleResize()
      }
    }, 400)
  }
)

watch(
  () => themeStore.theme,
  () => {
    if (latestData) {
      updateChart(latestData)
    }
  }
)

watch(locale, () => {
  if (latestData) {
    updateChart(latestData)
  }
})

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
  initChart()
  if (isUnmounted) {
    return
  }
  await loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configRequestId++
  mappingRequestId++
  if (resizeTimer !== null) {
    window.clearTimeout(resizeTimer)
    resizeTimer = null
  }
  isMapReady.value = false
  latestData = null
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
