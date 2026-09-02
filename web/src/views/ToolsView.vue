<template>
  <div class="page-shell tools-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('tools.title') }}</span>
        <h1 class="page-title">{{ $t('tools.title') }}</h1>
        <p class="page-subtitle">{{ $t('tools.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.probes') }}</span>
            <strong class="page-kpi__value">{{ checkedCount }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.loaded') }}</span>
            <strong class="page-kpi__value">{{ successCount }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.issues') }}</span>
            <strong class="page-kpi__value">{{ errorCount }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('tools.lastRun') }}</span>
            <strong class="page-kpi__value tools-view__time-kpi">{{ lastRunLabel || '--' }}</strong>
          </div>
        </div>
      </div>
    </div>

    <section v-if="configLoading && !config" class="surface-panel empty-state">
      <el-icon class="is-loading"><Loading /></el-icon>
      <span>{{ $t('common.loading') }}</span>
    </section>

    <section v-else-if="configError && !config" class="surface-panel empty-state">
      <el-icon class="tools-view__state-icon text-danger"><Warning /></el-icon>
      <span>{{ $t('common.configLoadFailedNetwork') }}</span>
      <el-button size="small" @click="loadConfig">{{ $t('common.retry') }}</el-button>
    </section>

    <template v-else>
      <section class="surface-panel">
        <div class="surface-panel__header">
          <div>
            <h2 class="surface-panel__title">{{ $t('tools.check') }}</h2>
            <p class="surface-panel__description">{{ target || $t('tools.enterTarget') }}</p>
          </div>
        </div>

        <div class="tools-view__search">
          <el-select
            v-model="toolType"
            class="tools-view__tool-select"
            :aria-label="$t('tools.toolType')"
          >
            <el-option label="ICMP PING" value="ping" />
          </el-select>
          <el-input
            v-model="target"
            :placeholder="$t('tools.enterTarget')"
            @keyup.enter="runCheck"
          />
          <el-button type="primary" :loading="checking" @click="runCheck">
            {{ $t('tools.check') }}
          </el-button>
        </div>
      </section>

      <section class="surface-panel">
        <div class="surface-panel__header">
          <div>
            <h2 class="surface-panel__title">{{ $t('tools.title') }}</h2>
            <p class="surface-panel__description">{{ results.length }} {{ $t('common.probes') }}</p>
          </div>
        </div>

        <div class="table-scroll">
          <el-table :data="results" stripe style="width: 100%">
            <el-table-column width="54" align="center">
              <template #default="{ row }">
                <el-checkbox
                  v-model="row.checked"
                  :aria-label="$t('tools.toggleProbe', { name: row.name })"
                />
              </template>
            </el-table-column>
            <el-table-column prop="name" :label="$t('common.node')" min-width="140" />
            <el-table-column :label="$t('tools.resolvedIP')" min-width="160">
              <template #default="{ row }">
                <span class="mono">{{ row.result?.ip || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('tools.sent')" width="80" align="center">
              <template #default="{ row }">
                {{ row.result?.ping?.SendPk ?? '-' }}
              </template>
            </el-table-column>
            <el-table-column :label="$t('tools.received')" width="80" align="center">
              <template #default="{ row }">
                {{ row.result?.ping?.RevcPk ?? '-' }}
              </template>
            </el-table-column>
            <el-table-column :label="$t('tools.packetLoss')" width="90" align="center">
              <template #default="{ row }">
                <span :class="{ 'text-danger': row.result?.ping?.LossPk > 0 }">
                  {{ row.result?.ping?.LossPk !== undefined ? row.result.ping.LossPk + '%' : '-' }}
                </span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('tools.latency')" min-width="180">
              <template #default="{ row }">
                <template v-if="row.result?.ping">
                  <span class="tools-view__delay mono">
                    {{ row.result.ping.MinDelay?.toFixed(1) || '-' }} /
                    {{ row.result.ping.AvgDelay?.toFixed(1) || '-' }} /
                    {{ row.result.ping.MaxDelay?.toFixed(1) || '-' }} ms
                  </span>
                </template>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column :label="$t('common.status')" width="108" align="center">
              <template #default="{ row }">
                <el-icon v-if="row.loading" class="is-loading"><Loading /></el-icon>
                <el-tooltip
                  v-else-if="row.error || row.result?.status === 'false'"
                  :content="rowErrorMessage(row as ResultRow)"
                  placement="top"
                >
                  <span class="tools-view__status text-danger">
                    <el-icon><Warning /></el-icon>
                    <span>{{ $t('tools.failed') }}</span>
                  </span>
                </el-tooltip>
                <span
                  v-else-if="row.result?.status === 'true'"
                  class="tools-view__status text-success"
                >
                  <el-icon><SuccessFilled /></el-icon>
                  <span>{{ $t('tools.succeeded') }}</span>
                </span>
                <span v-else class="tools-view__status tools-view__status--muted">
                  {{ $t('tools.notRun') }}
                </span>
              </template>
            </el-table-column>
            <template #empty>
              <div class="empty-state tools-view__empty">
                <span>{{ $t('tools.noProbeNodes') }}</span>
              </div>
            </template>
          </el-table>
        </div>
      </section>
    </template>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, onUnmounted, ref } from 'vue'
import {
  ElCheckbox,
  ElInput,
  ElMessage,
  ElOption,
  ElSelect,
  ElTable,
  ElTableColumn,
  ElTooltip
} from 'element-plus'
import { useI18n } from 'vue-i18n'
import '@/plugins/elementPlusToolsStyles'
import { Loading, SuccessFilled, Warning } from '@element-plus/icons-vue'
import { isRequestCanceled } from '@/api'
import { fetchConfig } from '@/api/config'
import { runTools } from '@/api/tools'
import { mapWithConcurrency } from '@/utils/concurrency'
import { formatTime } from '@/utils/format'
import type { Config, ToolsResult } from '@/types'

interface ResultRow {
  name: string
  addr: string
  port: number
  checked: boolean
  loading: boolean
  result: ToolsResult | null
  error: string | null
}

const { t } = useI18n()
const config = ref<Config | null>(null)
const toolType = ref('ping')
const target = ref('')
const checking = ref(false)
const results = ref<ResultRow[]>([])
const configLoading = ref(true)
const configError = ref(false)
const lastRunAt = ref<Date | null>(null)
let isUnmounted = false
let configRequestId = 0
let checkRequestId = 0
let configAbortController: AbortController | null = null
let checkAbortController: AbortController | null = null
const CHECK_CONCURRENCY = 4

const checkedCount = computed(() => results.value.filter((row) => row.checked).length)
const successCount = computed(
  () => results.value.filter((row) => row.result?.status === 'true').length
)
const errorCount = computed(
  () => results.value.filter((row) => row.error || row.result?.status === 'false').length
)
const lastRunLabel = computed(() => (lastRunAt.value ? formatTime(lastRunAt.value) : ''))

const loadConfig = async () => {
  configAbortController?.abort()
  checkAbortController?.abort()
  checkAbortController = null
  checkRequestId++
  checking.value = false
  results.value.forEach((row) => (row.loading = false))
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
    target.value = cfg.Addr

    results.value = Object.values(cfg.Network)
      .filter((node) => node.Smartping)
      .map((node) => ({
        name: node.Name,
        addr: node.Addr,
        port: cfg.Port,
        checked: true,
        loading: false,
        result: null,
        error: null
      }))
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

const runCheck = async () => {
  if (checking.value) {
    return
  }

  const normalizedTarget = target.value.trim()
  if (!normalizedTarget) {
    ElMessage.warning(t('tools.enterTarget'))
    return
  }

  const checkedRows = results.value.filter((row) => row.checked)
  if (checkedRows.length === 0) {
    ElMessage.warning(t('tools.selectProbe'))
    return
  }

  const requestId = ++checkRequestId
  const controller = new AbortController()
  checkAbortController = controller
  target.value = normalizedTarget
  checking.value = true
  results.value.forEach((row) => {
    row.loading = false
    row.result = null
    row.error = null
  })

  try {
    await mapWithConcurrency(checkedRows, CHECK_CONCURRENCY, async (row) => {
      row.loading = true

      try {
        const result = await runTools(
          `${row.addr}:${row.port}`,
          normalizedTarget,
          controller.signal
        )
        if (isUnmounted || requestId !== checkRequestId) {
          return
        }
        row.result = result
      } catch (error: unknown) {
        if (isRequestCanceled(error) || isUnmounted || requestId !== checkRequestId) {
          return
        }
        row.error = error instanceof Error ? error.message : t('tools.requestFailed')
      } finally {
        if (!isUnmounted && requestId === checkRequestId) {
          row.loading = false
        }
      }
    })
  } finally {
    if (checkAbortController === controller) {
      checkAbortController = null
    }
    if (!isUnmounted && requestId === checkRequestId) {
      checking.value = false
      lastRunAt.value = new Date()
    }
  }
}

const rowErrorMessage = (row: ResultRow) =>
  row.error || row.result?.error || t('tools.requestFailed')

onMounted(() => {
  loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configAbortController?.abort()
  checkAbortController?.abort()
  configRequestId++
  checkRequestId++
})
</script>

<style scoped lang="scss">
.tools-view__search {
  display: grid;
  grid-template-columns: 180px minmax(0, 1fr) auto;
  gap: 12px;
}

.tools-view__tool-select {
  width: 100%;
}

.tools-view__delay {
  color: var(--color-text-regular);
}

.tools-view__time-kpi {
  font-size: 14px;
}

.tools-view__state-icon {
  font-size: 28px;
}

.tools-view__status {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 5px;
  font-size: 12px;
  white-space: nowrap;
}

.tools-view__status--muted {
  color: var(--color-text-secondary);
}

.tools-view__empty {
  min-height: 160px;
}

@media (max-width: 900px) {
  .tools-view__search {
    grid-template-columns: 1fr;
  }
}
</style>
