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

        <section class="surface-panel surface-panel--tight reverse-view__switcher">
          <div class="reverse-view__switcher-copy">
            <span class="page-eyebrow">{{ $t('common.autoRefresh') }}</span>
            <strong>{{ autoRefresh ? $t('common.loaded') : $t('common.status') }}</strong>
          </div>
          <el-switch v-model="autoRefresh" size="small" />
        </section>
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

          <div class="monitor-grid">
            <article
              v-for="target in reverseTargets"
              :key="target.fromAddr"
              class="monitor-tile reverse-view__tile"
              @click="showDetail(target)"
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
          <el-button type="primary" @click="loadDetailData">{{ $t('common.query') }}</el-button>
          <el-button @click="saveChartImage">{{ $t('common.saveImage') }}</el-button>
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
          <el-switch v-model="detailAutoRefresh" size="small" />
        </div>

        <section class="surface-panel reverse-view__detail-panel">
          <PingChart v-if="detailData" ref="pingChartRef" :data="detailData" :height="400" />
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
import { Loading, Warning } from '@element-plus/icons-vue'
import PingChart from '@/components/charts/PingChart.vue'
import PingMiniChart from '@/components/charts/PingMiniChart.vue'
import { fetchConfig, fetchProxyConfig } from '@/api/config'
import { getProxyPingData } from '@/api/ping'
import { displayName, formatDateTime } from '@/utils/format'
import type { Config, PingLogData } from '@/types'

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
const startTime = ref('')
const endTime = ref('')
const currentTarget = ref<ReverseTarget | null>(null)
const pingChartRef = ref<{ saveAsImage: () => void } | null>(null)

const autoRefresh = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | null = null
const detailAutoRefresh = ref(false)
let detailRefreshTimer: ReturnType<typeof setInterval> | null = null
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
  () => reverseTargets.value.filter((target) => !!target.chartData).length
)
const failedTargets = computed(
  () => reverseTargets.value.filter((target) => !target.loading && !target.chartData).length
)

const loadConfig = async (proxyUrl?: string) => {
  try {
    const cfg = proxyUrl ? await fetchProxyConfig(proxyUrl) : await fetchConfig()
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
    console.error('加载配置失败', error)
  }
}

const loadAllCharts = async () => {
  await Promise.all(reverseTargets.value.map((target) => loadChartData(target)))
}

const loadChartData = async (target: ReverseTarget) => {
  target.loading = true
  try {
    target.chartData = await getProxyPingData(
      `http://${target.fromAddr}:${target.fromPort}`,
      target.targetIp
    )
  } catch (error) {
    console.error('加载图表数据失败', error)
    target.chartData = null
  } finally {
    target.loading = false
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  detailVisible.value = false
  currentAgent.value = agent.addr
  const proxyUrl = `http://${agent.addr}:${config.value?.Port}`
  await loadConfig(proxyUrl)
  agent.loading = false
}

const showDetail = async (target: ReverseTarget) => {
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

  try {
    detailData.value = await getProxyPingData(
      `http://${currentTarget.value.fromAddr}:${currentTarget.value.fromPort}`,
      currentTarget.value.targetIp,
      startTime.value,
      endTime.value
    )
  } catch (error) {
    console.error('加载数据失败', error)
    detailData.value = null
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
    detailAutoRefresh.value = false
  }
})

onUnmounted(() => {
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
.reverse-view__switcher {
  min-width: 172px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 14px;
}

.reverse-view__switcher-copy {
  display: flex;
  flex-direction: column;
  gap: 6px;

  strong {
    font-size: 14px;
    color: var(--color-text-primary);
  }
}

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

.reverse-view__refresh-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

@media (max-width: 900px) {
  .reverse-view__switcher {
    width: 100%;
  }
}
</style>
