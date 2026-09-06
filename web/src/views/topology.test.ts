import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
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
  loadedNodes: vue.ComputedRef<number>
  lastUpdatedAt: vue.Ref<Date | null>
  isRefreshing: vue.Ref<boolean>
  loadTopologyStatus: () => Promise<void>
  loadConfig: () => Promise<void>
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
  const getTopology = t.mock.fn(async (): Promise<Record<string, string>> => ({}))
  const config = {
    Addr: '127.0.0.1',
    Port: 8899,
    Network: {
      '127.0.0.1': member('127.0.0.1', ['192.0.2.1', '192.0.2.2']),
      '192.0.2.1': member('192.0.2.1', []),
      '192.0.2.2': member('192.0.2.2', [])
    }
  }
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
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
    '@/api/config': { fetchConfig: async () => config },
    '@/api/topology': { getTopology },
    '@/utils/concurrency': { mapWithConcurrency },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => TopologySetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    window: { innerHeight: 900 },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.config.value = config
  const status = () => view.getNodeStatus(view.topologyNodes.value[0]!)
  return { view, getTopology, status }
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
