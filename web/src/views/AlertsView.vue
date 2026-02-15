<template>
  <div class="alerts-view">
    <div class="alerts-header">
      <h2>报警记录</h2>
    </div>

    <div class="alerts-content">
      <div class="dates-sidebar">
        <el-card>
          <template #header>
            <span>报警存档</span>
          </template>
          <div class="date-list">
            <div
              v-for="date in dates"
              :key="date"
              class="date-item"
              :class="{ active: selectedDate === date }"
              @click="loadAlertsByDate(date)"
            >
              {{ date }}
            </div>
          </div>
        </el-card>
      </div>

      <div class="alerts-main">
        <el-card>
          <template #header>
            <span>报警历史</span>
          </template>
          <el-table :data="alerts" stripe style="width: 100%">
            <el-table-column prop="Logtime" label="报警日期" min-width="160" />
            <el-table-column prop="Fromname" label="源节点" min-width="100" />
            <el-table-column prop="Fromip" label="源IP" min-width="120" />
            <el-table-column prop="Targetname" label="目标节点" min-width="100" />
            <el-table-column prop="Targetip" label="目标IP" min-width="120" />
            <el-table-column label="工具" width="80">
              <template #default="{ row }">
                <el-button size="small" @click="showMtr(row)">MTR</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </div>

      <div class="nodes-sidebar">
        <el-button @click="$router.push('/topology')" style="width: 100%; margin-bottom: 20px;">
          <el-icon><ArrowLeft /></el-icon>
          返回拓扑图
        </el-button>

        <el-card>
          <template #header>
            <span>节点列表</span>
          </template>
          <div class="node-list">
            <div v-for="node in nodes" :key="node.name" class="node-item">
              <el-icon v-if="node.loading" class="is-loading"><Loading /></el-icon>
              <span>{{ node.name }}</span>
            </div>
          </div>
        </el-card>
      </div>
    </div>

    <!-- MTR 弹窗 -->
    <el-dialog v-model="mtrVisible" title="MTR 结果" width="700px">
      <el-table :data="mtrData" stripe style="width: 100%">
        <el-table-column prop="Host" label="主机" min-width="150" />
        <el-table-column label="丢包率" width="80">
          <template #default="{ row }">
            {{ ((row.Loss / row.Send) * 100).toFixed(2) }}%
          </template>
        </el-table-column>
        <el-table-column prop="Send" label="发送" width="60" />
        <el-table-column label="最近" width="70">
          <template #default="{ row }">
            {{ (row.Last / 1000000).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column label="平均" width="70">
          <template #default="{ row }">
            {{ (row.Avg / 1000000).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column label="最好" width="70">
          <template #default="{ row }">
            {{ (row.Best / 1000000).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column label="最差" width="70">
          <template #default="{ row }">
            {{ (row.Wrst / 1000000).toFixed(2) }}
          </template>
        </el-table-column>
        <el-table-column prop="StDev" label="标准差" width="70" />
      </el-table>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted } from 'vue'
import { ArrowLeft, Loading } from '@element-plus/icons-vue'
import { getConfig } from '@/api/topology'
import { getAlerts } from '@/api/alert'
import type { Config, AlertLog } from '@/types'

const config = ref<Config | null>(null)
const dates = ref<string[]>([])
const selectedDate = ref('')
const alerts = ref<AlertLog[]>([])
const nodes = ref<Array<{ name: string; addr: string; loading: boolean }>>([])

const mtrVisible = ref(false)
const mtrData = ref<any[]>([])

const loadConfig = async () => {
  try {
    const cfg = await getConfig()
    config.value = cfg

    // 获取有拓扑配置的节点
    nodes.value = Object.values(cfg.Network)
      .filter(n => n.Topology && n.Topology.length > 0)
      .map(n => ({ name: n.Name, addr: n.Addr, loading: false }))

    await loadAllAlerts()
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const loadAllAlerts = async () => {
  if (!config.value) return

  const allDates = new Set<string>()
  const allAlerts: AlertLog[] = []

  await Promise.all(nodes.value.map(async (node) => {
    node.loading = true
    try {
      const data = await getAlerts(`http://${node.addr}:${config.value!.Port}`)
      data.dates.forEach(d => allDates.add(d))
      allAlerts.push(...data.logs)
    } catch (e) {
      console.error(`获取 ${node.name} 报警记录失败`, e)
    } finally {
      node.loading = false
    }
  }))

  dates.value = Array.from(allDates).sort().reverse()
  alerts.value = allAlerts.sort((a, b) => b.Logtime.localeCompare(a.Logtime))
}

const loadAlertsByDate = async (date: string) => {
  selectedDate.value = date

  if (!config.value) return

  const allAlerts: AlertLog[] = []

  await Promise.all(nodes.value.map(async (node) => {
    try {
      const data = await getAlerts(`http://${node.addr}:${config.value!.Port}`, date)
      allAlerts.push(...data.logs)
    } catch (e) {
      console.error(`获取 ${node.name} 报警记录失败`, e)
    }
  }))

  alerts.value = allAlerts.sort((a, b) => b.Logtime.localeCompare(a.Logtime))
}

const showMtr = (row: AlertLog) => {
  try {
    mtrData.value = JSON.parse(row.Tracert)
    mtrVisible.value = true
  } catch (e) {
    console.error('解析MTR数据失败', e)
  }
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped lang="scss">
.alerts-view {
  height: 100%;
  overflow: hidden;
  display: flex;
  flex-direction: column;
}

.alerts-header {
  margin-bottom: 20px;
  flex-shrink: 0;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.alerts-content {
  display: flex;
  gap: 20px;
  overflow: hidden;
  flex: 1;
  min-width: 0;
}

.dates-sidebar {
  width: 180px;
  flex-shrink: 0;
}

.alerts-main {
  flex: 1;
  min-width: 0;
  overflow: hidden;

  :deep(.el-card) {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }

  :deep(.el-card__body) {
    flex: 1;
    overflow: auto;
    min-width: 0;
  }

  :deep(.el-table) {
    min-width: 0;
  }
}

.nodes-sidebar {
  width: 160px;
  flex-shrink: 0;
}

.date-list {
  .date-item {
    padding: 10px 12px;
    cursor: pointer;
    border-radius: var(--radius-sm);
    transition: background-color 0.2s;
    color: var(--color-text-primary);

    &:hover {
      background-color: var(--color-bg-secondary);
    }

    &.active {
      background-color: var(--color-primary);
      color: #fff;
    }
  }
}

.node-list {
  .node-item {
    padding: 10px 12px;
    display: flex;
    align-items: center;
    gap: 8px;
    border-radius: var(--radius-sm);
    color: var(--color-text-primary);

    &:hover {
      background-color: var(--color-bg-secondary);
    }
  }
}

@media (max-width: 1200px) {
  .alerts-content {
    flex-direction: column;
  }

  .dates-sidebar,
  .nodes-sidebar {
    width: 100%;
  }
}
</style>
