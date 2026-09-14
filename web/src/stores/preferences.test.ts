import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'

function loadStore<T>(name: string, storage: Pick<Storage, 'getItem' | 'setItem'>, language = 'en-US'): T {
  const source = readFileSync(new URL(`../../src/stores/${name}.ts`, import.meta.url), 'utf8')
  const compiled = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const exports: Record<string, () => T> = {}
  runInNewContext(compiled, {
    exports,
    localStorage: storage,
    navigator: { languages: [language], language },
    require: (dependency: string) => {
      if (dependency === 'vue') return vue
      assert.equal(dependency, 'pinia')
      return { defineStore: (_id: string, setup: () => T) => setup }
    }
  })
  return Object.values(exports)[0]!()
}

interface LocaleStore {
  locale: vue.Ref<string>
  setLocale: (locale: string) => void
}
interface SidebarStore {
  isCollapsed: vue.Ref<boolean>
  toggleCollapse: () => void
}
const unavailableStorage = {
  getItem: () => { throw new Error('storage denied') },
  setItem: () => { throw new Error('storage denied') }
}

test('language initializes from system settings and remains switchable when storage is denied', () => {
  for (const [language, expected] of [['en-US', 'en-US'], ['zh-TW', 'zh-CN']]) {
    const store = loadStore<LocaleStore>('locale', unavailableStorage, language)
    assert.equal(store.locale.value, expected)
    store.setLocale('en-US')
    assert.equal(store.locale.value, 'en-US')
    store.setLocale('zh-CN')
    assert.equal(store.locale.value, 'zh-CN')
  }
})

test('sidebar initializes and can be toggled repeatedly when storage is denied', () => {
  const store = loadStore<SidebarStore>('sidebar', unavailableStorage)
  assert.equal(store.isCollapsed.value, false)
  store.toggleCollapse()
  assert.equal(store.isCollapsed.value, true)
  store.toggleCollapse()
  assert.equal(store.isCollapsed.value, false)
})

test('language and sidebar preferences still persist when storage is available', () => {
  const values = new Map([['locale', 'zh-CN'], ['sidebar-collapsed', 'true']])
  const storage = {
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => { values.set(key, value) }
  }
  const locale = loadStore<LocaleStore>('locale', storage)
  const sidebar = loadStore<SidebarStore>('sidebar', storage)
  assert.equal(locale.locale.value, 'zh-CN')
  assert.equal(sidebar.isCollapsed.value, true)
  locale.setLocale('en-US')
  sidebar.toggleCollapse()
  assert.equal(values.get('locale'), 'en-US')
  assert.equal(values.get('sidebar-collapsed'), 'false')
})
