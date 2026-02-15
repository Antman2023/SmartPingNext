<template>
  <div class="tools-view">
    <div class="tools-header">
      <h2>检测工具</h2>
    </div>

    <div class="tools-content">
      <div class="search-box">
        <el-select v-model="toolType" style="width: 150px;">
          <el-option label="ICMP PING" value="ping" />
        </el-select>
        <el-input
          v-model="target"
          placeholder="输入目标地址"
          style="width: 300px;"
          @keyup.enter="runCheck"
        />
        <el-button type="primary" @click="runCheck" :loading="checking">
          检测一下!
        </el-button>
      </div>

      <el-table :data="results" stripe>
        <el-table-column width="50">
          <template #default="{ row }">
            <el-checkbox v-model="row.checked" />
          </template>
        </el-table-column>
        <el-table-column prop="name" label="节点名称" width="150" />
        <el-table-column prop="ip" label="解析IP" width="150">
          <template #default="{ row }">
            {{ row.result?.ip || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="sendPk" label="发送" width="80">
          <template #default="{ row }">
            {{ row.result?.ping?.SendPk || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="revcPk" label="接收" width="80">
          <template #default="{ row }">
            {{ row.result?.ping?.RevcPk || '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="lossPk" label="丢包" width="80">
          <template #default="{ row }">
            {{ row.result?.ping?.LossPk !== undefined ? row.result.ping.LossPk + '%' : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="maxDelay" label="最大延迟" width="100">
          <template #default="{ row }">
            {{ row.result?.ping?.MaxDelay ? row.result.ping.MaxDelay.toFixed(2) + 'ms' : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="minDelay" label="最小延迟" width="100">
          <template #default="{ row }">
            {{ row.result?.ping?.MinDelay ? row.result.ping.MinDelay.toFixed(2) + 'ms' : '-' }}
          </template>
        </el-table-column>
        <el-table-column prop="avgDelay" label="平均延迟" width="100">
          <template #default="{ row }">
            {{ row.result?.ping?.AvgDelay ? row.result.ping.AvgDelay.toFixed(2) + 'ms' : '-' }}
          </template>
        </el-table-column>
        <el-table-column label="状态" width="60">
          <template #default="{ row }">
            <el-icon v-if="row.loading" class="is-loading"><Loading /></el-icon>
            <el-icon v-else-if="row.error" class="text-danger"><Warning /></el-icon>
            <el-icon v-else-if="row.result?.status === 'true'" class="text-success"><SuccessFilled /></el-icon>
          </template>
        </el-table-column>
      </el-table>
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
}

.tools-header {
  margin-bottom: 20px;

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
}

.search-box {
  display: flex;
  gap: 12px;
  margin-bottom: 20px;
  justify-content: center;
}
</style>
