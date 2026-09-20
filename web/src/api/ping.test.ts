import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import type { PingLogData } from '../types/index.js'

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
        ['-1e2', '-']
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
