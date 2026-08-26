<template>
  <div class="page-shell alerts-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('alerts.title') }}</span>
        <h1 class="page-title">{{ $t('alerts.title') }}</h1>
        <p class="page-subtitle">{{ $t('alerts.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.records') }}</span>
            <strong class="page-kpi__value">{{ alerts.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.sources') }}</span>
            <strong class="page-kpi__value">{{ nodes.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.selectedDate') }}</span>
            <strong class="page-kpi__value alerts-view__date-kpi">{{
              selectedDate || '--'
            }}</strong>
          </div>
        </div>
      </div>
    </div>

    <div class="page-frame alerts-view__frame">
      <aside class="page-aside page-aside--narrow">
        <section class="surface-panel surface-panel--soft">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('alerts.alertArchive') }}</h2>
              <p class="surface-panel__description">
                {{ dates.length }} {{ $t('common.records') }}
              </p>
            </div>
          </div>

          <div class="list-stack alerts-view__archive">
            <button
              v-for="date in dates"
              :key="date"
              type="button"
              class="list-row alerts-view__archive-item"
              :class="{ 'is-active': selectedDate === date }"
              @click="loadAlertsByDate(date)"
            >
              <span class="list-row__title">{{ date }}</span>
            </button>
          </div>
        </section>
      </aside>

      <div class="page-main">
        <section class="surface-panel alerts-view__table-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('alerts.alertHistory') }}</h2>
              <p class="surface-panel__description">
                {{ alerts.length }} {{ $t('common.records') }}
              </p>
            </div>
          </div>

          <div class="table-scroll">
            <el-table :data="alerts" stripe style="width: 100%">
              <el-table-column prop="Logtime" :label="$t('alerts.alertDate')" min-width="170" />
              <el-table-column prop="Fromname" :label="$t('alerts.sourceNode')" min-width="120" />
              <el-table-column prop="Fromip" :label="$t('alerts.sourceIP')" min-width="140" />
              <el-table-column prop="Targetname" :label="$t('alerts.targetNode')" min-width="120" />
              <el-table-column prop="Targetip" :label="$t('alerts.targetIP')" min-width="140" />
              <el-table-column :label="$t('common.operation')" width="100">
                <template #default="{ row }">
                  <el-button size="small" @click="showMtr(row)">MTR</el-button>
                </template>
              </el-table-column>
            </el-table>
          </div>
        </section>
      </div>

      <aside class="page-aside page-aside--narrow">
        <button
          type="button"
          class="surface-panel alerts-view__back-link"
          @click="router.push('/topology')"
        >
          <div>
            <span class="page-eyebrow">{{ $t('alerts.backToTopology') }}</span>
            <strong>{{ $t('topology.title') }}</strong>
          </div>
          <el-icon><ArrowLeft /></el-icon>
        </button>

        <section class="surface-panel surface-panel--soft">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('common.nodeList') }}</h2>
              <p class="surface-panel__description">
                {{ nodes.length }} {{ $t('common.sources') }}
              </p>
            </div>
          </div>

          <div class="list-stack">
            <div v-for="node in nodes" :key="node.addr" class="list-row">
              <div class="list-row__meta">
                <el-icon v-if="node.loading" class="is-loading"><Loading /></el-icon>
                <div v-else class="alerts-view__node-dot"></div>
                <div class="list-row__text">
                  <span class="list-row__title">{{ displayName(node.name) }}</span>
                  <span class="list-row__caption">{{ node.addr }}</span>
                </div>
              </div>
            </div>
          </div>
        </section>
      </aside>
    </div>

    <el-dialog v-model="mtrVisible" :title="$t('alerts.mtrResult')" width="760px">
      <div class="table-scroll">
        <el-table :data="mtrData" stripe style="width: 100%">
          <el-table-column prop="Host" :label="$t('alerts.host')" min-width="150" />
          <el-table-column :label="$t('alerts.packetLossRate')" width="90">
            <template #default="{ row }">
              {{ ((row.Loss / row.Send) * 100).toFixed(2) }}%
            </template>
          </el-table-column>
          <el-table-column prop="Send" :label="$t('tools.sent')" width="70" />
          <el-table-column :label="$t('alerts.latest')" width="80">
            <template #default="{ row }">
              {{ (row.Last / 1000000).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.average')" width="80">
            <template #default="{ row }">
              {{ (row.Avg / 1000000).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.best')" width="80">
            <template #default="{ row }">
              {{ (row.Best / 1000000).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.worst')" width="80">
            <template #default="{ row }">
              {{ (row.Wrst / 1000000).toFixed(2) }}
            </template>
          </el-table-column>
          <el-table-column prop="StDev" :label="$t('alerts.standardDeviation')" width="90" />
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { onMounted, onUnmounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import { ArrowLeft, Loading } from '@element-plus/icons-vue'
import { fetchConfig } from '@/api/config'
import { getAlerts } from '@/api/alert'
import { displayName } from '@/utils/format'
import type { AlertLog, Config, MtrResult } from '@/types'

const router = useRouter()
const config = ref<Config | null>(null)
const dates = ref<string[]>([])
const selectedDate = ref('')
const alerts = ref<AlertLog[]>([])
const nodes = ref<Array<{ name: string; addr: string; loading: boolean }>>([])

const mtrVisible = ref(false)
const mtrData = ref<MtrResult[]>([])
let isUnmounted = false
let configRequestId = 0
let alertsRequestId = 0

const loadConfig = async () => {
  const requestId = ++configRequestId
  try {
    const cfg = await fetchConfig()
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    config.value = cfg

    nodes.value = Object.values(cfg.Network)
      .filter((node) => node.Topology && node.Topology.length > 0)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false }))

    await loadAllAlerts()
  } catch (error) {
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
  }
}

const loadAllAlerts = async () => {
  if (!config.value) {
    return
  }

  const requestId = ++alertsRequestId
  const port = config.value.Port
  const requestNodes = nodes.value
  requestNodes.forEach((node) => (node.loading = true))
  try {
    const results = await Promise.all(
      requestNodes.map(async (node) => {
        try {
          return await getAlerts(`http://${node.addr}:${port}`)
        } catch (error) {
          console.error(`获取 ${node.name} 报警记录失败`, error)
          return null
        }
      })
    )
    if (isUnmounted || requestId !== alertsRequestId) {
      return
    }

    const allDates = new Set<string>()
    const allAlerts: AlertLog[] = []
    results.forEach((data) => {
      data?.dates.forEach((date) => allDates.add(date))
      if (data) allAlerts.push(...data.logs)
    })
    dates.value = Array.from(allDates).sort().reverse()
    alerts.value = allAlerts.sort((a, b) => b.Logtime.localeCompare(a.Logtime))
  } finally {
    if (!isUnmounted && requestId === alertsRequestId) {
      requestNodes.forEach((node) => (node.loading = false))
    }
  }
}

const loadAlertsByDate = async (date: string) => {
  selectedDate.value = date

  if (!config.value) {
    return
  }

  const requestId = ++alertsRequestId
  const port = config.value.Port
  const requestNodes = nodes.value
  requestNodes.forEach((node) => (node.loading = true))
  try {
    const results = await Promise.all(
      requestNodes.map(async (node) => {
        try {
          return await getAlerts(`http://${node.addr}:${port}`, date)
        } catch (error) {
          console.error(`获取 ${node.name} 报警记录失败`, error)
          return null
        }
      })
    )
    if (isUnmounted || requestId !== alertsRequestId || selectedDate.value !== date) {
      return
    }

    alerts.value = results
      .flatMap((data) => data?.logs || [])
      .sort((a, b) => b.Logtime.localeCompare(a.Logtime))
  } finally {
    if (!isUnmounted && requestId === alertsRequestId) {
      requestNodes.forEach((node) => (node.loading = false))
    }
  }
}

const showMtr = (row: AlertLog) => {
  try {
    mtrData.value = JSON.parse(row.Tracert)
    mtrVisible.value = true
  } catch (error) {
    console.error('解析MTR数据失败', error)
  }
}

onMounted(() => {
  loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configRequestId++
  alertsRequestId++
})
</script>

<style scoped lang="scss">
.alerts-view__frame {
  align-items: stretch;
}

.alerts-view__date-kpi {
  font-size: 14px;
}

.alerts-view__archive {
  max-height: min(62vh, 620px);
  overflow: auto;
  padding-right: 2px;
}

.alerts-view__archive-item {
  width: 100%;
  border: none;
  cursor: pointer;
  font: inherit;
}

.alerts-view__table-panel {
  height: 100%;
  display: flex;
  flex-direction: column;
}

.alerts-view__back-link {
  width: 100%;
  text-align: left;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  font: inherit;
  color: var(--color-text-primary);

  .el-icon {
    font-size: 20px;
    color: var(--color-primary);
  }
}

.alerts-view__node-dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
  background: var(--color-primary);
  animation: subtle-pulse 2.4s ease infinite;
}
</style>
