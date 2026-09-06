import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import type { PingLogData } from '../types/index.js'

interface Target {
  targetIp: string
  loading: boolean
  chartData: PingLogData | null
  fromAddr: string
  fromPort: number
}
interface MonitorSetup {
  pingTargets?: vue.Ref<Target[]>
  reverseTargets?: vue.Ref<Target[]>
  loadedTargets: vue.ComputedRef<number>
  failedTargets: vue.ComputedRef<number>
  isRefreshing: vue.Ref<boolean>
  lastUpdatedAt: vue.Ref<Date | null>
  loadAllCharts: () => Promise<void>
  loadConfig: () => Promise<void>
}
const sample: PingLogData = {
  lastcheck: ['2026-09-06 12:00'],
  avgdelay: ['10'],
  maxdelay: ['10'],
  mindelay: ['10'],
  losspk: ['0']
}

for (const name of ['DashboardView', 'ReverseView']) {
  const source = readFileSync(new URL(`../../src/views/${name}.vue`, import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  const compiled = ts.transpileModule(compileScript(descriptor, { id: name }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText.replace(/import\.meta\.env/g, 'testEnv')

  function createView(t: test.TestContext) {
    const getPing = t.mock.fn(async (): Promise<PingLogData> => sample)
    const dependencies: Record<string, unknown> = {
      vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
      'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
      'element-plus': { ElMessage: { error: () => {} } },
      '@element-plus/icons-vue': {},
      '@/plugins/elementPlusMonitorStyles': {},
      '@/components/common/MonitorRefreshControl.vue': {},
      '@/components/charts/PingMiniChart.vue': {},
      '@/api': {
        isRequestCanceled: (error: unknown) =>
          error instanceof DOMException && error.name === 'AbortError'
      },
      '@/api/config': {
        fetchConfig: async () => ({ Addr: '127.0.0.1', Base: { Refresh: 1 }, Network: {} })
      },
      '@/api/ping': { getPingData: getPing, getProxyPingData: getPing },
      '@/utils/concurrency': { mapWithConcurrency },
      '@/utils/format': { displayName: (value: string) => value, formatTime: () => '12:00' }
    }
    const exports: { default?: { setup: (props: object, context: object) => MonitorSetup } } = {}
    runInNewContext(compiled, {
      exports,
      testEnv: {},
      AbortController,
      console: { error: () => {} },
      require: (id: string) => {
        assert.ok(id in dependencies, `Unexpected dependency: ${id}`)
        return dependencies[id]
      }
    })
    const scope = vue.effectScope()
    t.after(() => scope.stop())
    const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
    const targets = (view.pingTargets || view.reverseTargets)!
    targets.value = Array.from({ length: 6 }, (_, index) => ({
      targetIp: `192.0.2.${index + 1}`,
      fromAddr: `192.0.2.${index + 1}`,
      fromPort: 8899,
      loading: false,
      chartData: null
    }))
    return { view, targets, getPing }
  }

  test(`${name}: queued targets are loading rather than failed`, async (t) => {
    const { view, targets, getPing } = createView(t)
    let finish!: () => void
    const gate = new Promise<void>((resolve) => {
      finish = resolve
    })
    getPing.mock.mockImplementation(async () => {
      await gate
      return sample
    })
    const request = view.loadAllCharts()
    assert.ok(getPing.mock.callCount() < targets.value.length)
    assert.ok(targets.value.every((target) => target.loading))
    assert.equal(view.failedTargets.value, 0)
    assert.equal(view.loadedTargets.value, 0)
    finish()
    await request
    assert.equal(view.loadedTargets.value, 6)
    assert.ok(targets.value.every((target) => !target.loading))
  })

  test(`${name}: empty and failed requests do not claim successful updates`, async (t) => {
    const { view, targets, getPing } = createView(t)
    getPing.mock.mockImplementation(async () => {
      throw new Error('offline')
    })
    await view.loadAllCharts()
    assert.equal(view.lastUpdatedAt.value, null)
    assert.equal(view.failedTargets.value, 6)
    getPing.mock.mockImplementation(async () => sample)
    await view.loadAllCharts()
    const updatedAt = view.lastUpdatedAt.value
    assert.ok(updatedAt)
    getPing.mock.mockImplementation(async () => {
      throw new Error('offline')
    })
    await view.loadAllCharts()
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    targets.value = []
    await view.loadAllCharts()
    assert.equal(view.lastUpdatedAt.value, updatedAt)
    await view.loadConfig()
    assert.equal(view.lastUpdatedAt.value, null)
  })

  test(`${name}: superseded responses cannot replace the current refresh`, async (t) => {
    const { view, targets, getPing } = createView(t)
    let finishOld!: () => void
    const gate = new Promise<void>((resolve) => {
      finishOld = resolve
    })
    getPing.mock.mockImplementation(async () => {
      await gate
      return sample
    })
    const oldRequest = view.loadAllCharts()
    const oldCalls = getPing.mock.callCount()
    getPing.mock.mockImplementation(async () => {
      throw new Error('offline')
    })
    await view.loadAllCharts()
    finishOld()
    await oldRequest
    assert.equal(view.lastUpdatedAt.value, null)
    assert.ok(targets.value.every((target) => !target.loading && !target.chartData))
    assert.equal(getPing.mock.callCount(), oldCalls + 6)
    assert.equal(view.isRefreshing.value, false)
  })
}
