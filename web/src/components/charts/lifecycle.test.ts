import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

interface ChartSetup {
  chartRef: vue.Ref<object | undefined>
  resizeTimers: Set<number>
  safeSetTimeout: (callback: () => void, delay: number) => number
  handleResize: (() => void) & { cancel: () => void }
}

for (const component of ['PingChart', 'PingMiniChart', 'TopologyGraph']) {
  test(`${component}: completed timers are released and pending work is canceled on unmount`, async (t) => {
    const timers = new Map<number, () => void>()
    let timerId = 0
    const window = {
      setTimeout: (callback: () => void) => {
        timers.set(++timerId, callback)
        return timerId
      },
      clearTimeout: (id: number) => timers.delete(id),
      addEventListener: t.mock.fn(),
      removeEventListener: t.mock.fn()
    }
    const transpile = (source: string) => ts.transpileModule(source, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
    }).outputText
    const debounceExports = {}
    runInNewContext(transpile(readFileSync(new URL('../../../src/utils/debounce.ts', import.meta.url), 'utf8')), {
      exports: debounceExports, window
    })
    const source = readFileSync(new URL(`../../../src/components/charts/${component}.vue`, import.meta.url), 'utf8')
    const { descriptor } = parse(source)
    let mounted!: () => void | Promise<void>
    let unmounted!: () => void
    const chart = { setOption: t.mock.fn(), resize: t.mock.fn(), dispose: t.mock.fn() }
    const init = t.mock.fn(() => chart)
    const dependencies: Record<string, unknown> = {
      vue: { ...vue, onMounted: (fn: typeof mounted) => { mounted = fn }, onUnmounted: (fn: typeof unmounted) => { unmounted = fn } },
      'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale: vue.ref('zh-CN') }) },
      '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
      '@/stores/sidebar': { useSidebarStore: () => ({ isCollapsed: false }) },
      '@/utils/charts': { getPingChartOption: () => ({}), getPingMiniChartOption: () => ({}) },
      '@/utils/debounce': debounceExports,
      '@/utils/echartsLine': { echarts: { init } },
      '@/utils/echartsGraph': { echarts: { init } }
    }
    const exports: { default?: { setup: (props: object, context: object) => ChartSetup } } = {}
    runInNewContext(transpile(compileScript(descriptor, { id: component }).content), {
      exports, window,
      require: (name: string) => {
        assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
        return dependencies[name]
      }
    })
    const scope = vue.effectScope()
    t.after(() => scope.stop())
    const props = vue.reactive({ data: null, nodes: [], links: [], symbolSize: 50, lineWidth: 2 })
    const view = scope.run(() => exports.default!.setup(props, { expose: () => {} }))!
    view.chartRef.value = {}
    await mounted()
    if (component === 'TopologyGraph') {
      const calls = chart.setOption.mock.callCount()
      props.symbolSize = 80
      props.lineWidth = 4
      await vue.nextTick()
      assert.equal(chart.setOption.mock.callCount(), calls + 1)
      const option = chart.setOption.mock.calls.at(-1)!.arguments[0] as {
        series: { symbolSize: number; lineStyle: { width: number }; left: number }[]
      }
      assert.equal(option.series[0]!.symbolSize, 80)
      assert.equal(option.series[0]!.lineStyle.width, 4)
      assert.equal(option.series[0]!.left, 52)
      assert.equal(init.mock.callCount(), 1, 'style updates should reuse the chart instance')
    }
    const flushTimers = () => {
      for (const [id, callback] of Array.from(timers)) {
        timers.delete(id)
        callback()
      }
    }
    flushTimers()
    for (let i = 0; i < 100; i++) {
      view.safeSetTimeout(() => {}, 0)
      flushTimers()
    }
    assert.equal(view.resizeTimers.size, 0, 'completed timers must not accumulate')
    const callback = t.mock.fn()
    view.safeSetTimeout(callback, 500)
    view.handleResize()
    assert.equal(timers.size, 2)
    unmounted()
    assert.equal(view.resizeTimers.size, 0)
    assert.equal(timers.size, 0, 'unmount must also cancel the debounce timer')
    flushTimers()
    assert.equal(callback.mock.callCount(), 0)
    assert.equal(chart.dispose.mock.callCount(), 1)

    if (component === 'PingChart') {
      // Its async mount must not attach a listener after the component was destroyed.
      const calls = window.addEventListener.mock.callCount()
      const nextView = scope.run(() => exports.default!.setup({ data: null }, { expose: () => {} }))!
      nextView.chartRef.value = {}
      const pendingMount = mounted()
      unmounted()
      await pendingMount
      assert.equal(window.addEventListener.mock.callCount(), calls)
      assert.equal(init.mock.callCount(), 1)
    }
  })
}
