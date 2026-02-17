<template>
  <div class="topology-view">
    <div class="topology-header">
      <h2>{{ config?.Name || 'SmartPingNext' }} - {{ $t('topology.title') }}</h2>
    </div>

    <div class="topology-content">
      <div class="topology-chart-container">
        <TopologyGraph
          :nodes="topologyNodes"
          :links="topologyLinks"
          :symbol-size="Number(config?.Topology?.Tsymbolsize || 50)"
          :line-width="Number(config?.Topology?.Tline || 2)"
          :height="chartHeight"
        />
      </div>

      <div class="topology-sidebar">
        <el-card class="alert-card" @click="$router.push('/alerts')">
          <div class="alert-card__content">
            <el-icon><Bell /></el-icon>
            <span>{{ $t('topology.viewAlerts') }}</span>
          </div>
        </el-card>

        <el-card>
          <template #header>
            <span>{{ $t('topology.topologyList') }}</span>
          </template>
          <div class="topology-list">
            <div
              v-for="node in topologyNodes.filter(n => n.color !== 'green')"
              :key="node.name"
              class="topology-item"
            >
              <el-icon v-if="loadingNodes.has(node.name)" class="is-loading"><Loading /></el-icon>
              <el-icon v-else-if="node.color === 'red'" class="text-danger"><Warning /></el-icon>
              <span>{{ node.name }}</span>
            </div>
          </div>
        </el-card>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, computed } from 'vue'
import { Bell, Loading, Warning } from '@element-plus/icons-vue'
import TopologyGraph from '@/components/charts/TopologyGraph.vue'
import { getTopology } from '@/api/topology'
import { fetchConfig } from '@/api/config'
import type { Config } from '@/types'

const config = ref<Config | null>(null)
const loadingNodes = ref(new Set<string>())
const topologyStatus = ref<Record<string, Record<string, string>>>({})

const chartHeight = computed(() => window.innerHeight - 280)

interface TopoNode {
  name: string
  color: string
}

interface TopoLink {
  source: string
  target: string
  color: string
  curveness: number
}

const topologyNodes = computed<TopoNode[]>(() => {
  if (!config.value) return []

  const nodes: TopoNode[] = []
  Object.values(config.value.Network).forEach(network => {
    const hasTopology = network.Topology && network.Topology.length > 0
    nodes.push({
      name: network.Name,
      color: hasTopology ? 'gray' : 'green'
    })
  })
  return nodes
})

const topologyLinks = computed<TopoLink[]>(() => {
  if (!config.value) return []

  const links: TopoLink[] = []
  Object.entries(config.value.Network).forEach(([addr, network]) => {
    network.Topology?.forEach(topo => {
      const targetNetwork = config.value?.Network[topo.Addr]
      if (!targetNetwork) return

      const status = topologyStatus.value[addr]?.[topo.Addr]
      links.push({
        source: network.Name,
        target: targetNetwork.Name,
        color: status === 'true' ? 'green' : status === 'false' ? 'red' : 'gray',
        curveness: status === 'true' ? 0 : 0.2
      })
    })
  })
  return links
})

const loadConfig = async () => {
  try {
    const cfg = await fetchConfig()
    config.value = cfg

    // 加载拓扑状态
    await loadTopologyStatus()
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const loadTopologyStatus = async () => {
  if (!config.value) return

const networkWithTopology = Object.entries(config.value.Network)
    .filter(([, network]) => network.Topology && network.Topology.length > 0)

  const promises = networkWithTopology.map(async ([addr, network]) => {
    loadingNodes.value.add(network.Name)
    try {
      const status = await getTopology(addr, config.value!.Port)
      topologyStatus.value[addr] = status
    } catch (e) {
      console.error(`获取 ${network.Name} 拓扑状态失败`, e)
    } finally {
      loadingNodes.value.delete(network.Name)
    }
  })

  await Promise.all(promises)
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped lang="scss">
.topology-view {
  height: 100%;
}

.topology-header {
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.topology-content {
  display: flex;
  gap: 20px;
}

.topology-chart-container {
  flex: 1;
  background-color: var(--color-bg-primary);
  border-radius: var(--radius-md);
  box-shadow: var(--shadow-sm);
  overflow: hidden;
}

.topology-sidebar {
  width: 200px;
  flex-shrink: 0;
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.alert-card {
  cursor: pointer;
  transition: transform 0.2s;

  &:hover {
    transform: translateY(-2px);
  }

  &__content {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
    padding: 10px;
    color: var(--color-warning);
  }
}

.topology-list {
  .topology-item {
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
</style>
