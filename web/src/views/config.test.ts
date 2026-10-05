import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import type { Config, NetworkMember } from '../types/index.js'
import * as validation from '../utils/configValidation.js'
import { createConfigDraft } from '../utils/configDraft.js'
import { CanceledError } from 'axios'
import { isRequestCanceled } from '../utils/requestCancellation.js'

const source = readFileSync(new URL('../../src/views/ConfigView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'config-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface ConfigSetup {
  configRoot: vue.Ref<HTMLElement | null>
  formConfig: Config
  password: vue.Ref<string>
  savedSnapshot: vue.Ref<string>
  currentValidationIssue: vue.ComputedRef<validation.ConfigValidationIssue | null>
  saving: vue.Ref<boolean>
  loadingConfig: vue.Ref<boolean>
  configLoaded: vue.Ref<boolean>
  configError: vue.Ref<boolean>
  configReady: vue.ComputedRef<boolean>
  isDirty: vue.ComputedRef<boolean>
  provinceCount: vue.ComputedRef<number>
  loadConfig: () => Promise<void>
  handleBeforeUnload: (event: BeforeUnloadEvent) => void
  handleSave: () => Promise<void>
  importExportPassword: vue.Ref<string>
  importing: vue.Ref<boolean>
  exporting: vue.Ref<boolean>
  handleExport: () => Promise<void>
  handleImportFile: (file: { raw: { size?: number; text: () => Promise<string> } }) => Promise<void>
  newNodeName: vue.Ref<string>
  newNodeAddr: vue.Ref<string>
  addNodeVisible: vue.Ref<boolean>
  showAddNode: () => void
  editNodeName: vue.Ref<string>
  editNodeAddr: vue.Ref<string>
  editNodeVisible: vue.Ref<boolean>
  showEditNode: (row: NetworkMember & { _original: NetworkMember; isSelf: boolean }) => void
  addNode: () => void
  saveEditNode: () => void
  pingTargetList: vue.Ref<{ Addr: string; enabled: boolean }[]>
  editPingConfig: (row: NetworkMember & { _original: NetworkMember; isSelf: boolean }) => void
  pingConfigVisible: vue.Ref<boolean>
  savePingConfig: () => void
  topoConfigVisible: vue.Ref<boolean>
  editTopoConfig: (row: NetworkMember & { _original: NetworkMember; isSelf: boolean }) => void
  saveTopoConfig: () => void
  topoTargetList: vue.Ref<Array<{ Addr: string; Name: string; enabled: boolean; checkSeconds: number;
    occurrenceCount: number; avgDelay: number; lossPercent: number }>>
  chinaMapIps: { ctcc: string; cucc: string; cmcc: string }
  saveChinaMap: () => void
  newProvinceName: vue.Ref<string>
  addProvinceVisible: vue.Ref<boolean>
  showAddChinaMap: () => void
  addProvince: () => void
  networkList: vue.ComputedRef<Array<NetworkMember & { _original: NetworkMember; isSelf: boolean }>>
  deleteNode: (row: NetworkMember & { _original: NetworkMember; isSelf: boolean }) => Promise<void>
  currentProvince: vue.Ref<string>
  chinaMapVisible: vue.Ref<boolean>
  editChinaMap: (province: string) => void
  deleteChinaMap: () => Promise<void>
}

function createView(t: test.TestContext) {
  const createObjectURL = t.mock.fn((_blob: Blob) => 'blob:config-export')
  const revokeObjectURL = t.mock.fn((_url: string) => {})
  const link = { href: '', download: '', click: t.mock.fn(() => {}) }
  const createElement = t.mock.fn((_tag: string) => link)
  const focus = t.mock.fn((_options?: Parameters<HTMLElement['focus']>[0]) => {})
  const scrollIntoView = t.mock.fn((_options?: Exclude<Parameters<HTMLElement['scrollIntoView']>[0], boolean>) => {})
  const target = { focus, scrollIntoView }
  const getElementById = t.mock.fn((_id: string) => target)
  const querySelector = t.mock.fn((_selector: string): typeof target | null => target)
  const configRoot = { isConnected: true, querySelector }
  const nextTick = t.mock.fn(() => vue.nextTick())
  const matchMedia = t.mock.fn((_query: string) => ({ matches: false }))
  const saveConfig = t.mock.fn(async (_config: Config, _password?: string, _signal?: AbortSignal): Promise<void> => {})
  const loadConfig = t.mock.fn(async (): Promise<Config | null> => ({
    Ver: 'test', Port: 8899, Name: 'local', Addr: '127.0.0.1', Mode: {},
    Base: { Timeout: 3, Refresh: 5, Archive: 30 },
    Topology: { Tsound: '', Tline: '2', Tsymbolsize: '50' },
    Network: {
      '127.0.0.1': { Name: 'local', Addr: '127.0.0.1', Smartping: true, Ping: [], Topology: [] }
    },
    Chinamap: {}, Toollimit: 0, Authiplist: ''
  }))
  const unmountCallbacks: Array<() => void> = []
  let leaveRoute: () => Promise<boolean> = async () => { throw new Error('Route guard was not registered') }
  const showError = t.mock.fn((_message: string) => {})
  const showSuccess = t.mock.fn((_message: string) => {})
  const showWarning = t.mock.fn((_message: string) => {})
  const confirm = t.mock.fn(async (_message: string, _title: string, _options: object): Promise<void> => {})
  const verifyConfigPassword = t.mock.fn(
    async (_password: string, _signal?: AbortSignal) => 'valid'
  )
  const dependencies: Record<string, unknown> = {
    vue: {
      ...vue,
      nextTick,
      onMounted: () => {},
      onUnmounted: (fn: () => void) => unmountCallbacks.push(fn)
    },
    'vue-router': { onBeforeRouteLeave: (guard: () => Promise<boolean>) => { leaveRoute = guard } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'element-plus': {
      ElMessage: { success: showSuccess, error: showError, warning: showWarning },
      ElMessageBox: { confirm }
    },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusConfigStyles': {},
    '@/api': { isRequestCanceled },
    '@/api/config': { verifyConfigPassword, getConfigUrl: () => '/api/config.json' },
    '@/utils/configValidation': validation,
    '@/utils/configDraft': { createConfigDraft },
    '@/utils/format': { displayName: (name: string) => name },
    '@/stores/config': {
      useConfigStore: () => ({
        saveConfig,
        loadConfig
      })
    }
  }
  const exports: { default?: { setup: (props: object, context: object) => ConfigSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    Blob,
    URL: { createObjectURL, revokeObjectURL },
    document: { createElement, getElementById },
    URLSearchParams,
    window: { removeEventListener: () => {}, matchMedia },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  view.configRoot.value = configRoot as unknown as HTMLElement
  return {
    view,
    saveConfig,
    loadConfig,
    showError,
    showSuccess,
    showWarning,
    confirm,
    verifyConfigPassword,
    createObjectURL,
    revokeObjectURL,
    createElement,
    getElementById,
    configRoot,
    querySelector,
    focus,
    scrollIntoView,
    matchMedia,
    nextTick,
    link,
    leaveRoute: () => leaveRoute(),
    unmount: () => unmountCallbacks.forEach((fn) => fn())
  }
}

// The compiled component runs in another realm; let its awaited tick and all
// promise continuations finish before checking DOM side effects.
const flushValidationFocus = () => setImmediate()

test('obsolete validation focus ignores lifecycle, corrected drafts and newer operations', async (t) => {
  for (const state of ['unmount', 'detached', 'corrected', 'changed-issue', 'reload', 'failed-reload', 'pending-reload', 'import', 'pending-export']) {
    const { view, loadConfig, unmount, nextTick, configRoot, querySelector, getElementById, focus, scrollIntoView, verifyConfigPassword, saveConfig } = createView(t)
    await view.loadConfig()
    let finishTick!: () => void
    const rendering = new Promise<void>((resolve) => { finishTick = resolve })
    nextTick.mock.mockImplementationOnce(() => rendering)
    view.formConfig.Base.Timeout = 0
    await view.handleSave()
    assert.equal(nextTick.mock.callCount(), 1)
    let finishOperation: (() => void) | undefined
    let operation: Promise<void> | undefined
    if (state === 'unmount') unmount()
    if (state === 'detached') configRoot.isConnected = false
    if (state === 'corrected') view.formConfig.Base.Timeout = 3
    if (state === 'changed-issue') {
      view.formConfig.Base.Timeout = 3
      view.formConfig.Base.Refresh = 0
    }
    if (state === 'reload') await view.loadConfig()
    if (state === 'failed-reload') {
      loadConfig.mock.mockImplementationOnce(async () => null)
      await view.loadConfig()
    }
    if (state === 'pending-reload') {
      const loaded = (await loadConfig())!
      loadConfig.mock.mockImplementationOnce(() => new Promise<Config>((resolve) => { finishOperation = () => resolve(loaded) }))
      operation = view.loadConfig()
    }
    if (state === 'import') {
      const imported = (await loadConfig())!
      view.importExportPassword.value = 'password'
      await view.handleImportFile({ raw: { text: async () => JSON.stringify(imported) } })
    }
    if (state === 'pending-export') {
      verifyConfigPassword.mock.mockImplementationOnce(() => new Promise<'valid'>((resolve) => { finishOperation = () => resolve('valid') }))
      view.importExportPassword.value = 'password'
      operation = view.handleExport()
    }
    finishTick()
    await rendering
    await flushValidationFocus()
    assert.equal(getElementById.mock.callCount(), 0, `${state}: global lookup`)
    assert.equal(querySelector.mock.callCount(), 0, `${state}: scoped lookup`)
    assert.equal(focus.mock.callCount(), 0, state)
    assert.equal(scrollIntoView.mock.callCount(), 0, state)
    assert.equal(saveConfig.mock.callCount(), 0)
    finishOperation?.()
    await operation
  }
})

test('only the newest pending validation request focuses its current error', async (t) => {
  const { view, nextTick, querySelector, getElementById, focus, scrollIntoView } = createView(t)
  await view.loadConfig()
  let finishFirst!: () => void
  let finishSecond!: () => void
  const first = new Promise<void>((resolve) => { finishFirst = resolve })
  const second = new Promise<void>((resolve) => { finishSecond = resolve })
  nextTick.mock.mockImplementationOnce(() => first)
  nextTick.mock.mockImplementationOnce(() => second, 1)
  view.formConfig.Base.Timeout = 0
  await view.handleSave()
  await view.handleSave()
  finishFirst()
  await first
  await flushValidationFocus()
  assert.equal(focus.mock.callCount(), 0)
  assert.equal(querySelector.mock.callCount(), 0)
  finishSecond()
  await second
  await flushValidationFocus()
  assert.equal(getElementById.mock.callCount(), 0)
  assert.equal(querySelector.mock.calls[0]!.arguments[0], '#config-timeout')
  assert.equal(focus.mock.callCount(), 1)
  assert.equal(focus.mock.calls[0]!.arguments[0]?.preventScroll, true)
  assert.equal(scrollIntoView.mock.callCount(), 1)
})

test('current validation errors focus within their own page and respect reduced motion', async (t) => {
  for (const category of ['base', 'network', 'mapping', 'summary']) {
    for (const reducedMotion of [false, true]) {
      const { view, nextTick, querySelector, getElementById, matchMedia, focus, scrollIntoView, saveConfig } = createView(t)
      await view.loadConfig()
      if (category === 'base') view.formConfig.Base.Timeout = 0
      if (category === 'network') view.formConfig.Network['127.0.0.1']!.Name = ''
      if (category === 'mapping') view.formConfig.Chinamap.Beijing = { ctcc: ['invalid'], cucc: [], cmcc: [] }
      if (category === 'summary') view.formConfig.Base.PingCount = 0
      matchMedia.mock.mockImplementation(() => ({ matches: reducedMotion }))
      await view.handleSave()
      await nextTick()
      await flushValidationFocus()
      assert.equal(getElementById.mock.callCount(), 0)
      const expected = category === 'base' ? '#config-timeout' : category === 'network' ? '#config-network-settings' : category === 'mapping' ? '#config-mapping-settings' : '#config-validation-summary'
      assert.equal(querySelector.mock.calls[0]!.arguments[0], expected)
      assert.equal(focus.mock.callCount(), 1)
      assert.equal(focus.mock.calls[0]!.arguments[0]?.preventScroll, true)
      assert.equal(scrollIntoView.mock.calls[0]!.arguments[0]?.behavior, reducedMotion ? 'auto' : 'smooth')
      assert.equal(scrollIntoView.mock.calls[0]!.arguments[0]?.block, 'center')
      assert.equal(matchMedia.mock.calls[0]!.arguments[0], '(prefers-reduced-motion: reduce)')
      assert.equal(saveConfig.mock.callCount(), 0)
    }
  }
})

test('validation focus tolerates absent page roots and missing field elements', async (t) => {
  for (const missing of ['root', 'target']) {
    const { view, nextTick, querySelector, getElementById, focus, scrollIntoView } = createView(t)
    await view.loadConfig()
    if (missing === 'root') view.configRoot.value = null
    if (missing === 'target') querySelector.mock.mockImplementation(() => null)
    view.formConfig.Base.Timeout = 0
    await view.handleSave()
    await nextTick()
    await flushValidationFocus()
    assert.equal(getElementById.mock.callCount(), 0)
    assert.equal(focus.mock.callCount(), 0)
    assert.equal(scrollIntoView.mock.callCount(), 0)
  }
})

test('configuration leave confirmations cannot interrupt operations started while confirming', async (t) => {
  for (const operation of ['save', 'import', 'export']) {
    const { view, leaveRoute, confirm, saveConfig, verifyConfigPassword } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    let finishConfirmation!: () => void
    confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { finishConfirmation = resolve }))
    const leaving = leaveRoute()
    assert.equal(confirm.mock.callCount(), 1)
    let finishOperation!: () => void
    let pending: Promise<void>
    if (operation === 'save') {
      view.password.value = 'save-password'
      saveConfig.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { finishOperation = resolve }))
      pending = view.handleSave()
    } else {
      view.importExportPassword.value = 'file-password'
      verifyConfigPassword.mock.mockImplementationOnce(() => new Promise<'valid'>((resolve) => { finishOperation = () => resolve('valid') }))
      pending = operation === 'export' ? view.handleExport() : view.handleImportFile({ raw: { text: async () => JSON.stringify(view.formConfig) } })
    }
    const before = JSON.stringify(view.formConfig)
    finishConfirmation()
    assert.equal(await leaving, false, operation)
    assert.equal(await leaveRoute(), false, `busy ${operation}`)
    assert.equal(confirm.mock.callCount(), 1)
    assert.equal(JSON.stringify(view.formConfig), before)
    finishOperation()
    await pending
    assert.equal(await leaveRoute(), true, `completed ${operation}`)
  }
})

test('configuration leave guards preserve cancellation, normal navigation and unmount safety', async (t) => {
  const { view, leaveRoute, confirm, unmount } = createView(t)
  await view.loadConfig()
  assert.equal(await leaveRoute(), true)
  assert.equal(confirm.mock.callCount(), 0)
  view.formConfig.Base.Refresh = 10
  const before = JSON.stringify(view.formConfig)
  confirm.mock.mockImplementationOnce(async () => { throw new Error('cancel') })
  assert.equal(await leaveRoute(), false)
  assert.equal(JSON.stringify(view.formConfig), before)
  assert.equal(await leaveRoute(), true)
  assert.equal(JSON.stringify(view.formConfig), before)
  let finish!: () => void
  confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve }))
  const leaving = leaveRoute()
  unmount()
  const calls = confirm.mock.callCount()
  finish()
  assert.equal(await leaving, false)
  assert.equal(await leaveRoute(), false)
  assert.equal(confirm.mock.callCount(), calls)
  const preventDefault = t.mock.fn(() => {})
  const event = { preventDefault, returnValue: 'retained' } as unknown as BeforeUnloadEvent
  view.handleBeforeUnload(event)
  assert.equal(preventDefault.mock.callCount(), 0)
  assert.equal(event.returnValue, 'retained')
})

test('configuration unload prompts also protect active operations with an unchanged draft', async (t) => {
  for (const operation of ['save', 'import', 'export']) {
    const { view, saveConfig, verifyConfigPassword } = createView(t)
    await view.loadConfig()
    assert.equal(view.isDirty.value, false)
    let finish!: () => void
    let pending: Promise<void>
    if (operation === 'save') {
      // The user can restore the loaded value while the submitted edit is pending.
      view.formConfig.Base.Refresh = 10
      view.password.value = 'save-password'
      saveConfig.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve }))
      pending = view.handleSave()
      view.formConfig.Base.Refresh = 5
    } else {
      view.importExportPassword.value = 'file-password'
      verifyConfigPassword.mock.mockImplementationOnce(() => new Promise<'valid'>((resolve) => { finish = () => resolve('valid') }))
      pending = operation === 'export' ? view.handleExport() : view.handleImportFile({ raw: { text: async () => JSON.stringify(view.formConfig) } })
    }
    const preventDefault = t.mock.fn(() => {})
    const event = { preventDefault, returnValue: undefined } as unknown as BeforeUnloadEvent
    view.handleBeforeUnload(event)
    assert.equal(preventDefault.mock.callCount(), 1, operation)
    assert.equal(event.returnValue, '')
    finish()
    await pending
    if (operation === 'save') {
      assert.equal(view.isDirty.value, true)
      view.formConfig.Base.Refresh = 10
    }
    assert.equal(view.isDirty.value, false)
    event.returnValue = 'retained'
    view.handleBeforeUnload(event)
    assert.equal(preventDefault.mock.callCount(), 1)
    assert.equal(event.returnValue, 'retained')
    view.formConfig.Base.Refresh = 20
    view.handleBeforeUnload(event)
    assert.equal(preventDefault.mock.callCount(), 2)
    assert.equal(event.returnValue, '')
  }
})

test('configuration leave confirmations allow operations that finish before confirmation', async (t) => {
  for (const operation of ['save', 'import', 'export']) {
    const { view, leaveRoute, confirm } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    let finish!: () => void
    confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { finish = resolve }))
    const leaving = leaveRoute()
    if (operation === 'save') {
      view.password.value = 'save-password'
      await view.handleSave()
    } else {
      view.importExportPassword.value = 'file-password'
      if (operation === 'export') await view.handleExport()
      else await view.handleImportFile({ raw: { text: async () => JSON.stringify(view.formConfig) } })
    }
    const before = JSON.stringify(view.formConfig)
    finish()
    assert.equal(await leaving, true, operation)
    assert.equal(confirm.mock.callCount(), 1)
    assert.equal(JSON.stringify(view.formConfig), before)
  }
})

test('configuration callbacks cannot start work or change state after unmount', async (t) => {
  for (const operation of ['load', 'save', 'export', 'import']) {
    const { view, loadConfig, saveConfig, verifyConfigPassword, createObjectURL,
      showError, showSuccess, showWarning, unmount } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    view.password.value = 'save-password'
    view.importExportPassword.value = 'file-password'
    const before = JSON.stringify(view.formConfig)
    const baseline = view.savedSnapshot.value
    const text = t.mock.fn(async () => before)
    loadConfig.mock.resetCalls()
    unmount()
    if (operation === 'load') await view.loadConfig()
    if (operation === 'save') await view.handleSave()
    if (operation === 'export') await view.handleExport()
    if (operation === 'import') await view.handleImportFile({ raw: { text } })
    assert.equal(loadConfig.mock.callCount(), 0, operation)
    assert.equal(saveConfig.mock.callCount(), 0, operation)
    assert.equal(verifyConfigPassword.mock.callCount(), 0, operation)
    assert.equal(text.mock.callCount(), 0, operation)
    assert.equal(createObjectURL.mock.callCount(), 0, operation)
    assert.equal(showError.mock.callCount() + showSuccess.mock.callCount() + showWarning.mock.callCount(), 0)
    assert.equal(JSON.stringify(view.formConfig), before)
    assert.equal(view.savedSnapshot.value, baseline)
    assert.equal(view.password.value, 'save-password')
    assert.equal(view.importExportPassword.value, 'file-password')
    assert.equal(view.loadingConfig.value || view.saving.value || view.importing.value || view.exporting.value, false)
  }
})

test('configuration actions wait for a successful load and recover after loading', async (t) => {
  for (const state of ['initial', 'failed', 'reloading']) {
    for (const operation of ['save', 'export', 'import']) {
      const { view, loadConfig, saveConfig, verifyConfigPassword, createObjectURL,
        showError, showSuccess, showWarning } = createView(t)
      const loaded = (await loadConfig())!
      // A valid draft alone must not authorize saving or exporting an unloaded configuration.
      Object.assign(view.formConfig, loaded)
      if (state === 'failed') {
        loadConfig.mock.mockImplementationOnce(async () => null)
        await view.loadConfig()
      } else if (state === 'reloading') await view.loadConfig()
      let finish!: (config: Config) => void
      loadConfig.mock.mockImplementationOnce(() => new Promise<Config>((resolve) => { finish = resolve }))
      const pendingLoad = state === 'reloading' ? view.loadConfig() : null
      view.password.value = 'save-password'
      view.importExportPassword.value = 'file-password'
      const before = JSON.stringify(view.formConfig)
      const text = t.mock.fn(async () => JSON.stringify({ ...loaded, Toollimit: 60 }))
      const run = () => operation === 'save' ? view.handleSave() : operation === 'export'
        ? view.handleExport() : view.handleImportFile({ raw: { text } })
      await run()
      assert.equal(saveConfig.mock.callCount(), 0, `${state}/${operation}`)
      assert.equal(verifyConfigPassword.mock.callCount(), 0, `${state}/${operation}`)
      assert.equal(text.mock.callCount(), 0)
      assert.equal(createObjectURL.mock.callCount(), 0)
      assert.equal(showError.mock.callCount() + showSuccess.mock.callCount() + showWarning.mock.callCount(), 0)
      assert.equal(JSON.stringify(view.formConfig), before)
      assert.equal(view.password.value, 'save-password')
      assert.equal(view.importExportPassword.value, 'file-password')
      const loading = pendingLoad ?? view.loadConfig()
      finish({ ...loaded, Name: 'updated', Port: 9000 })
      await loading
      assert.equal(view.configLoaded.value, true)
      assert.equal(view.loadingConfig.value, false)
      assert.equal(view.configReady.value, true)
      await run()
      assert.equal(showSuccess.mock.callCount(), 1)
      if (operation === 'save') {
        assert.equal(saveConfig.mock.calls[0]!.arguments[0].Name, 'updated')
        assert.equal(saveConfig.mock.calls[0]!.arguments[0].Port, 9000)
      } else if (operation === 'export') {
        const exported = JSON.parse(await createObjectURL.mock.calls[0]!.arguments[0].text())
        assert.equal(exported.Name, 'updated')
        assert.equal(exported.Port, 9000)
      } else {
        assert.equal(view.formConfig.Name, 'updated')
        assert.equal(view.formConfig.Port, 9000)
        assert.equal(view.formConfig.Toollimit, 60)
      }
    }
  }
})

test('configuration reloads cannot overwrite a pending save, import or export', async (t) => {
  for (const operation of ['save', 'export', 'import']) {
    const { view, loadConfig, saveConfig, verifyConfigPassword, createObjectURL, showSuccess } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    view.password.value = 'save-password'
    view.importExportPassword.value = 'file-password'
    const baseline = view.savedSnapshot.value
    const content = JSON.stringify({ ...view.formConfig, Toollimit: 60 })
    let finish!: () => void
    const gate = new Promise<void>((resolve) => { finish = resolve })
    let readStarted!: () => void
    const reading = new Promise<void>((resolve) => { readStarted = resolve })
    if (operation === 'save') saveConfig.mock.mockImplementation(async () => { await gate })
    if (operation === 'export') verifyConfigPassword.mock.mockImplementation(async () => { await gate; return 'valid' })
    const pending = operation === 'save' ? view.handleSave() : operation === 'export'
      ? view.handleExport() : view.handleImportFile({ raw: { text: async () => {
        readStarted()
        await gate
        return content
      } } })
    if (operation === 'import') await reading
    view.formConfig.Base.Refresh = 15
    const draft = JSON.stringify(view.formConfig)
    const signal = operation === 'save' ? saveConfig.mock.calls[0]!.arguments[2]!
      : verifyConfigPassword.mock.calls[0]!.arguments[1]!
    await view.loadConfig()
    assert.equal(loadConfig.mock.callCount(), 1, operation)
    assert.equal(JSON.stringify(view.formConfig), draft)
    assert.equal(view.savedSnapshot.value, baseline)
    assert.equal(view.loadingConfig.value, false)
    assert.equal(signal.aborted, false)
    finish()
    await pending
    assert.equal(showSuccess.mock.callCount(), 1)
    if (operation === 'save') {
      assert.equal(view.formConfig.Base.Refresh, 15)
      assert.equal(JSON.parse(view.savedSnapshot.value).Base.Refresh, 10)
      assert.equal(view.isDirty.value, true)
    } else if (operation === 'export') {
      assert.equal(JSON.parse(await createObjectURL.mock.calls[0]!.arguments[0].text()).Base.Refresh, 15)
      assert.equal(view.savedSnapshot.value, baseline)
    } else {
      assert.equal(view.formConfig.Toollimit, 60)
      assert.equal(view.formConfig.Base.Refresh, 10)
      assert.equal(view.savedSnapshot.value, baseline)
    }
    await view.loadConfig()
    assert.equal(loadConfig.mock.callCount(), 2)
    assert.equal(view.formConfig.Base.Refresh, 5)
    assert.equal(view.isDirty.value, false)
  }
})

test('configuration loading ignores obsolete and unmounted completions', async (t) => {
  const { view, loadConfig, unmount } = createView(t)
  await view.loadConfig()
  const loaded = JSON.parse(JSON.stringify(view.formConfig)) as Config
  const baseline = view.savedSnapshot.value
  let finishFirst!: (config: Config | null) => void
  let finishSecond!: (config: Config | null) => void
  loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finishFirst = resolve }))
  const first = view.loadConfig()
  loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finishSecond = resolve }))
  const second = view.loadConfig()
  finishFirst(null)
  await first
  assert.equal(view.loadingConfig.value, true)
  assert.equal(view.configError.value, false)
  assert.equal(view.configReady.value, false)
  assert.equal(view.savedSnapshot.value, baseline)
  finishSecond({ ...loaded, Name: 'latest', Port: 9001 })
  await second
  assert.equal(view.formConfig.Name, 'latest')
  assert.equal(view.formConfig.Port, 9001)
  assert.equal(view.configReady.value, true)
  assert.equal(view.isDirty.value, false)

  for (const result of [null, { ...loaded, Name: 'obsolete' }]) {
    const fixture = createView(t)
    await fixture.view.loadConfig()
    let finish!: (config: Config | null) => void
    fixture.loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = resolve }))
    const before = JSON.stringify(fixture.view.formConfig)
    const saved = fixture.view.savedSnapshot.value
    const pending = fixture.view.loadConfig()
    fixture.unmount()
    finish(result)
    await pending
    assert.equal(JSON.stringify(fixture.view.formConfig), before)
    assert.equal(fixture.view.savedSnapshot.value, saved)
    assert.equal(fixture.view.configError.value, false)
  }
  unmount()
})

test('configuration saves abort on unmount and ignore late success or failure', async (t) => {
  for (const outcome of ['success', 'failure', 'canceled']) {
    const { view, saveConfig, showError, showSuccess, unmount } = createView(t)
    await view.loadConfig()
    const baseline = view.savedSnapshot.value
    view.formConfig.Base.Refresh = 10
    view.password.value = 'password'
    let finish!: () => void
    let fail!: (error: Error) => void
    saveConfig.mock.mockImplementationOnce(() => new Promise<void>((resolve, reject) => {
      finish = resolve
      fail = reject
    }))
    const pending = view.handleSave()
    const signal = saveConfig.mock.calls[0]!.arguments[2]!
    assert.equal(signal.aborted, false)
    view.formConfig.Base.Refresh = 15
    const before = JSON.stringify(view.formConfig)
    unmount()
    assert.equal(signal.aborted, true)
    if (outcome === 'success') finish()
    else fail(outcome === 'canceled' ? new CanceledError() : new Error('late failure'))
    await pending
    assert.equal(view.savedSnapshot.value, baseline)
    assert.equal(JSON.stringify(view.formConfig), before)
    assert.equal(showError.mock.callCount() + showSuccess.mock.callCount(), 0)
  }
})

test('province deletion applies only to the confirmed province and keeps a newer editor open', async (t) => {
  const { view, confirm, showSuccess } = createView(t)
  await view.loadConfig()
  view.formConfig.Chinamap = {
    Shanghai: { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] },
    Beijing: { ctcc: ['192.0.2.2'], cucc: [], cmcc: [] }
  }
  view.editChinaMap('Shanghai')
  let approve!: () => void
  confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { approve = resolve }))
  const pending = view.deleteChinaMap()
  view.editChinaMap('Beijing')
  approve()
  await pending
  assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, 'Shanghai'), false)
  assert.equal(view.formConfig.Chinamap.Beijing!.ctcc[0], '192.0.2.2')
  assert.equal(view.currentProvince.value, 'Beijing')
  assert.equal(view.chinaMapVisible.value, true)
  assert.equal(showSuccess.mock.calls[0]!.arguments[0], 'config.provinceDeleted')
})

test('pending deletions cannot mutate a reloaded, imported, replaced or unmounted draft', async (t) => {
  for (const operation of ['node', 'province']) {
    for (const change of ['reload', 'import', 'replace', 'remove', 'unmount', 'loading']) {
      const { view, loadConfig, confirm, showSuccess, unmount } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
      view.formConfig.Network['127.0.0.1']!.Ping = [addr]
      view.formConfig.Chinamap.Shanghai = { ctcc: [addr], cucc: [], cmcc: [] }
      view.editChinaMap('Shanghai')
      const row = view.networkList.value.find((node) => node.Addr === addr)!
      let approve!: () => void
      confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { approve = resolve }))
      const pending = operation === 'node' ? view.deleteNode(row) : view.deleteChinaMap()
      let finishLoad!: () => void
      let pendingLoad: Promise<void> | undefined
      if (change === 'reload') {
        const refreshed = JSON.parse(JSON.stringify(view.formConfig)) as Config
        refreshed.Network[addr]!.Name = 'new remote'
        loadConfig.mock.mockImplementationOnce(async () => refreshed)
        await view.loadConfig()
      } else if (change === 'import') {
        view.importExportPassword.value = 'password'
        const imported = JSON.parse(JSON.stringify(view.formConfig)) as Config
        imported.Toollimit = 60
        await view.handleImportFile({ raw: { text: async () => JSON.stringify(imported) } })
      } else if (change === 'replace') {
        if (operation === 'node') view.formConfig.Network[addr] = { ...view.formConfig.Network[addr]!, Name: 'replacement' }
        else view.formConfig.Chinamap.Shanghai = { ctcc: ['192.0.2.2'], cucc: [], cmcc: [] }
      } else if (change === 'remove') {
        if (operation === 'node') delete view.formConfig.Network[addr]
        else delete view.formConfig.Chinamap.Shanghai
      } else if (change === 'unmount') unmount()
      else {
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config>((resolve) => {
          finishLoad = () => resolve(JSON.parse(JSON.stringify(view.formConfig)) as Config)
        }))
        pendingLoad = view.loadConfig()
      }
      const before = JSON.stringify(view.formConfig)
      const saved = view.savedSnapshot.value
      const previousSuccesses = showSuccess.mock.callCount()
      approve()
      await pending
      assert.equal(JSON.stringify(view.formConfig), before, `${operation}/${change}`)
      assert.equal(view.savedSnapshot.value, saved)
      assert.equal(showSuccess.mock.callCount(), previousSuccesses)
      if (pendingLoad) { finishLoad(); await pendingLoad }
    }
  }
})

test('deletion entries reject unloaded, unmounted, stale and self node targets', async (t) => {
  for (const operation of ['node', 'province']) {
    for (const state of ['unmounted', 'loading', 'unloaded', 'stale', 'missing', 'self']) {
      const { view, loadConfig, confirm, showSuccess, showError, showWarning, unmount } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: false, Ping: [], Topology: [] }
      view.formConfig.Chinamap.Shanghai = { ctcc: [], cucc: [], cmcc: [] }
      view.editChinaMap('Shanghai')
      let row = view.networkList.value.find((node) => node.Addr === addr)!
      let finish!: () => void
      let loading: Promise<void> | undefined
      if (state === 'unmounted') unmount()
      else if (state === 'loading') {
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = () => resolve(null) }))
        loading = view.loadConfig()
      } else if (state === 'unloaded') view.configLoaded.value = false
      else if (state === 'stale') {
        if (operation === 'node') view.formConfig.Network[addr] = { ...view.formConfig.Network[addr]! }
        else view.currentProvince.value = 'missing'
      } else if (state === 'missing') {
        delete view.formConfig.Network[addr]
        delete view.formConfig.Chinamap.Shanghai
      } else if (operation === 'node') row = view.networkList.value.find((node) => node.isSelf)!
      else continue
      const before = JSON.stringify(view.formConfig)
      if (operation === 'node') await view.deleteNode(row)
      else await view.deleteChinaMap()
      assert.equal(confirm.mock.callCount(), 0, `${operation}/${state}`)
      assert.equal(JSON.stringify(view.formConfig), before)
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), 0)
      if (loading) { finish(); await loading }
    }
  }
})

test('deletion cancellation is silent and confirmed deletes clean references exactly once', async (t) => {
  for (const operation of ['node', 'province']) {
    const { view, confirm, showSuccess, showError, showWarning } = createView(t)
    await view.loadConfig()
    const addr = '192.0.2.1'
    view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
    view.formConfig.Network['127.0.0.1']!.Ping = [addr, addr, '127.0.0.1']
    view.formConfig.Network['127.0.0.1']!.Topology = [{
      Name: 'remote', Addr: addr, Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30'
    }]
    Object.defineProperty(view.formConfig.Chinamap, '__proto__', {
      value: { ctcc: [addr], cucc: [], cmcc: [] }, enumerable: true, writable: true, configurable: true
    })
    view.editChinaMap('__proto__')
    const row = view.networkList.value.find((node) => node.Addr === addr)!
    const run = () => operation === 'node' ? view.deleteNode(row) : view.deleteChinaMap()
    confirm.mock.mockImplementationOnce(async () => { throw 'cancel' })
    const before = JSON.stringify(view.formConfig)
    await run()
    assert.equal(JSON.stringify(view.formConfig), before)
    assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), 0)
    let approve!: () => void
    confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { approve = resolve }))
    const first = run()
    let approveSecond!: () => void
    confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { approveSecond = resolve }))
    const second = run()
    approve()
    await first
    const deleted = JSON.stringify(view.formConfig)
    approveSecond()
    await second
    assert.equal(JSON.stringify(view.formConfig), deleted)
    assert.equal(showSuccess.mock.callCount(), 1)
    if (operation === 'node') {
      assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Network, addr), false)
      assert.equal(view.formConfig.Network['127.0.0.1']!.Ping.join(','), '127.0.0.1')
      assert.equal(view.formConfig.Network['127.0.0.1']!.Topology.length, 0)
    } else {
      assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, '__proto__'), false)
      assert.equal(view.chinaMapVisible.value, false)
    }
    await run()
    assert.equal(confirm.mock.callCount(), 3)
    assert.equal(showSuccess.mock.callCount(), 1)
  }
})

test('editing dialogs cannot apply old drafts after replacement, loading, closing or unmount', async (t) => {
  for (const operation of ['node', 'ping', 'topology', 'province']) {
    for (const change of ['reload', 'import', 'replace', 'remove', 'close', 'unmount', 'loading']) {
      const { view, loadConfig, showSuccess, showWarning, showError, unmount } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
      view.formConfig.Chinamap.Shanghai = { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] }
      const row = view.networkList.value.find((node) => node.Addr === addr)!
      const visibility = operation === 'node' ? view.editNodeVisible : operation === 'ping'
        ? view.pingConfigVisible : operation === 'topology' ? view.topoConfigVisible : view.chinaMapVisible
      const run = () => operation === 'node' ? view.saveEditNode() : operation === 'ping'
        ? view.savePingConfig() : operation === 'topology' ? view.saveTopoConfig() : view.saveChinaMap()
      if (operation === 'node') {
        view.showEditNode(row)
        view.editNodeName.value = 'stale name'
      } else if (operation === 'ping') {
        view.editPingConfig(row)
        view.pingTargetList.value.find((target) => target.Addr === '127.0.0.1')!.enabled = true
      } else if (operation === 'topology') {
        view.editTopoConfig(row)
        view.topoTargetList.value.find((target) => target.Addr === '127.0.0.1')!.enabled = true
      } else {
        view.editChinaMap('Shanghai')
        view.chinaMapIps.ctcc = '192.0.2.2'
      }
      let finish!: () => void
      let pendingLoad: Promise<void> | undefined
      if (change === 'reload') {
        const refreshed = JSON.parse(JSON.stringify(view.formConfig)) as Config
        refreshed.Toollimit = 60
        loadConfig.mock.mockImplementationOnce(async () => refreshed)
        await view.loadConfig()
      } else if (change === 'import') {
        view.importExportPassword.value = 'password'
        const content = JSON.stringify({ ...view.formConfig, Toollimit: 60 })
        await view.handleImportFile({ raw: { text: async () => content } })
      } else if (change === 'replace') {
        if (operation === 'province') view.formConfig.Chinamap.Shanghai = { ctcc: ['192.0.2.3'], cucc: [], cmcc: [] }
        else view.formConfig.Network[addr] = { ...view.formConfig.Network[addr]!, Name: 'replacement' }
      } else if (change === 'remove') {
        if (operation === 'province') delete view.formConfig.Chinamap.Shanghai
        else delete view.formConfig.Network[addr]
      } else if (change === 'close') visibility.value = false
      else if (change === 'unmount') unmount()
      else {
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = () => resolve(null) }))
        pendingLoad = view.loadConfig()
      }
      const before = JSON.stringify(view.formConfig)
      const baseline = view.savedSnapshot.value
      const messages = showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount()
      run()
      assert.equal(JSON.stringify(view.formConfig), before, `${operation}/${change}`)
      assert.equal(view.savedSnapshot.value, baseline)
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), messages)
      if (pendingLoad) {
        finish()
        await pendingLoad
        run()
        assert.equal(showSuccess.mock.callCount(), 1)
      } else if (change === 'reload' || change === 'import') assert.equal(visibility.value, false)
    }
  }
})

test('Ping and topology editors keep their own source when both are open', async (t) => {
  const { view, showSuccess } = createView(t)
  await view.loadConfig()
  const addr = '192.0.2.1'
  view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
  const local = view.networkList.value.find((row) => row.isSelf)!
  const remote = view.networkList.value.find((row) => row.Addr === addr)!
  view.editPingConfig(local)
  view.pingTargetList.value.find((target) => target.Addr === addr)!.enabled = true
  view.editTopoConfig(remote)
  const rule = view.topoTargetList.value.find((target) => target.Addr === local.Addr)!
  Object.assign(rule, { enabled: true, checkSeconds: 1200, occurrenceCount: 4, avgDelay: 250, lossPercent: 15 })
  view.savePingConfig()
  assert.equal(local._original.Ping.join(','), addr)
  assert.equal(remote._original.Ping.length, 0)
  assert.equal(view.topoConfigVisible.value, true)
  view.saveTopoConfig()
  assert.equal(local._original.Topology.length, 0)
  assert.equal(remote._original.Topology[0]!.Addr, local.Addr)
  assert.equal(remote._original.Topology[0]!.Thdchecksec, '1200')
  assert.equal(remote._original.Topology[0]!.Thdoccnum, '4')
  assert.equal(remote._original.Topology[0]!.Thdavgdelay, '250')
  assert.equal(remote._original.Topology[0]!.Thdloss, '15')
  assert.equal(showSuccess.mock.callCount(), 2)
  view.savePingConfig()
  view.saveTopoConfig()
  assert.equal(showSuccess.mock.callCount(), 2)
  assert.equal(validation.validateConfigForEdit(view.formConfig), null)
})

test('Ping and topology editing cannot reintroduce removed or replaced targets and uses current names', async (t) => {
  for (const operation of ['ping', 'topology']) {
    for (const change of ['remove', 'replace', 'move', 'rename']) {
      const { view, showSuccess } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
      const local = view.networkList.value.find((row) => row.isSelf)!
      const remote = view.networkList.value.find((row) => row.Addr === addr)!
      const visible = operation === 'ping' ? view.pingConfigVisible : view.topoConfigVisible
      if (operation === 'ping') {
        view.editPingConfig(local)
        view.pingTargetList.value.find((target) => target.Addr === addr)!.enabled = true
      } else {
        view.editTopoConfig(local)
        view.topoTargetList.value.find((target) => target.Addr === addr)!.enabled = true
      }
      if (change === 'remove') await view.deleteNode(remote)
      else if (change === 'replace') view.formConfig.Network[addr] = { ...remote._original, Name: 'replacement' }
      else {
        view.showEditNode(remote)
        view.editNodeName.value = 'renamed'
        if (change === 'move') view.editNodeAddr.value = '192.0.2.10'
        view.saveEditNode()
      }
      const before = JSON.stringify(view.formConfig)
      const count = showSuccess.mock.callCount()
      if (operation === 'ping') view.savePingConfig()
      else view.saveTopoConfig()
      if (change === 'rename') {
        assert.equal(showSuccess.mock.callCount(), count + 1)
        if (operation === 'ping') assert.equal(local._original.Ping.join(','), addr)
        else {
          assert.equal(local._original.Topology[0]!.Name, 'renamed')
          assert.equal(local._original.Topology[0]!.Addr, addr)
        }
        assert.equal(validation.validateConfigForEdit(view.formConfig), null)
      } else {
        assert.equal(JSON.stringify(view.formConfig), before, `${operation}/${change}`)
        assert.equal(showSuccess.mock.callCount(), count)
      }
      assert.equal(visible.value, false)
    }
  }
})

test('editing entries ignore unmounted, unloaded, loading and stale node rows', async (t) => {
  for (const operation of ['node', 'ping', 'topology', 'province']) {
    for (const state of ['unmount', 'unloaded', 'loading', 'missing', 'stale']) {
      if (state === 'stale' && operation === 'province') continue
      const { view, loadConfig, showSuccess, showWarning, showError, unmount } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
      view.formConfig.Chinamap.Shanghai = { ctcc: [], cucc: [], cmcc: [] }
      const row = view.networkList.value.find((node) => node.Addr === addr)!
      let finish!: () => void
      let pending: Promise<void> | undefined
      if (state === 'unmount') unmount()
      else if (state === 'unloaded') view.configLoaded.value = false
      else if (state === 'missing') {
        delete view.formConfig.Network[addr]
        delete view.formConfig.Chinamap.Shanghai
      } else if (state === 'stale') view.formConfig.Network[addr] = { ...row._original }
      else {
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = () => resolve(null) }))
        pending = view.loadConfig()
      }
      const before = JSON.stringify(view.formConfig)
      if (operation === 'node') view.showEditNode(row)
      else if (operation === 'ping') view.editPingConfig(row)
      else if (operation === 'topology') view.editTopoConfig(row)
      else view.editChinaMap('Shanghai')
      assert.equal(view.editNodeVisible.value || view.pingConfigVisible.value ||
        view.topoConfigVisible.value || view.chinaMapVisible.value, false, `${operation}/${state}`)
      assert.equal(JSON.stringify(view.formConfig), before)
      assert.equal(showSuccess.mock.callCount() + showWarning.mock.callCount() + showError.mock.callCount(), 0)
      if (pending) { finish(); await pending }
    }
  }
})

test('editors reopen after configuration replacement and save each draft once', async (t) => {
  for (const operation of ['node', 'ping', 'topology', 'province']) {
    for (const replacement of ['reload', 'import']) {
      const { view, loadConfig, showSuccess } = createView(t)
      await view.loadConfig()
      const addr = '192.0.2.1'
      view.formConfig.Network[addr] = { Name: 'remote', Addr: addr, Smartping: true, Ping: [], Topology: [] }
      Object.defineProperty(view.formConfig.Chinamap, '__proto__', {
        value: { ctcc: [addr], cucc: [], cmcc: [] }, enumerable: true, writable: true, configurable: true
      })
      const open = () => {
        const row = view.networkList.value.find((node) => node.Addr === addr)!
        if (operation === 'node') view.showEditNode(row)
        else if (operation === 'ping') view.editPingConfig(row)
        else if (operation === 'topology') view.editTopoConfig(row)
        else view.editChinaMap('__proto__')
      }
      const save = () => operation === 'node' ? view.saveEditNode() : operation === 'ping'
        ? view.savePingConfig() : operation === 'topology' ? view.saveTopoConfig() : view.saveChinaMap()
      open()
      const next = JSON.parse(JSON.stringify(view.formConfig)) as Config
      next.Network[addr]!.Name = 'new remote'
      if (replacement === 'reload') {
        loadConfig.mock.mockImplementationOnce(async () => next)
        await view.loadConfig()
      } else {
        view.importExportPassword.value = 'password'
        await view.handleImportFile({ raw: { text: async () => JSON.stringify(next) } })
      }
      const before = JSON.stringify(view.formConfig)
      const baseline = view.savedSnapshot.value
      const count = showSuccess.mock.callCount()
      save()
      assert.equal(JSON.stringify(view.formConfig), before)
      assert.equal(showSuccess.mock.callCount(), count)
      open()
      if (operation === 'node') view.editNodeName.value = 'fresh name'
      else if (operation === 'ping') view.pingTargetList.value.find((target) => target.Addr === '127.0.0.1')!.enabled = true
      else if (operation === 'topology') view.topoTargetList.value.find((target) => target.Addr === '127.0.0.1')!.enabled = true
      else view.chinaMapIps.ctcc = '192.0.2.2'
      save()
      assert.equal(showSuccess.mock.callCount(), count + 1)
      assert.equal(view.savedSnapshot.value, baseline)
      assert.equal(view.isDirty.value, true)
      if (operation === 'node') assert.equal(view.formConfig.Network[addr]!.Name, 'fresh name')
      else if (operation === 'ping') assert.equal(view.formConfig.Network[addr]!.Ping.join(','), '127.0.0.1')
      else if (operation === 'topology') assert.equal(view.formConfig.Network[addr]!.Topology[0]!.Addr, '127.0.0.1')
      else assert.equal(view.formConfig.Chinamap['__proto__']!.ctcc.join(','), '192.0.2.2')
      const saved = JSON.stringify(view.formConfig)
      save()
      assert.equal(JSON.stringify(view.formConfig), saved)
      assert.equal(showSuccess.mock.callCount(), count + 1)
      assert.equal(validation.validateConfigForEdit(view.formConfig), null)
    }
  }
})

test('new node and province drafts cannot be applied after closing, replacement or unmount', async (t) => {
  for (const operation of ['node', 'province']) {
    for (const change of ['close', 'reload', 'import', 'replace', 'unmount', 'loading']) {
      const { view, loadConfig, showSuccess, showError, showWarning, unmount } = createView(t)
      await view.loadConfig()
      const visible = operation === 'node' ? view.addNodeVisible : view.addProvinceVisible
      const open = () => operation === 'node' ? view.showAddNode() : view.showAddChinaMap()
      const run = () => operation === 'node' ? view.addNode() : view.addProvince()
      const fill = () => {
        view.newNodeName.value = 'new remote'
        view.newNodeAddr.value = '192.0.2.1'
        view.newProvinceName.value = 'Shanghai'
      }
      open()
      fill()
      let finish!: () => void
      let pendingLoad: Promise<void> | undefined
      if (change === 'close') visible.value = false
      else if (change === 'reload') await view.loadConfig()
      else if (change === 'import') {
        view.importExportPassword.value = 'password'
        const content = JSON.stringify({ ...view.formConfig, Toollimit: 60 })
        await view.handleImportFile({ raw: { text: async () => content } })
      } else if (change === 'replace') {
        if (operation === 'node') view.formConfig.Network = { ...view.formConfig.Network }
        else view.formConfig.Chinamap = { ...view.formConfig.Chinamap }
      } else if (change === 'unmount') unmount()
      else {
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = () => resolve(null) }))
        pendingLoad = view.loadConfig()
      }
      const before = JSON.stringify(view.formConfig)
      const baseline = view.savedSnapshot.value
      const messages = showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount()
      run()
      assert.equal(JSON.stringify(view.formConfig), before, `${operation}/${change}`)
      assert.equal(view.savedSnapshot.value, baseline)
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), messages)
      if (pendingLoad) {
        finish()
        await pendingLoad
        run()
      } else if (change !== 'unmount') {
        if (change === 'reload' || change === 'import') assert.equal(visible.value, false)
        open()
        fill()
        run()
      } else continue
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), messages + 1)
      assert.equal(visible.value, false)
      assert.equal(view.savedSnapshot.value, baseline)
      assert.equal(view.isDirty.value, true)
      const after = JSON.stringify(view.formConfig)
      run()
      assert.equal(JSON.stringify(view.formConfig), after)
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), messages + 1)
      assert.equal(validation.validateConfigForEdit(view.formConfig), null)
    }
  }
})

test('new draft entries cannot open before loading or after leaving the configuration page', async (t) => {
  for (const operation of ['node', 'province']) {
    for (const state of ['initial', 'failed', 'loading', 'unmount']) {
      const { view, loadConfig, showSuccess, showError, showWarning, unmount } = createView(t)
      let finish!: () => void
      let pending: Promise<void> | undefined
      if (state === 'failed') {
        loadConfig.mock.mockImplementationOnce(async () => null)
        await view.loadConfig()
      } else if (state === 'loading') {
        await view.loadConfig()
        loadConfig.mock.mockImplementationOnce(() => new Promise<Config | null>((resolve) => { finish = () => resolve(null) }))
        pending = view.loadConfig()
      } else if (state === 'unmount') { await view.loadConfig(); unmount() }
      view.newNodeName.value = 'retained node'
      view.newNodeAddr.value = '192.0.2.1'
      view.newProvinceName.value = 'retained province'
      const before = JSON.stringify(view.formConfig)
      if (operation === 'node') { view.showAddNode(); view.addNode() }
      else { view.showAddChinaMap(); view.addProvince() }
      assert.equal(view.addNodeVisible.value || view.addProvinceVisible.value, false, `${operation}/${state}`)
      assert.equal(view.newNodeName.value, 'retained node')
      assert.equal(view.newNodeAddr.value, '192.0.2.1')
      assert.equal(view.newProvinceName.value, 'retained province')
      assert.equal(JSON.stringify(view.formConfig), before)
      assert.equal(showSuccess.mock.callCount() + showError.mock.callCount() + showWarning.mock.callCount(), 0)
      if (pending) { finish(); await pending }
    }
  }
})

test('new drafts retain validation feedback and can be corrected before a single successful add', async (t) => {
  const { view, showWarning, showSuccess } = createView(t)
  await view.loadConfig()
  view.showAddNode()
  view.newNodeName.value = ' \t '
  view.newNodeAddr.value = '192.0.2.1'
  view.addNode()
  assert.equal(showWarning.mock.calls[0]!.arguments[0], 'config.pleaseEnterNodeAndIP')
  view.newNodeName.value = 'remote'
  view.newNodeAddr.value = '256.1.1.1'
  view.addNode()
  assert.equal(showWarning.mock.calls[1]!.arguments[0], 'config.pleaseEnterValidIPv4')
  view.newNodeAddr.value = '127.0.0.1'
  view.addNode()
  assert.equal(showWarning.mock.calls[2]!.arguments[0], 'config.nodeIPExists')
  assert.equal(view.addNodeVisible.value, true)
  view.newNodeName.value = ' remote '
  view.newNodeAddr.value = ' 192.0.2.1 '
  view.addNode()
  assert.equal(view.formConfig.Network['192.0.2.1']!.Name, 'remote')
  assert.equal(view.addNodeVisible.value, false)
  const added = JSON.stringify(view.formConfig)
  view.addNode()
  assert.equal(JSON.stringify(view.formConfig), added)
  assert.equal(showWarning.mock.callCount(), 3)
  assert.equal(showSuccess.mock.callCount(), 1)
  view.showAddChinaMap()
  view.newProvinceName.value = ' \t '
  view.addProvince()
  assert.equal(showWarning.mock.calls[3]!.arguments[0], 'config.pleaseEnterProvinceName')
  view.newProvinceName.value = ' __proto__ '
  view.addProvince()
  assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, '__proto__'), true)
  view.showAddChinaMap()
  view.newProvinceName.value = '__proto__'
  view.addProvince()
  assert.equal(showWarning.mock.calls[4]!.arguments[0], 'config.provinceExists')
  assert.equal(view.addProvinceVisible.value, true)
  view.newProvinceName.value = 'Shanghai'
  view.addProvince()
  assert.equal(view.addProvinceVisible.value, false)
  assert.equal(showSuccess.mock.callCount(), 3)
  assert.equal(validation.validateConfigForEdit(view.formConfig), null)
})

test('new node drafts respect the network limit and retry after removing a node', async (t) => {
  const { view, showError, showSuccess } = createView(t)
  await view.loadConfig()
  for (let i = 0; i < validation.CONFIG_LIMITS.networkNodes - 1; i++) {
    const addr = `198.18.${Math.floor(i / 256)}.${i % 256}`
    view.formConfig.Network[addr] = { Name: `node ${i}`, Addr: addr, Smartping: false, Ping: [], Topology: [] }
  }
  assert.equal(validation.validateConfigForEdit(view.formConfig), null)
  view.showAddNode()
  view.newNodeName.value = 'extra node'
  view.newNodeAddr.value = '192.0.2.1'
  const before = JSON.stringify(view.formConfig)
  view.addNode()
  assert.ok(JSON.stringify(view.formConfig) === before, 'Adding beyond the limit changed the draft')
  assert.equal(showError.mock.calls[0]!.arguments[0], 'config.validationNetworkLimit')
  assert.equal(view.addNodeVisible.value, true)
  assert.equal(showSuccess.mock.callCount(), 0)
  const old = view.networkList.value.find((row) => row.Addr === '198.18.0.0')!
  await view.deleteNode(old)
  view.addNode()
  assert.equal(view.addNodeVisible.value, false)
  assert.equal(view.formConfig.Network['192.0.2.1']!.Name, 'extra node')
  assert.equal(Object.keys(view.formConfig.Network).length, validation.CONFIG_LIMITS.networkNodes)
  assert.equal(validation.validateConfigForEdit(view.formConfig), null)
})

test('configuration export omits credentials and releases its download URL', async (t) => {
  const { view, createObjectURL, revokeObjectURL, link, showSuccess, showError } = createView(t)
  await view.loadConfig()
  Object.assign(view.formConfig, { Password: 'private-config-password' })
  const before = JSON.stringify(view.formConfig)
  view.importExportPassword.value = 'verification-password'
  await view.handleExport()
  const blob = createObjectURL.mock.calls[0]!.arguments[0]
  const exported = JSON.parse(await blob.text())
  assert.equal(blob.type, 'application/json')
  assert.equal(Object.prototype.hasOwnProperty.call(exported, 'Password'), false)
  assert.equal(Object.prototype.hasOwnProperty.call(exported, 'Ver'), false)
  assert.equal(exported.Name, view.formConfig.Name)
  assert.equal(JSON.stringify(view.formConfig), before)
  assert.equal(link.href, 'blob:config-export')
  assert.match(link.download, /^smartping-config-\d{4}-\d{2}-\d{2}\.json$/)
  assert.equal(link.click.mock.callCount(), 1)
  assert.equal(revokeObjectURL.mock.calls[0]!.arguments[0], link.href)
  assert.equal(showSuccess.mock.calls[0]!.arguments[0], 'config.configExported')
  assert.equal(showError.mock.callCount(), 0)
  assert.equal(view.exporting.value, false)
  assert.equal(view.importExportPassword.value, '')
})

test('download failures release the export URL and allow another attempt', async (t) => {
  for (const failure of ['create', 'click']) {
    const { view, createElement, link, revokeObjectURL, showError, showSuccess } = createView(t)
    await view.loadConfig()
    const fail = () => { throw new Error('download unavailable') }
    if (failure === 'create') createElement.mock.mockImplementationOnce(fail)
    else link.click.mock.mockImplementationOnce(fail)
    view.importExportPassword.value = 'password'
    await view.handleExport()
    assert.equal(revokeObjectURL.mock.callCount(), 1)
    assert.equal(revokeObjectURL.mock.calls[0]!.arguments[0], 'blob:config-export')
    assert.equal(showError.mock.calls[0]!.arguments[0], 'config.configExportFailed')
    assert.equal(showSuccess.mock.callCount(), 0)
    assert.equal(view.exporting.value, false)
    assert.equal(view.importExportPassword.value, '')

    view.importExportPassword.value = 'password'
    await view.handleExport()
    assert.equal(showSuccess.mock.callCount(), 1)
    assert.equal(revokeObjectURL.mock.callCount(), 2)
  }
})

test('config import ignores file completion and errors after leaving the page', async (t) => {
  for (const outcome of ['valid', 'invalid', 'rejected']) {
    const { view, showError, showSuccess, verifyConfigPassword, unmount } = createView(t)
    await view.loadConfig()
    const baseline = JSON.stringify(view.formConfig)
    view.importExportPassword.value = 'password'
    let resolve!: (content: string) => void
    let reject!: (error: Error) => void
    let started!: () => void
    const readStarted = new Promise<void>((done) => {
      started = done
    })
    const pending = view.handleImportFile({
      raw: {
        text: () => {
          started()
          return new Promise<string>((done, fail) => {
            resolve = done
            reject = fail
          })
        }
      }
    })
    await readStarted
    assert.equal(view.importing.value, true)
    unmount()
    assert.equal(verifyConfigPassword.mock.calls[0]!.arguments[1]?.aborted, true)
    if (outcome === 'rejected') reject(new Error('file unavailable'))
    else
      resolve(
        outcome === 'invalid'
          ? 'invalid-json'
          : JSON.stringify({ ...view.formConfig, Toollimit: 60 })
      )
    await pending
    assert.equal(JSON.stringify(view.formConfig), baseline)
    assert.equal(showError.mock.callCount(), 0)
    assert.equal(showSuccess.mock.callCount(), 0)
  }
})

test('config import still reports malformed files while the page is active', async (t) => {
  const { view, showError } = createView(t)
  await view.loadConfig()
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { text: async () => 'invalid-json' } })
  assert.equal(showError.mock.calls[0]!.arguments[0], 'config.configParseFailed')
  assert.equal(view.importing.value, false)
  assert.equal(view.importExportPassword.value, '')
})

test('password request failures release both import and export controls', async (t) => {
  for (const operation of ['import', 'export']) {
    const { view, verifyConfigPassword, showError } = createView(t)
    await view.loadConfig()
    view.importExportPassword.value = 'password'
    verifyConfigPassword.mock.mockImplementation(async () => {
      throw new Error('request timeout')
    })
    const text = t.mock.fn(async () => '{}')
    if (operation === 'import') await view.handleImportFile({ raw: { text } })
    else await view.handleExport()
    assert.equal(text.mock.callCount(), 0)
    assert.equal(view.importing.value, false)
    assert.equal(view.exporting.value, false)
    assert.equal(view.importExportPassword.value, '')
    assert.equal(showError.mock.calls[0]!.arguments[0], 'config.passwordVerifyFailed')
  }
})

test('late password verification cannot import or download after the page is unmounted', async (t) => {
  for (const operation of ['import', 'export']) {
    for (const outcome of ['valid', 'invalid', 'rate-limited', 'rejected']) {
      const { view, verifyConfigPassword, createObjectURL, showError, showSuccess, unmount } = createView(t)
      await view.loadConfig()
      const baseline = JSON.stringify(view.formConfig)
      let resolve!: (value: string) => void
      let reject!: (error: Error) => void
      verifyConfigPassword.mock.mockImplementation(() => new Promise<string>((done, fail) => {
        resolve = done
        reject = fail
      }))
      const text = t.mock.fn(async () => JSON.stringify({ ...view.formConfig, Toollimit: 60 }))
      view.importExportPassword.value = 'password'
      const pending = operation === 'import'
        ? view.handleImportFile({ raw: { text } }) : view.handleExport()
      unmount()
      assert.equal(verifyConfigPassword.mock.calls[0]!.arguments[1]?.aborted, true)
      if (outcome === 'rejected') reject(new Error('late network error'))
      else resolve(outcome)
      await pending
      assert.equal(text.mock.callCount(), 0)
      assert.equal(createObjectURL.mock.callCount(), 0)
      assert.equal(showError.mock.callCount(), 0)
      assert.equal(showSuccess.mock.callCount(), 0)
      assert.equal(JSON.stringify(view.formConfig), baseline)
    }
  }
})

test('canceled password verification releases controls without reporting a failure', async (t) => {
  for (const operation of ['import', 'export']) {
    for (const error of [new CanceledError(), new DOMException('Aborted', 'AbortError')]) {
      const { view, verifyConfigPassword, showError, showSuccess, createObjectURL } = createView(t)
      await view.loadConfig()
      verifyConfigPassword.mock.mockImplementationOnce(async () => { throw error })
      const text = t.mock.fn(async () => JSON.stringify(view.formConfig))
      const run = () => operation === 'import'
        ? view.handleImportFile({ raw: { text } }) : view.handleExport()
      view.importExportPassword.value = 'password'
      await run()
      assert.equal(text.mock.callCount(), 0)
      assert.equal(createObjectURL.mock.callCount(), 0)
      assert.equal(showError.mock.callCount(), 0)
      assert.equal(showSuccess.mock.callCount(), 0)
      assert.equal(view.importing.value, false)
      assert.equal(view.exporting.value, false)
      assert.equal(view.importExportPassword.value, '')

      view.importExportPassword.value = 'password'
      await run()
      assert.equal(showSuccess.mock.callCount(), 1)
      assert.equal(verifyConfigPassword.mock.callCount(), 2)
    }
  }
})

test('config import rejects oversized files before authentication and permits a later valid import', async (t) => {
  const { view, verifyConfigPassword, showError, showSuccess } = createView(t)
  await view.loadConfig()
  const baseline = JSON.stringify(view.formConfig)
  const text = t.mock.fn(async () => JSON.stringify({ ...view.formConfig, Toollimit: 60 }))
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { size: validation.CONFIG_LIMITS.importFileBytes + 1, text } })
  assert.equal(verifyConfigPassword.mock.callCount(), 0)
  assert.equal(text.mock.callCount(), 0)
  assert.equal(showError.mock.calls[0]!.arguments[0], 'config.configFileTooLarge')
  assert.equal(view.importing.value, false)
  assert.equal(JSON.stringify(view.formConfig), baseline)
  await view.handleImportFile({ raw: { size: validation.CONFIG_LIMITS.importFileBytes, text } })
  assert.equal(verifyConfigPassword.mock.callCount(), 1)
  assert.equal(text.mock.callCount(), 1)
  assert.equal(view.formConfig.Toollimit, 60)
  assert.equal(showSuccess.mock.calls[0]!.arguments[0], 'config.configImported')
  assert.equal(view.importing.value, false)
  assert.equal(view.importExportPassword.value, '')
})

test('imports report missing topology fields without altering active drafts and can retry', async (t) => {
  const cases = [
    ['display', 'Tline', 'config.validationLineWidth'],
    ['display', 'Tsymbolsize', 'config.validationSymbolSize'],
    ['rule', 'Name', 'config.validationTopologyTarget'],
    ['rule', 'Addr', 'config.validationTopologyTarget'],
    ['rule', 'Thdchecksec', 'config.validationTopologyRule'],
    ['rule', 'Thdoccnum', 'config.validationTopologyRule'],
    ['rule', 'Thdavgdelay', 'config.validationTopologyRule'],
    ['rule', 'Thdloss', 'config.validationTopologyRule']
  ] as const
  for (const [kind, field, expected] of cases) {
    const { view, showError, showSuccess, saveConfig } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    view.showEditNode(view.networkList.value.find((row) => row.isSelf)!)
    view.editNodeName.value = 'pending edit'
    const before = JSON.stringify(view.formConfig)
    const saved = view.savedSnapshot.value
    const candidate = JSON.parse(before) as Config
    candidate.Network['192.0.2.1'] = { Name: 'remote', Addr: '192.0.2.1', Smartping: true, Ping: [], Topology: [] }
    candidate.Network['127.0.0.1']!.Topology = [{ Name: 'remote', Addr: '192.0.2.1', Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30' }]
    const malformed = JSON.parse(JSON.stringify(candidate)) as Config
    const dictionary = (kind === 'display' ? malformed.Topology : malformed.Network['127.0.0.1']!.Topology[0]) as unknown as Record<string, string>
    delete dictionary[field]
    view.importExportPassword.value = 'password'
    await view.handleImportFile({ raw: { text: async () => JSON.stringify(malformed) } })
    assert.equal(showError.mock.calls[0]!.arguments[0], expected, `${kind}/${field}`)
    assert.equal(showSuccess.mock.callCount(), 0)
    assert.equal(JSON.stringify(view.formConfig), before)
    assert.equal(view.savedSnapshot.value, saved)
    assert.equal(view.isDirty.value, true)
    assert.equal(view.editNodeVisible.value, true)
    assert.equal(view.editNodeName.value, 'pending edit')
    assert.equal(view.importing.value, false)
    assert.equal(view.importExportPassword.value, '')
    assert.equal(saveConfig.mock.callCount(), 0)
    view.importExportPassword.value = 'password'
    await view.handleImportFile({ raw: { text: async () => JSON.stringify(candidate) } })
    assert.equal(showError.mock.callCount(), 1)
    assert.equal(showSuccess.mock.callCount(), 1)
    assert.equal(view.editNodeVisible.value, false)
    view.password.value = 'password'
    await view.handleSave()
    assert.equal(saveConfig.mock.callCount(), 1)
    assert.equal(view.isDirty.value, false)
  }
})

test('saving checks extension integers without submitting or discarding the draft', async (t) => {
  for (const name of ['custom', '__proto__']) {
    for (const value of [0.5, Number.MAX_SAFE_INTEGER + 1]) {
      const { view, saveConfig, querySelector, focus, showSuccess } = createView(t)
      await view.loadConfig()
      const saved = view.savedSnapshot.value
      const base = view.formConfig.Base as unknown as Record<string, number>
      base[name] = value
      view.password.value = 'password'
      await view.handleSave()
      await flushValidationFocus()
      assert.equal(saveConfig.mock.callCount(), 0)
      assert.equal(view.saving.value, false)
      assert.equal(view.password.value, 'password')
      assert.equal(view.currentValidationIssue.value?.key, 'config.validationBaseInteger')
      assert.equal(view.currentValidationIssue.value?.params?.name, name)
      assert.equal(view.savedSnapshot.value, saved)
      assert.equal(view.isDirty.value, true)
      assert.equal(base[name], value)
      assert.equal(querySelector.mock.calls[0]!.arguments[0], '#config-validation-summary')
      assert.equal(focus.mock.callCount(), 1)
      base[name] = 2
      assert.equal(view.currentValidationIssue.value, null)
      await view.handleSave()
      assert.equal(saveConfig.mock.callCount(), 1)
      assert.equal((saveConfig.mock.calls[0]!.arguments[0].Base as unknown as Record<string, number>)[name], 2)
      assert.equal(view.isDirty.value, false)
      assert.equal(view.password.value, '')
      assert.equal(showSuccess.mock.callCount(), 1)
    }
  }
})

test('base extension imports reject fractional and imprecise integers before replacing a draft', async (t) => {
  for (const value of [0.5, -0.5, Number.MAX_SAFE_INTEGER + 1, -Number.MAX_SAFE_INTEGER - 1]) {
    const { view, showError, showSuccess, saveConfig } = createView(t)
    await view.loadConfig()
    view.formConfig.Base.Refresh = 10
    const before = JSON.stringify(view.formConfig)
    const saved = view.savedSnapshot.value
    const candidate = JSON.parse(before) as Config
    ;(candidate.Base as unknown as Record<string, number>).__v_isReactive = value
    view.importExportPassword.value = 'password'
    await view.handleImportFile({ raw: { text: async () => JSON.stringify(candidate) } })
    assert.equal(showError.mock.calls[0]!.arguments[0], 'config.validationBaseInteger')
    assert.equal(showSuccess.mock.callCount(), 0)
    assert.equal(JSON.stringify(view.formConfig), before)
    assert.equal(view.savedSnapshot.value, saved)
    assert.equal(view.isDirty.value, true)
    assert.equal(view.importing.value, false)
    ;(candidate.Base as unknown as Record<string, number>).__v_isReactive = -1
    view.importExportPassword.value = 'password'
    await view.handleImportFile({ raw: { text: async () => JSON.stringify(candidate) } })
    assert.equal(showError.mock.callCount(), 1)
    assert.equal(showSuccess.mock.callCount(), 1)
    assert.equal((view.formConfig.Base as unknown as Record<string, number>).__v_isReactive, -1)
    view.password.value = 'password'
    await view.handleSave()
    assert.equal(saveConfig.mock.callCount(), 1)
    assert.equal((saveConfig.mock.calls[0]!.arguments[0].Base as unknown as Record<string, number>).__v_isReactive, -1)
    assert.equal(view.isDirty.value, false)
  }
})

test('config imports normalize Go nil collections and still reject malformed lists', async (t) => {
  const { view, showError, showSuccess } = createView(t)
  await view.loadConfig()
  const original = JSON.parse(JSON.stringify(view.formConfig)) as Config
  const payload = {
    ...original,
    Toollimit: 60,
    Network: {
      '127.0.0.1': { Name: 'local', Addr: '127.0.0.1', Ping: null, Topology: null }
    },
    Chinamap: { Shanghai: { ctcc: null, cucc: null, cmcc: null }, Beijing: null }
  }
  const importPayload = async (value: unknown) => {
    view.importExportPassword.value = 'password'
    await view.handleImportFile({ raw: { text: async () => JSON.stringify(value) } })
  }
  await importPayload(payload)
  assert.equal(showError.mock.callCount(), 0)
  assert.equal(showSuccess.mock.callCount(), 1)
  assert.equal(view.formConfig.Toollimit, 60)
  const node = view.formConfig.Network['127.0.0.1']!
  assert.equal(node.Smartping, false)
  assert.equal(node.Ping.length, 0)
  assert.equal(node.Topology.length, 0)
  for (const province of ['Shanghai', 'Beijing']) {
    assert.equal(JSON.stringify(view.formConfig.Chinamap[province]), '{"ctcc":[],"cucc":[],"cmcc":[]}')
  }
  const baseline = JSON.stringify(view.formConfig)
  for (const invalid of [
    ...[{ Ping: [null] }, { Ping: '127.0.0.1' }, { Topology: [null] }].map((fields) => ({
      ...payload, Network: { '127.0.0.1': { ...payload.Network['127.0.0.1'], ...fields } }
    })),
    { ...payload, Chinamap: { Shanghai: { ctcc: [null] } } },
    { ...payload, Chinamap: { Shanghai: [] } }
  ]) {
    await importPayload(invalid)
    assert.equal(JSON.stringify(view.formConfig), baseline)
  }
  assert.equal(showSuccess.mock.callCount(), 1)
  assert.equal(showError.mock.callCount(), 5)
})

test('config map names remain own data keys through editing and import', async (t) => {
  const { view, showSuccess, showError } = createView(t)
  await view.loadConfig()
  for (const name of ['constructor', '__proto__', 'toString']) {
    view.showAddChinaMap()
    view.newProvinceName.value = name
    view.addProvince()
    assert.ok(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, name))
    view.formConfig.Chinamap[name]!.ctcc.push('192.0.2.1')
    view.showAddChinaMap()
    view.newProvinceName.value = name
    view.addProvince()
    assert.equal(view.formConfig.Chinamap[name]!.ctcc.length, 1)
  }
  assert.equal(showSuccess.mock.callCount(), 3)
  const exported = JSON.stringify(view.formConfig)
  view.formConfig.Chinamap = {}
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { text: async () => exported } })
  assert.equal(showError.mock.callCount(), 0)
  for (const name of ['constructor', '__proto__', 'toString']) {
    assert.ok(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, name))
    assert.equal(view.formConfig.Chinamap[name]!.ctcc[0], '192.0.2.1')
  }
})

test('config import rejects invalid prototype-named node keys without silently dropping them', async (t) => {
  const { view, showSuccess, showError } = createView(t)
  await view.loadConfig()
  const baseline = JSON.stringify(view.formConfig)
  const candidate = JSON.parse(baseline) as Config
  candidate.Network = {
    ...candidate.Network,
    ['__proto__']: { Name: 'invalid', Addr: '192.0.2.1', Smartping: false, Ping: [], Topology: [] }
  }
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { text: async () => JSON.stringify(candidate) } })
  assert.equal(showSuccess.mock.callCount(), 0)
  assert.equal(showError.mock.calls[0]!.arguments[0], 'config.validationNodeAddress')
  assert.equal(JSON.stringify(view.formConfig), baseline)
})

test('config save retains edits made while the submitted snapshot is pending', async (t) => {
  const { view, saveConfig } = createView(t)
  await view.loadConfig()
  view.formConfig.Base.Refresh = 10
  view.password.value = 'password'
  let finish!: () => void
  saveConfig.mock.mockImplementation(
    () =>
      new Promise<void>((resolve) => {
        finish = resolve
      })
  )
  const pending = view.handleSave()
  assert.equal(saveConfig.mock.callCount(), 1)
  assert.equal(view.saving.value, true)
  view.formConfig.Base.Refresh = 15
  assert.equal(saveConfig.mock.calls[0]!.arguments[0].Base.Refresh, 10)
  finish()
  await pending
  assert.equal(view.formConfig.Base.Refresh, 15)
  assert.equal(JSON.parse(view.savedSnapshot.value).Base.Refresh, 10)
  assert.equal(view.isDirty.value, true)
  assert.equal(view.saving.value, false)
  assert.equal(view.password.value, '')
})

test('config draft comparison ignores object key order even for distinct Unicode names', async (t) => {
  const { view, loadConfig, showSuccess, showError, confirm, leaveRoute } = createView(t)
  const loaded = (await loadConfig())!
  loaded.Chinamap = {
    '\u00e9': { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] },
    'e\u0301': { ctcc: ['192.0.2.2'], cucc: [], cmcc: [] },
    '\u00c5': { ctcc: [], cucc: ['192.0.2.3'], cmcc: [] },
    'A\u030a': { ctcc: [], cucc: ['192.0.2.4'], cmcc: [] }
  }
  assert.equal('\u00e9'.localeCompare('e\u0301'), 0)
  loadConfig.mock.mockImplementation(async () => JSON.parse(JSON.stringify(loaded)) as Config)
  await view.loadConfig()
  const baseline = view.savedSnapshot.value
  const reorder = (value: unknown): unknown => {
    if (Array.isArray(value)) return value.map(reorder)
    if (typeof value === 'object' && value !== null) {
      return Object.fromEntries(Object.entries(value).reverse().map(([key, item]) => [key, reorder(item)]))
    }
    return value
  }
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { text: async () => JSON.stringify(reorder(loaded)) } })
  assert.equal(showSuccess.mock.callCount(), 1)
  assert.equal(showError.mock.callCount(), 0)
  assert.equal(view.savedSnapshot.value, baseline)
  assert.equal(view.isDirty.value, false)
  assert.equal(await leaveRoute(), true)
  assert.equal(confirm.mock.callCount(), 0)
  for (const name of ['\u00e9', 'e\u0301', '\u00c5', 'A\u030a']) {
    assert.deepEqual(JSON.parse(JSON.stringify(view.formConfig.Chinamap[name])), JSON.parse(JSON.stringify(loaded.Chinamap[name])))
  }
  view.formConfig.Chinamap['e\u0301']!.ctcc[0] = '192.0.2.10'
  assert.equal(view.isDirty.value, true)
  view.formConfig.Chinamap['e\u0301']!.ctcc[0] = '192.0.2.2'
  assert.equal(view.isDirty.value, false)
  delete view.formConfig.Chinamap['e\u0301']
  assert.equal(view.isDirty.value, true)
})

test('config draft comparison preserves array order and prototype-named properties', async (t) => {
  const { view, loadConfig, saveConfig } = createView(t)
  const loaded = (await loadConfig())!
  loaded.Network['192.0.2.1'] = { Name: 'one', Addr: '192.0.2.1', Smartping: true, Ping: [], Topology: [] }
  loaded.Network['192.0.2.2'] = { Name: 'two', Addr: '192.0.2.2', Smartping: true, Ping: [], Topology: [] }
  loaded.Network['127.0.0.1']!.Ping = ['192.0.2.1', '192.0.2.2']
  loaded.Chinamap = Object.fromEntries(['__proto__', 'constructor', 'toString', '1', '01', '10', '2', '江苏', 'Jiangsu']
    .map((key) => [key, { ctcc: ['192.0.2.1', '192.0.2.2'], cucc: [], cmcc: [] }]))
  loadConfig.mock.mockImplementation(async () => JSON.parse(JSON.stringify(loaded)) as Config)
  await view.loadConfig()
  view.formConfig.Chinamap = Object.fromEntries(Object.entries(view.formConfig.Chinamap).reverse())
  view.formConfig.Network = Object.fromEntries(Object.entries(view.formConfig.Network).reverse())
  assert.equal(view.isDirty.value, false)
  const saved = JSON.parse(view.savedSnapshot.value) as Config
  assert.equal(Object.prototype.hasOwnProperty.call(saved.Chinamap, '__proto__'), true)
  assert.equal(Object.keys(saved.Chinamap).length, Object.keys(loaded.Chinamap).length)
  view.formConfig.Network['127.0.0.1']!.Ping.reverse()
  assert.equal(view.isDirty.value, true)
  view.formConfig.Network['127.0.0.1']!.Ping.reverse()
  assert.equal(view.isDirty.value, false)
  view.editChinaMap('__proto__')
  view.chinaMapIps.ctcc = '192.0.2.2\n192.0.2.1'
  view.saveChinaMap()
  assert.equal(view.isDirty.value, true)
  view.editChinaMap('__proto__')
  view.chinaMapIps.ctcc = '192.0.2.1\n192.0.2.2'
  view.saveChinaMap()
  assert.equal(view.isDirty.value, false)
  view.formConfig.Chinamap['constructor']!.cmcc.push('192.0.2.3')
  assert.equal(view.isDirty.value, true)
  view.editChinaMap('__proto__')
  view.chinaMapIps.ctcc = '192.0.2.2\n192.0.2.1'
  view.saveChinaMap()
  view.password.value = 'password'
  await view.handleSave()
  assert.equal(saveConfig.mock.callCount(), 1)
  const submitted = saveConfig.mock.calls[0]!.arguments[0]
  assert.equal(Object.prototype.hasOwnProperty.call(submitted.Chinamap, '__proto__'), true)
  assert.equal(submitted.Chinamap['__proto__']!.ctcc.join(','), '192.0.2.2,192.0.2.1')
  assert.equal(Object.keys(submitted.Chinamap).length, Object.keys(loaded.Chinamap).length)
  assert.equal(view.isDirty.value, false)
})

test('map names matching reactive metadata remain data through load, edit, export and save', async (t) => {
  const names = ['__v_isReactive', '__v_isReadonly', '__v_isShallow', '__v_raw', '__v_skip', '__v_isRef', '__isVue', '__proto__', 'hasOwnProperty']
  for (const name of names) {
    const { view, loadConfig, saveConfig, createObjectURL, showError } = createView(t)
    const loaded = (await loadConfig())!
    loaded.Chinamap = { [name]: { ctcc: ['192.0.2.1'], cucc: ['192.0.2.2'], cmcc: [] } }
    loadConfig.mock.mockImplementation(async () => JSON.parse(JSON.stringify(loaded)) as Config)
    await view.loadConfig()
    assert.equal(view.isDirty.value, false, name)
    view.editChinaMap(name)
    assert.equal(view.chinaMapIps.ctcc, '192.0.2.1', name)
    assert.equal(view.chinaMapIps.cucc, '192.0.2.2', name)
    view.chinaMapIps.ctcc = '192.0.2.3'
    view.saveChinaMap()
    assert.equal(view.isDirty.value, true, name)
    view.importExportPassword.value = 'password'
    await view.handleExport()
    const exported = JSON.parse(await createObjectURL.mock.calls[0]!.arguments[0].text()) as Config
    assert.equal(Object.keys(exported.Chinamap).join(','), name)
    assert.equal(exported.Chinamap[name]!.ctcc.join(','), '192.0.2.3', name)
    view.password.value = 'password'
    await view.handleSave()
    assert.equal(showError.mock.callCount(), 0, name)
    assert.equal(saveConfig.mock.callCount(), 1, name)
    assert.equal(saveConfig.mock.calls[0]!.arguments[0].Chinamap[name]!.ctcc.join(','), '192.0.2.3', name)
    assert.equal(view.isDirty.value, false, name)
  }
})

test('reactive map names can be added, imported and deleted without phantom keys or stale counts', async (t) => {
  const names = ['__v_isReactive', '__v_isReadonly', '__v_isShallow', '__v_raw', '__v_skip', '__v_isRef', '__isVue', '__proto__', 'hasOwnProperty', 'toJSON']
  for (const operation of ['add', 'import']) {
    const { view, showError } = createView(t)
    await view.loadConfig()
    const baseline = view.savedSnapshot.value
    assert.equal(view.provinceCount.value, 0)
    if (operation === 'add') {
      for (const name of names) {
        view.showAddChinaMap()
        view.newProvinceName.value = name
        view.addProvince()
        view.editChinaMap(name)
        view.chinaMapIps.ctcc = '192.0.2.1'
        view.saveChinaMap()
      }
    } else {
      const imported = { ...view.formConfig, Chinamap: Object.fromEntries(names.map((name) => [name, { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] }])) }
      view.importExportPassword.value = 'password'
      await view.handleImportFile({ raw: { text: async () => JSON.stringify(imported) } })
    }
    assert.equal(view.provinceCount.value, names.length, operation)
    assert.equal(Object.keys(view.formConfig.Chinamap).sort().join(','), [...names].sort().join(','))
    assert.equal(validation.validateConfigForEdit(view.formConfig), null)
    for (const [index, name] of names.entries()) {
      assert.equal(view.formConfig.Chinamap[name]!.ctcc.join(','), '192.0.2.1', name)
      view.editChinaMap(name)
      await view.deleteChinaMap()
      assert.equal(view.provinceCount.value, names.length - index - 1)
      assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, name), false)
    }
    assert.equal(view.savedSnapshot.value, baseline)
    assert.equal(view.isDirty.value, false)
    assert.equal(showError.mock.callCount(), 0)
  }
})

test('map changes keep independent province editors and pending deletions valid', async (t) => {
  const { view, loadConfig, confirm, showError, showSuccess } = createView(t)
  const loaded = (await loadConfig())!
  loaded.Chinamap = { Shanghai: { ctcc: ['192.0.2.1'], cucc: [], cmcc: [] }, Beijing: { ctcc: ['192.0.2.2'], cucc: [], cmcc: [] } }
  loadConfig.mock.mockImplementation(async () => loaded)
  await view.loadConfig()
  view.editChinaMap('Shanghai')
  let approve!: () => void
  confirm.mock.mockImplementationOnce(() => new Promise<void>((resolve) => { approve = resolve }))
  const deleting = view.deleteChinaMap()
  view.editChinaMap('Beijing')
  view.chinaMapIps.ctcc = '192.0.2.3'
  view.showAddChinaMap()
  view.newProvinceName.value = '__v_isReactive'
  view.addProvince()
  view.saveChinaMap()
  assert.equal(view.formConfig.Chinamap.Beijing!.ctcc.join(','), '192.0.2.3')
  view.editChinaMap('Beijing')
  view.chinaMapIps.ctcc = '192.0.2.4'
  approve()
  await deleting
  assert.equal(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, 'Shanghai'), false)
  assert.equal(view.chinaMapVisible.value, true)
  assert.equal(view.chinaMapIps.ctcc, '192.0.2.4')
  view.saveChinaMap()
  assert.equal(view.formConfig.Chinamap.Beijing!.ctcc.join(','), '192.0.2.4')
  assert.equal(view.provinceCount.value, 2)
  assert.equal(showError.mock.callCount(), 0)
  assert.equal(showSuccess.mock.callCount(), 4)
  assert.equal(view.isDirty.value, true)
})

test('configuration dictionary extensions retain their scalar values and reactive updates', async (t) => {
  const names = ['__v_isReactive', '__v_isReadonly', '__v_isShallow', '__v_raw', '__v_skip', '__v_isRef', '__isVue', '__proto__', 'hasOwnProperty', 'toJSON']
  for (const field of ['Mode', 'Base', 'Topology', 'rule']) {
    for (const name of names) {
      const { view, loadConfig, saveConfig, createObjectURL, showError } = createView(t)
      const loaded = (await loadConfig())!
      loaded.Mode.Type = 'local'
      loaded.Network['192.0.2.1'] = { Name: 'remote', Addr: '192.0.2.1', Smartping: true, Ping: [], Topology: [] }
      loaded.Network['127.0.0.1']!.Topology = [{ Name: 'remote', Addr: '192.0.2.1', Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30' }]
      const dictionary = (config: Config): Record<string, string | number> =>
        (field === 'rule' ? config.Network['127.0.0.1']!.Topology[0] : field === 'Mode' ? config.Mode : field === 'Base' ? config.Base : config.Topology) as unknown as Record<string, string | number>
      const expected = field === 'Base' ? 42 : 'extension'
      Object.defineProperty(dictionary(loaded), name, { value: expected, enumerable: true, writable: true, configurable: true })
      loadConfig.mock.mockImplementation(async () => JSON.parse(JSON.stringify(loaded)) as Config)
      await view.loadConfig()
      assert.equal(dictionary(view.formConfig)[name], expected, `${field}/${name}`)
      assert.equal(view.isDirty.value, false)
      dictionary(view.formConfig)[name] = field === 'Base' ? 84 : 'changed'
      assert.equal(view.isDirty.value, true, `${field}/${name} mutation`)
      dictionary(view.formConfig)[name] = expected
      assert.equal(view.isDirty.value, false)
      view.formConfig.Base.Refresh = 10
      view.importExportPassword.value = 'password'
      await view.handleExport()
      const exported = JSON.parse(await createObjectURL.mock.calls[0]!.arguments[0].text()) as Config
      assert.equal(dictionary(exported)[name], expected, `${field}/${name} export`)
      view.password.value = 'password'
      await view.handleSave()
      assert.equal(saveConfig.mock.callCount(), 1)
      assert.equal(dictionary(saveConfig.mock.calls[0]!.arguments[0])[name], expected, `${field}/${name} save`)
      assert.equal(showError.mock.callCount(), 0)
      assert.equal(view.isDirty.value, false)
    }
  }
})

test('imported empty dictionary extensions survive replacement and rule insertion', async (t) => {
  const { view, loadConfig, saveConfig, showError } = createView(t)
  await view.loadConfig()
  const imported = (await loadConfig())!
  imported.Mode.Type = 'local'
  imported.Mode.__v_raw = ''
  ;(imported.Base as unknown as Record<string, number>).__v_isReactive = 0
  ;(imported.Topology as unknown as Record<string, string>).__v_skip = ''
  imported.Network['192.0.2.1'] = { Name: 'remote', Addr: '192.0.2.1', Smartping: true, Ping: [], Topology: [] }
  view.importExportPassword.value = 'password'
  await view.handleImportFile({ raw: { text: async () => JSON.stringify(imported) } })
  assert.equal(view.formConfig.Mode.__v_raw, '')
  assert.equal((view.formConfig.Base as unknown as Record<string, number>).__v_isReactive, 0)
  assert.equal((view.formConfig.Topology as unknown as Record<string, string>).__v_skip, '')
  const local = view.formConfig.Network['127.0.0.1']!
  local.Topology.push({ Name: 'remote', Addr: '192.0.2.1', Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30', ...Object.fromEntries([['__v_isRef', '']]) })
  view.password.value = 'password'
  await view.handleSave()
  assert.equal(showError.mock.callCount(), 0)
  assert.equal(saveConfig.mock.callCount(), 1)
  assert.equal(view.isDirty.value, false)
  const rule = local.Topology[0] as unknown as Record<string, string>
  rule.__v_isRef = 'changed'
  assert.equal(view.isDirty.value, true)
  rule.__v_isRef = ''
  assert.equal(view.isDirty.value, false)
  const submitted = saveConfig.mock.calls[0]!.arguments[0]
  assert.equal(submitted.Mode.__v_raw, '')
  assert.equal((submitted.Base as unknown as Record<string, number>).__v_isReactive, 0)
  assert.equal((submitted.Topology as unknown as Record<string, string>).__v_skip, '')
  assert.equal((submitted.Network['127.0.0.1']!.Topology[0] as unknown as Record<string, string>).__v_isRef, '')
})

test('topology editing keeps rule extensions and existing order before newly selected targets', async (t) => {
  const { view, loadConfig, saveConfig } = createView(t)
  const loaded = (await loadConfig())!
  for (const [index, name] of ['one', 'two', 'three'].entries()) {
    const addr = `192.0.2.${index + 1}`
    loaded.Network[addr] = { Name: name, Addr: addr, Smartping: true, Ping: [], Topology: [] }
  }
  const common = { Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30' }
  const one = { ...common, Name: 'one', Addr: '192.0.2.1', custom: 'keep-one', ...Object.fromEntries([['__proto__', 'own-one']]) }
  const two = { ...common, Name: 'two', Addr: '192.0.2.2', custom: 'keep-two', __v_isReactive: 'own-two' }
  loaded.Network['127.0.0.1']!.Topology = [two, one]
  loadConfig.mock.mockImplementation(async () => loaded)
  await view.loadConfig()
  const local = view.networkList.value.find((row) => row.isSelf)!
  view.editTopoConfig(local)
  view.saveTopoConfig()
  assert.equal(view.isDirty.value, false)
  view.editTopoConfig(local)
  const first = view.topoTargetList.value.find((row) => row.Addr === '192.0.2.1')!
  first.avgDelay = 300
  view.topoTargetList.value.find((row) => row.Addr === '192.0.2.3')!.enabled = true
  view.saveTopoConfig()
  assert.equal(view.formConfig.Network['127.0.0.1']!.Topology.map((rule) => rule.Addr).join(','), '192.0.2.2,192.0.2.1,192.0.2.3')
  view.password.value = 'password'
  await view.handleSave()
  const submitted = saveConfig.mock.calls[0]!.arguments[0].Network['127.0.0.1']!.Topology
  const secondRule = submitted[0] as unknown as Record<string, string>
  const firstRule = submitted[1] as unknown as Record<string, string>
  assert.equal(secondRule.custom, 'keep-two')
  assert.equal(secondRule.__v_isReactive, 'own-two')
  assert.equal(firstRule.custom, 'keep-one')
  assert.equal(firstRule['__proto__'], 'own-one')
  assert.equal(firstRule.Thdavgdelay, '300')
  assert.equal(view.isDirty.value, false)
})

test('rule aliases stay observable after collection replacement and removal', async (t) => {
  const { view, saveConfig, showError } = createView(t)
  await view.loadConfig()
  for (const [index, name] of ['one', 'two'].entries()) {
    const addr = `192.0.2.${index + 1}`
    view.formConfig.Network[addr] = { Name: name, Addr: addr, Smartping: true, Ping: [], Topology: [] }
  }
  const local = view.formConfig.Network['127.0.0.1']!
  const other = view.formConfig.Network['192.0.2.1']!
  const makeRule = () => ({ Name: 'two', Addr: '192.0.2.2', Thdchecksec: '900', Thdoccnum: '3', Thdavgdelay: '200', Thdloss: '30', __v_isReactive: 'data' })
  local.Topology.push(makeRule())
  local.Topology[0]!.Thdavgdelay = '250'
  const shared = local.Topology[0]!
  other.Topology = [shared]
  assert.equal(other.Topology[0], shared)
  const save = async () => {
    view.password.value = 'password'
    await view.handleSave()
    assert.equal(showError.mock.callCount(), 0)
    assert.equal(view.isDirty.value, false)
  }
  await save()
  shared.Thdavgdelay = '300'
  assert.equal(other.Topology[0]!.Thdavgdelay, '300')
  assert.equal(view.isDirty.value, true)
  shared.Thdavgdelay = '250'
  assert.equal(view.isDirty.value, false)
  local.Topology = []
  await save()
  shared.Thdloss = '25'
  assert.equal(view.isDirty.value, true)
  assert.equal(other.Topology[0]!.Thdloss, '25')
  shared.Thdloss = '30'
  assert.equal(view.isDirty.value, false)
  other.Topology.splice(0, 1)
  await save()
  shared.Thdloss = '20'
  assert.equal(view.isDirty.value, false)
  for (const insert of [
    () => local.Topology.push(makeRule()),
    () => local.Topology.unshift(makeRule()),
    () => local.Topology.splice(0, 0, makeRule()),
    () => { local.Topology[0] = makeRule() }
  ]) {
    insert()
    await save()
    const inserted = local.Topology[0] as unknown as Record<string, string>
    assert.equal(inserted.__v_isReactive, 'data')
    inserted.__v_isReactive = 'changed'
    assert.equal(view.isDirty.value, true)
    inserted.__v_isReactive = 'data'
    assert.equal(view.isDirty.value, false)
    local.Topology.length = 0
  }
  assert.equal(saveConfig.mock.callCount(), 7)
})

test('failed config save preserves the saved baseline and supports retry', async (t) => {
  const { view, saveConfig } = createView(t)
  await view.loadConfig()
  const baseline = view.savedSnapshot.value
  view.formConfig.Base.Refresh = 10
  view.password.value = 'password'
  saveConfig.mock.mockImplementationOnce(async () => {
    throw new Error('offline')
  })
  await view.handleSave()
  assert.equal(view.savedSnapshot.value, baseline)
  assert.equal(view.isDirty.value, true)
  assert.equal(view.saving.value, false)
  view.password.value = 'password'
  await view.handleSave()
  assert.equal(saveConfig.mock.callCount(), 2)
  assert.equal(view.isDirty.value, false)
  assert.equal(JSON.parse(view.savedSnapshot.value).Base.Refresh, 10)
})

test('editing ping targets preserves self checks and allows explicitly disabling them', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  const node = view.formConfig.Network['127.0.0.1']!
  node.Ping = ['127.0.0.1']
  view.editPingConfig(view.networkList.value.find((row) => row.Addr === node.Addr)!)
  const self = view.pingTargetList.value.find((item) => item.Addr === node.Addr)
  assert.ok(self)
  assert.equal(self.enabled, true)
  view.savePingConfig()
  assert.equal(node.Ping.join(','), '127.0.0.1')
  view.editPingConfig(view.networkList.value.find((row) => row.Addr === node.Addr)!)
  view.pingTargetList.value.find((item) => item.Addr === node.Addr)!.enabled = false
  view.savePingConfig()
  assert.equal(node.Ping.length, 0)
})

test('node dialogs reject whitespace-only names without changing configuration', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  view.showAddNode()
  view.newNodeName.value = ' \t '
  view.newNodeAddr.value = '192.0.2.1'
  view.addNode()
  assert.equal(view.formConfig.Network['192.0.2.1'], undefined)
  view.showEditNode(view.networkList.value.find((row) => row.isSelf)!)
  view.editNodeAddr.value = '127.0.0.1'
  view.editNodeName.value = ' \t '
  view.saveEditNode()
  assert.equal(view.formConfig.Network['127.0.0.1']!.Name, 'local')
  assert.equal(view.formConfig.Name, 'local')
  assert.equal(view.isDirty.value, false)
})

test('editing a node migrates incoming checks and alert rules without losing their settings', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  const originalAddress = '192.0.2.1'
  const movedAddress = '192.0.2.10'
  const rule = {
    Name: 'target', Addr: originalAddress, Thdchecksec: '900', Thdoccnum: '3',
    Thdavgdelay: '250', Thdloss: '20'
  }
  view.formConfig.Network[originalAddress] = {
    Name: 'target', Addr: originalAddress, Smartping: true,
    Ping: [originalAddress, '127.0.0.1'], Topology: []
  }
  view.formConfig.Network['192.0.2.2'] = {
    Name: 'other source', Addr: '192.0.2.2', Smartping: false,
    Ping: [originalAddress], Topology: [{ ...rule }]
  }
  const local = view.formConfig.Network['127.0.0.1']!
  local.Ping = [originalAddress, '192.0.2.2']
  local.Topology = [{ ...rule }]
  view.showEditNode(view.networkList.value.find((row) => row.Addr === originalAddress)!)
  view.editNodeAddr.value = movedAddress
  view.editNodeName.value = ' renamed target '
  view.saveEditNode()

  assert.equal(view.formConfig.Network[originalAddress], undefined)
  const moved = view.formConfig.Network[movedAddress]!
  assert.equal(moved.Name, 'renamed target')
  assert.equal(moved.Addr, movedAddress)
  assert.equal(moved.Smartping, true)
  assert.equal(moved.Ping.join(','), `${movedAddress},127.0.0.1`)
  assert.equal(local.Ping.join(','), `${movedAddress},192.0.2.2`)
  for (const address of ['127.0.0.1', '192.0.2.2']) {
    assert.deepEqual(JSON.parse(JSON.stringify(view.formConfig.Network[address]!.Topology)), [
      { ...rule, Name: 'renamed target', Addr: movedAddress }
    ])
  }
  assert.equal(validation.validateConfigForEdit(view.formConfig), null)

  view.showEditNode(view.networkList.value.find((row) => row.Addr === movedAddress)!)
  view.editNodeName.value = 'name only'
  view.saveEditNode()
  assert.equal(local.Topology[0]!.Name, 'name only')
  assert.equal(local.Topology[0]!.Addr, movedAddress)
  assert.equal(view.formConfig.Name, 'local')
  assert.equal(view.formConfig.Addr, '127.0.0.1')
})

test('conflicting node addresses leave node data and all references unchanged', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  view.formConfig.Network['192.0.2.1'] = {
    Name: 'remote', Addr: '192.0.2.1', Smartping: true, Ping: ['127.0.0.1'], Topology: []
  }
  view.formConfig.Network['127.0.0.1']!.Ping = ['192.0.2.1']
  const baseline = JSON.stringify(view.formConfig)
  view.showEditNode(view.networkList.value.find((row) => row.isSelf)!)
  view.editNodeAddr.value = '192.0.2.1'
  view.editNodeName.value = 'conflicting edit'
  view.saveEditNode()
  assert.equal(JSON.stringify(view.formConfig), baseline)
})
