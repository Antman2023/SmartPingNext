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
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.issues') }}</span>
            <strong class="page-kpi__value">{{ failedNodes }}</strong>
          </div>
        </div>

        <RefreshStatus
          class="surface-panel surface-panel--tight"
          :loading="configLoading || alertsLoading"
          :last-updated="lastUpdatedLabel"
          @refresh="retryAlerts"
        />
      </div>
    </div>

    <div class="page-frame alerts-view__frame">
      <aside class="page-aside page-aside--narrow">
        <section
          v-loading="configLoading || alertsLoading"
          class="surface-panel surface-panel--soft"
        >
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('alerts.alertArchive') }}</h2>
              <p class="surface-panel__description">
                {{ $t('alerts.archiveDays', { count: dates.length }) }}
              </p>
            </div>
          </div>

          <div v-if="dates.length" class="list-stack alerts-view__archive">
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
          <div
            v-else-if="!configLoading && !alertsLoading"
            class="empty-state alerts-view__archive-empty"
          >
            <span>{{ $t('alerts.noArchiveDates') }}</span>
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
            <el-table
              v-loading="configLoading || alertsLoading"
              :data="alerts"
              stripe
              style="width: 100%"
            >
              <el-table-column prop="Logtime" :label="$t('alerts.alertDate')" min-width="170" />
              <el-table-column prop="Fromname" :label="$t('alerts.sourceNode')" min-width="120" />
              <el-table-column prop="Fromip" :label="$t('alerts.sourceIP')" min-width="140" />
              <el-table-column prop="Targetname" :label="$t('alerts.targetNode')" min-width="120" />
              <el-table-column prop="Targetip" :label="$t('alerts.targetIP')" min-width="140" />
              <el-table-column :label="$t('common.operation')" width="100">
                <template #default="{ row }">
                  <el-button size="small" @click="showMtr(row as AlertLog)">MTR</el-button>
                </template>
              </el-table-column>
              <template #empty>
                <div class="empty-state alerts-view__empty">
                  <span>{{
                    alertsLoadError ? $t('alerts.loadFailed') : $t('alerts.noRecords')
                  }}</span>
                  <el-button v-if="alertsLoadError" size="small" @click="retryAlerts">
                    {{ $t('common.retry') }}
                  </el-button>
                </div>
              </template>
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
                <el-icon
                  v-else-if="node.error"
                  class="alerts-view__node-error"
                  :title="$t('alerts.sourceLoadFailed')"
                >
                  <Warning />
                </el-icon>
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
      <div class="table-scroll alerts-view__mtr-scroll">
        <el-table :data="mtrData" stripe style="width: 100%">
          <el-table-column :label="$t('alerts.hop')" width="64">
            <template #default="{ $index }">{{ $index + 1 }}</template>
          </el-table-column>
          <el-table-column prop="Host" :label="$t('alerts.host')" min-width="150" />
          <el-table-column :label="$t('alerts.packetLossRate')" width="90">
            <template #default="{ row }">
              {{ formatLossRate(row as MtrResult) }}
            </template>
          </el-table-column>
          <el-table-column prop="Send" :label="$t('tools.sent')" width="70" />
          <el-table-column :label="$t('alerts.latest')" width="80">
            <template #default="{ row }">
              {{ formatMtrDuration(row.Last) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.average')" width="80">
            <template #default="{ row }">
              {{ formatMtrDuration(row.Avg) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.best')" width="80">
            <template #default="{ row }">
              {{ formatMtrDuration(row.Best) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.worst')" width="80">
            <template #default="{ row }">
              {{ formatMtrDuration(row.Wrst) }}
            </template>
          </el-table-column>
          <el-table-column :label="$t('alerts.standardDeviation')" width="90">
            <template #default="{ row }">
              {{ formatMtrValue(row.StDev) }}
            </template>
          </el-table-column>
          <template #empty>
            <span>{{ $t('alerts.noMtrData') }}</span>
          </template>
        </el-table>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElDialog, ElMessage, ElTable, ElTableColumn } from 'element-plus'
import { ArrowLeft, Loading, Warning } from '@element-plus/icons-vue'
import '@/plugins/elementPlusAlertsStyles'
import RefreshStatus from '@/components/common/RefreshStatus.vue'
import { isRequestCanceled } from '@/api'
import { fetchConfig } from '@/api/config'
import { getAlerts } from '@/api/alert'
import { mapWithConcurrency } from '@/utils/concurrency'
import { displayName, formatTime } from '@/utils/format'
import type { AlertData, AlertLog, Config, MtrResult } from '@/types'

interface AlertNode {
  name: string
  addr: string
  loading: boolean
  error: boolean
}

const router = useRouter()
const { t } = useI18n()
const config = ref<Config | null>(null)
const dates = ref<string[]>([])
const selectedDate = ref('')
const alerts = ref<AlertLog[]>([])
const nodes = ref<AlertNode[]>([])
const alertsLoading = ref(false)
const alertsLoadError = ref(false)
const configLoading = ref(true)
const lastUpdatedAt = ref<Date | null>(null)

const mtrVisible = ref(false)
const mtrData = ref<MtrResult[]>([])
let isUnmounted = false
let configRequestId = 0
let alertsRequestId = 0
let configAbortController: AbortController | null = null
let alertsAbortController: AbortController | null = null
const ALERT_CONCURRENCY = 4
const failedNodes = computed(() => nodes.value.filter((node) => node.error).length)
const lastUpdatedLabel = computed(() =>
  lastUpdatedAt.value ? formatTime(lastUpdatedAt.value) : ''
)

const loadConfig = async () => {
  configAbortController?.abort()
  alertsAbortController?.abort()
  alertsAbortController = null
  alertsRequestId++
  alertsLoading.value = false
  nodes.value.forEach((node) => (node.loading = false))
  const controller = new AbortController()
  configAbortController = controller
  const requestId = ++configRequestId
  configLoading.value = true
  try {
    const cfg = await fetchConfig(controller.signal)
    if (isUnmounted || requestId !== configRequestId) {
      return
    }
    config.value = cfg

    nodes.value = Object.values(cfg.Network)
      .filter((node) => node.Topology && node.Topology.length > 0)
      .map((node) => ({ name: node.Name, addr: node.Addr, loading: false, error: false }))

    await loadAllAlerts()
  } catch (error) {
    if (isRequestCanceled(error) || isUnmounted || requestId !== configRequestId) {
      return
    }
    console.error('加载配置失败', error)
    alertsLoadError.value = true
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

const loadAlerts = async (date?: string) => {
  if (!config.value) {
    return
  }

  alertsAbortController?.abort()
  const controller = new AbortController()
  alertsAbortController = controller
  const requestId = ++alertsRequestId
  const port = config.value.Port
  const requestNodes = nodes.value
  alertsLoading.value = true
  alertsLoadError.value = false
  requestNodes.forEach((node) => {
    node.loading = true
    node.error = false
  })
  try {
    const results = await mapWithConcurrency(requestNodes, ALERT_CONCURRENCY, async (node) => {
      try {
        const data = await getAlerts(`http://${node.addr}:${port}`, date, controller.signal)
        return { node, data, error: false }
      } catch (error) {
        if (isRequestCanceled(error)) {
          return { node, data: null, error: false }
        }
        console.error(`获取 ${node.name} 报警记录失败`, error)
        return { node, data: null, error: true }
      }
    })
    if (isUnmounted || requestId !== alertsRequestId) {
      return
    }

    results.forEach((result) => {
      result.node.error = result.error
    })
    const successfulData = results
      .map((result) => result.data)
      .filter((data): data is AlertData => data !== null)
    alertsLoadError.value = requestNodes.length > 0 && successfulData.length === 0

    if (date === undefined) {
      const allDates = new Set<string>()
      successfulData.forEach((data) => data.dates.forEach((item) => allDates.add(item)))
      dates.value = Array.from(allDates).sort().reverse()
    }

    alerts.value = successfulData
      .flatMap((data) => data.logs)
      .sort((a, b) => b.Logtime.localeCompare(a.Logtime))
  } finally {
    if (alertsAbortController === controller) {
      alertsAbortController = null
    }
    if (!isUnmounted && requestId === alertsRequestId) {
      requestNodes.forEach((node) => (node.loading = false))
      alertsLoading.value = false
      lastUpdatedAt.value = new Date()
    }
  }
}

const loadAllAlerts = () => loadAlerts()

const loadAlertsByDate = (date: string) => {
  selectedDate.value = date
  return loadAlerts(date)
}

const retryAlerts = () => {
  if (!config.value) {
    return loadConfig()
  }
  return selectedDate.value ? loadAlertsByDate(selectedDate.value) : loadAllAlerts()
}

const isFiniteNumber = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value)

const isMtrResult = (value: unknown): value is MtrResult => {
  if (typeof value !== 'object' || value === null) {
    return false
  }
  const item = value as Record<string, unknown>
  return (
    typeof item.Host === 'string' &&
    isFiniteNumber(item.Send) &&
    isFiniteNumber(item.Loss) &&
    isFiniteNumber(item.Last) &&
    isFiniteNumber(item.Avg) &&
    isFiniteNumber(item.Best) &&
    isFiniteNumber(item.Wrst) &&
    isFiniteNumber(item.StDev)
  )
}

const showMtr = (row: AlertLog) => {
  try {
    const data: unknown = JSON.parse(row.Tracert)
    if (!Array.isArray(data) || !data.every(isMtrResult)) {
      throw new Error('Invalid MTR data')
    }
    mtrData.value = data
    mtrVisible.value = true
  } catch (error) {
    console.error('解析MTR数据失败', error)
    ElMessage.error(t('alerts.mtrUnavailable'))
  }
}

const formatLossRate = (row: MtrResult): string => {
  if (!Number.isFinite(row.Send) || row.Send <= 0 || !Number.isFinite(row.Loss)) {
    return '--'
  }
  return `${((row.Loss / row.Send) * 100).toFixed(2)}%`
}

const formatMtrDuration = (value: number): string =>
  Number.isFinite(value) ? (value / 1_000_000).toFixed(2) : '--'

const formatMtrValue = (value: number): string => (Number.isFinite(value) ? value.toFixed(2) : '--')

onMounted(() => {
  loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configAbortController?.abort()
  alertsAbortController?.abort()
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

.alerts-view__archive-empty {
  min-height: 120px;
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

.alerts-view__node-error {
  color: var(--color-danger);
}

.alerts-view__mtr-scroll {
  max-height: min(68vh, 620px);
  max-height: min(68svh, 620px);
}

.alerts-view__empty {
  min-height: 220px;
}
</style>
