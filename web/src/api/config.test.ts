import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { normalizeConfigResponse } from '../utils/configResponse.js'

const source = readFileSync(new URL('../../src/api/config.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

class ApiError extends Error {
  constructor(public status: number) {
    super(`HTTP ${status}`)
  }
}

test('local and proxy config requests reject malformed payloads and normalize empty Go collections', async (t) => {
  const valid = {
    Ver: 'test', Name: 'Local', Addr: '192.0.2.1', Port: 8899, Authiplist: '', Toollimit: 0,
    Base: { Timeout: 5, Refresh: 1, Archive: 30 }, Mode: null,
    Topology: { Tline: '2', Tsymbolsize: '50' },
    Network: {
      '192.0.2.1': { Name: 'Local', Addr: '192.0.2.1', Smartping: true, Ping: null, Topology: null }
    },
    Chinamap: { Shanghai: { ctcc: null }, Beijing: null }
  }
  const get = t.mock.fn(async (_url: string, _options: { signal?: AbortSignal }): Promise<unknown> => valid)
  const exports = {} as {
    fetchConfig: (signal?: AbortSignal) => Promise<ReturnType<typeof normalizeConfigResponse>>
    fetchProxyConfig: (url: string, signal?: AbortSignal) => Promise<ReturnType<typeof normalizeConfigResponse>>
  }
  runInNewContext(compiled, {
    exports, URLSearchParams,
    require: (name: string) => {
      if (name === './index') return { default: { get }, proxyRequestConfig: (signal?: AbortSignal) => ({ signal }) }
      if (name === '@/utils/error') return { ApiError }
      if (name === '@/utils/configResponse') return { normalizeConfigResponse }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const signal = new AbortController().signal
  for (const fetch of [() => exports.fetchConfig(signal), () => exports.fetchProxyConfig('http://192.0.2.2:8899', signal)]) {
    get.mock.mockImplementation(async () => valid)
    const config = (await fetch())!
    assert.deepEqual(config.Network['192.0.2.1']!.Ping, [])
    assert.deepEqual(config.Network['192.0.2.1']!.Topology, [])
    assert.deepEqual(config.Chinamap.Shanghai, { ctcc: [], cucc: [], cmcc: [] })
    assert.deepEqual(config.Chinamap.Beijing, { ctcc: [], cucc: [], cmcc: [] })
    assert.deepEqual(config.Mode, {})
    assert.equal(config.Topology.Tsound, '')
    assert.equal(valid.Network['192.0.2.1'].Ping, null)
    assert.equal(get.mock.calls.at(-1)!.arguments[1].signal, signal)
    const node = valid.Network['192.0.2.1']
    for (const payload of [
      null, [], {}, { ...valid, Name: null }, { ...valid, Port: '8899' },
      { ...valid, Base: { Refresh: Infinity } }, { ...valid, Base: { ...valid.Base, Refresh: '1' } },
      { ...valid, Topology: {} }, { ...valid, Mode: { Type: false } },
      { ...valid, Network: { bad: null } },
      ...[{ Smartping: 'true' }, { Ping: [null] }, { Ping: {} }, { Topology: [{}] },
        { Topology: [null] }, { Name: [] }].map((fields) => ({ ...valid, Network: { node: { ...node, ...fields } } })),
      { ...valid, Chinamap: [] }, { ...valid, Chinamap: { Shanghai: { ctcc: [123] } } }
    ]) {
      get.mock.mockImplementation(async () => payload)
      await assert.rejects(fetch, { message: 'common.invalidConfigResponse' })
    }
    get.mock.mockImplementation(async () => ({ ...valid, Chinamap: null }))
    assert.equal(Object.keys((await fetch())!.Chinamap).length, 0)
  }
})

test('config save requires explicit acknowledgement and propagates request failures', async (t) => {
  const post = t.mock.fn(async (_url: string, _body: URLSearchParams, _options: { signal?: AbortSignal }): Promise<unknown> => null)
  const exports = {} as {
    saveConfig: (config: object, password: string, signal?: AbortSignal) => Promise<{ status: string; info?: string }>
  }
  runInNewContext(compiled, {
    exports, URLSearchParams,
    require: (name: string) => {
      if (name === './index') return { default: { post } }
      if (name === '@/utils/error') return { ApiError }
      if (name === '@/utils/configResponse') return { normalizeConfigResponse }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const config = { Name: 'local & test' }
  const signal = new AbortController().signal
  const save = () => exports.saveConfig(config, 'password & test', signal)
  for (const payload of [null, undefined, '', '<html>proxy error</html>', [], {}, { info: 'OK' },
    { status: false }, { status: 'false' }, { status: {} }, { status: '200' }]) {
    post.mock.mockImplementation(async () => payload)
    await assert.rejects(save, { message: 'common.configSaveUnconfirmed' })
  }
  for (const status of ['true', true, 200]) {
    post.mock.mockImplementation(async () => ({ status, info: 'saved' }))
    const result = await save()
    assert.equal(result.status, 'true')
    assert.equal(result.info, 'saved')
  }
  const [url, body, options] = post.mock.calls.at(-1)!.arguments
  assert.equal(url, '/saveconfig.json')
  assert.deepEqual(JSON.parse(body.get('config')!), config)
  assert.equal(body.get('password'), 'password & test')
  assert.equal(options.signal, signal)
  for (const error of [new ApiError(429), new ApiError(503), new DOMException('aborted', 'AbortError')]) {
    post.mock.mockImplementation(async () => { throw error })
    await assert.rejects(save, (caught) => caught === error)
  }
})

test('password verification uses the shared request client and preserves result categories', async (t) => {
  const post = t.mock.fn(
    async (
      _url: string,
      _body: URLSearchParams,
      _options: { signal?: AbortSignal }
    ): Promise<unknown> => ({ status: 'true' })
  )
  const exports = {} as {
    verifyConfigPassword: (password: string, signal?: AbortSignal) => Promise<string>
  }
  runInNewContext(compiled, {
    exports,
    URLSearchParams,
    require: (name: string) => {
      if (name === './index') return { default: { post } }
      if (name === '@/utils/error') return { ApiError }
      if (name === '@/utils/configResponse') return { normalizeConfigResponse }
      if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const signal = new AbortController().signal
  assert.equal(await exports.verifyConfigPassword('test & password', signal), 'valid')
  const [url, body, options] = post.mock.calls[0]!.arguments
  assert.equal(url, '/verify-password.json')
  assert.equal(body.get('password'), 'test & password')
  assert.equal(options.signal, signal)
  for (const [status, expected] of [
    [200, 'invalid'],
    [429, 'rate-limited']
  ] as const) {
    post.mock.mockImplementation(async () => {
      throw new ApiError(status)
    })
    assert.equal(await exports.verifyConfigPassword('test'), expected)
  }
  for (const error of [
    new ApiError(0),
    new ApiError(503),
    new DOMException('aborted', 'AbortError')
  ]) {
    post.mock.mockImplementation(async () => {
      throw error
    })
    await assert.rejects(
      () => exports.verifyConfigPassword('test'),
      (got) => got === error
    )
  }
  for (const payload of [null, {}, { status: 'false' }]) {
    post.mock.mockImplementation(async () => payload)
    assert.equal(await exports.verifyConfigPassword('test'), 'invalid')
  }
})
