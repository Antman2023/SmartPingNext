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
        </div>
      </div>
    </div>

    <section class="surface-panel">
      <div class="surface-panel__header">
        <div>
          <h2 class="surface-panel__title">{{ $t('tools.check') }}</h2>
          <p class="surface-panel__description">{{ target || $t('tools.enterTarget') }}</p>
        </div>
      </div>

      <div class="tools-view__search">
        <el-select v-model="toolType" class="tools-view__tool-select">
          <el-option label="ICMP PING" value="ping" />
        </el-select>
        <el-input v-model="target" :placeholder="$t('tools.enterTarget')" @keyup.enter="runCheck" />
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
              <el-checkbox v-model="row.checked" />
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
          <el-table-column :label="$t('common.status')" width="80" align="center">
            <template #default="{ row }">
              <el-icon v-if="row.loading" class="is-loading"><Loading /></el-icon>
              <el-icon v-else-if="row.error" class="text-danger"><Warning /></el-icon>
              <el-icon v-else-if="row.result?.status === 'true'" class="text-success"
                ><SuccessFilled
              /></el-icon>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </section>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Loading, SuccessFilled, Warning } from '@element-plus/icons-vue'
import { fetchConfig } from '@/api/config'
import { runTools } from '@/api/tools'
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

const checkedCount = computed(() => results.value.filter((row) => row.checked).length)
const successCount = computed(
  () => results.value.filter((row) => row.result?.status === 'true').length
)
const errorCount = computed(
  () => results.value.filter((row) => row.error || row.result?.status === 'false').length
)

const loadConfig = async () => {
  try {
    const cfg = await fetchConfig()
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
    console.error('加载配置失败', error)
  }
}

const runCheck = async () => {
  if (!target.value) {
    return
  }

  checking.value = true
  const checkedRows = results.value.filter((row) => row.checked)

  await Promise.all(
    checkedRows.map(async (row) => {
      row.loading = true
      row.result = null
      row.error = null

      try {
        const result = await runTools(`${row.addr}:${row.port}`, target.value)
        row.result = result
      } catch (error: unknown) {
        row.error = error instanceof Error ? error.message : t('tools.requestFailed')
      } finally {
        row.loading = false
      }
    })
  )

  checking.value = false
}

onMounted(() => {
  loadConfig()
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

@media (max-width: 900px) {
  .tools-view__search {
    grid-template-columns: 1fr;
  }
}
</style>
