import { preloadAsync } from '../utils/preloadAsync.js'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import type { Config, NetworkMember } from '../types/index.js'

const source = readFileSync(new URL('../../src/views/TopologyView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'topology-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface TopologyNode {
  id: string
  color: string
}

interface TopologySetup {
  config: vue.Ref<Pick<Config, 'Addr' | 'Port' | 'Network'> | null>
  topologyNodes: vue.ComputedRef<TopologyNode[]>
  topologyLinks: vue.ComputedRef<Array<{ source: string; target: string; color: string }>>
  degradedLinks: vue.ComputedRef<number>
  loadedNodes: vue.ComputedRef<number>
  lastUpdatedAt: vue.Ref<Date | null>
  isRefreshing: vue.Ref<boolean>
  configLoading: vue.Ref<boolean>
  configError: vue.Ref<boolean>
  topologyStatus: vue.Ref<Record<string, Record<string, string>>>
  loadingNodes: vue.Ref<Set<string>>
  failedNodes: vue.Ref<Set<string>>
  loadTopologyStatus: () => Promise<void>
  loadConfig: () => Promise<void>
  refreshTopology: () => Promise<void> | undefined
  getNodeStatus: (node: TopologyNode) => string
}

const member = (addr: string, targets: string[]): NetworkMember => ({
  Addr: addr,
  Name: addr,
  Smartping: true,
  Ping: targets,
  Topology: targets.map((target) => ({
    Addr: target,
    Name: target,
    Thdchecksec: '60',
    Thdoccnum: '1',
    Thdavgdelay: '100',
    Thdloss: '10'
  }))
})

function createView(t: test.TestContext) {
  const getTopology = t.mock.fn(async (_addr?: string, _port?: number, _localAddr?: string,
    _signal?: AbortSignal): Promise<Record<string, string>> => ({}))
  const config: Pick<Config, 'Addr' | 'Port' | 'Network'> = {
    Addr: '127.0.0.1',
    Port: 8899,
    Network: {
      '127.0.0.1': member('127.0.0.1', ['192.0.2.1', '192.0.2.2']),
      '192.0.2.1': member('192.0.2.1', []),
      '192.0.2.2': member('192.0.2.2', [])
    }
  }
  const fetchConfig = t.mock.fn(async (_signal?: AbortSignal) => config)
  let unmount!: () => void
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: (callback: () => void) => { unmount = callback } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': { useRouter: () => ({}) },
    'element-plus': { ElMessage: { error: () => {} } },
    '@element-plus/icons-vue': {},
    '@/components/common/RefreshStatus.vue': {},
    '@/components/charts/TopologyGraph.vue': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig },
    '@/api/topology': { getTopology },
    '@/utils/concurrency': { mapWithConcurrency },
    '@/utils/preloadAsync': { preloadAsync },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => TopologySetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    window: { innerHeight: 900, removeEventListener: () => {} },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => { unmount(); scope.stop() })
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.config.value = config
  const status = () => view.getNodeStatus(view.topologyNodes.value[0]!)
  return { view, getTopology, fetchConfig, status, unmount: () => unmount() }
}

test('topology distinguishes missing, partial, healthy and alert responses', async (t) => {
  const { view, getTopology, status } = createView(t)
  assert.equal(view.loadedNodes.value, 0)
  assert.equal(status(), 'common.unknown')
  await view.loadTopologyStatus()
  assert.equal(status(), 'common.unknown')
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'unknown', '192.0.2.2': 'true' }))
  await view.loadTopologyStatus()
  assert.equal(status(), 'common.unknown')
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'true' }))
  await view.loadTopologyStatus()
  assert.equal(status(), 'common.unknown')
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'true', '192.0.2.2': 'true' }))
  await view.loadTopologyStatus()
  assert.equal(status(), 'common.active')
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'false' }))
  await view.loadTopologyStatus()
  assert.equal(status(), 'common.alert')
})

test('failed topology refresh preserves the last successful timestamp', async (t) => {
  const { view, getTopology, status } = createView(t)
  getTopology.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.loadTopologyStatus()
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.loadedNodes.value, 0)
  assert.equal(status(), 'common.loadFailed')
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'true', '192.0.2.2': 'true' }))
  await view.loadTopologyStatus()
  const timestamp = view.lastUpdatedAt.value
  assert.ok(timestamp)
  assert.equal(view.loadedNodes.value, 1)
  getTopology.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.loadTopologyStatus()
  assert.equal(view.lastUpdatedAt.value, timestamp)
  assert.equal(view.loadedNodes.value, 0)
  assert.equal(status(), 'common.loadFailed')
})

test('malformed HTTP success responses fail the topology source without advancing its timestamp', async (t) => {
  const { view, getTopology, status } = createView(t)
  const apiSource = readFileSync(new URL('../../src/api/topology.ts', import.meta.url), 'utf8')
  const apiCode = ts.transpileModule(apiSource, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const api = {} as { getTopology: (addr: string, port: number, localAddr: string) => Promise<Record<string, string>> }
  let response: unknown
  runInNewContext(apiCode, {
    exports: api,
    URLSearchParams,
    require: (name: string) => {
      if (name === './index') return {
        default: { get: async () => response }, proxyRequestConfig: () => ({})
      }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  for (const localAddr of ['127.0.0.1', '192.0.2.10']) {
    // Exercise both the direct and proxy API paths through the real response validator.
    getTopology.mock.mockImplementation(() => api.getTopology('127.0.0.1', 8899, localAddr))
    for (const malformed of ['<html>Sign in</html>', [], null, { '192.0.2.1': true }]) {
      response = { '192.0.2.1': 'true', '192.0.2.2': 'true' }
      await view.loadTopologyStatus()
      assert.equal(status(), 'common.active')
      const updated = view.lastUpdatedAt.value
      response = malformed
      await view.loadTopologyStatus()
      assert.equal(status(), 'common.loadFailed')
      assert.equal(view.loadedNodes.value, 0)
      assert.equal(view.lastUpdatedAt.value, updated)
      assert.equal(view.isRefreshing.value, false)
    }
    response = { '192.0.2.1': 'unknown', '192.0.2.2': 'true' }
    await view.loadTopologyStatus()
    assert.equal(status(), 'common.unknown')
    assert.equal(view.loadedNodes.value, 1)
  }
})

test('topology counts completed responses while reporting pending nodes as loading', async (t) => {
  const { view, getTopology, status } = createView(t)
  let resolve!: (value: Record<string, string>) => void
  getTopology.mock.mockImplementation(
    () =>
      new Promise((done) => {
        resolve = done
      })
  )
  const request = view.loadTopologyStatus()
  assert.equal(view.loadedNodes.value, 0)
  assert.equal(status(), 'common.loading')
  assert.equal(view.isRefreshing.value, true)
  resolve({ '192.0.2.1': 'true', '192.0.2.2': 'true' })
  await request
  assert.equal(view.loadedNodes.value, 1)
  assert.equal(view.isRefreshing.value, false)
  assert.equal(status(), 'common.active')
})

test('late topology responses cannot replace newer status or timestamp', async (t) => {
  const { view, getTopology, status } = createView(t)
  let resolveOld!: (value: Record<string, string>) => void
  getTopology.mock.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveOld = resolve
      })
  )
  const old = view.loadTopologyStatus()
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'false' }))
  await view.loadTopologyStatus()
  const timestamp = view.lastUpdatedAt.value
  resolveOld({ '192.0.2.1': 'true', '192.0.2.2': 'true' })
  await old
  assert.equal(status(), 'common.alert')
  assert.equal(view.lastUpdatedAt.value, timestamp)
  assert.equal(view.loadedNodes.value, 1)
})

test('reloading topology configuration clears the old update timestamp', async (t) => {
  const { view, getTopology } = createView(t)
  await view.loadTopologyStatus()
  assert.ok(view.lastUpdatedAt.value)
  getTopology.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.loadConfig()
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.loadedNodes.value, 0)
})

test('topology unmount blocks late configuration, status and refresh callbacks', async (t) => {
  const { view, getTopology, fetchConfig, unmount } = createView(t)
  await view.loadConfig()
  const configCalls = fetchConfig.mock.callCount()
  const topologyCalls = getTopology.mock.callCount()
  const updatedAt = view.lastUpdatedAt.value
  const statuses = view.topologyStatus.value
  const loading = view.loadingNodes.value
  unmount()
  await view.loadConfig()
  await view.loadTopologyStatus()
  await view.refreshTopology()
  assert.equal(fetchConfig.mock.callCount(), configCalls)
  assert.equal(getTopology.mock.callCount(), topologyCalls)
  assert.equal(view.lastUpdatedAt.value, updatedAt)
  assert.equal(view.topologyStatus.value, statuses)
  assert.equal(view.loadingNodes.value, loading)
  assert.equal(view.configLoading.value, false)
  assert.equal(view.isRefreshing.value, false)
  view.config.value = null
  await view.refreshTopology()
  assert.equal(fetchConfig.mock.callCount(), configCalls)
})

test('topology refresh pauses during config reload then uses the new nodes and port', async (t) => {
  const { view, getTopology, fetchConfig } = createView(t)
  await view.loadConfig()
  const previousStatuses = view.topologyStatus.value
  const previousUpdated = view.lastUpdatedAt.value
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  t.after(() => release())
  fetchConfig.mock.mockImplementation(async () => {
    await gate
    return { Addr: '127.0.0.2', Port: 9000, Network: {
      '192.0.2.3': member('192.0.2.3', ['192.0.2.4']),
      '192.0.2.4': member('192.0.2.4', [])
    } }
  })
  getTopology.mock.resetCalls()
  const pending = view.loadConfig()
  await view.refreshTopology()
  try {
    assert.equal(getTopology.mock.callCount(), 0)
    assert.equal(view.topologyStatus.value, previousStatuses)
    assert.equal(view.lastUpdatedAt.value, previousUpdated)
    assert.equal(view.isRefreshing.value, false)
  } finally {
    release()
    await pending
  }
  assert.equal(getTopology.mock.callCount(), 1)
  assert.deepEqual(Array.from(getTopology.mock.calls[0]!.arguments).slice(0, 3), ['192.0.2.3', 9000, '127.0.0.2'])
  await view.refreshTopology()
  assert.equal(getTopology.mock.callCount(), 2)
  assert.equal(view.configLoading.value, false)
  assert.equal(view.isRefreshing.value, false)
})

test('topology refresh retries configuration after an initial loading failure', async (t) => {
  const { view, fetchConfig, getTopology } = createView(t)
  const currentConfig = () => view.config.value
  view.config.value = null
  fetchConfig.mock.mockImplementationOnce(async () => { throw new Error('offline') })
  await view.loadConfig()
  assert.equal(view.configError.value, true)
  assert.equal(view.configLoading.value, false)
  assert.equal(view.config.value, null)
  assert.equal(getTopology.mock.callCount(), 0)
  await view.refreshTopology()
  assert.equal(fetchConfig.mock.callCount(), 2)
  assert.equal(currentConfig()?.Addr, '127.0.0.1')
  assert.equal(view.configError.value, false)
  assert.equal(view.configLoading.value, false)
  assert.equal(getTopology.mock.callCount(), 1)
  assert.equal(view.loadedNodes.value, 1)
})

test('topology unmount cancels active nodes and discards queued work and late responses', async (t) => {
  for (const fail of [false, true]) {
    const { view, getTopology, unmount } = createView(t)
    view.config.value!.Network = Object.fromEntries(Array.from({ length: 6 }, (_, i) => {
      const addr = `192.0.2.${i + 1}`
      return [addr, member(addr, ['127.0.0.1'])]
    }))
    await view.loadTopologyStatus()
    const updatedAt = view.lastUpdatedAt.value
    const previousStatuses = view.topologyStatus.value
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    t.after(() => release())
    getTopology.mock.resetCalls()
    getTopology.mock.mockImplementation(async () => {
      await gate
      if (fail) throw new Error('late failure')
      return { '127.0.0.1': 'false' }
    })
    const pending = view.loadTopologyStatus()
    assert.equal(getTopology.mock.callCount(), 4)
    assert.equal(view.loadingNodes.value.size, 6)
    const loading = view.loadingNodes.value
    unmount()
    assert.ok(getTopology.mock.calls.every((call) => call.arguments[3]!.aborted))
    release()
    await pending
    assert.equal(getTopology.mock.callCount(), 4)
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    assert.equal(view.topologyStatus.value, previousStatuses)
    assert.equal(view.loadingNodes.value, loading)
    assert.equal(view.failedNodes.value.size, 0)
  }
})

test('topology unmount cancels config loading without accepting its late success or failure', async (t) => {
  for (const fail of [false, true]) {
    const { view, getTopology, fetchConfig, unmount } = createView(t)
    const previousConfig = view.config.value
    const cfg = await fetchConfig()
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    t.after(() => release())
    fetchConfig.mock.mockImplementation(async () => {
      await gate
      if (fail) throw new Error('late config failure')
      return { ...cfg, Addr: '127.0.0.2', Port: 9000 }
    })
    const pending = view.loadConfig()
    const signal = fetchConfig.mock.calls.at(-1)!.arguments[0]!
    unmount()
    assert.equal(signal.aborted, true)
    release()
    await pending
    assert.equal(view.config.value, previousConfig)
    assert.equal(getTopology.mock.callCount(), 0)
    assert.equal(view.configError.value, false)
    assert.equal(view.lastUpdatedAt.value, null)
  }
})

test('topology publishes reactive node and link progress as individual sources finish', async (t) => {
  const { view, getTopology } = createView(t)
  view.config.value!.Network = {
    '127.0.0.1': member('127.0.0.1', ['192.0.2.1']),
    '192.0.2.1': member('192.0.2.1', ['192.0.2.2']),
    '192.0.2.2': member('192.0.2.2', ['127.0.0.1'])
  }
  const pending = new Map<string, {
    resolve: (status: Record<string, string>) => void
    reject: (error: Error) => void
  }>()
  getTopology.mock.mockImplementation((addr) => new Promise((resolve, reject) => {
    pending.set(addr!, { resolve, reject })
  }))
  t.after(() => pending.forEach(({ resolve }) => resolve({})))
  const frames: Array<{ colors: string; links: string; loaded: number; failed: number; pending: number }> = []
  const stop = vue.watchEffect(() => {
    frames.push({
      colors: view.topologyNodes.value.map((node) => node.color).join(','),
      links: view.topologyLinks.value.map((link) => link.color).join(','),
      loaded: view.loadedNodes.value,
      failed: view.failedNodes.value.size,
      pending: view.loadingNodes.value.size
    })
  })
  t.after(stop)
  const request = view.loadTopologyStatus()
  await setImmediate()
  assert.deepEqual(frames.at(-1), { colors: 'gray,gray,gray', links: 'gray,gray,gray', loaded: 0, failed: 0, pending: 3 })
  pending.get('127.0.0.1')!.resolve({ '192.0.2.1': 'true' })
  await setImmediate()
  assert.deepEqual(frames.at(-1), { colors: 'green,gray,gray', links: 'green,gray,gray', loaded: 1, failed: 0, pending: 2 })
  pending.get('192.0.2.1')!.resolve({ '192.0.2.2': 'false' })
  await setImmediate()
  assert.deepEqual(frames.at(-1), { colors: 'green,red,gray', links: 'green,red,gray', loaded: 2, failed: 0, pending: 1 })
  pending.get('192.0.2.2')!.reject(new Error('offline'))
  await request
  await setImmediate()
  assert.deepEqual(frames.at(-1), { colors: 'green,red,gray', links: 'green,red,gray', loaded: 2, failed: 1, pending: 0 })
  assert.equal(view.degradedLinks.value, 1)
  assert.equal(view.getNodeStatus(view.topologyNodes.value[2]!), 'common.loadFailed')
})

test('topology refresh releases obsolete source states including an empty monitored list', async (t) => {
  const { view, getTopology } = createView(t)
  getTopology.mock.mockImplementation(async () => ({ '192.0.2.1': 'true', '192.0.2.2': 'true' }))
  await view.loadTopologyStatus()
  assert.equal(Object.keys(view.topologyStatus.value).length, 1)
  view.config.value!.Network['127.0.0.1']!.Topology = []
  getTopology.mock.resetCalls()
  await view.loadTopologyStatus()
  assert.equal(getTopology.mock.callCount(), 0)
  assert.equal(Object.keys(view.topologyStatus.value).length, 0)
  assert.equal(view.loadedNodes.value, 0)
  assert.equal(view.loadingNodes.value.size, 0)
  assert.equal(view.failedNodes.value.size, 0)
  assert.equal(view.topologyLinks.value.length, 0)
  assert.equal(view.isRefreshing.value, false)
})

test('large topology refresh isolates source failures and recovers all 1000 monitored nodes', async (t) => {
  const { view, getTopology } = createView(t)
  const sourceIndex = new Map<string, number>()
  view.config.value!.Network = Object.fromEntries(Array.from({ length: 1000 }, (_, i) => {
    const addr = `10.0.${Math.floor(i / 256)}.${i % 256}`
    sourceIndex.set(addr, i)
    return [addr, member(addr, ['127.0.0.1'])]
  }))
  view.config.value!.Network['127.0.0.1'] = member('127.0.0.1', [])
  getTopology.mock.mockImplementation(async (addr) => {
    const i = sourceIndex.get(addr!)!
    if (i % 7 === 0) throw new Error('offline')
    return { '127.0.0.1': i % 10 === 0 ? 'false' : 'true' }
  })
  const started = performance.now()
  await view.loadTopologyStatus()
  assert.equal(getTopology.mock.callCount(), 1000)
  assert.equal(view.loadedNodes.value, 857)
  assert.equal(view.failedNodes.value.size, 143)
  assert.equal(view.loadingNodes.value.size, 0)
  assert.equal(Object.keys(view.topologyStatus.value).length, 857)
  assert.equal(view.topologyLinks.value.length, 1000)
  assert.equal(view.degradedLinks.value, 85)
  getTopology.mock.mockImplementation(async () => ({ '127.0.0.1': 'true' }))
  await view.loadTopologyStatus()
  assert.equal(getTopology.mock.callCount(), 2000)
  assert.equal(view.loadedNodes.value, 1000)
  assert.equal(view.failedNodes.value.size, 0)
  assert.equal(view.loadingNodes.value.size, 0)
  assert.equal(Object.keys(view.topologyStatus.value).length, 1000)
  assert.equal(view.degradedLinks.value, 0)
  assert.ok(view.topologyNodes.value.every((node) => node.color === 'green'))
  assert.ok(view.topologyLinks.value.every((link) => link.color === 'green'))
  assert.equal(view.isRefreshing.value, false)
  t.diagnostic(`1000 sources, two simulated refreshes and final computed state: ${(performance.now() - started).toFixed(1)} ms`)
})
