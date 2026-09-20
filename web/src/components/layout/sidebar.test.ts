import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'

test('mobile navigation focuses the current route and restores focus only when needed', async (t) => {
  const { descriptor } = parse(readFileSync(new URL('../../../src/components/layout/AppSidebar.vue', import.meta.url), 'utf8'))
  const compiled = ts.transpileModule(compileScript(descriptor, { id: 'sidebar' }).content, {
    compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
  }).outputText
  const compact = vue.ref(true)
  const store = vue.reactive({ isMobileOpen: false, isCollapsed: false,
    closeMobile: () => { store.isMobileOpen = false } })
  const document = {
    activeElement: null as object | null,
    documentElement: { classList: { toggle: () => {}, remove: () => {} } },
    querySelector: () => trigger
  }
  const trigger = { focus: t.mock.fn(() => { document.activeElement = trigger }) }
  const selected = { focus: t.mock.fn(() => { document.activeElement = selected }) }
  const first = { focus: t.mock.fn(() => { document.activeElement = first }) }
  const backdrop = vue.markRaw({})
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onBeforeUnmount: () => {} },
    'vue-router': { useRoute: () => ({ path: '/alerts' }) },
    '@element-plus/icons-vue': {},
    '@/stores/sidebar': { useSidebarStore: () => store },
    '@/stores/config': { useConfigStore: () => ({ config: null }) },
    '@/utils/format': { displayName: (value: string) => value },
    '@/composables/useViewportCompact': { useViewportCompact: () => ({ isCompact: compact }) }
  }
  const exports = {} as { default: { setup: (props: object, context: object) => {
    sidebarRef: vue.Ref<unknown>; backdropRef: vue.Ref<unknown>; handleEscape: (event: { key: string }) => void
  } } }
  runInNewContext(compiled, { exports, document, require: (name: string) => {
    assert.ok(name in dependencies, name)
    return dependencies[name]
  } })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default.setup({}, { expose: () => {} }))!
  view.sidebarRef.value = vue.markRaw({
    querySelector: (selector: string) => selector.includes('aria-current') ? selected : first,
    contains: (element: object) => element === selected || element === first
  })
  view.backdropRef.value = backdrop
  const flush = async () => { await vue.nextTick(); await vue.nextTick() }
  store.isMobileOpen = true
  await flush()
  assert.equal(document.activeElement, selected)
  assert.equal(first.focus.mock.callCount(), 0)
  view.handleEscape({ key: 'Escape' })
  await flush()
  assert.equal(document.activeElement, trigger)
  assert.equal(store.isMobileOpen, false)

  store.isMobileOpen = true
  await flush()
  document.activeElement = backdrop
  store.closeMobile()
  await flush()
  assert.equal(document.activeElement, trigger)

  store.isMobileOpen = true
  await flush()
  const external = {}
  document.activeElement = external
  store.closeMobile()
  await flush()
  assert.equal(document.activeElement, external)

  const focusCount = selected.focus.mock.callCount()
  store.isMobileOpen = true
  store.closeMobile()
  await flush()
  assert.equal(selected.focus.mock.callCount(), focusCount)
  compact.value = false
  store.isMobileOpen = true
  await flush()
  assert.equal(store.isMobileOpen, false)
  assert.equal(selected.focus.mock.callCount(), focusCount)
})
