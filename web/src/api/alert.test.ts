import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { isAlertData } from '../utils/alertData.js'
import type { AlertData } from '../types/index.js'

test('alert API validates the wire response before returning dates and logs', async (t) => {
  const compiled = ts.transpileModule(readFileSync(new URL('../../src/api/alert.ts', import.meta.url), 'utf8'), {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const get = t.mock.fn(async (_url: string, _options: { signal?: AbortSignal }): Promise<unknown> => [[], []])
  const exports = {} as { getAlerts: (url: string, date?: string, signal?: AbortSignal) => Promise<AlertData> }
  runInNewContext(compiled, {
    exports, URL, URLSearchParams,
    require: (name: string) => {
      if (name === './index') return { default: { get }, proxyRequestConfig: (signal?: AbortSignal) => ({ signal }) }
      if (name === '@/utils/alertData') return { isAlertData }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const signal = new AbortController().signal
  const request = () => exports.getAlerts('http://192.0.2.1:8899', '2026-09-20', signal)
  assert.equal((await request()).logs.length, 0)
  const [url, options] = get.mock.calls[0]!.arguments
  const remote = new URL(new URL(url, 'http://localhost').searchParams.get('g')!)
  assert.equal(remote.pathname, '/api/alert.json')
  assert.equal(remote.searchParams.get('date'), '2026-09-20')
  assert.equal(options.signal, signal)
  for (const date of [undefined, '2026-09-20', 'invalid&injected=value']) {
    await exports.getAlerts('', date, signal)
    const [localUrl, localOptions] = get.mock.calls.at(-1)!.arguments
    const parsed = new URL(localUrl, 'http://localhost/api/')
    assert.equal(parsed.pathname, '/alert.json')
    assert.equal(parsed.searchParams.get('date'), date ?? null)
    assert.equal(parsed.searchParams.has('g'), false)
    assert.equal(parsed.searchParams.has('injected'), false)
    assert.equal(localOptions.signal, signal)
  }
  const log = { Logtime: '2026-09-20 12:00', Targetip: '192.0.2.2', Targetname: 'Remote',
    Tracert: '', Fromip: '192.0.2.1', Fromname: 'Local' }
  for (const response of [null, {}, 'html', [], [[]], [[], [], []], [null, []], [[42], []],
    [['not-a-date'], []], [['2026-02-30'], []],
    [[], [{ ...log, Logtime: 'tomorrow' }]], [[], [{ ...log, Logtime: '2026-09-20 12:60' }]],
    [[], [null]], ...Object.keys(log).map((key) => [[], [{ ...log, [key]: 123 }]])]) {
    get.mock.mockImplementation(async () => response)
    await assert.rejects(request, { message: 'common.invalidAlertResponse' })
    await assert.rejects(() => exports.getAlerts(''), { message: 'common.invalidAlertResponse' })
  }
  get.mock.mockImplementation(async () => [['2026-09-20'], [log]])
  assert.equal((await request()).logs[0], log)
  for (const [date, time] of [
    ['2026-09-21', '2026-09-20 23:59:59.123-08:00'],
    ['0000-02-29', '0000-02-29 00:00'],
    ['-0001-01-01', '-0001-01-01 00:00']
  ]) {
    const record = { ...log, Logtime: time, Tracert: 'legacy trace failed' }
    get.mock.mockImplementation(async () => [[date], [record]])
    assert.equal((await request()).logs[0], record)
    assert.equal((await exports.getAlerts('')).logs[0], record)
  }
  const failure = new Error('offline')
  get.mock.mockImplementation(async () => { throw failure })
  await assert.rejects(request, (error) => error === failure)
})
