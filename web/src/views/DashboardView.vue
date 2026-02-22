<template>
  <div class="dashboard-view">
    <div class="dashboard-header">
      <h2>{{ displayName(config?.Name || 'SmartPingNext') }} - {{ $t('dashboard.title') }}</h2>
      <div class="header-actions">
        <span class="auto-refresh-label">{{ $t('common.autoRefresh') }}</span>
        <el-switch
          v-model="autoRefresh"
          size="small"
        />
      </div>
    </div>

    <div class="dashboard-content">
      <div class="charts-grid">
        <div
          v-for="target in pingTargets"
          :key="target.addr"
          class="chart-card"
          @click="showDetail(target)"
        >
          <div class="chart-card__header">
            <span class="chart-card__title">{{ displayName(config?.Name || '') }} -> {{ displayName(target.name) }}</span>
            <span v-if="target.loading" class="chart-card__loading-text">{{ $t('common.loading') }}</span>
          </div>
          <div class="chart-card__body">
            <PingMiniChart
              v-if="target.chartData"
              :data="target.chartData"
              :height="130"
            />
            <div v-else-if="target.loading" class="chart-card__loading">
              <el-icon class="is-loading"><Loading /></el-icon>
            </div>
            <div v-else class="chart-card__error">
              <el-icon><Warning /></el-icon>
              <span>{{ $t('common.loadFailed') }}</span>
            </div>
          </div>
        </div>
      </div>

      <div class="agent-sidebar">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>{{ $t('common.nodeList') }}</span>
            </div>
          </template>
          <div class="agent-list">
            <div
              v-for="agent in agents"
              :key="agent.addr"
              class="agent-item"
              @click="switchAgent(agent)"
            >
              <el-icon v-if="agent.loading" class="is-loading"><Loading /></el-icon>
              <span>{{ displayName(agent.name) }}</span>
            </div>
          </div>
        </el-card>
      </div>
    </div>

    <!-- 详情弹窗 -->
    <el-dialog
      v-model="detailVisible"
      :title="detailTitle"
      width="900px"
      destroy-on-close
    >
      <div class="detail-content">
        <div class="time-picker">
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
            <el-button v-for="range in timeRanges" :key="range.hours" @click="setTimeRange(range.hours)">
              {{ range.label }}
            </el-button>
          </el-button-group>
          <span class="auto-refresh-label">{{ $t('common.autoRefresh') }}</span>
          <el-switch v-model="detailAutoRefresh" size="small" />
        </div>
        <PingChart v-if="detailData" ref="pingChartRef" :data="detailData" :height="400" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, computed, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { Loading, Warning } from '@element-plus/icons-vue'
import PingChart from '@/components/charts/PingChart.vue'
import PingMiniChart from '@/components/charts/PingMiniChart.vue'
import { fetchConfig, fetchProxyConfig } from '@/api/config'
import { getPingData } from '@/api/ping'
import { formatDateTime, displayName } from '@/utils/format'
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
const currentAgent = ref<string>('')
const pingTargets = ref<PingTarget[]>([])

const detailVisible = ref(false)
const detailTitle = ref('')
const detailData = ref<PingLogData | null>(null)
const startTime = ref('')
const endTime = ref('')
const currentTargetIp = ref('')
const pingChartRef = ref<{ saveAsImage: () => void } | null>(null)

const autoRefresh = ref(false)
let refreshTimer: ReturnType<typeof setInterval> | null = null
const detailAutoRefresh = ref(false)
let detailRefreshTimer: ReturnType<typeof setInterval> | null = null
const REFRESH_INTERVAL = 60 * 1000 // 1分钟

const timeRanges = computed(() => [
  { label: t('dashboard.timeRanges.hour1'), hours: 1 },
  { label: t('dashboard.timeRanges.hour3'), hours: 3 },
  { label: t('dashboard.timeRanges.hour6'), hours: 6 },
  { label: t('dashboard.timeRanges.hour12'), hours: 12 },
  { label: t('dashboard.timeRanges.day1'), hours: 24 },
  { label: t('dashboard.timeRanges.day3'), hours: 72 },
  { label: t('dashboard.timeRanges.day7'), hours: 168 }
])

const loadConfig = async (proxyUrl?: string) => {
  try {
    let cfg: Config
    if (proxyUrl) {
      cfg = await fetchProxyConfig(proxyUrl)
    } else {
      cfg = await fetchConfig()
    }
    config.value = cfg

    // 构建节点列表
    agents.value = Object.values(cfg.Network)
      .filter(n => n.Smartping)
      .map(n => ({ name: n.Name, addr: n.Addr, loading: false }))

    // 构建PING目标
    const selfNetwork = cfg.Network[cfg.Addr]
    if (selfNetwork) {
      pingTargets.value = selfNetwork.Ping.map(addr => {
        const target = cfg!.Network[addr]
        return {
          name: target?.Name || addr,
          addr,
          chartData: null,
          loading: false,
          targetIp: addr
        }
      })

      // 加载所有图表数据
      loadAllCharts()
    }
  } catch (e) {
    console.error('加载配置失败', e)
    ElMessage.error(t('common.configLoadFailedNetwork'))
  }
}

const loadAllCharts = async () => {
  // 限制并发请求数量为 3
  const batchSize = 3
  for (let i = 0; i < pingTargets.value.length; i += batchSize) {
    const batch = pingTargets.value.slice(i, i + batchSize)
    await Promise.all(batch.map(target => loadChartData(target)))
  }
}

const loadChartData = async (target: PingTarget) => {
  target.loading = true
  try {
    target.chartData = await getPingData(target.targetIp)
  } catch (e) {
    console.error(t('common.chartLoadFailed'), e)
    target.chartData = null
  } finally {
    target.loading = false
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  currentAgent.value = agent.addr
  const proxyUrl = `http://${agent.addr}:${config.value?.Port}`
  await loadConfig(proxyUrl)
  agent.loading = false
}

const DEFAULT_TIME_RANGE_HOURS = Number(import.meta.env.VITE_DEFAULT_TIME_RANGE) || 6

const showDetail = async (target: PingTarget) => {
  detailTitle.value = `${displayName(config.value?.Name || '')} -> ${displayName(target.name)}`
  currentTargetIp.value = target.targetIp

  setTimeRange(DEFAULT_TIME_RANGE_HOURS)

  detailVisible.value = true
  await loadDetailData()
}

const loadDetailData = async () => {
  if (!currentTargetIp.value) return

  try {
    detailData.value = await getPingData(currentTargetIp.value, startTime.value, endTime.value)
  } catch (e) {
    console.error('加载数据失败', e)
    ElMessage.error(t('common.loadFailed'))
  }
}

const setTimeRange = (hours: number) => {
  const end = new Date()
  const start = new Date(end.getTime() - hours * 60 * 60 * 1000)

  startTime.value = formatDateTime(start)
  endTime.value = formatDateTime(end)
  loadDetailData()
}

const saveChartImage = () => {
  pingChartRef.value?.saveAsImage()
}

onMounted(() => {
  loadConfig()
})

watch(autoRefresh, (enabled) => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
    refreshTimer = null
  }
  if (enabled) {
    refreshTimer = setInterval(() => {
      loadAllCharts()
    }, REFRESH_INTERVAL)
  }
})

watch(detailAutoRefresh, (enabled) => {
  if (detailRefreshTimer) {
    clearInterval(detailRefreshTimer)
    detailRefreshTimer = null
  }
  if (enabled) {
    detailRefreshTimer = setInterval(() => {
      loadDetailData()
    }, REFRESH_INTERVAL)
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
.dashboard-view {
  height: 100%;
}

.dashboard-header {
  margin-bottom: 20px;
  display: flex;
  align-items: center;
  justify-content: space-between;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.header-actions {
  display: flex;
  align-items: center;
  gap: 8px;
}

.auto-refresh-label {
  font-size: 13px;
  color: var(--color-text-secondary);
}

.dashboard-content {
  display: flex;
  gap: 20px;
}

.charts-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 16px;
}

.chart-card {
  background-color: var(--color-bg-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
  cursor: pointer;
  transition: transform 0.2s, box-shadow 0.2s;

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-md);
  }

  &__header {
    padding: 12px 16px;
    background-color: var(--color-bg-secondary);
    border-bottom: 1px solid var(--color-border-lighter);
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  &__title {
    font-size: 14px;
    font-weight: 500;
    color: var(--color-text-primary);
  }

  &__loading-text {
    font-size: 12px;
    color: var(--color-text-secondary);
  }

  &__body {
    height: 130px;
    display: flex;
    align-items: center;
    justify-content: center;
    width: 100%;
    padding: 0 4px;
    box-sizing: border-box;
  }

  &__loading {
    color: var(--color-text-secondary);
  }

  &__error {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 8px;
    color: var(--color-danger);
  }
}

.agent-sidebar {
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

.detail-content {
  width: 100%;

  .time-picker {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
    flex-wrap: wrap;
  }
}

@media (max-width: 1200px) {
  .charts-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .dashboard-content {
    flex-direction: column;
  }

  .charts-grid {
    grid-template-columns: 1fr;
  }

  .agent-sidebar {
    width: 100%;
  }
}
</style>
