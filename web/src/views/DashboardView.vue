<template>
  <div class="dashboard-view">
    <div class="dashboard-header">
      <h2>{{ config?.Name || 'SmartPing' }} - 正向监控</h2>
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
            <span class="chart-card__title">{{ config?.Name }} -> {{ target.name }}</span>
            <span v-if="target.loading" class="chart-card__loading-text">加载中...</span>
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
              <span>加载失败</span>
            </div>
          </div>
        </div>
      </div>

      <div class="agent-sidebar">
        <el-card>
          <template #header>
            <div class="card-header">
              <span>节点列表</span>
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
              <span>{{ agent.name }}</span>
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
            placeholder="开始时间"
            format="YYYY-MM-DD HH:mm"
            value-format="YYYY-MM-DD HH:mm"
          />
          <el-date-picker
            v-model="endTime"
            type="datetime"
            placeholder="结束时间"
            format="YYYY-MM-DD HH:mm"
            value-format="YYYY-MM-DD HH:mm"
          />
          <el-button type="primary" @click="loadDetailData">查询</el-button>
          <el-button @click="saveChartImage">保存图片</el-button>
          <el-button-group>
            <el-button v-for="range in timeRanges" :key="range.hours" @click="setTimeRange(range.hours)">
              {{ range.label }}
            </el-button>
          </el-button-group>
        </div>
        <PingChart ref="pingChartRef" v-if="detailData" :data="detailData" :height="400" />
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loading, Warning } from '@element-plus/icons-vue'
import PingChart from '@/components/charts/PingChart.vue'
import PingMiniChart from '@/components/charts/PingMiniChart.vue'
import { getConfig, getProxyConfig } from '@/api/topology'
import type { Config, PingLogData } from '@/types'

interface PingTarget {
  name: string
  addr: string
  chartData: PingLogData | null
  loading: boolean
  targetIp: string
}

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

const timeRanges = [
  { label: '1小时', hours: 1 },
  { label: '3小时', hours: 3 },
  { label: '6小时', hours: 6 },
  { label: '12小时', hours: 12 },
  { label: '1天', hours: 24 },
  { label: '3天', hours: 72 },
  { label: '7天', hours: 168 }
]

const loadConfig = async (proxyUrl?: string) => {
  try {
    let cfg: Config
    if (proxyUrl) {
      cfg = await getProxyConfig(proxyUrl)
    } else {
      cfg = await getConfig()
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
  }
}

const loadAllCharts = async () => {
  await Promise.all(pingTargets.value.map(target => loadChartData(target)))
}

const loadChartData = async (target: PingTarget) => {
  target.loading = true
  try {
    const response = await fetch(`/api/ping.json?ip=${encodeURIComponent(target.targetIp)}`)
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
  currentAgent.value = agent.addr
  const proxyUrl = `http://${agent.addr}:${config.value?.Port}`
  await loadConfig(proxyUrl)
  agent.loading = false
}

const showDetail = async (target: PingTarget) => {
  detailTitle.value = `${config.value?.Name} -> ${target.name}`
  currentTargetIp.value = target.targetIp

  // 设置默认时间范围
  setTimeRange(6)

  detailVisible.value = true
  await loadDetailData()
}

const loadDetailData = async () => {
  if (!currentTargetIp.value) return

  let url = `/api/ping.json?ip=${encodeURIComponent(currentTargetIp.value)}`
  if (startTime.value) {
    url += `&starttime=${encodeURIComponent(startTime.value)}`
  }
  if (endTime.value) {
    url += `&endtime=${encodeURIComponent(endTime.value)}`
  }

  try {
    const response = await fetch(url)
    detailData.value = await response.json()
  } catch (e) {
    console.error('加载数据失败', e)
  }
}

const setTimeRange = (hours: number) => {
  const end = new Date()
  const start = new Date(end.getTime() - hours * 60 * 60 * 1000)

  const format = (d: Date) => {
    const pad = (n: number) => (n < 10 ? '0' + n : n)
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }

  startTime.value = format(start)
  endTime.value = format(end)
  loadDetailData()
}

const saveChartImage = () => {
  pingChartRef.value?.saveAsImage()
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped lang="scss">
.dashboard-view {
  height: 100%;
}

.dashboard-header {
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
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
