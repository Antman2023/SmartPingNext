import { preserveLegendSelection } from '../utils/chartInteraction.js'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { SVGRenderer } from 'echarts/renderers'
import { echarts } from '../utils/echartsMap.js'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import type { ChinaMapData } from '../types/index.js'
import { formatMappingTooltip } from '../utils/chartTooltip.js'

// Execute the actual view setup with controlled API responses and no browser renderer.
const source = readFileSync(new URL('../../src/views/MappingView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'mapping-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface MappingSetup {
  agents: vue.Ref<Array<{ name: string; addr: string; loading: boolean }>>
  switchAgent: (agent: { name: string; addr: string; loading: boolean }) => Promise<void>
  selectedDate: vue.Ref<string>
  currentBaseUrl: vue.Ref<string>
  currentAgent: vue.Ref<string>
  latestData: vue.Ref<ChinaMapData | null>
  lastUpdatedAt: vue.Ref<Date | null>
  mappingError: vue.Ref<boolean>
  mappingLoading: vue.Ref<boolean>
  configLoading: vue.Ref<boolean>
  configError: vue.Ref<boolean>
  hasRetainedMapping: vue.ComputedRef<boolean>
  chart: { clear: () => void; dispose: () => void }
  isMapReady: vue.Ref<boolean>
  updateChart: (data: ChinaMapData) => void
  loadMappingData: () => Promise<void>
  loadConfig: () => Promise<void>
  refreshMapping: () => Promise<void> | undefined
}

const sample = (name: string): ChinaMapData => ({
  text: name,
  subtext: '2026-09-06 12:00',
  avgdelay: { ctcc: [{ name: '广东', value: 10 }], cucc: [], cmcc: [] }
})

function createView(t: test.TestContext) {
  const locale = vue.ref('zh-CN')
  const getMapping = t.mock.fn(async (_date?: string, _signal?: AbortSignal): Promise<ChinaMapData> => sample('local'))
  const getProxyMapping = t.mock.fn(async (_base: string, _date?: string, _signal?: AbortSignal): Promise<ChinaMapData> => sample('remote'))
  const fetchConfig = t.mock.fn(async (_signal?: AbortSignal) => ({ Addr: '127.0.0.1', Port: 8899, Network: {} }))
  const clear = t.mock.fn()
  const dispose = t.mock.fn()
  let unmount!: () => void
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: (callback: () => void) => { unmount = callback } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => `${locale.value}:${key}`, locale }) },
    'element-plus': { ElMessage: { error: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusMappingStyles': {},
    '@/components/common/RefreshStatus.vue': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig },
    '@/api/mapping': { getMapping, getProxyMapping },
    '@/utils/chartInteraction': { preserveLegendSelection },
    '@/utils/chartTooltip': { formatMappingTooltip },
    '@/stores/sidebar': { useSidebarStore: () => ({ isCollapsed: false }) },
    '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => MappingSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    document: { documentElement: {} },
    window: { getComputedStyle: () => ({ getPropertyValue: () => '' }), removeEventListener: () => {} },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => { unmount(); scope.stop() })
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.chart = { clear, dispose }
  return { view, getMapping, getProxyMapping, fetchConfig, clear, dispose, locale, unmount: () => unmount() }
}

test('map legend selection survives repeated language changes without duplicate series', async (t) => {
  echarts.use(SVGRenderer)
  echarts.registerMap('china', {
    type: 'FeatureCollection',
    features: [{ type: 'Feature', properties: { name: '广东', cp: [1, 1] },
      geometry: { type: 'Polygon', coordinates: [[[0, 0], [2, 0], [2, 2], [0, 2], [0, 0]]] } }]
  })
  const chart = echarts.init(null, undefined, { renderer: 'svg', ssr: true, width: 640, height: 480 })
  const { view, locale } = createView(t)
  view.chart = chart
  view.isMapReady.value = true
  view.latestData.value = sample('map')
  view.updateChart(view.latestData.value)
  chart.dispatchAction({ type: 'legendUnSelect', name: 'zh-CN:mapping.telecom' })
  for (const language of ['en-US', 'zh-CN', 'en-US']) {
    locale.value = language
    await setImmediate()
    const option = chart.getOption() as {
      series: Array<{ id: string; name: string }>
      legend: Array<{ selected: Record<string, boolean> }>
    }
    assert.equal(option.series.length, 3)
    assert.equal(option.series[0]!.id, 'ctcc')
    assert.equal(option.series[0]!.name, `${language}:mapping.telecom`)
    assert.equal(option.legend[0]!.selected[`${language}:mapping.telecom`], false)
    assert.equal(option.legend[0]!.selected[`${language}:mapping.unicom`], true)
  }
  chart.clear()
  view.updateChart(sample('new query'))
  const option = chart.getOption() as { legend: Array<{ selected: Record<string, boolean> }> }
  assert.notEqual(option.legend[0]!.selected['en-US:mapping.telecom'], false)
})

test('changing map source clears old data even when the new source fails', async (t) => {
  const { view, getProxyMapping, clear } = createView(t)
  await view.loadConfig()
  assert.equal(view.latestData.value?.text, 'local')
  assert.ok(view.lastUpdatedAt.value)

  getProxyMapping.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  view.currentBaseUrl.value = 'http://192.0.2.1:8899'
  const pending = view.loadMappingData()
  assert.equal(view.latestData.value, null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(clear.mock.callCount(), 2)
  await pending
  assert.equal(view.mappingError.value, true)
  assert.equal(view.mappingLoading.value, false)
  assert.equal(view.latestData.value, null)
  assert.equal(view.hasRetainedMapping.value, false)
})

test('changing map time prevents a superseded response from restoring old data', async (t) => {
  const { view, getMapping } = createView(t)
  await view.loadConfig()
  let resolveOld!: (value: ChinaMapData) => void
  getMapping.mock.mockImplementationOnce(
    () =>
      new Promise((resolve) => {
        resolveOld = resolve
      })
  )
  const oldRequest = view.loadMappingData()
  view.selectedDate.value = '2026-09-05 12:00'
  getMapping.mock.mockImplementationOnce(async () => {
    throw new Error('offline')
  })
  await view.loadMappingData()
  resolveOld(sample('outdated'))
  await oldRequest
  assert.equal(view.latestData.value, null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.mappingError.value, true)
})

test('refreshing the same map preserves the previous sample and timestamp on failure', async (t) => {
  const { view, getMapping, clear } = createView(t)
  await view.loadConfig()
  const data = view.latestData.value
  const updatedAt = view.lastUpdatedAt.value
  getMapping.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.loadMappingData()
  assert.equal(view.latestData.value, data)
  assert.equal(view.lastUpdatedAt.value, updatedAt)
  assert.equal(clear.mock.callCount(), 1)
  assert.equal(view.mappingError.value, true)
})

test('reloading configuration returns map requests to the local node', async (t) => {
  const { view, getMapping, getProxyMapping } = createView(t)
  await view.loadConfig()
  getMapping.mock.resetCalls()
  view.currentAgent.value = '192.0.2.1'
  view.currentBaseUrl.value = 'http://192.0.2.1:8899'
  await view.loadMappingData()
  await view.loadConfig()
  assert.equal(view.currentAgent.value, '127.0.0.1')
  assert.equal(view.currentBaseUrl.value, '')
  assert.equal(getProxyMapping.mock.callCount(), 1)
  assert.equal(getMapping.mock.callCount(), 1)
  assert.equal(view.latestData.value?.text, 'local')
})

test('switching back to the local map uses the direct API and releases source loading states', async (t) => {
  const { view, getMapping, getProxyMapping } = createView(t)
  const currentData = () => view.latestData.value
  await view.loadConfig()
  view.agents.value = [
    { name: 'local', addr: '127.0.0.1', loading: false },
    { name: 'remote', addr: '192.0.2.1', loading: false }
  ]
  await view.switchAgent(view.agents.value[1]!)
  assert.equal(view.currentBaseUrl.value, 'http://192.0.2.1:8899')
  assert.equal(view.latestData.value?.text, 'remote')
  assert.equal(getProxyMapping.mock.callCount(), 1)
  getProxyMapping.mock.mockImplementation(async () => { throw new Error('proxy unavailable') })
  const pending = view.switchAgent(view.agents.value[0]!)
  assert.equal(view.agents.value[0]!.loading, true)
  assert.equal(currentData(), null)
  await pending
  assert.equal(view.currentBaseUrl.value, '')
  assert.equal(view.currentAgent.value, '127.0.0.1')
  assert.equal(view.latestData.value?.text, 'local')
  assert.equal(view.mappingError.value, false)
  assert.equal(getMapping.mock.callCount(), 2)
  assert.equal(getProxyMapping.mock.callCount(), 1)
  assert.ok(view.agents.value.every((agent) => !agent.loading))
})

test('map refresh failures keep a persistent stale-data state until recovery or a query change', async (t) => {
  const { view, getMapping, clear } = createView(t)
  view.selectedDate.value = '2026-09-06 12:00'
  await view.loadConfig()
  assert.equal(view.hasRetainedMapping.value, false)
  const previous = view.latestData.value
  const updatedAt = view.lastUpdatedAt.value
  getMapping.mock.mockImplementation(async () => { throw new Error('offline') })
  for (let retry = 0; retry < 2; retry++) {
    await view.loadMappingData()
    assert.equal(view.hasRetainedMapping.value, true)
    assert.equal(view.latestData.value, previous)
    assert.equal(view.lastUpdatedAt.value, updatedAt)
  }
  assert.equal(clear.mock.callCount(), 1)
  getMapping.mock.mockImplementation(async () => sample('recovered'))
  await view.loadMappingData()
  assert.equal(view.hasRetainedMapping.value, false)
  assert.equal(view.latestData.value?.text, 'recovered')
  assert.notEqual(view.lastUpdatedAt.value, updatedAt)
  view.selectedDate.value = '2026-09-06 13:00'
  getMapping.mock.mockImplementation(async () => { throw new Error('new query failed') })
  await view.loadMappingData()
  assert.equal(view.mappingError.value, true)
  assert.equal(view.hasRetainedMapping.value, false)
  assert.equal(view.latestData.value, null)
})

test('map stale-data state distinguishes initial failures and a retained empty result', async (t) => {
  const { view, getMapping } = createView(t)
  getMapping.mock.mockImplementation(async () => { throw new Error('offline') })
  await view.loadConfig()
  assert.equal(view.mappingError.value, true)
  assert.equal(view.hasRetainedMapping.value, false)
  getMapping.mock.mockImplementation(async () => ({ ...sample('empty'), avgdelay: { ctcc: [], cucc: [], cmcc: [] } }))
  await view.loadMappingData()
  assert.equal(view.hasRetainedMapping.value, false)
  const previous = view.latestData.value
  getMapping.mock.mockImplementation(async () => { throw new Error('offline') })
  await view.loadMappingData()
  assert.equal(view.latestData.value, previous)
  assert.equal(view.hasRetainedMapping.value, true)
})

test('map config reload blocks old source selections and ignores removed source callbacks', async (t) => {
  const { view, getMapping, getProxyMapping, fetchConfig } = createView(t)
  await view.loadConfig()
  const oldSource = { name: 'old', addr: '192.0.2.1', loading: false }
  view.agents.value = [oldSource]
  const previous = view.latestData.value
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  t.after(() => release())
  fetchConfig.mock.mockImplementation(async () => {
    await gate
    return { Addr: '127.0.0.2', Port: 9000, Network: {
      '192.0.2.2': { Name: 'new', Addr: '192.0.2.2', Smartping: true }
    } }
  })
  getMapping.mock.resetCalls()
  getProxyMapping.mock.resetCalls()
  const pending = view.loadConfig()
  await view.switchAgent(oldSource)
  try {
    assert.equal(getProxyMapping.mock.callCount(), 0)
    assert.equal(view.currentAgent.value, '127.0.0.1')
    assert.equal(view.latestData.value, previous)
  } finally {
    release()
    await pending
  }
  await view.switchAgent(oldSource)
  assert.equal(getProxyMapping.mock.callCount(), 0)
  assert.equal(view.currentAgent.value, '127.0.0.2')
  await view.switchAgent(view.agents.value[0]!)
  assert.equal(getProxyMapping.mock.callCount(), 1)
  assert.equal(getProxyMapping.mock.calls[0]!.arguments[0], 'http://192.0.2.2:9000')
  assert.equal(view.latestData.value?.text, 'remote')
})

test('map unmount prevents late configuration and source callbacks from starting requests', async (t) => {
  const { view, fetchConfig, getMapping, getProxyMapping, unmount } = createView(t)
  await view.loadConfig()
  view.agents.value = [{ name: 'remote', addr: '192.0.2.1', loading: false }]
  const agent = view.agents.value[0]!
  const configCalls = fetchConfig.mock.callCount()
  const mappingCalls = getMapping.mock.callCount()
  const source = view.currentAgent.value
  unmount()
  await view.loadConfig()
  await view.loadMappingData()
  await view.switchAgent(agent)
  assert.equal(fetchConfig.mock.callCount(), configCalls)
  assert.equal(getMapping.mock.callCount(), mappingCalls)
  assert.equal(getProxyMapping.mock.callCount(), 0)
  assert.equal(view.currentAgent.value, source)
  assert.equal(view.configLoading.value, false)
  assert.equal(agent.loading, false)
  assert.equal(view.latestData.value, null)
})

test('map unmount cancels active data queries and rejects their late success or failure', async (t) => {
  for (const fail of [false, true]) {
    const { view, getMapping, unmount } = createView(t)
    await view.loadConfig()
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    t.after(() => release())
    getMapping.mock.mockImplementation(async () => {
      await gate
      if (fail) throw new Error('late failure')
      return sample('late data')
    })
    const updatedAt = view.lastUpdatedAt.value
    const pending = view.loadMappingData()
    const signal = getMapping.mock.calls.at(-1)!.arguments[1]!
    unmount()
    assert.equal(signal.aborted, true)
    release()
    await pending
    assert.equal(view.latestData.value, null)
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    assert.equal(view.mappingError.value, false)
    assert.equal(view.hasRetainedMapping.value, false)
  }
})

test('map date and refresh callbacks pause during configuration reload then use the new endpoints', async (t) => {
  const { view, getMapping, getProxyMapping, fetchConfig, clear } = createView(t)
  view.selectedDate.value = '2026-09-06 12:00'
  await view.loadConfig()
  const previous = view.latestData.value
  const updatedAt = view.lastUpdatedAt.value
  const clearCalls = clear.mock.callCount()
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  t.after(() => release())
  fetchConfig.mock.mockImplementation(async () => {
    await gate
    return { Addr: '127.0.0.2', Port: 9000, Network: {
      remote: { Name: 'new', Addr: '192.0.2.2', Smartping: true }
    } }
  })
  getMapping.mock.resetCalls()
  getProxyMapping.mock.resetCalls()
  const pending = view.loadConfig()
  try {
    await view.loadMappingData()
    await view.refreshMapping()
    await view.loadMappingData()
    assert.equal(getMapping.mock.callCount(), 0)
    assert.equal(getProxyMapping.mock.callCount(), 0)
    assert.equal(view.latestData.value, previous)
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    assert.equal(clear.mock.callCount(), clearCalls)
    assert.equal(view.mappingLoading.value, false)
  } finally {
    release()
    await pending
  }
  assert.equal(getMapping.mock.callCount(), 1, 'the completed config load must still query the map')
  assert.equal(getMapping.mock.calls[0]!.arguments[0], '2026-09-06 12:00')
  assert.equal(view.currentAgent.value, '127.0.0.2')
  await view.switchAgent(view.agents.value[0]!)
  await view.refreshMapping()
  assert.equal(getProxyMapping.mock.callCount(), 2)
  assert.ok(getProxyMapping.mock.calls.every((call) => call.arguments[0] === 'http://192.0.2.2:9000'))
})

test('map callbacks wait for initial configuration and refresh recovers an initial config failure', async (t) => {
  const { view, getMapping, getProxyMapping, fetchConfig } = createView(t)
  await view.loadMappingData()
  await view.refreshMapping()
  assert.equal(fetchConfig.mock.callCount(), 0)
  assert.equal(getMapping.mock.callCount(), 0)
  assert.equal(getProxyMapping.mock.callCount(), 0)
  assert.equal(view.lastUpdatedAt.value, null)
  fetchConfig.mock.mockImplementationOnce(async () => { throw new Error('offline') })
  await view.loadConfig()
  assert.equal(view.configError.value, true)
  assert.equal(view.configLoading.value, false)
  await view.loadMappingData()
  assert.equal(getMapping.mock.callCount(), 0)
  await view.refreshMapping()
  assert.equal(fetchConfig.mock.callCount(), 2)
  assert.equal(getMapping.mock.callCount(), 1)
  assert.equal(view.configError.value, false)
  assert.equal(view.latestData.value?.text, 'local')
  assert.ok(view.lastUpdatedAt.value)
})

test('map config reload cancels old requests and rejects late results while callbacks are paused', async (t) => {
  for (const fail of [false, true]) {
    const { view, getMapping, fetchConfig } = createView(t)
    await view.loadConfig()
    let releaseOld!: () => void
    const oldGate = new Promise<void>((resolve) => { releaseOld = resolve })
    t.after(() => releaseOld())
    getMapping.mock.mockImplementationOnce(async () => {
      await oldGate
      if (fail) throw new Error('late failure')
      return sample('obsolete')
    })
    const oldRequest = view.loadMappingData()
    const oldSignal = getMapping.mock.calls.at(-1)!.arguments[1]!
    let releaseConfig!: () => void
    const configGate = new Promise<void>((resolve) => { releaseConfig = resolve })
    t.after(() => releaseConfig())
    fetchConfig.mock.mockImplementation(async () => {
      await configGate
      return { Addr: '127.0.0.2', Port: 9000, Network: {} }
    })
    const pending = view.loadConfig()
    try {
      assert.equal(oldSignal.aborted, true)
      const calls = getMapping.mock.callCount()
      await view.refreshMapping()
      await view.loadMappingData()
      assert.equal(getMapping.mock.callCount(), calls)
    } finally {
      releaseConfig()
      await pending
      releaseOld()
      await oldRequest
    }
    assert.equal(view.latestData.value?.text, 'local')
    assert.equal(view.currentAgent.value, '127.0.0.2')
    assert.equal(view.mappingError.value, false)
    assert.equal(view.mappingLoading.value, false)
    assert.equal(view.configLoading.value, false)
  }
})

test('map callbacks do not cancel the query started by an unfinished config load', async (t) => {
  for (const reload of [false, true]) {
    const { view, getMapping } = createView(t)
    if (reload) await view.loadConfig()
    const previous = view.latestData.value
    const updatedAt = view.lastUpdatedAt.value
    getMapping.mock.resetCalls()
    let release!: () => void
    const gate = new Promise<void>((resolve) => { release = resolve })
    t.after(() => release())
    getMapping.mock.mockImplementationOnce(async () => {
      await gate
      return sample('fresh config query')
    })
    const pending = view.loadConfig()
    await setImmediate()
    try {
      assert.equal(getMapping.mock.callCount(), 1)
      const signal = getMapping.mock.calls[0]!.arguments[1]!
      assert.equal(view.configLoading.value, true)
      assert.equal(view.mappingLoading.value, true)
      await view.loadMappingData()
      await view.refreshMapping()
      assert.equal(signal.aborted, false)
      assert.equal(getMapping.mock.callCount(), 1)
      assert.equal(view.latestData.value, previous)
      assert.equal(view.lastUpdatedAt.value, updatedAt)
      assert.equal(view.mappingLoading.value, true)
    } finally {
      release()
      await pending
    }
    assert.equal(view.latestData.value?.text, 'fresh config query')
    assert.equal(view.configLoading.value, false)
    assert.equal(view.mappingLoading.value, false)
    assert.notEqual(view.lastUpdatedAt.value, updatedAt)
    await view.refreshMapping()
    assert.equal(getMapping.mock.callCount(), 2)
  }
})
