import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { computed, ref, shallowRef, type Ref } from 'vue'
import type { Config } from '../types/index.js'

const source = readFileSync(new URL('../../src/stores/config.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface ConfigStore {
  config: Ref<Config | null>
  loading: Ref<boolean>
  error: Ref<string | null>
  loadConfig: () => Promise<Config | null>
  saveConfig: (config: Config, password: string, signal?: AbortSignal) => Promise<void>
}
const config = (name: string) => ({ Name: name }) as Config
function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: Error) => void
  const promise = new Promise<T>((done, fail) => {
    resolve = done
    reject = fail
  })
  return { promise, resolve, reject }
}
function createStore(t: test.TestContext) {
  const fetchConfig = t.mock.fn(async (): Promise<Config> => config('server'))
  const saveConfig = t.mock.fn(async (_value: Config): Promise<void> => {})
  const dependencies: Record<string, unknown> = {
    pinia: { defineStore: (_name: string, setup: () => ConfigStore) => setup },
    vue: { ref, shallowRef },
    '@/api/config': { fetchConfig, saveConfig },
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/locales': { default: { global: { t: (key: string) => key } } }
  }
  const exports: { useConfigStore?: () => ConfigStore } = {}
  runInNewContext(compiled, {
    exports,
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  return { store: exports.useConfigStore!(), fetchConfig, saveConfig }
}

test('config loads wait for a pending save and share the subsequent fetch', async (t) => {
  const { store, fetchConfig, saveConfig } = createStore(t)
  const gate = deferred<void>()
  saveConfig.mock.mockImplementation(() => gate.promise)
  const saving = store.saveConfig(config('new'), 'password')
  const first = store.loadConfig()
  const second = store.loadConfig()
  assert.equal(fetchConfig.mock.callCount(), 0)
  assert.equal(store.loading.value, true)
  gate.resolve()
  await saving
  await Promise.all([first, second])
  assert.equal(fetchConfig.mock.callCount(), 1)
  assert.equal(store.config.value?.Name, 'server')
  assert.equal(store.loading.value, false)
})

test('config saves are serialized and snapshot input when queued', async (t) => {
  const { store, saveConfig } = createStore(t)
  const gate = deferred<void>()
  saveConfig.mock.mockImplementationOnce(() => gate.promise)
  const first = store.saveConfig(config('first'), 'password')
  const next = config('second')
  const second = store.saveConfig(next, 'password')
  next.Name = 'unsaved edit'
  assert.equal(saveConfig.mock.callCount(), 1)
  gate.resolve()
  await Promise.all([first, second])
  assert.equal(saveConfig.mock.callCount(), 2)
  assert.equal(saveConfig.mock.calls[1]!.arguments[0].Name, 'second')
  assert.equal(store.config.value?.Name, 'second')
})

test('failed config saves do not block queued saves or subsequent loads', async (t) => {
  const { store, saveConfig, fetchConfig } = createStore(t)
  const gate = deferred<void>()
  saveConfig.mock.mockImplementationOnce(() => gate.promise)
  const first = store.saveConfig(config('first'), 'password')
  const failed = assert.rejects(first, /offline/)
  const second = store.saveConfig(config('second'), 'password')
  const loading = store.loadConfig()
  gate.reject(new Error('offline'))
  await failed
  await second
  await loading
  assert.equal(saveConfig.mock.callCount(), 2)
  assert.equal(fetchConfig.mock.callCount(), 1)
  assert.equal(store.error.value, null)
  assert.equal(store.loading.value, false)
})

test('a canceled queued config save never reaches the API', async (t) => {
  const { store, saveConfig } = createStore(t)
  const gate = deferred<void>()
  const controller = new AbortController()
  saveConfig.mock.mockImplementationOnce(() => gate.promise)
  const first = store.saveConfig(config('first'), 'password')
  const second = store.saveConfig(config('second'), 'password', controller.signal)
  const canceled = assert.rejects(second, { name: 'AbortError' })
  controller.abort()
  gate.resolve()
  await first
  await canceled
  assert.equal(saveConfig.mock.callCount(), 1)
  assert.equal(store.config.value?.Name, 'first')
  assert.equal(store.error.value, null)
})

test('a load started before saving returns fresh configuration to its caller', async (t) => {
  const { store, fetchConfig } = createStore(t)
  const gate = deferred<Config>()
  fetchConfig.mock.mockImplementationOnce(() => gate.promise)
  const loading = store.loadConfig()
  await store.saveConfig(config('saved'), 'password')
  gate.resolve(config('outdated'))
  const result = await loading
  assert.equal(result?.Name, 'server')
  assert.equal(store.config.value?.Name, 'server')
  assert.equal(fetchConfig.mock.callCount(), 2)
})

test('failed shared loads retain the previous configuration and allow a fresh retry', async (t) => {
  const { store, fetchConfig } = createStore(t)
  await store.loadConfig()
  const previous = store.config.value
  const failure = deferred<Config>()
  fetchConfig.mock.mockImplementationOnce(() => failure.promise)
  const first = store.loadConfig()
  const second = store.loadConfig()
  assert.equal(first, second)
  failure.reject(new Error('offline'))
  assert.deepEqual(await Promise.all([first, second]), [null, null])
  assert.equal(store.config.value, previous)
  assert.equal(store.error.value, 'common.configLoadFailed')
  assert.equal(store.loading.value, false)

  const recovery = deferred<Config>()
  fetchConfig.mock.mockImplementationOnce(() => recovery.promise)
  const retry = store.loadConfig()
  assert.equal(store.error.value, null)
  assert.equal(store.loading.value, true)
  assert.equal(store.config.value, previous)
  recovery.resolve(config('recovered'))
  assert.equal((await retry)?.Name, 'recovered')
  assert.equal(store.config.value?.Name, 'recovered')
  assert.equal(store.loading.value, false)
  assert.equal(fetchConfig.mock.callCount(), 3)
})

test('an obsolete load failure waits for saving and shares the recovery fetch', async (t) => {
  const { store, fetchConfig, saveConfig } = createStore(t)
  const oldLoad = deferred<Config>()
  const save = deferred<void>()
  fetchConfig.mock.mockImplementationOnce(() => oldLoad.promise)
  saveConfig.mock.mockImplementationOnce(() => save.promise)
  const loading = store.loadConfig()
  const saving = store.saveConfig(config('saved'), 'password')
  const reload = store.loadConfig()
  oldLoad.reject(new Error('obsolete request failed'))
  await Promise.resolve()
  assert.equal(store.error.value, null)
  assert.equal(store.loading.value, true)
  assert.equal(fetchConfig.mock.callCount(), 1)

  save.resolve()
  await saving
  const results = await Promise.all([loading, reload])
  assert.equal(results[0]?.Name, 'server')
  assert.equal(results[1]?.Name, 'server')
  assert.equal(fetchConfig.mock.callCount(), 2)
  assert.equal(store.config.value?.Name, 'server')
  assert.equal(store.error.value, null)
  assert.equal(store.loading.value, false)
})

test('shared configuration snapshots preserve map names that match reactive metadata', async (t) => {
  const names = ['__v_isReactive', '__v_isReadonly', '__v_isShallow', '__v_raw', '__v_skip', '__proto__', 'hasOwnProperty']
  // Some metadata names suppress proxying and can hide other collisions.
  for (const keys of [...names.map((name) => [name]), names]) {
    const { store, fetchConfig } = createStore(t)
    const loaded: Config = {
      Ver: 'test', Port: 8899, Name: 'server', Addr: '127.0.0.1', Mode: {},
      Base: { Timeout: 3, Refresh: 5, Archive: 30 }, Topology: { Tline: '2', Tsymbolsize: '50', Tsound: '' },
      Network: { '127.0.0.1': { Name: 'server', Addr: '127.0.0.1', Smartping: true, Ping: [], Topology: [] } },
      Chinamap: Object.fromEntries(keys.map((name) => [name, { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] }])),
      Toollimit: 0, Authiplist: ''
    }
    const header = computed(() => `${store.config.value?.Name}:${Object.keys(store.config.value?.Network ?? {}).length}`)
    fetchConfig.mock.mockImplementation(async () => loaded)
    await store.loadConfig()
    assert.deepEqual(JSON.parse(JSON.stringify(store.config.value)), loaded)
    assert.equal(header.value, 'server:1')
    for (const name of keys) {
      assert.equal(store.config.value!.Chinamap[name]!.ctcc.join(','), '192.0.2.1', name)
    }
    const name = keys[0]!
    loaded.Chinamap[name]!.ctcc.push('192.0.2.2')
    loaded.Name = 'saved'
    loaded.Network['192.0.2.1'] = { Name: 'remote', Addr: '192.0.2.1', Smartping: true, Ping: [], Topology: [] }
    assert.equal(header.value, 'server:1')
    assert.equal(store.config.value!.Chinamap[name]!.ctcc.join(','), '192.0.2.1')
    await store.saveConfig(loaded, 'password')
    assert.deepEqual(JSON.parse(JSON.stringify(store.config.value)), loaded)
    assert.equal(header.value, 'saved:2')
    loaded.Chinamap[name]!.ctcc.push('192.0.2.3')
    loaded.Name = 'unpublished'
    assert.equal(header.value, 'saved:2')
    assert.equal(store.config.value!.Chinamap[name]!.ctcc.join(','), '192.0.2.1,192.0.2.2')
  }
})
