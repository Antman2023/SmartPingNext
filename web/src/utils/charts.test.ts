import assert from 'node:assert/strict'
import test from 'node:test'
import { getPingChartOption, getPingMiniChartOption } from './charts.js'

const labels = {
  maxDelay: 'Max', averageDelay: 'Average', minDelay: 'Min',
  lossRate: 'Loss rate', latency: 'Latency', loss: 'Loss'
}
const data = {
  lastcheck: ['10:00', '10:01', '10:02'], maxdelay: ['0', '-', '0'],
  mindelay: ['0', '-', '0'], avgdelay: ['0', '-', '0'], losspk: ['0', '-', '100']
}

test('ping charts preserve missing samples separately from zero and full loss', () => {
  for (const build of [getPingChartOption, getPingMiniChartOption]) {
    const option = build(data, false, labels)
    const series = option.series as { data: string[]; connectNulls?: boolean }[]
    assert.deepEqual(series[0].data, ['0', '-', '0'])
    assert.deepEqual(series[series.length - 1].data, ['0', '-', '100'])
    assert.ok(series.every((item) => !item.connectNulls))
  }
})

test('ping tooltip renders gaps without NaN and keeps measured values', () => {
  const option = getPingChartOption(data, false, labels)
  const tooltip = option.tooltip as { formatter: (params: unknown) => string }
  const render = (value: unknown, seriesName = labels.averageDelay) => tooltip.formatter([
    { name: '10:01', value, seriesName, marker: '', dataIndex: 1 }
  ])
  for (const missing of ['-', null, undefined, Number.NaN]) {
    assert.equal(render(missing), '10:01<br/>Average: -<br/>')
  }
  assert.equal(render('0'), '10:01<br/>Average: 0.00ms<br/>')
  assert.equal(render('100', labels.lossRate), '10:01<br/>Loss rate: 100%<br/>')
})
