import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { ref, type Ref } from 'vue'

interface PreferenceStores {
  useLocaleStore: () => { locale: Ref<string>; setLocale: (locale: string) => void }
  useSidebarStore: () => { isCollapsed: Ref<boolean>; toggleCollapse: () => void }
}

function loadStores(storage: () => unknown, language = 'zh-CN'): PreferenceStores {
  const exports = {} as PreferenceStores
  const sandbox = {
    exports,
    navigator: { languages: [language], language },
    get localStorage() {
      return storage()
    },
    require: (name: string) => {
      if (name === 'vue') return { ref }
      if (name === 'pinia') return { defineStore: (_id: string, setup: () => unknown) => setup }
      throw new Error(`Unexpected dependency: ${name}`)
    }
  }
  for (const name of ['locale', 'sidebar']) {
    const source = readFileSync(new URL(`../../src/stores/${name}.ts`, import.meta.url), 'utf8')
    const compiled = ts.transpileModule(source, {
      compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
    }).outputText
    runInNewContext(`(() => { ${compiled} })()`, sandbox)
  }
  return exports
}

test('blocked browser storage does not prevent preference initialization or changes', () => {
  for (const language of ['zh-TW', 'en-US']) {
    const stores = loadStores(() => {
      throw new Error('storage blocked')
    }, language)
    const locale = stores.useLocaleStore()
    const sidebar = stores.useSidebarStore()
    assert.equal(locale.locale.value, language.startsWith('zh') ? 'zh-CN' : 'en-US')
    assert.equal(sidebar.isCollapsed.value, false)
    locale.setLocale('en-US')
    sidebar.toggleCollapse()
    assert.equal(locale.locale.value, 'en-US')
    assert.equal(sidebar.isCollapsed.value, true)
  }
})

test('readable but unwritable storage keeps saved preferences and allows session changes', () => {
  const stores = loadStores(() => ({
    getItem: (key: string) => (key === 'locale' ? 'en-US' : 'true'),
    setItem: () => {
      throw new Error('quota exceeded')
    }
  }))
  const locale = stores.useLocaleStore()
  const sidebar = stores.useSidebarStore()
  assert.equal(locale.locale.value, 'en-US')
  assert.equal(sidebar.isCollapsed.value, true)
  locale.setLocale('zh-CN')
  sidebar.toggleCollapse()
  assert.equal(locale.locale.value, 'zh-CN')
  assert.equal(sidebar.isCollapsed.value, false)
})

test('invalid stored values fall back and normal changes remain persistent', () => {
  const values = new Map([
    ['locale', 'invalid'],
    ['sidebar-collapsed', 'invalid']
  ])
  const stores = loadStores(() => ({
    getItem: (key: string) => values.get(key) ?? null,
    setItem: (key: string, value: string) => values.set(key, value)
  }))
  const locale = stores.useLocaleStore()
  const sidebar = stores.useSidebarStore()
  assert.equal(locale.locale.value, 'zh-CN')
  assert.equal(sidebar.isCollapsed.value, false)
  locale.setLocale('en-US')
  sidebar.toggleCollapse()
  assert.equal(values.get('locale'), 'en-US')
  assert.equal(values.get('sidebar-collapsed'), 'true')
})
