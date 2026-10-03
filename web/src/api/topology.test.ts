import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'

const source = readFileSync(new URL('../../src/api/topology.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

function createApi(t: test.TestContext) {
  const get = t.mock.fn(async (_url: string, _options: { signal?: AbortSignal; timeout?: number }): Promise<unknown> => ({}))
  const exports = {} as {
    getTopology: (addr: string, port: number, localAddr: string, signal?: AbortSignal) => Promise<Record<string, string>>
  }
  runInNewContext(compiled, {
    exports,
    URLSearchParams,
    require: (name: string) => {
      if (name === './index') return {
        default: { get },
        proxyRequestConfig: (signal?: AbortSignal) => ({ signal, timeout: 16000 })
      }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const signal = new AbortController().signal
  const requests = [
    () => exports.getTopology('192.0.2.1', 8899, '192.0.2.1', signal),
    () => exports.getTopology('192.0.2.2', 8899, '192.0.2.1', signal)
  ]
  return { get, requests, signal }
}

test('topology APIs preserve empty, partial, healthy, alert and unknown status maps', async (t) => {
  const { get, requests } = createApi(t)
  for (const response of [{}, { '192.0.2.1': 'true' }, {
    '192.0.2.1': 'true', '192.0.2.2': 'false', '192.0.2.3': 'unknown'
  }]) {
    get.mock.mockImplementation(async () => response)
    for (const request of requests) assert.equal(await request(), response)
  }
})

test('topology APIs reject invalid wire responses instead of publishing successful updates', async (t) => {
  const { get, requests } = createApi(t)
  const invalid = [
    undefined, null, '', '<html>Sign in</html>', true, 200, [], ['true'],
    ...[null, true, false, 0, 1, [], {}, '', 'invalid', 'TRUE', 'FALSE'].map((state) => ({
      '192.0.2.1': 'true', '192.0.2.2': state
    }))
  ]
  for (const response of invalid) {
    get.mock.mockImplementation(async () => response)
    for (const request of requests) {
      await assert.rejects(request, { message: 'common.invalidTopologyResponse' })
    }
  }
})

test('topology APIs preserve routing, cancellation and network failures', async (t) => {
  const { get, requests, signal } = createApi(t)
  await requests[0]!()
  const [localUrl, localOptions] = get.mock.calls[0]!.arguments
  assert.equal(localUrl, '/topology.json')
  assert.equal(localOptions.signal, signal)
  assert.equal(localOptions.timeout, undefined)

  await requests[1]!()
  const [proxyUrl, proxyOptions] = get.mock.calls[1]!.arguments
  const parsed = new URL(proxyUrl, 'http://localhost/api/')
  assert.equal(parsed.pathname, '/proxy.json')
  assert.equal(parsed.searchParams.get('g'), 'http://192.0.2.2:8899/api/topology.json')
  assert.equal(proxyOptions.signal, signal)
  assert.equal(proxyOptions.timeout, 16000)

  for (const error of [new Error('offline'), new DOMException('Canceled', 'AbortError')]) {
    get.mock.mockImplementation(async () => { throw error })
    for (const request of requests) await assert.rejects(request, (failure) => failure === error)
  }
})
