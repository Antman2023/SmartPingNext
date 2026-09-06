import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import type { AlertData, AlertLog } from '../types/index.js'

const source = readFileSync(new URL('../../src/views/AlertsView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'alerts-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface AlertsSetup {
  dates: vue.Ref<string[]>
  selectedDate: vue.Ref<string>
  alerts: vue.Ref<AlertLog[]>
  lastUpdatedAt: vue.Ref<Date | null>
  alertsLoadError: vue.Ref<boolean>
  failedNodes: vue.ComputedRef<number>
  loadConfig: () => Promise<void>
  retryAlerts: () => Promise<void>
  loadAlertsByDate: (date: string) => Promise<void>
}

function createView(t: test.TestContext) {
  const getAlerts = t.mock.fn(async (_baseUrl: string, _date?: string): Promise<AlertData> => ({
    dates: [],
    logs: []
  }))
  const config = {
    Port: 8899,
    Network: {
      a: { Addr: '192.0.2.1', Name: 'a', Topology: [{}] },
      b: { Addr: '192.0.2.2', Name: 'b', Topology: [{}] }
    }
  }
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'vue-router': { useRouter: () => ({}) },
    'element-plus': { ElMessage: { error: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusAlertsStyles': {},
    '@/components/common/RefreshStatus.vue': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig: async () => config },
    '@/api/alert': { getAlerts },
    '@/utils/concurrency': { mapWithConcurrency },
    '@/utils/format': { displayName: (name: string) => name, formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => AlertsSetup } } = {}
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
  return { view, getAlerts, config }
}

test('alert archives retain offline node dates and replace them after recovery', async (t) => {
  const { view, getAlerts } = createView(t)
  getAlerts.mock.mockImplementation(async (url) => ({
    dates: [url.includes('192.0.2.1') ? '2026-09-06' : '2026-09-05'],
    logs: []
  }))
  await view.loadConfig()
  assert.deepEqual([...view.dates.value], ['2026-09-06', '2026-09-05'])
  getAlerts.mock.mockImplementation(async (url) => {
    if (url.includes('192.0.2.2')) throw new Error('offline')
    return { dates: ['2026-09-07'], logs: [] }
  })
  await view.retryAlerts()
  assert.deepEqual([...view.dates.value], ['2026-09-07', '2026-09-05'])
  assert.equal(view.failedNodes.value, 1)
  getAlerts.mock.mockImplementation(async () => ({ dates: [], logs: [] }))
  await view.retryAlerts()
  assert.equal(view.dates.value.length, 0)
  assert.equal(view.failedNodes.value, 0)
})

test('failed alert refresh does not record a successful update', async (t) => {
  const { view, getAlerts } = createView(t)
  getAlerts.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.loadConfig()
  assert.equal(view.lastUpdatedAt.value, null)
  getAlerts.mock.mockImplementation(async () => ({ dates: ['2026-09-06'], logs: [] }))
  await view.retryAlerts()
  const updatedAt = view.lastUpdatedAt.value
  assert.ok(updatedAt)
  getAlerts.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  await view.retryAlerts()
  assert.equal(view.lastUpdatedAt.value, updatedAt)
  assert.equal(view.alertsLoadError.value, true)
  assert.deepEqual([...view.dates.value], ['2026-09-06'])
})

test('switching alert dates clears old records and ignores superseded results', async (t) => {
  const { view, getAlerts } = createView(t)
  const log: AlertLog = {
    Logtime: '2026-09-06 12:00',
    Targetip: '192.0.2.3',
    Targetname: 'target',
    Fromip: '192.0.2.1',
    Fromname: 'a',
    Tracert: '[]'
  }
  getAlerts.mock.mockImplementation(async () => ({ dates: ['2026-09-06'], logs: [log] }))
  await view.loadConfig()
  const resolvers: Array<(data: AlertData) => void> = []
  getAlerts.mock.mockImplementation(() => new Promise((resolve) => resolvers.push(resolve)))
  const oldRequest = view.retryAlerts()
  getAlerts.mock.mockImplementation(async () => {
    throw new Error('offline')
  })
  const nextRequest = view.loadAlertsByDate('2026-09-05')
  assert.equal(view.alerts.value.length, 0)
  assert.equal(view.lastUpdatedAt.value, null)
  await nextRequest
  resolvers.forEach((resolve) => resolve({ dates: ['outdated'], logs: [log] }))
  await oldRequest
  assert.equal(view.alerts.value.length, 0)
  assert.equal(view.lastUpdatedAt.value, null)
  assert.deepEqual([...view.dates.value], ['2026-09-06'])
})

test('alert config reload preserves the selected date and removes obsolete archives', async (t) => {
  const { view, getAlerts, config } = createView(t)
  getAlerts.mock.mockImplementation(async () => ({ dates: ['2026-09-06'], logs: [] }))
  await view.loadConfig()
  await view.loadAlertsByDate('2026-09-05')
  getAlerts.mock.mockImplementation(async (_url, date) => {
    assert.equal(date, '2026-09-05')
    return { dates: ['2026-09-05'], logs: [] }
  })
  await view.loadConfig()
  assert.deepEqual([...view.dates.value], ['2026-09-05'])
  config.Network = {} as typeof config.Network
  await view.loadConfig()
  assert.equal(view.dates.value.length, 0)
  assert.equal(view.lastUpdatedAt.value, null)
})
