import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import type { PingLogData } from '../types/index.js'
import { isPingLogData } from '../utils/pingData.js'

const source = readFileSync(new URL('../../src/api/ping.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

function createApi(response: unknown) {
  const get = async () => response
  const exports = {} as {
    getPingData: (ip: string) => Promise<PingLogData>
    getProxyPingData: (url: string, ip: string) => Promise<PingLogData>
  }
  runInNewContext(compiled, {
    exports,
    URL,
    URLSearchParams,
    require: (name: string) => {
      if (name === './index') return { default: { get }, proxyRequestConfig: () => ({}) }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      if (name === '@/utils/pingData') return { isPingLogData }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  return [
    () => exports.getPingData('192.0.2.1'),
    () => exports.getProxyPingData('http://192.0.2.2:8899', '192.0.2.1')
  ]
}

const sample: PingLogData = {
  lastcheck: ['2026-09-20 12:00', '2026-09-20 12:01'],
  maxdelay: ['0', '-'],
  mindelay: ['0', '-'],
  avgdelay: ['0', '-'],
  losspk: ['100', '-']
}

test('ping APIs preserve empty timelines, real zeroes and missing samples', async () => {
  for (const data of [
    sample,
    { ...sample, maxdelay: ['6e4', '-'], mindelay: ['.5', '-'], avgdelay: ['+1.5', '-'], losspk: ['1e2', '0.5'] },
    { lastcheck: [], maxdelay: [], mindelay: [], avgdelay: [], losspk: [] }
  ]) {
    for (const request of createApi(data)) assert.equal(await request(), data)
  }
})

test('ping APIs reject malformed data before it reaches any chart', async () => {
  const invalid = [
    null,
    {},
    { ...sample, lastcheck: [42, null] },
    ...['maxdelay', 'mindelay', 'avgdelay', 'losspk'].flatMap((key) =>
      [
        null,
        [],
        ['1'],
        ['1', '2', '3'],
        [0, '-'],
        ['', '-'],
        ['NaN', '-'],
        ['Infinity', '-'],
        ['1e999', '-'],
        ['10ms', '-'],
        ['0x10', '-'],
        ['-1', '-'],
        ['-0.5', '-'],
        ['-1e2', '-'],
        ...['\n', '\r', '\r\n', '\u2028', '\u2029'].map((ending) => ['1' + ending, '-'])
      ].map((values) => ({ ...sample, [key]: values }))
    ),
    { ...sample, losspk: ['101', '-'] },
    { ...sample, losspk: ['100.01', '-'] },
    { ...sample, losspk: ['1e3', '-'] }
  ]
  for (const data of invalid) {
    for (const request of createApi(data))
      await assert.rejects(request, /common.invalidPingResponse/)
  }
})

test('local and proxy ping APIs reject invalid minute labels', async () => {
  for (const time of ['', 'tomorrow', '2026-02-30 12:00', '2025-02-29 00:00',
    '1900-02-29 00:00', '2026-13-01 12:00', '2026-09-20 24:00', '2026-09-20 12:60',
    '2026-09-20 12:00\n', '2026-09-20 12:00 garbage']) {
    for (const request of createApi({ ...sample, lastcheck: [sample.lastcheck[0], time] })) {
      await assert.rejects(request, /common.invalidPingResponse/, time)
    }
  }
})

test('local and proxy ping APIs preserve node civil labels including clock rollbacks', async () => {
  for (const lastcheck of [
    ['2024-02-29 00:00', '2024-02-29 23:59'],
    ['0000-02-29 00:00', '0099-12-31 23:59'],
    ['-0001-12-31 23:59', '0000-01-01 00:00'],
    ['2011-11-06 01:59', '2011-11-06 01:00'],
    ['2011-11-06 01:30', '2011-11-06 01:30'],
    ['2011-03-13 02:30', '2011-12-30 12:00']
  ]) {
    const data = { ...sample, lastcheck }
    for (const request of createApi(data)) assert.equal(await request(), data)
  }
})
