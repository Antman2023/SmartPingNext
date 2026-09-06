import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { ref, type Ref } from 'vue'
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
    vue: { ref },
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
