<template>
  <div class="page-shell reverse-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('reverse.title') }}</span>
        <h1 class="page-title">{{ displayName(config?.Name || 'SmartPingNext') }}</h1>
        <p class="page-subtitle">{{ $t('reverse.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.targets') }}</span>
            <strong class="page-kpi__value">{{ reverseTargets.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.loaded') }}</span>
            <strong class="page-kpi__value">{{ loadedTargets }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.issues') }}</span>
            <strong class="page-kpi__value">{{ failedTargets }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.probes') }}</span>
            <strong class="page-kpi__value">{{ agents.length }}</strong>
          </div>
        </div>

        <MonitorRefreshControl
          v-model="autoRefresh"
          class="surface-panel surface-panel--tight"
          :refreshing="configLoading || isRefreshing"
          :last-updated="lastUpdatedLabel"
          @refresh="refreshMonitor"
        />
      </div>
    </div>

    <div class="page-frame">
      <div class="page-main">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('reverse.title') }}</h2>
              <p class="surface-panel__description">
                {{ reverseTargets.length }} {{ $t('common.targets') }} ·
                {{ displayName(config?.Name || 'SmartPingNext') }}
              </p>
            </div>
            <div class="metric-inline">
              <strong>{{ loadedTargets }}</strong>
              <span>{{ $t('common.loaded') }}</span>
            </div>
          </div>

          <div v-if="configLoading && !config" class="empty-state" aria-live="polite">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>{{ $t('common.loading') }}</span>
          </div>

          <div v-else-if="configError && !config" class="empty-state">
            <el-icon class="reverse-view__empty-icon reverse-view__empty-icon--danger">
              <Warning />
            </el-icon>
            <span>{{ $t('common.configLoadFailedNetwork') }}</span>
            <el-button size="small" @click="loadConfig()">{{ $t('common.retry') }}</el-button>
          </div>

          <div v-else-if="!reverseTargets.length" class="empty-state">
            <el-icon class="reverse-view__empty-icon"><InfoFilled /></el-icon>
            <span>{{ $t('common.noMonitorTargets') }}</span>
          </div>

          <div v-else class="monitor-grid">
            <article
              v-for="target in reverseTargets"
              :key="target.fromAddr"
              class="monitor-tile reverse-view__tile"
              role="button"
              tabindex="0"
              @click="showDetail(target)"
              @keydown.enter.prevent="showDetail(target)"
              @keydown.space.prevent="showDetail(target)"
            >
              <div class="monitor-tile__header">
                <div>
                  <span class="monitor-tile__eyebrow">{{ displayName(target.fromName) }}</span>
                  <h3 class="monitor-tile__title">
                    {{ displayName(config?.Name || 'SmartPingNext') }}
                  </h3>
                </div>
                <span class="status-chip monitor-tile__status" :class="getStatusClass(target)">
                  {{ getStatusText(target) }}
                </span>
              </div>

              <div class="monitor-tile__body">
                <PingMiniChart v-if="target.chartData" :data="target.chartData" :height="138" />
                <div v-else-if="target.loading" class="reverse-view__state">
                  <el-icon class="is-loading"><Loading /></el-icon>
                </div>
                <div v-else class="reverse-view__state reverse-view__state--danger">
                  <el-icon><Warning /></el-icon>
                  <span>{{ $t('common.loadFailed') }}</span>
                </div>
              </div>
              <span class="sr-only">{{
                $t('reverse.openDetail', { name: displayName(target.fromName) })
              }}</span>
            </article>
          </div>
        </section>
      </div>

      <aside class="page-aside page-aside--narrow">
        <section class="surface-panel surface-panel--soft">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('common.nodeList') }}</h2>
              <p class="surface-panel__description">
                {{ $t('common.probes') }} · {{ agents.length }}
              </p>
            </div>
          </div>

          <div class="list-stack">
            <button
              v-for="agent in agents"
              :key="agent.addr"
              type="button"
              class="list-row reverse-view__agent"
              :class="{ 'is-active': currentAgent === agent.addr }"
              @click="switchAgent(agent)"
            >
              <div class="list-row__meta">
                <el-icon v-if="agent.loading" class="is-loading"><Loading /></el-icon>
                <div v-else class="reverse-view__agent-dot"></div>
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

    <el-dialog v-model="detailVisible" :title="detailTitle" width="960px" destroy-on-close>
      <div class="detail-content">
        <div class="detail-toolbar">
          <el-date-picker
            v-model="startTime"
            type="datetime"
            :placeholder="$t('dashboard.startTime')"
            format="YYYY-MM-DD HH:mm"
            value-format="YYYY-MM-DD HH:mm"
          />
          <el-date-picker
            v-model="endTime"
            type="datetime"
            :placeholder="$t('dashboard.endTime')"
            format="YYYY-MM-DD HH:mm"
            value-format="YYYY-MM-DD HH:mm"
          />
          <el-button type="primary" :loading="detailLoading" @click="loadDetailData">
            {{ $t('common.query') }}
          </el-button>
          <el-button :disabled="!detailData" @click="saveChartImage">
            {{ $t('common.saveImage') }}
          </el-button>
          <el-button-group>
            <el-button
              v-for="range in timeRanges"
              :key="range.hours"
              @click="setTimeRange(range.hours)"
            >
              {{ range.label }}
            </el-button>
          </el-button-group>
          <span class="reverse-view__refresh-label">{{ $t('common.autoRefresh') }}</span>
          <el-switch
            v-model="detailAutoRefresh"
            size="small"
            :aria-label="$t('common.autoRefresh')"
          />
        </div>

        <section v-loading="detailLoading" class="surface-panel reverse-view__detail-panel">
          <PingChart v-if="detailData" ref="pingChartRef" :data="detailData" :height="400" />
          <div v-else-if="detailError" class="empty-state reverse-view__detail-error">
            <el-icon><Warning /></el-icon>
            <span>{{ $t('common.dataLoadFailed') }}</span>
            <el-button size="small" @click="loadDetailData">{{ $t('common.retry') }}</el-button>
          </div>
          <div v-else class="empty-state">
            <span>{{ $t('common.loading') }}</span>
          </div>
        </section>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElDatePicker, ElDialog, ElMessage, ElSwitch } from 'element-plus'
import { InfoFilled, Loading, Warning } from '@element-plus/icons-vue'
import '@/plugins/elementPlusMonitorStyles'
import MonitorRefreshControl from '@/components/common/MonitorRefreshControl.vue'
import { fetchConfig, fetchProxyConfig } from '@/api/config'
import { getProxyPingData } from '@/api/ping'
import { displayName, formatDateTime, formatTime } from '@/utils/format'
import type { Config, PingLogData } from '@/types'

const pingMiniChartModule = import('@/components/charts/PingMiniChart.vue')
const PingMiniChart = defineAsyncComponent(() => pingMiniChartModule)
const PingChart = defineAsyncComponent(() => import('@/components/charts/PingChart.vue'))

interface ReverseTarget {
  fromName: string
  fromAddr: string
  fromPort: number
  chartData: PingLogData | null
  loading: boolean
  targetIp: string
}

const { t } = useI18n()
const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
const currentAgent = ref('')
const reverseTargets = ref<ReverseTarget[]>([])

const detailVisible = ref(false)
const detailTitle = ref('')
const detailData = ref<PingLogData | null>(null)
const detailLoading = ref(false)
const detailError = ref(false)
const startTime = ref('')
const endTime = ref('')
const currentTarget = ref<ReverseTarget | null>(null)
const pingChartRef = ref<{ saveAsImage: () => void } | null>(null)

const autoRefresh = ref(false)
const configLoading = ref(true)
const configError = ref(false)
const isRefreshing = ref(false)
const lastUpdatedAt = ref<Date | null>(null)
let refreshTimer: ReturnType<typeof setInterval> | null = null
const detailAutoRefresh = ref(false)
let detailRefreshTimer: ReturnType<typeof setInterval> | null = null
let isUnmounted = false
let configRequestId = 0
let chartRequestId = 0
let detailRequestId = 0
const refreshInterval = computed(() => Math.max(config.value?.Base.Refresh || 1, 1) * 60 * 1000)
const lastUpdatedLabel = computed(() =>
  lastUpdatedAt.value ? formatTime(lastUpdatedAt.value) : ''
)

const timeRanges = computed(() => [
  { label: t('dashboard.timeRanges.hour1'), hours: 1 },
  { label: t('dashboard.timeRanges.hour3'), hours: 3 },
  { label: t('dashboard.timeRanges.hour6'), hours: 6 },
  { label: t('dashboard.timeRanges.hour12'), hours: 12 },
  { label: t('dashboard.timeRanges.day1'), hours: 24 },
  { label: t('dashboard.timeRanges.day3'), hours: 72 },
  { label: t('dashboard.timeRanges.day7'), hours: 168 }
])

const loadedTargets = computed(
  () => reverseTargets.value.filter((target) => !!target.chartData).length
)
const failedTargets = computed(
  () => reverseTargets.value.filter((target) => !target.loading && !target.chartData).length
)

const loadConfig = async (proxyUrl?: string) => {
  const requestId = ++configRequestId
  configLoading.value = true
  configError.value = false
  try {
    const cfg = proxyUrl ? await fetchProxyConfig(proxyUrl) : await fetchConfig()
    if (isUnmounted || requestId !== configRequestId) {
      return
    }

    chartRequestId++
    config.value = cfg
    currentAgent.value = cfg.Addr

    agents.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    reverseTargets.value = []
    Object.entries(cfg.Network).forEach(([addr, network]) => {
      if (addr === cfg.Addr) {
        return
      }

      if (network.Ping.includes(cfg.Addr)) {
        reverseTargets.value.push({
          fromName: network.Name,
          fromAddr: addr,
          fromPort: cfg.Port,
          chartData: null,
          loading: false,
          targetIp: cfg.Addr
        })
      }
    })

    await loadAllCharts()
  } catch (error) {
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
    configError.value = true
    if (config.value) {
      ElMessage.error(t('common.configLoadFailedNetwork'))
    }
  } finally {
    if (!isUnmounted && requestId === configRequestId) {
      configLoading.value = false
    }
  }
}

const loadAllCharts = async () => {
  const requestId = ++chartRequestId
  const targets = [...reverseTargets.value]
  const batchSize = 4
  isRefreshing.value = true
  try {
    for (let index = 0; index < targets.length; index += batchSize) {
      if (isUnmounted || requestId !== chartRequestId) {
        return
      }
      const batch = targets.slice(index, index + batchSize)
      await Promise.all(batch.map((target) => loadChartData(target, requestId)))
    }
  } finally {
    if (!isUnmounted && requestId === chartRequestId) {
      isRefreshing.value = false
      lastUpdatedAt.value = new Date()
    }
  }
}

const loadChartData = async (target: ReverseTarget, requestId: number) => {
  target.loading = true
  const baseUrl = `http://${target.fromAddr}:${target.fromPort}`
  const targetIp = target.targetIp
  try {
    const data = await getProxyPingData(baseUrl, targetIp)
    if (isUnmounted || requestId !== chartRequestId) {
      return
    }
    target.chartData = data
  } catch (error) {
    if (isUnmounted || requestId !== chartRequestId) {
      return
    }
    console.error('加载图表数据失败', error)
    target.chartData = null
  } finally {
    if (!isUnmounted && requestId === chartRequestId) {
      target.loading = false
    }
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  detailVisible.value = false
  const proxyUrl = `http://${agent.addr}:${config.value?.Port}`
  await loadConfig(proxyUrl)
  agent.loading = false
}

const showDetail = async (target: ReverseTarget) => {
  detailRequestId++
  detailData.value = null
  detailLoading.value = false
  detailError.value = false
  detailTitle.value = `${displayName(target.fromName)} -> ${displayName(config.value?.Name || '')}`
  currentTarget.value = target
  setTimeRange(6, false)
  detailVisible.value = true
  await loadDetailData()
}

const loadDetailData = async () => {
  if (!currentTarget.value) {
    return
  }

  const requestId = ++detailRequestId
  const target = currentTarget.value
  const baseUrl = `http://${target.fromAddr}:${target.fromPort}`
  const start = startTime.value
  const end = endTime.value
  if (start && end && start > end) {
    ElMessage.warning(t('common.invalidTimeRange'))
    return
  }
  detailLoading.value = true
  detailError.value = false
  try {
    const data = await getProxyPingData(baseUrl, target.targetIp, start, end)
    if (isUnmounted || requestId !== detailRequestId || !detailVisible.value) {
      return
    }
    detailData.value = data
  } catch (error) {
    if (isUnmounted || requestId !== detailRequestId || !detailVisible.value) {
      return
    }
    console.error('加载数据失败', error)
    detailData.value = null
    detailError.value = true
  } finally {
    if (!isUnmounted && requestId === detailRequestId && detailVisible.value) {
      detailLoading.value = false
    }
  }
}

const setTimeRange = (hours: number, shouldLoad = true) => {
  const end = new Date()
  const start = new Date(end.getTime() - hours * 60 * 60 * 1000)
  startTime.value = formatDateTime(start)
  endTime.value = formatDateTime(end)

  if (shouldLoad) {
    loadDetailData()
  }
}

const saveChartImage = () => {
  pingChartRef.value?.saveAsImage()
}

const getStatusClass = (target: ReverseTarget) => {
  if (target.loading) {
    return 'status-chip--active'
  }
  if (!target.chartData) {
    return 'status-chip--danger'
  }
  return 'status-chip--success'
}

const getStatusText = (target: ReverseTarget) => {
  if (target.loading) {
    return t('common.loading')
  }
  if (!target.chartData) {
    return t('common.loadFailed')
  }
  return t('common.loaded')
}

const refreshChartsIfVisible = () => {
  if (document.visibilityState === 'visible') {
    loadAllCharts()
  }
}

const refreshMonitor = () => {
  if (config.value) {
    return loadAllCharts()
  }
  return loadConfig()
}

const refreshDetailIfVisible = () => {
  if (document.visibilityState === 'visible' && detailVisible.value) {
    loadDetailData()
  }
}

const handleVisibilityChange = () => {
  if (document.visibilityState !== 'visible') {
    return
  }
  if (autoRefresh.value) {
    loadAllCharts()
  }
  if (detailAutoRefresh.value && detailVisible.value) {
    loadDetailData()
  }
}

onMounted(() => {
  document.addEventListener('visibilitychange', handleVisibilityChange)
  loadConfig()
})

watch([autoRefresh, refreshInterval], ([enabled, interval]) => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }

  if (enabled) {
    refreshTimer = setInterval(refreshChartsIfVisible, interval)
  }
})

watch([detailAutoRefresh, refreshInterval], ([enabled, interval]) => {
  if (detailRefreshTimer) {
    clearInterval(detailRefreshTimer)
    detailRefreshTimer = null
  }

  if (enabled) {
    detailRefreshTimer = setInterval(refreshDetailIfVisible, interval)
  }
})

watch(detailVisible, (visible) => {
  if (!visible) {
    detailRequestId++
    detailAutoRefresh.value = false
    detailLoading.value = false
    detailError.value = false
  }
})

onUnmounted(() => {
  isUnmounted = true
  document.removeEventListener('visibilitychange', handleVisibilityChange)
  configRequestId++
  chartRequestId++
  detailRequestId++
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }

  if (detailRefreshTimer) {
    clearInterval(detailRefreshTimer)
    detailRefreshTimer = null
  }
})
</script>

<style scoped lang="scss">
.reverse-view__tile {
  animation: page-rise 0.45s ease both;
}

.reverse-view__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--color-text-secondary);
}

.reverse-view__state--danger {
  color: var(--color-danger);
}

.reverse-view__empty-icon {
  font-size: 26px;
  color: var(--color-text-secondary);
}

.reverse-view__empty-icon--danger {
  color: var(--color-danger);
}

.reverse-view__agent {
  width: 100%;
  border: none;
  cursor: pointer;
  font: inherit;
}

.reverse-view__agent-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-primary);
  animation: subtle-pulse 2.4s ease infinite;
}

.reverse-view__detail-panel {
  padding: 16px 16px 12px;
}

.reverse-view__detail-error .el-icon {
  font-size: 24px;
  color: var(--color-danger);
}

.reverse-view__refresh-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}
</style>
