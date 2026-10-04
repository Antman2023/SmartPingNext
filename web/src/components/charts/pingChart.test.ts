import { preserveLegendSelection } from '../../utils/chartInteraction.js'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { SVGRenderer } from 'echarts/renderers'
import { echarts } from '../../utils/echartsLine.js'
import { getPingChartOption, getPingMiniChartOption } from '../../utils/charts.js'

echarts.use(SVGRenderer)

test('detail and mini charts render signed-year minute labels in the actual SVG axis', () => {
  const labels = { maxDelay: 'Max', averageDelay: 'Average', minDelay: 'Min',
    lossRate: 'Loss rate', latency: 'Latency', loss: 'Loss' }
  const history = { lastcheck: ['-0001-12-31 23:59', '0000-01-01 00:00'],
    maxdelay: ['0', '1'], mindelay: ['0', '1'], avgdelay: ['0', '1'], losspk: ['0', '100'] }
  for (const build of [getPingChartOption, getPingMiniChartOption]) {
    const chart = echarts.init(null, undefined, { renderer: 'svg', ssr: true, width: 640, height: 400 })
    try {
      chart.setOption(build(history, false, labels))
      chart.renderToSVGString()
      const texts = chart.getZr().storage.getDisplayList()
        .filter((element) => element.type === 'tspan')
        .map((element) => (element.style as { text?: string }).text)
      assert.ok(texts.includes('23:59'), `missing 23:59 in rendered labels: ${JSON.stringify(texts)}`)
      assert.ok(texts.includes('00:00'), `missing 00:00 in rendered labels: ${JSON.stringify(texts)}`)
      if (build === getPingChartOption) {
        assert.ok(texts.includes('12-31'))
        assert.ok(texts.includes('01-01'))
      }
    } finally {
      chart.dispose()
    }
  }
})

test('detail chart keeps numeric axis labels inside narrow and wide canvases', () => {
  const labels = {
    maxDelay: 'Max', averageDelay: 'Average', minDelay: 'Min',
    lossRate: 'Loss rate', latency: 'Latency', loss: 'Loss'
  }
  const data = {
    lastcheck: ['2026-09-20 12:00', '2026-09-20 12:01'],
    maxdelay: ['60000', '60000'], mindelay: ['0', '0'],
    avgdelay: ['50000', '60000'], losspk: ['0', '100']
  }
  for (const width of [320, 960]) {
    for (const dark of [false, true]) {
      const chart = echarts.init(null, undefined, { renderer: 'svg', ssr: true, width, height: 400 })
      try {
        chart.setOption(getPingChartOption(data, dark, labels, true, width < 480))
        chart.renderToSVGString()
        let numericLabels = 0
        for (const element of chart.getZr().storage.getDisplayList()) {
          if (element.type !== 'tspan') continue
          const text = (element.style as { text?: unknown }).text
          if (typeof text !== 'string' || !/^[\d,.]+%?$/.test(text)) continue
          numericLabels++
          const bounds = element.getBoundingRect().clone()
          if (element.transform) bounds.applyTransform(element.transform)
          assert.ok(bounds.x >= -1 && bounds.x + bounds.width <= width + 1,
            `label ${text} exceeds width ${width}: ${bounds.x}, ${bounds.width}`)
          assert.ok(bounds.y >= -1 && bounds.y + bounds.height <= 401,
            `label ${text} exceeds chart height`)
        }
        assert.ok(numericLabels >= 6)
      } finally {
        chart.dispose()
      }
    }
  }
})

test('detail chart preserves zoom and legend choices through updates, resizing and translation', async (t) => {
  const chart = echarts.init(null, undefined, { renderer: 'svg', ssr: true, width: 640, height: 400 })
  t.after(() => chart.dispose())
  const source = readFileSync(new URL('../../../src/components/charts/PingChart.vue', import.meta.url), 'utf8')
  const { descriptor } = parse(source)
  const compiled = ts.transpileModule(compileScript(descriptor, { id: 'ping-chart' }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const locale = vue.ref('en')
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => locale.value === 'en' ? key : `zh:${key}`, locale }) },
    '@/stores/theme': { useThemeStore: () => ({ theme: 'light' }) },
    '@/stores/sidebar': { useSidebarStore: () => ({ isCollapsed: false }) },
    '@/utils/chartInteraction': { preserveLegendSelection },
    '@/utils/charts': { getPingChartOption },
    '@/utils/debounce': { debounce: (callback: () => void) => callback },
    '@/utils/echartsLine': { echarts: { init: () => chart } }
  }
  interface Setup {
    chartRef: vue.Ref<{ clientWidth: number }>
    initChart: () => void
    updateChart: () => void
    handleResize: () => void
  }
  const exports: { default?: { setup: (props: object, context: object) => Setup } } = {}
  runInNewContext(compiled, {
    exports,
    window: { setTimeout: () => 1 },
    require: (id: string) => {
      assert.ok(id in dependencies, `Unexpected dependency: ${id}`)
      return dependencies[id]
    }
  })
  const props = {
    data: {
      lastcheck: ['2026-09-20 12:00', '2026-09-20 12:01', '2026-09-20 12:02'],
      maxdelay: ['5', '6', '7'], mindelay: ['1', '2', '3'],
      avgdelay: ['3', '4', '5'], losspk: ['0', '0', '0']
    }
  }
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup(props, { expose: () => {} }))!
  view.chartRef.value = { clientWidth: 640 }
  view.initChart()
  chart.dispatchAction({ type: 'legendSelect', name: 'charts.maxDelay' })
  chart.dispatchAction({ type: 'legendUnSelect', name: 'charts.averageDelay' })
  chart.dispatchAction({ type: 'dataZoom', start: 25, end: 75 })
  props.data.avgdelay = ['8', '9', '10']
  for (const update of [view.updateChart, view.handleResize]) {
    view.chartRef.value.clientWidth = 320
    update()
    const option = chart.getOption() as {
      legend: Array<{ selected: Record<string, boolean>; type: string }>
      dataZoom: Array<{ start: number; end: number }>
      series: Array<{ data: string[] }>
    }
    assert.equal(option.legend[0]!.selected['charts.maxDelay'], true)
    assert.equal(option.legend[0]!.selected['charts.averageDelay'], false)
    assert.equal(option.legend[0]!.type, 'scroll')
    assert.equal(option.dataZoom[0]!.start, 25)
    assert.equal(option.dataZoom[0]!.end, 75)
    assert.deepEqual(option.series[1]!.data, ['8', '9', '10'])
  }
  for (const language of ['zh', 'en', 'zh']) {
    locale.value = language
    await setImmediate()
    const prefix = language === 'en' ? '' : 'zh:'
    const option = chart.getOption() as {
      legend: Array<{ selected: Record<string, boolean>; data: string[] }>
      dataZoom: Array<{ start: number; end: number }>
      series: Array<{ id: string; name: string }>
    }
    assert.equal(option.series.length, 4)
    assert.equal(option.series[0]!.name, `${prefix}charts.maxDelay`)
    assert.equal(option.legend[0]!.selected[`${prefix}charts.maxDelay`], true)
    assert.equal(option.legend[0]!.selected[`${prefix}charts.averageDelay`], false)
    assert.equal(option.legend[0]!.selected[`${prefix}charts.minDelay`], false)
    assert.equal(option.legend[0]!.data.length, 4)
    assert.ok(option.legend[0]!.data.every((label) => label.startsWith(`${prefix}charts.`)))
    assert.equal(option.dataZoom[0]!.start, 25)
    assert.equal(option.dataZoom[0]!.end, 75)
  }
})
