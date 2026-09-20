import { preloadAsync } from '../utils/preloadAsync.js'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import { isValidTimeRange, normalizeTimeRangeHours } from '../utils/timeRange.js'
import { resolveRefreshInterval } from '../utils/refreshInterval.js'
import type { PingLogData } from '../types/index.js'

interface Target {
  name: string
  fromName: string
  targetIp: string
  loading: boolean
  chartData: PingLogData | null
  fromAddr: string
  fromPort: number
}
interface MonitorSetup {
  config: vue.Ref<{ Base: { Refresh: unknown } } | null>
  autoRefresh: vue.Ref<boolean>
  detailAutoRefresh: vue.Ref<boolean>
  pingTargets?: vue.Ref<Target[]>
  reverseTargets?: vue.Ref<Target[]>
  loadedTargets: vue.ComputedRef<number>
  failedTargets: vue.ComputedRef<number>
  isRefreshing: vue.Ref<boolean>
  lastUpdatedAt: vue.Ref<Date | null>
  loadAllCharts: () => Promise<void>
  loadConfig: () => Promise<void>
  configLoading: vue.Ref<boolean>
  detailVisible: vue.Ref<boolean>
  detailTitle: vue.ComputedRef<string>
  detailLoading: vue.Ref<boolean>
  detailData: vue.Ref<PingLogData | null>
  currentTargetIp?: vue.Ref<string>
  currentTarget?: vue.Ref<Target | null>
  loadDetailData: () => Promise<void>
  showDetail: (target: Target) => Promise<void>
  startTime: vue.Ref<string>
  endTime: vue.Ref<string>
  setTimeRange: (hours: number, shouldLoad?: boolean) => void
  useCustomTimeRange: () => void
  refreshChartsIfVisible: () => void
  refreshDetailIfVisible: () => Promise<void> | void
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

  function createView(t: test.TestContext, testEnv: Record<string, string> = {}) {
    const setInterval = t.mock.fn((_callback: () => void, _interval: number) => 1)
    const clearInterval = t.mock.fn((_id: number) => {})
    const showWarning = t.mock.fn((_message: string) => {})
    const locale = vue.ref('zh')
    let now = Date.parse('2026-09-20T12:00:00Z')
    class ClockDate extends Date {
      constructor(value?: number) { super(value ?? now) }
    }
    const getPing = t.mock.fn(async (): Promise<PingLogData> => sample)
    const document = { visibilityState: 'visible' }
    const dependencies: Record<string, unknown> = {
      vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
      'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
      'element-plus': { ElMessage: { error: () => {}, warning: showWarning } },
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
      '@/utils/timeRange': { isValidTimeRange, normalizeTimeRangeHours },
      '@/utils/refreshInterval': { resolveRefreshInterval },
      '@/utils/preloadAsync': { preloadAsync },
      '@/utils/format': {
        displayName: (value: string) => value === '本机' && locale.value === 'en' ? 'Local' : value,
        formatTime: () => '12:00',
        formatDateTime: (value: Date) => value.toISOString().slice(0, 16).replace('T', ' ')
      }
    }
    const exports: { default?: { setup: (props: object, context: object) => MonitorSetup } } = {}
    runInNewContext(compiled, {
      exports,
      testEnv,
      AbortController,
      setInterval,
      clearInterval,
      Date: ClockDate,
      document,
      console: { error: () => {} },
      require: (id: string) => {
        assert.ok(id in dependencies, `Unexpected dependency: ${id}`)
        return dependencies[id]
      }
    })
    const scope = vue.effectScope()
    t.after(() => scope.stop())
    const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
    view.startTime.value = '2026-09-06 06:00'
    view.endTime.value = '2026-09-06 12:00'
    const targets = (view.pingTargets || view.reverseTargets)!
    targets.value = Array.from({ length: 6 }, (_, index) => ({
      name: '本机',
      fromName: '本机',
      targetIp: `192.0.2.${index + 1}`,
      fromAddr: `192.0.2.${index + 1}`,
      fromPort: 8899,
      loading: false,
      chartData: null
    }))
    return { view, targets, getPing, document, locale, showWarning, warning: showWarning, setInterval, clearInterval,
      advanceTime: (ms: number) => { now += ms } }
  }

  test(`${name}: list and detail refresh timers stay within the configured safe range`, async (t) => {
    const { view, setInterval, clearInterval } = createView(t)
    for (const [minutes, expected] of [
      [undefined, 60_000], [null, 60_000], [NaN, 60_000], [Infinity, 60_000],
      [-1, 60_000], [0, 60_000], [0.5, 60_000], ['5', 60_000],
      [1, 60_000], [5, 300_000], [1440, 86_400_000],
      [1441, 86_400_000], [Number.MAX_VALUE, 86_400_000]
    ] as const) {
      view.config.value = { Base: { Refresh: minutes } }
      view.autoRefresh.value = true
      view.detailAutoRefresh.value = true
      await vue.nextTick()
      const calls = setInterval.mock.calls.slice(-2)
      assert.equal(calls.length, 2)
      assert.deepEqual(calls.map((call) => call.arguments[1]), [expected, expected])
      view.autoRefresh.value = false
      view.detailAutoRefresh.value = false
      await vue.nextTick()
    }
    assert.equal(clearInterval.mock.callCount(), setInterval.mock.callCount())
  })

  test(`${name}: incomplete and reversed detail ranges preserve data without sending requests`, async (t) => {
    const { view, targets, getPing, showWarning } = createView(t)
    await view.showDetail(targets.value[0]!)
    const loaded = view.detailData.value
    view.useCustomTimeRange()
    for (const [start, end] of [
      ['', '2026-09-20 12:00'],
      ['2026-09-20 12:00', ''],
      ['2026-09-20 12:00', '2026-09-20 11:00']
    ]) {
      view.startTime.value = start!
      view.endTime.value = end!
      await view.loadDetailData()
      assert.equal(getPing.mock.callCount(), 1)
      assert.equal(view.detailData.value, loaded)
      assert.equal(view.detailLoading.value, false)
    }
    assert.equal(showWarning.mock.callCount(), 3)
    assert.ok(showWarning.mock.calls.every((call) => call.arguments[0] === 'common.invalidTimeRange'))
    view.startTime.value = '2026-09-20 11:00'
    view.endTime.value = '2026-09-20 12:00'
    await view.loadDetailData()
    assert.equal(getPing.mock.callCount(), 2)
  })

  test(`${name}: open detail title follows language changes without another request`, async (t) => {
    const { view, targets, locale, getPing } = createView(t)
    await view.showDetail(targets.value[0]!)
    assert.ok(view.detailTitle.value.includes('本机'))
    locale.value = 'en'
    assert.ok(view.detailTitle.value.includes('Local'))
    assert.ok(!view.detailTitle.value.includes('本机'))
    locale.value = 'zh'
    assert.ok(view.detailTitle.value.includes('本机'))
    assert.equal(getPing.mock.callCount(), 1)
  })

  test(`${name}: relative detail ranges advance on refresh and custom ranges stay fixed`, async (t) => {
    const { view, targets, getPing, document, advanceTime } = createView(t)
    await view.showDetail(targets.value[0]!)
    const initialEnd = Date.parse(view.endTime.value)
    advanceTime(60_000)
    document.visibilityState = 'hidden'
    view.refreshDetailIfVisible()
    assert.equal(getPing.mock.callCount(), 1)
    assert.equal(Date.parse(view.endTime.value), initialEnd)
    document.visibilityState = 'visible'
    await view.refreshDetailIfVisible()
    assert.equal(getPing.mock.callCount(), 2)
    assert.equal(Date.parse(view.endTime.value), initialEnd + 60_000)
    assert.equal(Date.parse(view.endTime.value) - Date.parse(view.startTime.value), 6 * 3_600_000)

    view.startTime.value = '2026-09-01 00:00'
    view.endTime.value = '2026-09-01 03:00'
    view.useCustomTimeRange()
    advanceTime(60_000)
    await view.refreshDetailIfVisible()
    assert.equal(getPing.mock.callCount(), 3)
    const args = getPing.mock.calls[2]!.arguments as unknown as unknown[]
    // Local requests start with the target IP; proxy requests also have the base URL.
    assert.deepEqual([...args.slice(-3, -1)], [view.startTime.value, view.endTime.value])
    assert.equal(view.endTime.value, '2026-09-01 03:00')

    view.setTimeRange(3, false)
    advanceTime(60_000)
    await view.loadDetailData()
    assert.equal(Date.parse(view.endTime.value), initialEnd + 180_000)
    assert.equal(Date.parse(view.endTime.value) - Date.parse(view.startTime.value), 3 * 3_600_000)
  })

  if (name === 'DashboardView') {
    test('DashboardView: detail queries use a valid configured time range or the six-hour default', async (t) => {
      const cases: Array<[string | undefined, number]> = [
        [undefined, 6], ['', 6], ['invalid', 6], ['Infinity', 6], ['-3', 6],
        ['0', 6], ['0.001', 6], ['745', 6], ['1e300', 6],
        ['0.5', 0.5], ['12', 12], ['744', 744]
      ]
      for (const [configured, expectedHours] of cases) {
        const env: Record<string, string> =
          configured === undefined ? {} : { VITE_DEFAULT_TIME_RANGE: configured }
        const { view, targets, getPing } = createView(t, env)
        await view.showDetail(targets.value[0]!)
        assert.equal(getPing.mock.callCount(), 1, `configuration: ${configured}`)
        const args = getPing.mock.calls[0]!.arguments as unknown as unknown[]
        const start = Date.parse(args[1] as string)
        const end = Date.parse(args[2] as string)
        assert.equal(end - start, expectedHours * 3_600_000, `configuration: ${configured}`)
        assert.deepEqual(view.detailData.value, sample)
      }
    })
  }

  test(`${name}: detail respects valid time ranges and falls back for invalid settings`, async (t) => {
    for (const [input, hours] of [
      ['12', 12], ['0.5', 0.5], ['744', 744], ['-1', 6], ['0', 6],
      ['Infinity', 6], ['NaN', 6], ['745', 6], ['0.001', 6], ['', 6]
    ] as const) {
      const { view, targets } = createView(t, { VITE_DEFAULT_TIME_RANGE: input })
      await view.showDetail(targets.value[0]!)
      const duration = Date.parse(view.endTime.value) - Date.parse(view.startTime.value)
      assert.equal(duration, hours * 60 * 60 * 1000, `configured hours: ${input}`)
    }
  })

  test(`${name}: invalid detail dates retain the chart and pause automatic requests`, async (t) => {
    const { view, targets, getPing, warning } = createView(t)
    await view.showDetail(targets.value[0]!)
    view.useCustomTimeRange()
    const chart = view.detailData.value
    const calls = getPing.mock.callCount()
    for (const [start, end] of [
      ['', '2026-09-06 12:00'], ['2026-09-06 12:00', ''],
      ['2026-09-07 12:00', '2026-09-06 12:00'],
      ['2026-08-01 12:00', '2026-09-06 12:00'],
      ['2026-02-30 12:00', '2026-03-01 12:00']
    ]) {
      view.startTime.value = start!
      view.endTime.value = end!
      await view.loadDetailData()
      const notices = warning.mock.callCount()
      view.refreshDetailIfVisible()
      assert.equal(warning.mock.callCount(), notices)
      assert.equal(getPing.mock.callCount(), calls)
      assert.equal(view.detailData.value, chart)
      assert.equal(view.detailLoading.value, false)
    }
    view.startTime.value = '2026-08-06 12:00'
    view.endTime.value = '2026-09-06 12:00'
    await view.loadDetailData()
    assert.equal(getPing.mock.callCount(), calls + 1)
  })

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

  test(`${name}: automatic refresh skips hidden pages and overlapping requests`, async (t) => {
    const { view, getPing, document } = createView(t)
    view.configLoading.value = false
    document.visibilityState = 'hidden'
    view.refreshChartsIfVisible()
    assert.equal(getPing.mock.callCount(), 0)
    document.visibilityState = 'visible'
    view.configLoading.value = true
    view.refreshChartsIfVisible()
    assert.equal(getPing.mock.callCount(), 0)
    view.configLoading.value = false
    let finish!: () => void
    const gate = new Promise<void>((resolve) => { finish = resolve })
    getPing.mock.mockImplementation(async () => { await gate; return sample })
    const pending = view.loadAllCharts()
    const calls = getPing.mock.callCount()
    assert.ok(calls > 0)
    view.refreshChartsIfVisible()
    assert.equal(getPing.mock.callCount(), calls)
    finish()
    await pending
  })

  test(`${name}: closing detail cancels its request and ignores its late response`, async (t) => {
    const { view, targets, getPing } = createView(t)
    if (view.currentTargetIp) view.currentTargetIp.value = targets.value[0]!.targetIp
    if (view.currentTarget) view.currentTarget.value = targets.value[0]!
    view.detailVisible.value = true
    await vue.nextTick()
    let finish!: () => void
    const gate = new Promise<void>((resolve) => { finish = resolve })
    getPing.mock.mockImplementation(async () => { await gate; return sample })
    const pending = view.loadDetailData()
    assert.equal(view.detailLoading.value, true)
    // Both local and proxy APIs receive the cancellation signal as their final argument.
    const args = getPing.mock.calls[0]!.arguments as unknown as unknown[]
    const signal = args[args.length - 1] as AbortSignal
    view.detailVisible.value = false
    await vue.nextTick()
    assert.equal(signal.aborted, true)
    assert.equal(view.detailLoading.value, false)
    finish()
    await pending
    assert.equal(view.detailData.value, null)
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
