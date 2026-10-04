import assert from 'node:assert/strict'
import test from 'node:test'
import { isAlertData, sortAlertDates, sortAlertRecords } from './alertData.js'

const log = {
  Logtime: '2026-09-06 12:00', Targetip: '192.0.2.1', Targetname: '目标',
  Tracert: '', Fromip: '192.0.2.2', Fromname: '来源'
}

test('alert dates must be real calendar labels rather than arbitrary strings', () => {
  for (const date of ['2026-09-06', '2024-02-29', '2000-02-29', '0000-02-29',
    '0001-01-01', '0099-12-31', '-0001-01-01', '-0400-02-29', '9999-12-31']) {
    assert.equal(isAlertData({ dates: [date], logs: [] }), true, date)
  }
  for (const date of ['', 'tomorrow', '2026-2-03', '2026-02-30', '2025-02-29',
    '1900-02-29', '-0100-02-29', '2026-04-31', '2026-00-01', '2026-13-01',
    '2026-01-00', '2026-01-32', '+2026-09-06', '10000-01-01',
    '2026-09-06 ', '2026-09-06\n', '2026-09-06 12:00', '2026-09-06T00:00Z']) {
    assert.equal(isAlertData({ dates: ['2026-09-06', date], logs: [] }), false, date)
  }
})

test('alert records preserve civil timestamps and compatible stored time formats', () => {
  for (const time of ['2026-09-06 00:00', '2026-09-06 23:59', '2026-09-06',
    '2026-09-06 12:00:59', '2026-09-06T12:00:00.123456Z',
    '2026-09-06T12:00:00.1z', '2026-09-06 12:00-08:00',
    '2026-09-06  12:00 +14:59  ', '2026-09-06 24:00', '2026-09-06 24:59:59',
    '2024-02-29 12:00', '0000-02-29 00:00', '0001-01-01 00:00', '-0001-01-01 00:00',
    // These node-local labels must not be normalized in the browser's timezone.
    '2011-11-06 01:30', '2011-03-13 02:30', '2011-12-30 12:00']) {
    const record = { ...log, Logtime: time, Tracert: 'legacy trace failed' }
    const data = { dates: ['2026-09-06'], logs: [record] }
    const before = JSON.stringify(data)
    assert.equal(isAlertData(data), true, time)
    assert.equal(data.logs[0], record)
    assert.equal(JSON.stringify(data), before, 'validation must not rewrite stored labels')
  }
  assert.equal(isAlertData({ dates: [], logs: [] }), true)
  assert.equal(isAlertData({ dates: [], logs: [log] }), true, 'older responses need not list every log day')
})

test('alert records reject impossible dates, invalid time fields and timestamp suffixes', () => {
  for (const time of ['', 'tomorrow', '2026-02-30 12:00', '1900-02-29 00:00',
    '2026-09-06 25:00', '2026-09-06 12:60', '2026-09-06 12:00:60',
    '2026-09-06 1:00', '2026-09-06 12:0', '2026-09-06 12:00.5',
    '2026-09-06 12:00:00.', '2026-09-06 12:00+15:00', '2026-09-06 12:00+01:60',
    '2026-09-06 12:00+0800', '2026-09-06 12:00Zjunk',
    '2026-09-06 12:00:00NaN', '2026-09-06 12:00\u0000junk']) {
    assert.equal(isAlertData({ dates: ['2026-09-06'], logs: [log, { ...log, Logtime: time }] }), false, time)
  }
})

test('long timestamp whitespace and fractions preserve valid labels and reject trailing garbage', () => {
  const spaces = ' '.repeat(64 * 1024)
  const fraction = '1'.repeat(64 * 1024)
  for (const time of [`2026-09-06 12:00${spaces}`, `2026-09-06 12:00${spaces}+08:00`,
    `2026-09-06T12:00:00.${fraction}Z`]) {
    assert.equal(isAlertData({ dates: [], logs: [{ ...log, Logtime: time }] }), true)
    assert.equal(isAlertData({ dates: [], logs: [{ ...log, Logtime: `${time}!` }] }), false)
  }
})

test('alert sorting uses civil time fields across stored formats without changing records', () => {
  const cases = [
    ['noon T', '2026-09-06T12:00'], ['afternoon', '2026-09-06 13:00'],
    ['early whitespace', '2026-09-06  01:00'], ['late whitespace', '2026-09-06  14:00'],
    ['fraction 1', '2026-09-06 12:00:00.1'], ['fraction 09', '2026-09-06T12:00:00.09Z'],
    ['fraction same', '2026-09-06 12:00:00.1000+08:00'],
    ['same second', '2026-09-06 12:00:00.000Z'],
    ['day only', '2026-09-06'], ['midnight', '2026-09-06 00:00'],
    ['offset morning', '2026-09-06 11:00-08:00']
  ]
  const records = Object.freeze(cases.map(([name, time]) => Object.freeze({
    ...log, Targetname: name!, Logtime: time!, refreshFailed: name === 'afternoon'
  })))
  const sorted = sortAlertRecords(records)
  assert.deepEqual(sorted.map((record) => record.Targetname), [
    'late whitespace', 'afternoon', 'fraction 1', 'fraction same', 'fraction 09',
    'noon T', 'same second', 'offset morning', 'early whitespace', 'day only', 'midnight'
  ])
  assert.equal(sorted.find((record) => record.Targetname === 'afternoon')!.refreshFailed, true)
  assert.ok(sorted.every((record) => records.includes(record)), 'keep the original record objects')
  assert.deepEqual(records.map((record) => record.Targetname), cases.map(([name]) => name))
})

test('alert sorting preserves early years, rollover times and submillisecond ordering', () => {
  const records = [
    ['next day', '2026-09-07 00:30'], ['rollover', '2026-09-06 24:59:59'],
    ['midnight', '2026-09-07 00:00'], ['24 hours', '2026-09-06 24:00'],
    ['last century', '0099-12-31 24:00'], ['century start', '0100-01-01 00:00'],
    ['year zero', '0000-02-29 00:00'], ['negative one', '-0001-01-01 00:00'],
    ['negative two', '-0002-01-01 00:00'],
    ['precise low', '2026-09-06 12:00:00.000000000000000001'],
    ['precise high', '2026-09-06 12:00:00.000000000000000002']
  ].map(([name, time]) => ({ ...log, Targetname: name!, Logtime: time! }))
  assert.deepEqual(sortAlertRecords(records).map((record) => record.Targetname), [
    'rollover', 'next day', 'midnight', '24 hours', 'precise high', 'precise low',
    'last century', 'century start', 'year zero', 'negative one', 'negative two'
  ])
  const dates = Object.freeze(['-0002-01-01', '0000-02-29', '-0001-01-01',
    '0099-12-31', '0100-01-01', '2026-09-06'])
  assert.deepEqual(sortAlertDates(dates), [
    '2026-09-06', '0100-01-01', '0099-12-31', '0000-02-29', '-0001-01-01', '-0002-01-01'
  ])
  const zeros = '0'.repeat(64 * 1024)
  const precise = ['1000', '2', '1'].map((fraction) => ({
    ...log, Logtime: `2026-09-06 12:00:00.${zeros}${fraction}`
  }))
  assert.deepEqual(sortAlertRecords(precise), [precise[1], precise[0], precise[2]])
})
