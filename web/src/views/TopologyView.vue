<template>
  <div class="page-shell topology-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('topology.title') }}</span>
        <h1 class="page-title">{{ displayName(config?.Name || 'SmartPingNext') }}</h1>
        <p class="page-subtitle">{{ $t('topology.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.node') }}</span>
            <strong class="page-kpi__value">{{ topologyNodes.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.links') }}</span>
            <strong class="page-kpi__value">{{ topologyLinks.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.issues') }}</span>
            <strong class="page-kpi__value">{{ degradedLinks }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.loaded') }}</span>
            <strong class="page-kpi__value">{{ loadedNodes }}</strong>
          </div>
        </div>

        <RefreshStatus
          class="surface-panel surface-panel--tight"
          :loading="configLoading || isRefreshing"
          :last-updated="lastUpdatedLabel"
          @refresh="refreshTopology"
        />
      </div>
    </div>

    <div class="page-frame">
      <div class="page-main">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('topology.title') }}</h2>
              <p class="surface-panel__description">
                {{ topologyLinks.length }} {{ $t('common.links') }} · {{ degradedLinks }}
                {{ $t('common.issues') }}
              </p>
            </div>
          </div>

          <div v-if="configLoading && !config" class="empty-state" aria-live="polite">
            <el-icon class="is-loading"><Loading /></el-icon>
            <span>{{ $t('common.loading') }}</span>
          </div>

          <div v-else-if="configError && !config" class="empty-state">
            <el-icon class="topology-view__empty-icon topology-view__empty-icon--danger">
              <Warning />
            </el-icon>
            <span>{{ $t('common.configLoadFailedNetwork') }}</span>
            <el-button size="small" @click="loadConfig">{{ $t('common.retry') }}</el-button>
          </div>

          <div v-else-if="!topologyNodes.length" class="empty-state">
            <el-icon class="topology-view__empty-icon"><InfoFilled /></el-icon>
            <span>{{ $t('topology.noTopology') }}</span>
          </div>

          <div
            v-else
            v-loading="!topologyGraphRef"
            class="topology-view__graph-shell"
            :style="{ height: `${graphHeight}px` }"
          >
            <TopologyGraph
              ref="topologyGraphRef"
              :nodes="topologyNodes"
              :links="topologyLinks"
              :symbol-size="Number(config?.Topology?.Tsymbolsize || 50)"
              :line-width="Number(config?.Topology?.Tline || 2)"
              :height="graphHeight"
            />
          </div>
        </section>
      </div>

      <aside class="page-aside page-aside--narrow">
        <button
          type="button"
          class="surface-panel topology-view__alert-link"
          @click="router.push('/alerts')"
        >
          <div class="topology-view__alert-copy">
            <span class="page-eyebrow">{{ $t('topology.viewAlerts') }}</span>
            <strong>{{ degradedLinks }} {{ $t('common.issues') }}</strong>
          </div>
          <el-icon><Bell /></el-icon>
        </button>

        <section class="surface-panel surface-panel--soft">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('topology.topologyList') }}</h2>
              <p class="surface-panel__description">
                {{ monitoredNodes.length }} {{ $t('common.node') }}
              </p>
            </div>
          </div>

          <div class="list-stack">
            <div v-for="node in monitoredNodes" :key="node.id" class="list-row">
              <div class="list-row__meta">
                <el-icon v-if="loadingNodes.has(node.id)" class="is-loading"><Loading /></el-icon>
                <el-icon
                  v-else-if="node.color === 'red' || failedNodes.has(node.id)"
                  class="text-danger"
                  ><Warning
                /></el-icon>
                <div
                  v-else
                  class="topology-view__dot"
                  :class="`topology-view__dot--${node.color}`"
                ></div>
                <div class="list-row__text">
                  <span class="list-row__title">{{ node.name }}</span>
                  <span class="list-row__caption">{{ getNodeStatus(node) }}</span>
                </div>
              </div>
            </div>
          </div>
        </section>
      </aside>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, defineAsyncComponent, onMounted, onUnmounted, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { Bell, InfoFilled, Loading, Warning } from '@element-plus/icons-vue'
import RefreshStatus from '@/components/common/RefreshStatus.vue'
import { isRequestCanceled } from '@/api'
import { fetchConfig } from '@/api/config'
import { getTopology } from '@/api/topology'
import { mapWithConcurrency } from '@/utils/concurrency'
import { displayName, formatTime } from '@/utils/format'
import type { Config } from '@/types'

const topologyGraphModule = import('@/components/charts/TopologyGraph.vue')
const TopologyGraph = defineAsyncComponent(() => topologyGraphModule)

const router = useRouter()
const { t } = useI18n()
const config = ref<Config | null>(null)
const loadingNodes = ref(new Set<string>())
const failedNodes = ref(new Set<string>())
const topologyStatus = ref<Record<string, Record<string, string>>>({})
const configLoading = ref(true)
const configError = ref(false)
const isRefreshing = ref(false)
const lastUpdatedAt = ref<Date | null>(null)
const graphHeight = ref(Math.max(window.innerHeight - 300, 420))
const topologyGraphRef = ref<unknown>(null)
let isUnmounted = false
let configRequestId = 0
let topologyRequestId = 0
let configAbortController: AbortController | null = null
let topologyAbortController: AbortController | null = null
const TOPOLOGY_CONCURRENCY = 4

interface TopoNode {
  id: string
  name: string
  color: string
  monitored: boolean
}

interface TopoLink {
  source: string
  target: string
  color: string
  curveness: number
}

const topologyNodes = computed<TopoNode[]>(() => {
  if (!config.value) {
    return []
  }

  const nodes: TopoNode[] = []
  Object.entries(config.value.Network).forEach(([addr, network]) => {
    const hasTopology = network.Topology && network.Topology.length > 0
    const statuses = hasTopology
      ? network.Topology.map((topology) => topologyStatus.value[addr]?.[topology.Addr])
      : []
    const color = statuses.some((status) => status === 'false')
      ? 'red'
      : statuses.length > 0 && statuses.every((status) => status === 'true')
        ? 'green'
        : hasTopology
          ? 'gray'
          : 'green'
    nodes.push({
      id: addr,
      name: displayName(network.Name),
      color,
      monitored: hasTopology
    })
  })
  return nodes
})

const topologyLinks = computed<TopoLink[]>(() => {
  if (!config.value) {
    return []
  }

  const links: TopoLink[] = []
  Object.entries(config.value.Network).forEach(([addr, network]) => {
    network.Topology?.forEach((topo) => {
      const targetNetwork = config.value?.Network[topo.Addr]
      if (!targetNetwork) {
        return
      }

      const status = topologyStatus.value[addr]?.[topo.Addr]
      links.push({
        source: addr,
        target: topo.Addr,
        color: status === 'true' ? 'green' : status === 'false' ? 'red' : 'gray',
        curveness: status === 'true' ? 0 : 0.2
      })
    })
  })
  return links
})

const monitoredNodes = computed(() => topologyNodes.value.filter((node) => node.monitored))
const loadedNodes = computed(
  () =>
    monitoredNodes.value.filter(
      (node) =>
        topologyStatus.value[node.id] &&
        !loadingNodes.value.has(node.id) &&
        !failedNodes.value.has(node.id)
    ).length
)
const degradedLinks = computed(
  () => topologyLinks.value.filter((link) => link.color === 'red').length
)
const lastUpdatedLabel = computed(() =>
  lastUpdatedAt.value ? formatTime(lastUpdatedAt.value) : ''
)

const loadConfig = async () => {
  configAbortController?.abort()
  topologyAbortController?.abort()
  topologyAbortController = null
  topologyRequestId++
  isRefreshing.value = false
  loadingNodes.value = new Set()
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
    topologyStatus.value = {}
    lastUpdatedAt.value = null
    await loadTopologyStatus()
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

const loadTopologyStatus = async () => {
  if (!config.value) {
    return
  }

  topologyAbortController?.abort()
  const controller = new AbortController()
  topologyAbortController = controller
  const requestId = ++topologyRequestId
  const cfg = config.value
  const networkWithTopology = Object.entries(cfg.Network).filter(
    ([, network]) => network.Topology && network.Topology.length > 0
  )
  const nextStatus = Object.fromEntries(
    networkWithTopology.flatMap(([addr]) =>
      topologyStatus.value[addr] ? [[addr, topologyStatus.value[addr]]] : []
    )
  ) as Record<string, Record<string, string>>

  isRefreshing.value = true
  loadingNodes.value = new Set(networkWithTopology.map(([addr]) => addr))
  failedNodes.value = new Set()

  try {
    await mapWithConcurrency(
      networkWithTopology,
      TOPOLOGY_CONCURRENCY,
      async ([addr, network]) => {
        try {
          const status = await getTopology(addr, cfg.Port, cfg.Addr, controller.signal)
          if (isUnmounted || requestId !== topologyRequestId) {
            return
          }
          nextStatus[addr] = status
          topologyStatus.value = { ...nextStatus }
          lastUpdatedAt.value = new Date()
        } catch (error) {
          if (isRequestCanceled(error) || isUnmounted || requestId !== topologyRequestId) {
            return
          }
          delete nextStatus[addr]
          topologyStatus.value = { ...nextStatus }
          failedNodes.value = new Set(failedNodes.value).add(addr)
          console.error(`获取 ${network.Name} 拓扑状态失败`, error)
        } finally {
          if (!isUnmounted && requestId === topologyRequestId) {
            const nextLoadingNodes = new Set(loadingNodes.value)
            nextLoadingNodes.delete(addr)
            loadingNodes.value = nextLoadingNodes
          }
        }
      },
      controller.signal
    )
  } catch (error) {
    if (!isRequestCanceled(error)) throw error
  } finally {
    if (topologyAbortController === controller) {
      topologyAbortController = null
    }
    if (!isUnmounted && requestId === topologyRequestId) {
      isRefreshing.value = false
    }
  }
}

const refreshTopology = () => (config.value ? loadTopologyStatus() : loadConfig())

const handleResize = () => {
  graphHeight.value = Math.max(window.innerHeight - 300, 420)
}

const getNodeStatus = (node: TopoNode) => {
  if (loadingNodes.value.has(node.id)) {
    return t('common.loading')
  }
  if (failedNodes.value.has(node.id)) {
    return t('common.loadFailed')
  }
  if (node.color === 'red') {
    return t('common.alert')
  }
  if (node.color === 'gray') {
    return t('common.unknown')
  }
  return t('common.active')
}

onMounted(() => {
  loadConfig()
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  isUnmounted = true
  configAbortController?.abort()
  topologyAbortController?.abort()
  configRequestId++
  topologyRequestId++
  window.removeEventListener('resize', handleResize)
})
</script>

<style scoped lang="scss">
.topology-view__alert-link {
  width: 100%;
  text-align: left;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 16px;
  font: inherit;
  color: var(--color-text-primary);
  transition:
    transform 0.24s ease,
    box-shadow 0.24s ease,
    border-color 0.24s ease;

  &:hover {
    transform: translateY(-2px);
    box-shadow: var(--shadow-md);
    border-color: color-mix(in srgb, var(--color-primary) 22%, transparent);
  }

  .el-icon {
    font-size: 20px;
    color: var(--color-warning);
  }
}

.topology-view__graph-shell {
  width: 100%;
  min-height: 420px;
}

.topology-view__alert-copy {
  display: flex;
  flex-direction: column;
  gap: 8px;

  strong {
    font-size: 20px;
    font-weight: 700;
    letter-spacing: -0.04em;
  }
}

.topology-view__dot {
  width: 10px;
  height: 10px;
  border-radius: 999px;
}

.topology-view__dot--gray {
  background: var(--color-text-secondary);
}

.topology-view__dot--green {
  background: var(--color-success);
}

.topology-view__empty-icon {
  color: var(--color-text-secondary);
  font-size: 26px;
}

.topology-view__empty-icon--danger {
  color: var(--color-danger);
}
</style>
