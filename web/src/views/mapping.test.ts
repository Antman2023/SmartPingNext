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
import * as calendarDate from '../utils/calendarDate.js'

// Execute the actual view setup with controlled API responses and no browser renderer.
const source = readFileSync(new URL('../../src/views/MappingView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'mapping-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText
const compiledMappingApi = ts.transpileModule(readFileSync(new URL('../../src/api/mapping.ts', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText
const compiledDebounce = ts.transpileModule(readFileSync(new URL('../../src/utils/debounce.ts', import.meta.url), 'utf8'), {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText
const compiledChartExport = ts.transpileModule(readFileSync(new URL('../../src/utils/chartExport.ts', import.meta.url), 'utf8'), {
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
  chart: { clear: () => void; dispose: () => void; getOption: () => Record<string, unknown>; setOption: (option: unknown) => void }
  isMapReady: vue.Ref<boolean>
  chartRef: vue.Ref<object | undefined>
  chartLoading: vue.Ref<boolean>
  chartError: vue.Ref<boolean>
  initChart: () => Promise<void>
  updateChart: (data: ChinaMapData) => void
  loadMappingData: () => Promise<void>
  loadConfig: () => Promise<void>
  refreshMapping: () => Promise<void> | undefined
  canSaveMapImage: vue.ComputedRef<boolean>
  saveMapImage: () => void
  handleResize: () => void
}

const sample = (name: string): ChinaMapData => ({
  text: name,
  subtext: '2026-09-06 12:00',
  avgdelay: { ctcc: [{ name: '广东', value: 10 }], cucc: [], cmcc: [] }
})

function createView(t: test.TestContext, validateResponses = false) {
  const locale = vue.ref('zh-CN')
  const theme = vue.reactive({ theme: 'light' })
  const sidebar = vue.reactive({ isCollapsed: false })
  const timers = new Map<number, () => void>()
  let timerId = 0
  const window = {
    getComputedStyle: () => ({ getPropertyValue: () => '' }),
    removeEventListener: t.mock.fn(),
    setTimeout: (callback: () => void) => { timers.set(++timerId, callback); return timerId },
    clearTimeout: (id: number) => { timers.delete(id) }
  }
  const debounceExports = {}
  runInNewContext(compiledDebounce, { exports: debounceExports, window })
  const getMapping = t.mock.fn(async (_date?: string, _signal?: AbortSignal): Promise<ChinaMapData> => sample('local'))
  const getProxyMapping = t.mock.fn(async (_base: string, _date?: string, _signal?: AbortSignal): Promise<ChinaMapData> => sample('remote'))
  const mappingApi: Record<string, unknown> = validateResponses ? {} : { getMapping, getProxyMapping }
  if (validateResponses) {
    runInNewContext(compiledMappingApi, {
      exports: mappingApi, URL, URLSearchParams,
      require: (id: string) => {
        if (id === './index') return { default: { get: (url: string) =>
          url.startsWith('/proxy.json?') ? getProxyMapping(url) : getMapping(url)
        }, proxyRequestConfig: () => ({}) }
        if (id === '@/locales') return { default: { global: { t: (key: string) => key } } }
        if (id === '@/utils/calendarDate') return calendarDate
        throw new Error(`Unexpected mapping API dependency: ${id}`)
      }
    })
  }
  const fetchConfig = t.mock.fn(async (_signal?: AbortSignal) => ({ Addr: '127.0.0.1', Port: 8899, Network: {} }))
  const clear = t.mock.fn()
  const dispose = t.mock.fn()
  const setOption = t.mock.fn((_option: unknown) => {})
  const getDataURL = t.mock.fn(() => 'data:image/png;base64,test')
  const resize = t.mock.fn()
  const click = t.mock.fn()
  const link = { download: '', href: '', click }
  const createElement = t.mock.fn(() => link)
  const document = { documentElement: {}, createElement }
  const chartExport = {}
  runInNewContext(compiledChartExport, { exports: chartExport, document })
  const messageError = t.mock.fn((_message: string) => {})
  const chart = { clear, dispose, setOption, getOption: () => ({}), getDataURL, resize }
  const init = t.mock.fn(() => chart)
  const registerMap = t.mock.fn()
  const loadMap = t.mock.fn(() => ({ default: {} }))
  let unmount!: () => void
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: (callback: () => void) => { unmount = callback } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => `${locale.value}:${key}`, locale }) },
    'element-plus': { ElMessage: { error: messageError } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusMappingStyles': {},
    '@/components/common/RefreshStatus.vue': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig },
    '@/api/mapping': mappingApi,
    '@/utils/chartInteraction': { preserveLegendSelection },
    '@/utils/chartExport': chartExport,
    '@/utils/chartTooltip': { formatMappingTooltip },
    '@/stores/sidebar': { useSidebarStore: () => sidebar },
    '@/utils/debounce': debounceExports,
    '@/stores/theme': { useThemeStore: () => theme },
    '@/utils/echartsMap': { echarts: { init, registerMap } },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => MappingSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    document,
    window,
    console: { error: () => {} },
    require: (name: string) => {
      if (name === 'china-map-geojson/lib/china') return loadMap()
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => { unmount(); scope.stop() })
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.chart = chart
  return { view, getMapping, getProxyMapping, fetchConfig, clear, dispose, setOption,
    locale, theme, init, registerMap, loadMap, getDataURL, link, createElement, messageError, resize, sidebar, timers,
    removeEventListener: window.removeEventListener,
    flushTimers: () => {
      for (const [id, callback] of Array.from(timers)) {
        timers.delete(id)
        callback()
      }
    }, unmount: () => unmount() }
}

test('map unmount releases listeners and chart references even when renderer disposal fails', async (t) => {
  const { view, dispose, resize, sidebar, timers, flushTimers, removeEventListener, unmount } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  const disposals = dispose.mock.callCount()
  sidebar.isCollapsed = true
  await vue.nextTick()
  view.handleResize()
  dispose.mock.mockImplementationOnce(() => { throw new Error('dispose failed') })
  assert.doesNotThrow(unmount)
  assert.equal(view.chart, null)
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.latestData.value, null)
  assert.equal(timers.size, 0)
  assert.equal(removeEventListener.mock.callCount(), 1)
  assert.equal(removeEventListener.mock.calls[0]!.arguments[0], 'resize')
  assert.equal(removeEventListener.mock.calls[0]!.arguments[1], view.handleResize)
  unmount()
  assert.equal(dispose.mock.callCount(), disposals + 1, 'failed disposal must not leave a retained instance')
  view.handleResize()
  flushTimers()
  assert.equal(resize.mock.callCount(), 0)
})

for (const failure of ['encode', 'empty', 'canvas-limit', 'empty-png', 'create-link', 'download'] as const) {
  test(`map image export handles ${failure} failure without losing data and allows retry`, async (t) => {
    const { view, getDataURL, createElement, link, messageError, locale, getMapping } = createView(t)
    view.chartRef.value = {}
    await view.loadConfig()
    await view.initChart()
    const data = view.latestData.value
    const updated = view.lastUpdatedAt.value
    const requests = getMapping.mock.callCount()
    if (failure === 'encode') getDataURL.mock.mockImplementationOnce(() => { throw new Error('canvas unavailable') })
    if (failure === 'empty') getDataURL.mock.mockImplementationOnce(() => '')
    if (failure === 'canvas-limit') getDataURL.mock.mockImplementationOnce(() => 'data:,')
    if (failure === 'empty-png') getDataURL.mock.mockImplementationOnce(() => 'data:image/png;base64,')
    if (failure === 'create-link') createElement.mock.mockImplementationOnce(() => { throw new Error('link unavailable') })
    if (failure === 'download') link.click.mock.mockImplementationOnce(() => { throw new Error('download unavailable') })
    locale.value = 'en-US'
    await vue.nextTick()
    assert.doesNotThrow(() => view.saveMapImage())
    assert.equal(messageError.mock.callCount(), 1)
    assert.equal(messageError.mock.calls[0]!.arguments[0], 'en-US:common.saveImageFailed')
    if (failure !== 'download') assert.equal(link.click.mock.callCount(), 0)
    assert.equal(view.latestData.value, data)
    assert.equal(view.lastUpdatedAt.value, updated)
    assert.equal(view.chartError.value, false)
    assert.equal(view.mappingError.value, false)
    assert.equal(view.canSaveMapImage.value, true)
    const clicks = link.click.mock.callCount()
    view.saveMapImage()
    assert.equal(link.click.mock.callCount(), clicks + 1)
    assert.equal(link.href, 'data:image/png;base64,test')
    assert.equal(messageError.mock.callCount(), 1)
    assert.equal(getMapping.mock.callCount(), requests)
  })
}

test('map coalesces rapid resize events without redrawing data or querying the network', async (t) => {
  const { view, resize, timers, flushTimers, setOption, getMapping } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  const draws = setOption.mock.callCount()
  const requests = getMapping.mock.callCount()
  for (let i = 0; i < 100; i++) view.handleResize()
  assert.equal(resize.mock.callCount(), 0)
  assert.equal(timers.size, 1)
  flushTimers()
  assert.equal(resize.mock.callCount(), 1)
  assert.equal(timers.size, 0)
  assert.equal(setOption.mock.callCount(), draws)
  assert.equal(getMapping.mock.callCount(), requests)
})

test('map unmount cancels window and sidebar resize work and blocks late callbacks', async (t) => {
  const { view, resize, sidebar, timers, flushTimers, unmount } = createView(t)
  view.chartRef.value = {}
  await view.initChart()
  sidebar.isCollapsed = true
  await vue.nextTick()
  view.handleResize()
  assert.equal(timers.size, 2)
  unmount()
  assert.equal(timers.size, 0)
  view.handleResize()
  sidebar.isCollapsed = false
  await vue.nextTick()
  flushTimers()
  assert.equal(timers.size, 0)
  assert.equal(resize.mock.callCount(), 0)
})

test('map resize failure exposes chart retry and stops subsequent resizing until recovery', async (t) => {
  const { view, resize, flushTimers } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  const data = view.latestData.value
  resize.mock.mockImplementationOnce(() => { throw new Error('resize failed') })
  view.handleResize()
  assert.doesNotThrow(flushTimers)
  assert.equal(view.chartError.value, true)
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.latestData.value, data)
  view.handleResize()
  flushTimers()
  assert.equal(resize.mock.callCount(), 1)
  await view.initChart()
  view.handleResize()
  flushTimers()
  assert.equal(resize.mock.callCount(), 2)
  assert.equal(view.chartError.value, false)
})

test('map rendering failure preserves accepted data and exposes chart retry instead of a request error', async (t) => {
  const { view, setOption, getMapping } = createView(t)
  view.chartRef.value = {}
  await view.initChart()
  setOption.mock.mockImplementationOnce(() => { throw new Error('render failure') })
  await view.loadConfig()
  assert.equal(view.latestData.value?.text, 'local')
  assert.ok(view.lastUpdatedAt.value)
  assert.equal(view.mappingError.value, false)
  assert.equal(view.configError.value, false)
  assert.equal(view.chartError.value, true)
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.canSaveMapImage.value, false)
  const requests = getMapping.mock.callCount()
  await view.initChart()
  assert.equal(view.chartError.value, false)
  assert.equal(view.isMapReady.value, true)
  assert.equal(view.canSaveMapImage.value, true)
  assert.equal(getMapping.mock.callCount(), requests, 'chart retry should reuse accepted data')
})

for (const change of ['date', 'source', 'config'] as const) {
  test(`map ${change} query continues when clearing the old chart fails`, async (t) => {
    const { view, clear, getMapping, getProxyMapping, fetchConfig, setOption } = createView(t)
    view.chartRef.value = {}
    await view.loadConfig()
    await view.initChart()
    const previous = view.latestData.value
    const draws = setOption.mock.callCount()
    clear.mock.mockImplementationOnce(() => { throw new Error('clear failed') })
    getMapping.mock.mockImplementation(async () => sample('new query'))
    getProxyMapping.mock.mockImplementation(async () => sample('new query'))
    if (change === 'date') {
      view.selectedDate.value = '2026-10-13 12:00'
      await view.loadMappingData()
    } else if (change === 'source') {
      view.agents.value = [{ name: 'remote', addr: '192.0.2.1', loading: false }]
      await view.switchAgent(view.agents.value[0]!)
    } else {
      fetchConfig.mock.mockImplementation(async () => ({ Addr: '127.0.0.2', Port: 9000, Network: {} }))
      await view.loadConfig()
    }
    assert.notEqual(view.latestData.value, previous)
    assert.equal(view.latestData.value?.text, 'new query')
    assert.ok(view.lastUpdatedAt.value)
    assert.equal(view.chartError.value, true)
    assert.equal(view.isMapReady.value, false)
    assert.equal(view.mappingError.value, false)
    assert.equal(view.configError.value, false)
    assert.equal(view.mappingLoading.value, false)
    assert.equal(view.configLoading.value, false)
    assert.ok(view.agents.value.every((agent) => !agent.loading))
    assert.equal(setOption.mock.callCount(), draws, 'failed chart must wait for explicit retry')
    const requests = getMapping.mock.callCount() + getProxyMapping.mock.callCount()
    await view.initChart()
    assert.equal(view.chartError.value, false)
    assert.equal(view.isMapReady.value, true)
    const option = setOption.mock.calls.at(-1)!.arguments[0] as { title: { text: string } }
    assert.equal(option.title.text, 'new query')
    assert.equal(getMapping.mock.callCount() + getProxyMapping.mock.callCount(), requests)
  })
}

test('map clear and request failures remain separate and do not restore another query', async (t) => {
  const { view, clear, getMapping } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  clear.mock.mockImplementationOnce(() => { throw new Error('clear failed') })
  getMapping.mock.mockImplementationOnce(async () => { throw new Error('offline') })
  view.selectedDate.value = '2026-10-13 12:00'
  await view.loadMappingData()
  assert.equal(view.latestData.value, null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.chartError.value, true)
  assert.equal(view.mappingError.value, true)
  assert.equal(view.mappingLoading.value, false)
  await view.initChart()
  assert.equal(view.chartError.value, false)
  assert.equal(view.mappingError.value, true)
  assert.equal(view.canSaveMapImage.value, false)
  await view.loadMappingData()
  assert.equal(view.mappingError.value, false)
  assert.equal(view.canSaveMapImage.value, true)
})

test('map display-change rendering failures are handled and recover on retry', async (t) => {
  const { view, setOption, locale } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  const data = view.latestData.value
  const updated = view.lastUpdatedAt.value
  setOption.mock.mockImplementationOnce(() => { throw new Error('render failure') })
  locale.value = 'en-US'
  await vue.nextTick()
  assert.equal(view.chartError.value, true)
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.latestData.value, data)
  assert.equal(view.lastUpdatedAt.value, updated)
  await view.initChart()
  assert.equal(view.chartError.value, false)
  assert.equal(view.isMapReady.value, true)
})

test('map export blocks incomplete states and late callbacks but permits retained data', async (t) => {
  const { view, getDataURL, link, getMapping, unmount } = createView(t)
  view.chartRef.value = {}
  view.saveMapImage()
  assert.equal(getDataURL.mock.callCount(), 0)
  await view.loadConfig()
  await view.initChart()
  for (const state of [view.configLoading, view.mappingLoading, view.chartLoading, view.chartError]) {
    state.value = true
    assert.equal(view.canSaveMapImage.value, false)
    view.saveMapImage()
    assert.equal(getDataURL.mock.callCount(), 0)
    state.value = false
  }
  view.isMapReady.value = false
  view.saveMapImage()
  assert.equal(getDataURL.mock.callCount(), 0)
  view.isMapReady.value = true
  getMapping.mock.mockImplementationOnce(async () => { throw new Error('offline') })
  await view.loadMappingData()
  assert.equal(view.hasRetainedMapping.value, true)
  assert.equal(view.canSaveMapImage.value, true)
  view.saveMapImage()
  assert.equal(getDataURL.mock.callCount(), 1)
  assert.equal(link.click.mock.callCount(), 1)
  assert.equal(link.href, 'data:image/png;base64,test')
  assert.match(link.download, /^smartping-map-\d+\.png$/)
  unmount()
  view.saveMapImage()
  assert.equal(getDataURL.mock.callCount(), 1)
})

test('map initialization after unmount does not load modules or change loading state', async (t) => {
  const { view, loadMap, init, unmount } = createView(t)
  view.chartRef.value = {}
  unmount()
  await view.initChart()
  assert.equal(loadMap.mock.callCount(), 0)
  assert.equal(init.mock.callCount(), 0)
  assert.equal(view.chartLoading.value, false)
  assert.equal(view.chartError.value, false)
  assert.equal(view.isMapReady.value, false)
})

test('map initialization is shared and its pending completion cannot revive an unmounted chart', async (t) => {
  const { view, loadMap, init, registerMap, unmount } = createView(t)
  view.chartRef.value = {}
  const pending = view.initChart()
  assert.equal(view.initChart(), pending)
  unmount()
  await pending
  assert.equal(loadMap.mock.callCount(), 1)
  assert.equal(init.mock.callCount(), 0)
  assert.equal(registerMap.mock.callCount(), 0)
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.chartLoading.value, false)
})

test('failed map reinitialization clears readiness and can retry with the latest data', async (t) => {
  const { view, init, setOption } = createView(t)
  view.chartRef.value = {}
  await view.loadConfig()
  await view.initChart()
  assert.equal(view.isMapReady.value, true)
  init.mock.mockImplementationOnce(() => { throw new Error('renderer unavailable') })
  await view.initChart()
  assert.equal(view.isMapReady.value, false)
  assert.equal(view.chartError.value, true)
  assert.equal(view.chartLoading.value, false)
  const draws = setOption.mock.callCount()
  await view.loadMappingData()
  assert.equal(setOption.mock.callCount(), draws, 'failed renderer must not receive refreshed data')
  await view.initChart()
  assert.equal(view.isMapReady.value, true)
  assert.equal(view.chartError.value, false)
  assert.equal(setOption.mock.callCount(), draws + 1)
})

test('map batches simultaneous theme and language changes without requesting new data', async (t) => {
  const { view, theme, locale, setOption, getMapping } = createView(t)
  view.isMapReady.value = true
  await view.loadConfig()
  const draws = setOption.mock.callCount()
  const requests = getMapping.mock.callCount()
  theme.theme = 'dark'
  locale.value = 'en-US'
  await vue.nextTick()
  assert.equal(setOption.mock.callCount(), draws + 1)
  assert.equal(getMapping.mock.callCount(), requests)
})

test('invalid local map minute labels retain chart output and successful time until recovery', async (t) => {
  const { view, getMapping, clear, setOption } = createView(t, true)
  const currentData = () => view.latestData.value
  view.isMapReady.value = true
  getMapping.mock.mockImplementation(async () => ({ ...sample('invalid'), subtext: 'tomorrow' }))
  await view.loadConfig()
  assert.equal(currentData(), null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.mappingError.value, true)
  assert.equal(view.configError.value, false)
  assert.equal(setOption.mock.callCount(), 0)

  getMapping.mock.mockImplementation(async () => sample('valid'))
  await view.refreshMapping()
  const previous = view.latestData.value
  const updatedAt = view.lastUpdatedAt.value
  const chartUpdates = setOption.mock.callCount()
  const clears = clear.mock.callCount()
  for (const subtext of ['2026-02-30 12:00', '2026-09-06 24:00', '2026-09-06 12:00 garbage']) {
    getMapping.mock.mockImplementation(async () => ({ ...sample('invalid'), subtext }))
    await view.loadMappingData()
    assert.equal(view.latestData.value, previous)
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    assert.equal(view.mappingError.value, true)
    assert.equal(view.hasRetainedMapping.value, true)
    assert.equal(view.mappingLoading.value, false)
    assert.equal(setOption.mock.callCount(), chartUpdates)
    assert.equal(clear.mock.callCount(), clears)
  }

  getMapping.mock.mockImplementation(async () => ({ ...sample('empty'), subtext: '0000-02-29 00:00',
    avgdelay: { ctcc: [], cucc: [], cmcc: [] } }))
  await view.loadMappingData()
  assert.equal(view.mappingError.value, false)
  assert.equal(view.hasRetainedMapping.value, false)
  assert.equal(view.latestData.value?.subtext, '0000-02-29 00:00')
  assert.notEqual(view.lastUpdatedAt.value, updatedAt)
  const option = setOption.mock.calls.at(-1)!.arguments[0] as { title: { subtext: string }; series: Array<{ data: unknown[] }> }
  assert.equal(option.title.subtext, '0000-02-29 00:00')
  assert.ok(option.series.every((series) => series.data.length === 0))
})

test('invalid proxy map minute labels cannot restore another query or replace retained data', async (t) => {
  const { view, getProxyMapping } = createView(t, true)
  const currentData = () => view.latestData.value
  await view.loadConfig()
  view.agents.value = [
    { name: 'local', addr: '127.0.0.1', loading: false },
    { name: 'remote', addr: '192.0.2.1', loading: false }
  ]
  getProxyMapping.mock.mockImplementation(async () => ({ ...sample('invalid'), subtext: '2026-02-30 12:00' }))
  await view.switchAgent(view.agents.value[1]!)
  assert.equal(currentData(), null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.mappingError.value, true)
  assert.equal(view.hasRetainedMapping.value, false)
  assert.ok(view.agents.value.every((agent) => !agent.loading))

  getProxyMapping.mock.mockImplementation(async () => ({ ...sample('remote'), subtext: '2011-03-13 02:30' }))
  await view.refreshMapping()
  const previous = view.latestData.value
  const updatedAt = view.lastUpdatedAt.value
  assert.equal(previous?.subtext, '2011-03-13 02:30')
  getProxyMapping.mock.mockImplementation(async () => ({ ...sample('invalid'), subtext: '' }))
  await view.loadMappingData()
  assert.equal(view.latestData.value, previous)
  assert.equal(view.lastUpdatedAt.value, updatedAt)
  assert.equal(view.hasRetainedMapping.value, true)
  view.selectedDate.value = '2026-09-06 13:00'
  await view.loadMappingData()
  assert.equal(view.latestData.value, null)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.equal(view.hasRetainedMapping.value, false)
  assert.equal(view.mappingError.value, true)
})

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
