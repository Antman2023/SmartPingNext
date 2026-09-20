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
  chart: { clear: () => void }
  isMapReady: vue.Ref<boolean>
  updateChart: (data: ChinaMapData) => void
  loadMappingData: () => Promise<void>
  loadConfig: () => Promise<void>
}

const sample = (name: string): ChinaMapData => ({
  text: name,
  subtext: '2026-09-06 12:00',
  avgdelay: { ctcc: [{ name: '广东', value: 10 }], cucc: [], cmcc: [] }
})

function createView(t: test.TestContext) {
  const locale = vue.ref('zh-CN')
  const getMapping = t.mock.fn(async (): Promise<ChinaMapData> => sample('local'))
  const getProxyMapping = t.mock.fn(async (): Promise<ChinaMapData> => sample('remote'))
  const clear = t.mock.fn()
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => `${locale.value}:${key}`, locale }) },
    'element-plus': { ElMessage: { error: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusMappingStyles': {},
    '@/components/common/RefreshStatus.vue': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig: async () => ({ Addr: '127.0.0.1', Port: 8899, Network: {} }) },
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
    window: { getComputedStyle: () => ({ getPropertyValue: () => '' }) },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.chart = { clear }
  return { view, getMapping, getProxyMapping, clear, locale }
}

test('map legend selection survives repeated language changes without duplicate series', async (t) => {
  echarts.use(SVGRenderer)
  echarts.registerMap('china', {
    type: 'FeatureCollection',
    features: [{ type: 'Feature', properties: { name: '广东', cp: [1, 1] },
      geometry: { type: 'Polygon', coordinates: [[[0, 0], [2, 0], [2, 2], [0, 2], [0, 0]]] } }]
  })
  const chart = echarts.init(null, undefined, { renderer: 'svg', ssr: true, width: 640, height: 480 })
  t.after(() => chart.dispose())
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
  await view.loadMappingData()
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
})

test('changing map time prevents a superseded response from restoring old data', async (t) => {
  const { view, getMapping } = createView(t)
  await view.loadMappingData()
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
  await view.loadMappingData()
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
