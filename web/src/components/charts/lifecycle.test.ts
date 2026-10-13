import { preserveLegendSelection } from '../../utils/chartInteraction.js'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import type { PingLogData } from '../../types/index.js'

interface ChartSetup {
  chartRef: vue.Ref<object | undefined>
  resizeTimers: Set<number>
  safeSetTimeout: (callback: () => void, delay: number) => number
  handleResize: (() => void) & { cancel: () => void }
  saveAsImage: () => void
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
    const getDataURL = t.mock.fn((_options: unknown) => 'data:image/png;base64,test')
    const link = { download: '', href: '', click: t.mock.fn() }
    const document = { createElement: () => link }
    const chartExport = {}
    runInNewContext(transpile(readFileSync(new URL('../../../src/utils/chartExport.ts', import.meta.url), 'utf8')), {
      exports: chartExport, document
    })
    const messageError = t.mock.fn((_message: string) => {})
    const chart = { getOption: () => ({}), setOption: t.mock.fn(), resize: t.mock.fn(), dispose: t.mock.fn(), getDataURL }
    const init = t.mock.fn(() => chart)
    const locale = vue.ref('zh-CN')
    const theme = vue.reactive({ theme: 'light' })
    const dependencies: Record<string, unknown> = {
      vue: { ...vue, onMounted: (fn: typeof mounted) => { mounted = fn }, onUnmounted: (fn: typeof unmounted) => { unmounted = fn } },
      'vue-i18n': { useI18n: () => ({ t: (key: string) => key, locale }) },
      'element-plus': { ElMessage: { error: messageError } },
      '@/stores/theme': { useThemeStore: () => theme },
      '@/stores/sidebar': { useSidebarStore: () => ({ isCollapsed: false }) },
      '@/utils/chartInteraction': { preserveLegendSelection },
      '@/utils/chartExport': chartExport,
      '@/utils/charts': { getPingChartOption: () => ({}), getPingMiniChartOption: () => ({}) },
      '@/utils/debounce': debounceExports,
      '@/utils/echartsLine': { echarts: { init } },
      '@/utils/echartsGraph': { echarts: { init } }
    }
    const exports: { default?: { setup: (props: object, context: object) => ChartSetup } } = {}
    runInNewContext(transpile(compileScript(descriptor, { id: component }).content), {
      exports, window, document, console: { error: () => {} },
      require: (name: string) => {
        assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
        return dependencies[name]
      }
    })
    const scope = vue.effectScope()
    t.after(() => scope.stop())
    const props = vue.reactive({ data: null as PingLogData | null, height: 200,
      nodes: [{ id: 'a', name: 'Alpha', color: 'green' }, { id: 'b', name: 'Beta', color: 'green' }],
      links: [{ source: 'a', target: 'b', color: 'green', curveness: 0 }], symbolSize: 50, lineWidth: 2 })
    const view = scope.run(() => exports.default!.setup(props, { expose: () => {} }))!
    view.chartRef.value = {}
    await mounted()
    if (component !== 'TopologyGraph') {
      props.data = { lastcheck: ['2026-10-13 12:00'], maxdelay: ['3'],
        mindelay: ['1'], avgdelay: ['2'], losspk: ['0'] }
      await vue.nextTick()
      const axis = props.data.lastcheck
      for (const metric of ['maxdelay', 'mindelay', 'avgdelay', 'losspk'] as const) {
        const calls = chart.setOption.mock.callCount()
        props.data[metric] = ['5']
        await vue.nextTick()
        assert.equal(chart.setOption.mock.callCount(), calls + 1, `${metric} must refresh with an unchanged axis`)
        assert.equal(props.data.lastcheck, axis)
      }
      const calls = chart.setOption.mock.callCount()
      props.data.avgdelay = ['4']
      props.data.losspk = ['100']
      theme.theme = 'dark'
      locale.value = 'en-US'
      await vue.nextTick()
      assert.equal(chart.setOption.mock.callCount(), calls + 1, 'simultaneous changes should draw once')
      const resizes = chart.resize.mock.callCount()
      props.height = 400
      await vue.nextTick()
      assert.equal(chart.resize.mock.callCount(), resizes + 1, 'height changes must resize the canvas')
      assert.equal(init.mock.callCount(), 1)
    }
    if (component === 'TopologyGraph') {
      const calls = chart.setOption.mock.callCount()
      props.symbolSize = 80
      props.lineWidth = 4
      props.nodes[0]!.color = 'red'
      props.links[0]!.color = 'red'
      theme.theme = 'dark'
      await vue.nextTick()
      assert.equal(chart.setOption.mock.callCount(), calls + 1, 'data, style and theme changes should draw once')
      const option = chart.setOption.mock.calls.at(-1)!.arguments[0] as {
        series: { symbolSize: number; lineStyle: { width: number }; left: number;
          label: { color: string }; data: { itemStyle: { color: string } }[];
          links: { lineStyle: { color: string } }[] }[]
      }
      assert.equal(option.series[0]!.symbolSize, 80)
      assert.equal(option.series[0]!.lineStyle.width, 4)
      assert.equal(option.series[0]!.left, 52)
      assert.equal(option.series[0]!.data[0]!.itemStyle.color, 'red')
      assert.equal(option.series[0]!.links[0]!.lineStyle.color, 'red')
      assert.equal(option.series[0]!.label.color, '#e5eaf3')
      assert.equal(init.mock.callCount(), 1, 'style updates should reuse the chart instance')
      const resizes = chart.resize.mock.callCount()
      const draws = chart.setOption.mock.callCount()
      props.height = 600
      await vue.nextTick()
      assert.equal(chart.resize.mock.callCount(), resizes + 1, 'height changes must resize the topology canvas')
      assert.equal(chart.setOption.mock.callCount(), draws, 'resizing should not rebuild graph data')
      props.links[0]!.color = 'orange'
      await vue.nextTick()
      assert.equal(chart.setOption.mock.callCount(), draws + 1, 'in-place link changes must remain observable')
      const latest = chart.setOption.mock.calls.at(-1)!.arguments[0] as typeof option
      assert.equal(latest.series[0]!.links[0]!.lineStyle.color, 'orange')
    }
    const flushTimers = () => {
      for (const [id, callback] of Array.from(timers)) {
        timers.delete(id)
        callback()
      }
    }
    if (component === 'PingChart') {
      await t.test('image export reports failures and retries without replacing or redrawing the chart', () => {
        const draws = chart.setOption.mock.callCount()
        for (const failure of ['encode', '', 'data:,', 'data:image/png;base64,', 'download']) {
          const messages = messageError.mock.callCount()
          const clicks = link.click.mock.callCount()
          if (failure === 'encode') {
            getDataURL.mock.mockImplementationOnce(() => { throw new Error('encode failed') })
          } else if (failure === 'download') {
            link.click.mock.mockImplementationOnce(() => { throw new Error('download failed') })
          } else {
            getDataURL.mock.mockImplementationOnce(() => failure)
          }
          assert.doesNotThrow(view.saveAsImage)
          assert.equal(messageError.mock.callCount(), messages + 1)
          assert.equal(messageError.mock.calls.at(-1)!.arguments[0], 'common.saveImageFailed')
          assert.equal(link.click.mock.callCount(), clicks + (failure === 'download' ? 1 : 0))
          view.saveAsImage()
          assert.equal(messageError.mock.callCount(), messages + 1)
          assert.equal(link.click.mock.callCount(), clicks + (failure === 'download' ? 2 : 1))
          assert.equal(link.href, 'data:image/png;base64,test')
          assert.match(link.download, /^smartping-chart-\d+\.png$/)
        }
        const options = getDataURL.mock.calls.at(-1)!.arguments[0] as { type: string; pixelRatio: number; backgroundColor: string }
        assert.equal(options.type, 'png')
        assert.equal(options.pixelRatio, 2)
        assert.equal(options.backgroundColor, '#1a1a1a')
        assert.equal(chart.setOption.mock.callCount(), draws)
        assert.equal(chart.dispose.mock.callCount(), 0)
      })
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
      const exportCalls = getDataURL.mock.callCount()
      view.saveAsImage()
      assert.equal(getDataURL.mock.callCount(), exportCalls, 'late export must not use a disposed chart')
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
