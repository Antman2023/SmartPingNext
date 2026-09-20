import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import * as pinia from 'pinia'

const source = readFileSync(new URL('../../src/stores/theme.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

test('theme follows system changes, honors manual overrides and releases its listener on disposal', async (t) => {
  const listeners = new Set<(event: { matches: boolean }) => void>()
  const preferences = new Map<string, string>()
  const attributes = new Map<string, string>()
  const media = {
    matches: false,
    addEventListener: (_name: string, callback: (event: { matches: boolean }) => void) =>
      listeners.add(callback),
    removeEventListener: (_name: string, callback: (event: { matches: boolean }) => void) =>
      listeners.delete(callback)
  }
  const changeSystem = (matches: boolean) => {
    media.matches = matches
    listeners.forEach((listener) => listener({ matches }))
  }
  const exports = {} as {
    useThemeStore: (instance: pinia.Pinia) => {
      theme: string
      initTheme: () => void
      setTheme: (value: string) => void
      $dispose: () => void
    }
  }
  runInNewContext(compiled, {
    exports,
    localStorage: {
      getItem: (key: string) => preferences.get(key),
      setItem: (key: string, value: string) => preferences.set(key, value)
    },
    window: { matchMedia: () => media },
    document: {
      documentElement: {
        setAttribute: (key: string, value: string) => attributes.set(key, value),
        classList: { toggle: () => {} },
        style: {}
      }
    },
    require: (name: string) => {
      if (name === 'vue') return vue
      if (name === 'pinia') return pinia
      throw new Error(`Unexpected dependency: ${name}`)
    }
  })
  const store = exports.useThemeStore(pinia.createPinia())
  t.after(() => store.$dispose())
  store.initTheme()
  store.initTheme()
  assert.equal(listeners.size, 1)
  assert.equal(attributes.get('data-theme'), 'light')
  changeSystem(true)
  await vue.nextTick()
  assert.equal(store.theme, 'dark')
  assert.equal(attributes.get('data-theme'), 'dark')
  store.setTheme('light')
  await vue.nextTick()
  changeSystem(false)
  changeSystem(true)
  await vue.nextTick()
  assert.equal(store.theme, 'light')
  assert.equal(preferences.get('theme'), 'light')
  store.setTheme('system')
  await vue.nextTick()
  assert.equal(store.theme, 'dark')
  assert.equal(preferences.get('theme'), 'system')
  store.$dispose()
  assert.equal(listeners.size, 0)
  changeSystem(false)
  await vue.nextTick()
  assert.equal(store.theme, 'dark')
  assert.equal(attributes.get('data-theme'), 'dark')
})
