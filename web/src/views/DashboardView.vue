<template>
  <div class="page-shell dashboard-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('dashboard.title') }}</span>
        <h1 class="page-title">{{ displayName(config?.Name || 'SmartPingNext') }}</h1>
        <p class="page-subtitle">{{ $t('dashboard.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.targets') }}</span>
            <strong class="page-kpi__value">{{ pingTargets.length }}</strong>
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

        <section class="surface-panel surface-panel--tight dashboard-view__switcher">
          <div class="dashboard-view__switcher-copy">
            <span class="page-eyebrow">{{ $t('common.autoRefresh') }}</span>
            <strong>{{ autoRefresh ? $t('common.loaded') : $t('common.status') }}</strong>
          </div>
          <el-switch v-model="autoRefresh" size="small" :aria-label="$t('common.autoRefresh')" />
        </section>
      </div>
    </div>

    <div class="page-frame">
      <div class="page-main">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('dashboard.title') }}</h2>
              <p class="surface-panel__description">
                {{ displayName(config?.Name || 'SmartPingNext') }} · {{ pingTargets.length }}
                {{ $t('common.targets') }}
              </p>
            </div>
            <div class="metric-inline">
              <strong>{{ loadedTargets }}</strong>
              <span>{{ $t('common.loaded') }}</span>
            </div>
          </div>

          <div class="monitor-grid">
            <article
              v-for="target in pingTargets"
              :key="target.addr"
              class="monitor-tile dashboard-view__tile"
              role="button"
              tabindex="0"
              @click="showDetail(target)"
              @keydown.enter.prevent="showDetail(target)"
              @keydown.space.prevent="showDetail(target)"
            >
              <div class="monitor-tile__header">
                <div>
                  <span class="monitor-tile__eyebrow">{{
                    displayName(config?.Name || 'SmartPingNext')
                  }}</span>
                  <h3 class="monitor-tile__title">{{ displayName(target.name) }}</h3>
                </div>
                <span class="status-chip monitor-tile__status" :class="getStatusClass(target)">
                  {{ getStatusText(target) }}
                </span>
              </div>

              <div class="monitor-tile__body">
                <PingMiniChart v-if="target.chartData" :data="target.chartData" :height="138" />
                <div v-else-if="target.loading" class="dashboard-view__state">
                  <el-icon class="is-loading"><Loading /></el-icon>
                </div>
                <div v-else class="dashboard-view__state dashboard-view__state--danger">
                  <el-icon><Warning /></el-icon>
                  <span>{{ $t('common.loadFailed') }}</span>
                </div>
              </div>
              <span class="sr-only">{{
                $t('dashboard.openDetail', { name: displayName(target.name) })
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
              class="list-row dashboard-view__agent"
              :class="{ 'is-active': currentAgent === agent.addr }"
              @click="switchAgent(agent)"
            >
              <div class="list-row__meta">
                <el-icon v-if="agent.loading" class="is-loading"><Loading /></el-icon>
                <div v-else class="dashboard-view__agent-dot"></div>
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
          <span class="dashboard-view__refresh-label">{{ $t('common.autoRefresh') }}</span>
          <el-switch
            v-model="detailAutoRefresh"
            size="small"
            :aria-label="$t('common.autoRefresh')"
          />
        </div>

        <section v-loading="detailLoading" class="surface-panel dashboard-view__detail-panel">
          <PingChart v-if="detailData" ref="pingChartRef" :data="detailData" :height="400" />
          <div v-else-if="detailError" class="empty-state dashboard-view__detail-error">
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
import { computed, onMounted, onUnmounted, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElDatePicker, ElDialog, ElMessage, ElSwitch } from 'element-plus'
import { Loading, Warning } from '@element-plus/icons-vue'
import PingChart from '@/components/charts/PingChart.vue'
import PingMiniChart from '@/components/charts/PingMiniChart.vue'
import { fetchConfig, fetchProxyConfig } from '@/api/config'
import { getPingData, getProxyPingData } from '@/api/ping'
import { displayName, formatDateTime } from '@/utils/format'
import type { Config, PingLogData } from '@/types'

interface PingTarget {
  name: string
  addr: string
  chartData: PingLogData | null
  loading: boolean
  targetIp: string
}

const { t } = useI18n()
const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
const currentAgent = ref('')
const currentBaseUrl = ref('')
const pingTargets = ref<PingTarget[]>([])

const detailVisible = ref(false)
const detailTitle = ref('')
const detailData = ref<PingLogData | null>(null)
const detailLoading = ref(false)
const detailError = ref(false)
const startTime = ref('')
const endTime = ref('')
const currentTargetIp = ref('')
const pingChartRef = ref<{ saveAsImage: () => void } | null>(null)

const autoRefresh = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | null = null
const detailAutoRefresh = ref(false)
let detailRefreshTimer: ReturnType<typeof setInterval> | null = null
let isUnmounted = false
let configRequestId = 0
let chartRequestId = 0
let detailRequestId = 0
const refreshInterval = computed(() => Math.max(config.value?.Base.Refresh || 1, 1) * 60 * 1000)

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
  () => pingTargets.value.filter((target) => !!target.chartData).length
)
const failedTargets = computed(
  () => pingTargets.value.filter((target) => !target.loading && !target.chartData).length
)

const loadConfig = async (proxyUrl?: string) => {
  const requestId = ++configRequestId
  try {
    const cfg = proxyUrl ? await fetchProxyConfig(proxyUrl) : await fetchConfig()
    if (isUnmounted || requestId !== configRequestId) {
      return
    }

    chartRequestId++
    config.value = cfg
    currentAgent.value = cfg.Addr
    currentBaseUrl.value = proxyUrl || ''

    agents.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    const selfNetwork = cfg.Network[cfg.Addr]
    if (!selfNetwork) {
      pingTargets.value = []
      return
    }

    pingTargets.value = selfNetwork.Ping.map((addr) => {
      const target = cfg.Network[addr]
      return {
        name: target?.Name || addr,
        addr,
        chartData: null,
        loading: false,
        targetIp: addr
      }
    })

    await loadAllCharts()
  } catch (error) {
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
    ElMessage.error(t('common.configLoadFailedNetwork'))
  }
}

const loadAllCharts = async () => {
  const requestId = ++chartRequestId
  const targets = pingTargets.value
  const batchSize = 3
  for (let index = 0; index < targets.length; index += batchSize) {
    if (isUnmounted || requestId !== chartRequestId) {
      return
    }
    const batch = targets.slice(index, index + batchSize)
    await Promise.all(batch.map((target) => loadChartData(target, requestId)))
  }
}

const loadChartData = async (target: PingTarget, requestId: number) => {
  target.loading = true
  const baseUrl = currentBaseUrl.value
  try {
    const data = baseUrl
      ? await getProxyPingData(baseUrl, target.targetIp)
      : await getPingData(target.targetIp)
    if (isUnmounted || requestId !== chartRequestId) {
      return
    }
    target.chartData = data
  } catch (error) {
    if (isUnmounted || requestId !== chartRequestId) {
      return
    }
    console.error(t('common.chartLoadFailed'), error)
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

const DEFAULT_TIME_RANGE_HOURS = Number(import.meta.env.VITE_DEFAULT_TIME_RANGE) || 6

const showDetail = async (target: PingTarget) => {
  detailRequestId++
  detailData.value = null
  detailLoading.value = false
  detailError.value = false
  detailTitle.value = `${displayName(config.value?.Name || '')} -> ${displayName(target.name)}`
  currentTargetIp.value = target.targetIp
  setTimeRange(DEFAULT_TIME_RANGE_HOURS, false)
  detailVisible.value = true
  await loadDetailData()
}

const loadDetailData = async () => {
  if (!currentTargetIp.value) {
    return
  }

  const requestId = ++detailRequestId
  const baseUrl = currentBaseUrl.value
  const targetIp = currentTargetIp.value
  const start = startTime.value
  const end = endTime.value
  detailLoading.value = true
  detailError.value = false
  try {
    const data = baseUrl
      ? await getProxyPingData(baseUrl, targetIp, start, end)
      : await getPingData(targetIp, start, end)
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

const getStatusClass = (target: PingTarget) => {
  if (target.loading) {
    return 'status-chip--active'
  }
  if (!target.chartData) {
    return 'status-chip--danger'
  }
  return 'status-chip--success'
}

const getStatusText = (target: PingTarget) => {
  if (target.loading) {
    return t('common.loading')
  }
  if (!target.chartData) {
    return t('common.loadFailed')
  }
  return t('common.loaded')
}

onMounted(() => {
  loadConfig()
})

watch([autoRefresh, refreshInterval], ([enabled, interval]) => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }

  if (enabled) {
    refreshTimer = setInterval(() => {
      loadAllCharts()
    }, interval)
  }
})

watch([detailAutoRefresh, refreshInterval], ([enabled, interval]) => {
  if (detailRefreshTimer) {
    clearInterval(detailRefreshTimer)
    detailRefreshTimer = null
  }

  if (enabled) {
    detailRefreshTimer = setInterval(() => {
      loadDetailData()
    }, interval)
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
.dashboard-view__switcher {
  min-width: 172px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.dashboard-view__switcher-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;

  strong {
    font-size: 14px;
    color: var(--color-text-primary);
  }
}

.dashboard-view__tile {
  animation: page-rise 0.45s ease both;
}

.dashboard-view__state {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: var(--color-text-secondary);
}

.dashboard-view__state--danger {
  color: var(--color-danger);
}

.dashboard-view__agent {
  width: 100%;
  border: none;
  cursor: pointer;
  font: inherit;
}

.dashboard-view__agent-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-primary);
  animation: subtle-pulse 2.4s ease infinite;
}

.dashboard-view__detail-panel {
  padding: 16px 16px 12px;
}

.dashboard-view__detail-error .el-icon {
  font-size: 24px;
  color: var(--color-danger);
}

.dashboard-view__refresh-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

@media (max-width: 900px) {
  .dashboard-view__switcher {
    width: 100%;
  }
}
</style>
