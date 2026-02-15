<template>
  <div class="tools-view">
    <div class="tools-header">
      <h2>检测工具</h2>
    </div>

    <div class="tools-content">
      <div class="search-box">
        <el-select v-model="toolType" style="width: 140px;">
          <el-option label="ICMP PING" value="ping" />
        </el-select>
        <el-input
          v-model="target"
          placeholder="输入目标地址"
          style="width: 280px;"
          @keyup.enter="runCheck"
        />
        <el-button type="primary" @click="runCheck" :loading="checking">
          检测
        </el-button>
      </div>

      <div class="table-wrapper">
        <el-table :data="results" stripe style="width: 100%">
          <el-table-column width="50" align="center">
            <template #default="{ row }">
              <el-checkbox v-model="row.checked" />
            </template>
          </el-table-column>
          <el-table-column prop="name" label="节点" min-width="100" />
          <el-table-column label="解析IP" min-width="120">
            <template #default="{ row }">
              {{ row.result?.ip || '-' }}
            </template>
          </el-table-column>
          <el-table-column label="发送" width="70" align="center">
            <template #default="{ row }">
              {{ row.result?.ping?.SendPk || '-' }}
            </template>
          </el-table-column>
          <el-table-column label="接收" width="70" align="center">
            <template #default="{ row }">
              {{ row.result?.ping?.RevcPk || '-' }}
            </template>
          </el-table-column>
          <el-table-column label="丢包" width="80" align="center">
            <template #default="{ row }">
              <span :class="{'text-danger': row.result?.ping?.LossPk > 0}">
                {{ row.result?.ping?.LossPk !== undefined ? row.result.ping.LossPk + '%' : '-' }}
              </span>
            </template>
          </el-table-column>
          <el-table-column label="延迟" min-width="140">
            <template #default="{ row }">
              <template v-if="row.result?.ping">
                <span class="delay-info">
                  {{ row.result.ping.MinDelay?.toFixed(1) || '-' }} /
                  {{ row.result.ping.AvgDelay?.toFixed(1) || '-' }} /
                  {{ row.result.ping.MaxDelay?.toFixed(1) || '-' }} ms
                </span>
              </template>
              <span v-else>-</span>
            </template>
          </el-table-column>
          <el-table-column label="状态" width="60" align="center">
            <template #default="{ row }">
              <el-icon v-if="row.loading" class="is-loading"><Loading /></el-icon>
              <el-icon v-else-if="row.error" class="text-danger"><Warning /></el-icon>
              <el-icon v-else-if="row.result?.status === 'true'" class="text-success"><SuccessFilled /></el-icon>
            </template>
          </el-table-column>
        </el-table>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { Loading, Warning, SuccessFilled } from '@element-plus/icons-vue'
import { getConfig } from '@/api/topology'
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

const config = ref<Config | null>(null)
const toolType = ref('ping')
const target = ref('')
const checking = ref(false)
const results = ref<ResultRow[]>([])

const loadConfig = async () => {
  try {
    const cfg = await getConfig()
    config.value = cfg
    target.value = cfg.Addr

    results.value = Object.values(cfg.Network)
      .filter(n => n.Smartping)
      .map(n => ({
        name: n.Name,
        addr: n.Addr,
        port: cfg.Port,
        checked: true,
        loading: false,
        result: null,
        error: null
      }))
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const runCheck = async () => {
  if (!target.value) {
    return
  }

  checking.value = true

  const checkedRows = results.value.filter(r => r.checked)

  await Promise.all(checkedRows.map(async (row) => {
    row.loading = true
    row.result = null
    row.error = null

    try {
      const result = await runTools(`${row.addr}:${row.port}`, target.value)
      row.result = result
    } catch (e: any) {
      row.error = e.message || '请求失败'
    } finally {
      row.loading = false
    }
  }))

  checking.value = false
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped lang="scss">
.tools-view {
  height: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.tools-header {
  margin-bottom: 20px;
  flex-shrink: 0;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.tools-content {
  background-color: var(--color-bg-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  padding: 20px;
  flex: 1;
  overflow: hidden;
  display: flex;
  flex-direction: column;
  min-width: 0;
}

.search-box {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  flex-shrink: 0;
}

.table-wrapper {
  flex: 1;
  overflow: auto;
  min-width: 0;
}

.delay-info {
  font-family: monospace;
  font-size: 13px;
  color: var(--color-text-regular);
}

.text-danger {
  color: var(--color-danger);
}

.text-success {
  color: var(--color-success);
}
</style>
