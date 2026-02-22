<template>
  <div class="reverse-view">
    <div class="reverse-header">
      <h2>{{ displayName(config?.Name || 'SmartPingNext') }} - {{ $t('reverse.title') }}</h2>
      <div class="header-actions">
        <span class="auto-refresh-label">{{ $t('common.autoRefresh') }}</span>
        <el-switch
          v-model="autoRefresh"
          size="small"
        />
      </div>
    </div>

    <div class="reverse-content">
      <div class="charts-grid">
        <div
          v-for="target in reverseTargets"
          :key="target.fromAddr"
          class="chart-card"
          @click="showDetail(target)"
        >
          <div class="chart-card__header">
            <span class="chart-card__title">{{ displayName(target.fromName) }} -> {{ displayName(config?.Name || '') }}</span>
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
          <span class="auto-refresh-label">{{ $t('common.autoRefresh') }}</span>
          <el-switch v-model="detailAutoRefresh" size="small" />
        </div>
        <PingChart v-if="detailData" ref="pingChartRef" :data="detailData" :height="400" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch } from 'vue'
import { Loading, Warning } from '@element-plus/icons-vue'
import PingChart from '@/components/charts/PingChart.vue'
import PingMiniChart from '@/components/charts/PingMiniChart.vue'
import { fetchConfig, fetchProxyConfig } from '@/api/config'
import { formatDateTime, displayName } from '@/utils/format'
import type { Config, PingLogData } from '@/types'

interface ReverseTarget {
  fromName: string
  fromAddr: string
  fromPort: number
  chartData: PingLogData | null
  loading: boolean
  targetIp: string
}

const config = ref<Config | null>(null)
const agents = ref<Array<{ name: string; addr: string; loading: boolean }>>([])
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
const REFRESH_INTERVAL = 60 * 1000 // 1分钟

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

    // 构建反向PING目标
    reverseTargets.value = []
    Object.entries(cfg.Network).forEach(([addr, network]) => {
      if (addr === cfg.Addr) return
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

    // 加载所有图表数据
    loadAllCharts()
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const loadAllCharts = async () => {
  await Promise.all(reverseTargets.value.map(target => loadChartData(target)))
}

const loadChartData = async (target: ReverseTarget) => {
  target.loading = true
  try {
    const proxyUrl = `http://${target.fromAddr}:${target.fromPort}/api/ping.json?ip=${target.targetIp}`
    const response = await fetch(`/api/proxy.json?g=${encodeURIComponent(proxyUrl)}`)
    if (!response.ok) {
      throw new Error(`HTTP error! status: ${response.status}`)
    }
    target.chartData = await response.json()
  } catch (e) {
    console.error('加载图表数据失败', e)
    target.chartData = null
  } finally {
    target.loading = false
  }
}

const switchAgent = async (agent: { name: string; addr: string; loading: boolean }) => {
  agent.loading = true
  const proxyUrl = `http://${agent.addr}:${config.value?.Port}`
  await loadConfig(proxyUrl)
  agent.loading = false
}

const showDetail = async (target: ReverseTarget) => {
  detailTitle.value = `${displayName(target.fromName)} -> ${displayName(config.value?.Name || '')}`
  currentTarget.value = target

  const end = new Date()
  const start = new Date(end.getTime() - 6 * 60 * 60 * 1000)
  startTime.value = formatDateTime(start)
  endTime.value = formatDateTime(end)

  detailVisible.value = true
  await loadDetailData()
}

const loadDetailData = async () => {
  if (!currentTarget.value) return

  let remoteUrl = `http://${currentTarget.value.fromAddr}:${currentTarget.value.fromPort}/api/ping.json?ip=${currentTarget.value.targetIp}`
  if (startTime.value) {
    remoteUrl += `&starttime=${encodeURIComponent(startTime.value)}`
  }
  if (endTime.value) {
    remoteUrl += `&endtime=${encodeURIComponent(endTime.value)}`
  }

  try {
    const response = await fetch(`/api/proxy.json?g=${encodeURIComponent(remoteUrl)}`)
    detailData.value = await response.json()
  } catch (e) {
    console.error('加载数据失败', e)
  }
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
.reverse-view {
  height: 100%;
}

.reverse-header {
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

.reverse-content {
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
  .time-picker {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-bottom: 20px;
  }
}

@media (max-width: 1200px) {
  .charts-grid {
    grid-template-columns: repeat(2, 1fr);
  }
}

@media (max-width: 768px) {
  .reverse-content {
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
