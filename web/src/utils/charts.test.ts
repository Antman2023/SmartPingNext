import assert from 'node:assert/strict'
import test from 'node:test'
import { getPingChartOption, getPingMiniChartOption } from './charts.js'
import { formatMappingTooltip } from './chartTooltip.js'

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
  for (const missing of ['-', '', ' ', null, undefined, Number.NaN, '12invalid', false]) {
    assert.equal(render(missing), '10:01<br/>Average: -<br/>')
  }
  assert.equal(render('0'), '10:01<br/>Average: 0.00ms<br/>')
  assert.equal(render('100', labels.lossRate), '10:01<br/>Loss rate: 100%<br/>')
})

test('ping tooltip escapes text while retaining the chart-generated marker', () => {
  const option = getPingChartOption(data, false, labels)
  const tooltip = option.tooltip as { formatter: (params: unknown) => string }
  const result = tooltip.formatter([{
    name: '<img src=x onerror="alert(1)">', value: 0,
    seriesName: "A&B's <label>", marker: '<span></span>', dataIndex: 0
  }])
  assert.equal(result, '&lt;img src=x onerror=&quot;alert(1)&quot;&gt;<br/><span></span>A&amp;B&#39;s &lt;label&gt;: 0.00ms<br/>')
})

test('map tooltip distinguishes missing values from zero and escapes names', () => {
  for (const value of [null, undefined, '', ' ', '-', false, [], Number.NaN, Infinity, '12invalid']) {
    assert.equal(formatMappingTooltip({ name: '广东', seriesName: '电信', value }), '广东<br/>电信: --')
  }
  for (const value of [0, '0']) {
    assert.equal(formatMappingTooltip({ name: '广东', seriesName: '电信', value }), '广东<br/>电信: 0.00ms')
  }
  assert.equal(formatMappingTooltip({ name: '<b>广东</b>', seriesName: 'A&B', value: '12.5' }), '&lt;b&gt;广东&lt;/b&gt;<br/>A&amp;B: 12.50ms')
})

test('ping axes retain minute labels across signed years and redraws without changing sample order', () => {
  const timestamps = Object.freeze(['-0001-12-31 23:58', '-0001-12-31 23:59',
    '0000-01-01 00:00', '0000-01-01 00:01'])
  const history = { ...data, lastcheck: [...timestamps], maxdelay: ['0', '-', '1', '2'],
    mindelay: ['0', '-', '1', '2'], avgdelay: ['0', '-', '1', '2'], losspk: ['0', '-', '100', '0'] }
  const option = getPingChartOption(history, false, labels)
  const axis = option.xAxis as { data: string[]; axisLabel: { formatter: (value: string, index: number) => string } }
  const render = axis.axisLabel.formatter
  assert.equal(axis.data, history.lastcheck)
  assert.equal(render(timestamps[0]!, 0), '{date|12-31}\n{time|23:58}')
  assert.equal(render(timestamps[1]!, 1), '{time|23:59}')
  assert.equal(render(timestamps[2]!, 2), '{date|01-01}\n{time|00:00}')
  assert.equal(render(timestamps[3]!, 3), '{time|00:01}')
  // A zoom or redraw can restart formatter indices partway through the input.
  assert.equal(render(timestamps[1]!, 0), '{date|12-31}\n{time|23:59}')
  assert.equal(render(timestamps[2]!, 2), '{date|01-01}\n{time|00:00}')

  const mini = getPingMiniChartOption(history, true, labels).xAxis as typeof axis
  assert.equal(mini.data, history.lastcheck)
  assert.deepEqual(timestamps.map((value, index) => mini.axisLabel.formatter(value, index)),
    ['23:58', '23:59', '00:00', '00:01'])
  assert.deepEqual(history.lastcheck, [...timestamps])
  assert.equal(render('10:00', 0), '{time|10:00}')
  assert.equal(mini.axisLabel.formatter('10:00', 0), '10:00')
  assert.equal(render('', 0), '')
  assert.equal(mini.axisLabel.formatter('', 0), '')
})
