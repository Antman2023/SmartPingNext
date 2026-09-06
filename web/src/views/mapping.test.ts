import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
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
  selectedDate: vue.Ref<string>
  currentBaseUrl: vue.Ref<string>
  currentAgent: vue.Ref<string>
  latestData: vue.Ref<ChinaMapData | null>
  lastUpdatedAt: vue.Ref<Date | null>
  mappingError: vue.Ref<boolean>
  mappingLoading: vue.Ref<boolean>
  chart: { clear: () => void }
  loadMappingData: () => Promise<void>
  loadConfig: () => Promise<void>
}

const sample = (name: string): ChinaMapData => ({
  text: name,
  subtext: '2026-09-06 12:00',
  avgdelay: { ctcc: [{ name: '广东', value: 10 }], cucc: [], cmcc: [] }
})

function createView(t: test.TestContext) {
  const getMapping = t.mock.fn(async (): Promise<ChinaMapData> => sample('local'))
  const getProxyMapping = t.mock.fn(async (): Promise<ChinaMapData> => sample('remote'))
  const clear = t.mock.fn()
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale: vue.ref('zh-CN') }) },
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
    '@/utils/chartTooltip': { formatMappingTooltip },
    '@/stores/sidebar': { useSidebarStore: () => ({ isCollapsed: false }) },
    '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => MappingSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
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
  return { view, getMapping, getProxyMapping, clear }
}

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
