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
              :disabled="configLoading || !config"
              format="YYYY-MM-DD HH:mm"
              value-format="YYYY-MM-DD HH:mm"
              @change="loadMappingData"
            />
            <el-button
              :icon="Download"
              :disabled="!latestData || mappingLoading || !isMapReady"
              @click="saveMapImage"
            >
              {{ $t('common.saveImage') }}
            </el-button>
          </div>
        </section>

        <RefreshStatus
          class="surface-panel surface-panel--tight"
          :loading="configLoading || mappingLoading"
          :last-updated="lastUpdatedLabel"
          @refresh="refreshMapping"
        />
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

          <div
            v-loading="configLoading || mappingLoading || chartLoading"
            class="mapping-view__map-shell"
          >
            <div ref="chartRef" class="mapping-view__map"></div>

            <div v-if="chartError" class="empty-state mapping-view__state">
              <el-icon class="mapping-view__state-icon text-danger"><Warning /></el-icon>
              <span>{{ $t('mapping.chartLoadFailed') }}</span>
              <el-button size="small" @click="initChart">{{ $t('common.retry') }}</el-button>
            </div>

            <div v-else-if="configError && !config" class="empty-state mapping-view__state">
              <el-icon class="mapping-view__state-icon text-danger"><Warning /></el-icon>
              <span>{{ $t('common.configLoadFailedNetwork') }}</span>
              <el-button size="small" @click="loadConfig">{{ $t('common.retry') }}</el-button>
            </div>

            <div v-else-if="mappingError && !latestData" class="empty-state mapping-view__state">
              <el-icon class="mapping-view__state-icon text-danger"><Warning /></el-icon>
              <span>{{ $t('common.dataLoadFailed') }}</span>
              <el-button size="small" @click="loadMappingData">{{ $t('common.retry') }}</el-button>
            </div>

            <div v-else-if="latestData && !hasMappingData" class="empty-state mapping-view__state">
              <span>{{ $t('mapping.noData') }}</span>
            </div>
          </div>
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
import { Download, Loading, Warning } from '@element-plus/icons-vue'
import type { EChartsOption } from 'echarts'
import type { EChartsType } from 'echarts/core'
import '@/plugins/elementPlusMappingStyles'
import RefreshStatus from '@/components/common/RefreshStatus.vue'
import { isRequestCanceled } from '@/api'
import { fetchConfig } from '@/api/config'
import { getMapping, getProxyMapping } from '@/api/mapping'
import { useSidebarStore } from '@/stores/sidebar'
import { useThemeStore } from '@/stores/theme'
import { displayName, formatTime } from '@/utils/format'
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
const chartLoading = ref(true)
const chartError = ref(false)
const configLoading = ref(true)
const configError = ref(false)
const mappingLoading = ref(false)
const mappingError = ref(false)
const lastUpdatedAt = ref<Date | null>(null)
const latestData = ref<ChinaMapData | null>(null)
let chart: EChartsType | null = null
let chartLoadPromise: Promise<void> | null = null
let isUnmounted = false
let configRequestId = 0
let mappingRequestId = 0
let configAbortController: AbortController | null = null
let mappingAbortController: AbortController | null = null
let mappingQueryKey: string | null = null
let resizeTimer: number | null = null

const currentAgentName = computed(() => {
  if (!currentAgent.value) {
    return displayName(config.value?.Name || 'SmartPingNext')
  }

  const matched = agents.value.find((agent) => agent.addr === currentAgent.value)
  return matched ? displayName(matched.name) : currentAgent.value
})
const hasMappingData = computed(() => {
  const data = latestData.value
  return data
    ? data.avgdelay.ctcc.length + data.avgdelay.cucc.length + data.avgdelay.cmcc.length > 0
    : false
})
const lastUpdatedLabel = computed(() =>
  lastUpdatedAt.value ? formatTime(lastUpdatedAt.value) : ''
)

const loadConfig = async () => {
  configAbortController?.abort()
  mappingAbortController?.abort()
  mappingAbortController = null
  mappingRequestId++
  mappingLoading.value = false
  agents.value.forEach((agent) => (agent.loading = false))
  const controller = new AbortController()
  configAbortController = controller
  const requestId = ++configRequestId
  configLoading.value = true
  configError.value = false
  try {
    const cfg = await fetchConfig(controller.signal)
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    config.value = cfg
    currentAgent.value = cfg.Addr
    currentBaseUrl.value = ''

    agents.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    await loadMappingData()
  } catch (error) {
    if (isRequestCanceled(error) || isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
    configError.value = true
    if (config.value) {
      ElMessage.error(t('common.configLoadFailedNetwork'))
    }
  } finally {
    if (configAbortController === controller) {
      configAbortController = null
    }
    if (!isUnmounted && requestId === configRequestId) {
      configLoading.value = false
    }
  }
}

const loadMappingData = async () => {
  if (isUnmounted) {
    return
  }

  mappingAbortController?.abort()
  const controller = new AbortController()
  mappingAbortController = controller
  const requestId = ++mappingRequestId
  const baseUrl = currentBaseUrl.value
  const date = selectedDate.value
  const queryKey = JSON.stringify([currentAgent.value, baseUrl, date || ''])
  if (queryKey !== mappingQueryKey) {
    mappingQueryKey = queryKey
    latestData.value = null
    lastUpdatedAt.value = null
    chart?.clear()
  }
  mappingLoading.value = true
  mappingError.value = false
  try {
    const data = baseUrl
      ? await getProxyMapping(baseUrl, date, controller.signal)
      : await getMapping(date, controller.signal)

    if (isUnmounted || requestId !== mappingRequestId) {
      return
    }

    latestData.value = data
    if (isMapReady.value) {
      updateChart(data)
    }
    lastUpdatedAt.value = new Date()
  } catch (error) {
    if (isRequestCanceled(error) || isUnmounted || requestId !== mappingRequestId) {
      return
    }
    console.error('加载地图数据失败', error)
    mappingError.value = true
    if (latestData.value) {
      ElMessage.error(t('common.dataLoadFailed'))
    }
  } finally {
    if (mappingAbortController === controller) {
      mappingAbortController = null
    }
    if (!isUnmounted && requestId === mappingRequestId) {
      mappingLoading.value = false
      agents.value.forEach((agent) => {
        agent.loading = false
      })
    }
  }
}

const refreshMapping = () => (config.value ? loadMappingData() : loadConfig())

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
  const secondaryTextColor = styles.getPropertyValue('--color-text-secondary').trim() || '#606266'

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

const initChart = (): Promise<void> => {
  if (chartLoadPromise) {
    return chartLoadPromise
  }

  chartLoading.value = true
  chartError.value = false
  const promise = (async () => {
    try {
      const [{ echarts }, { default: chinaMap }] = await Promise.all([
        import('@/utils/echartsMap'),
        import('china-map-geojson/lib/china')
      ])
      if (isUnmounted || !chartRef.value) {
        return
      }

      echarts.registerMap('china', chinaMap)
      chart?.dispose()
      chart = echarts.init(chartRef.value)
      isMapReady.value = true
      if (latestData.value) {
        updateChart(latestData.value)
      }
    } catch (error) {
      if (!isUnmounted) {
        chartError.value = true
        console.error('加载地图组件失败', error)
      }
    } finally {
      if (!isUnmounted) {
        chartLoading.value = false
      }
    }
  })()

  chartLoadPromise = promise
  void promise.finally(() => {
    if (chartLoadPromise === promise) {
      chartLoadPromise = null
    }
  })
  return promise
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
    if (latestData.value) {
      updateChart(latestData.value)
    }
  }
)

watch(locale, () => {
  if (latestData.value) {
    updateChart(latestData.value)
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
  void initChart()
  await loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configAbortController?.abort()
  mappingAbortController?.abort()
  configRequestId++
  mappingRequestId++
  if (resizeTimer !== null) {
    window.clearTimeout(resizeTimer)
    resizeTimer = null
  }
  isMapReady.value = false
  chartLoading.value = false
  latestData.value = null
  chartLoadPromise = null
  chart?.dispose()
  chart = null
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped lang="scss">
.mapping-view__date-kpi {
  font-size: 14px;
}

.mapping-view__map-shell {
  position: relative;
  min-height: 520px;
  height: min(72vh, 760px);
}

.mapping-view__map {
  width: 100%;
  height: 100%;
}

.mapping-view__state {
  position: absolute;
  inset: 0;
  min-height: 0;
  padding: 24px;
  background: color-mix(in srgb, var(--color-bg-primary) 94%, transparent);
}

.mapping-view__state-icon {
  font-size: 28px;
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
  .mapping-view__map-shell {
    min-height: 420px;
    height: 58vh;
  }
}
</style>
