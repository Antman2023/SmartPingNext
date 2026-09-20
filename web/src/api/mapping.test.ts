import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import type { ChinaMapData } from '../types/index.js'

const source = readFileSync(new URL('../../src/api/mapping.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

function requests(response: unknown) {
  const exports = {} as {
    getMapping: () => Promise<ChinaMapData>
    getProxyMapping: (url: string) => Promise<ChinaMapData>
  }
  runInNewContext(compiled, {
    exports,
    URL,
    URLSearchParams,
    require: (name: string) => {
      if (name === './index')
        return { default: { get: async () => response }, proxyRequestConfig: () => ({}) }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  return [() => exports.getMapping(), () => exports.getProxyMapping('http://192.0.2.1:8899')]
}

const sample: ChinaMapData = {
  text: 'node',
  subtext: '2026-09-20 12:00',
  avgdelay: { ctcc: [{ name: '广东', value: 0 }], cucc: [{ name: '北京', value: 2000 }], cmcc: [] }
}

test('map APIs preserve measured zero, failure penalties and empty carrier data', async () => {
  for (const data of [sample, { ...sample, avgdelay: { ctcc: [], cucc: [], cmcc: [] } }]) {
    for (const request of requests(data)) assert.equal(await request(), data)
  }
})

test('map APIs reject malformed carrier data before publishing it', async () => {
  const invalid: unknown[] = [
    null,
    {},
    { ...sample, text: null },
    { ...sample, subtext: 42 },
    { ...sample, avgdelay: null },
    ...['ctcc', 'cucc', 'cmcc'].flatMap((carrier) =>
      [
        undefined,
        null,
        {},
        [null],
        [{ name: 42, value: 1 }],
        [{ name: '广东', value: '12' }],
        [{ name: '广东', value: Infinity }],
        [{ name: '广东', value: -1 }],
        [{ name: '广东', value: -0.001 }],
        [{ name: '广东', value: -1e100 }],
        [{ name: '广东' }]
      ].map((values) => ({ ...sample, avgdelay: { ...sample.avgdelay, [carrier]: values } }))
    )
  ]
  for (const data of invalid) {
    for (const request of requests(data))
      await assert.rejects(request, /common.invalidMappingResponse/)
  }
})
