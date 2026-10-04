import assert from 'node:assert/strict'
import test from 'node:test'
import { isPingLogData } from './pingData.js'

const dataFor = (lastcheck: readonly string[]) => ({
  lastcheck,
  maxdelay: lastcheck.map(() => '0'),
  mindelay: lastcheck.map(() => '-'),
  avgdelay: lastcheck.map(() => '.5'),
  losspk: lastcheck.map(() => '100')
})

test('ping minute labels validate Gregorian dates without the browser timezone', () => {
  for (const time of ['2026-09-20 00:00', '2026-09-20 23:59', '2024-02-29 12:00',
    '2000-02-29 12:00', '0000-02-29 00:00', '0001-01-01 00:00', '0099-12-31 23:59',
    '-0001-01-01 00:00', '-0400-02-29 12:00', '9999-12-31 23:59',
    '2011-03-13 02:30', '2011-12-30 12:00']) {
    assert.equal(isPingLogData(dataFor([time])), true, time)
  }
  for (const time of ['2025-02-29 12:00', '1900-02-29 12:00', '-0100-02-29 12:00',
    '2026-02-30 12:00', '2026-04-31 12:00', '2026-00-01 12:00', '2026-13-01 12:00',
    '2026-01-00 12:00', '2026-01-32 12:00', '2026-09-20 24:00', '2026-09-20 12:60']) {
    assert.equal(isPingLogData(dataFor(['2026-09-20 12:00', time])), false, time)
  }
})

test('ping minute labels require the complete node output format', () => {
  for (const time of ['', 'tomorrow', '12:00', '2026-09-20', '2026-9-20 12:00',
    '2026-09-20 1:00', '2026-09-20 12:0', '2026-09-20T12:00', '2026-09-20  12:00',
    '2026-09-20 12:00:00', '2026-09-20 12:00Z', '2026-09-20 12:00+08:00',
    '2026-09-20 12:00.5', '+2026-09-20 12:00', '10000-01-01 12:00',
    ' 2026-09-20 12:00', '2026-09-20 12:00 ', '2026-09-20 12:00\n',
    '2026-09-20 12:00\r', '2026-09-20 12:00\u2028', '2026-09-20 12:00 garbage']) {
    assert.equal(isPingLogData(dataFor([time])), false, JSON.stringify(time))
  }
})

test('ping validation preserves empty, duplicate and rollback timelines with their metric positions', () => {
  for (const timeline of [[], ['2011-11-06 01:59', '2011-11-06 01:00', '2011-11-06 01:59'],
    ['2026-09-20 12:00', '2026-09-20 12:00']]) {
    const data = dataFor(Object.freeze(timeline))
    data.avgdelay = timeline.map((_label, index) => String(index))
    for (const values of Object.values(data)) Object.freeze(values)
    Object.freeze(data)
    const before = JSON.stringify(data)
    assert.equal(isPingLogData(data), true)
    assert.equal(JSON.stringify(data), before)
    assert.equal(data.lastcheck, timeline)
  }
})

test('ping labels and numeric samples reject long malformed values and trailing line terminators', () => {
  const valid = dataFor(['2026-09-20 12:00'])
  for (const label of ['9'.repeat(65536), '2026-09-20 12:00' + ' '.repeat(65536)]) {
    assert.equal(isPingLogData({ ...valid, lastcheck: [label] }), false)
  }
  for (const key of ['maxdelay', 'mindelay', 'avgdelay', 'losspk']) {
    for (const ending of ['\n', '\r', '\r\n', '\u2028', '\u2029']) {
      assert.equal(isPingLogData({ ...valid, [key]: ['1' + ending] }), false)
    }
    assert.equal(isPingLogData({ ...valid, [key]: ['9'.repeat(65536) + 'x'] }), false)
    assert.equal(isPingLogData({ ...valid, [key]: ['9'.repeat(65536)] }), false)
    assert.equal(isPingLogData({ ...valid, [key]: ['0'.repeat(65536)] }), true)
  }
})
