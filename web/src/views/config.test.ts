import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import type { Config } from '../types/index.js'
import * as validation from '../utils/configValidation.js'
import { CanceledError } from 'axios'
import { isRequestCanceled } from '../utils/requestCancellation.js'

const source = readFileSync(new URL('../../src/views/ConfigView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'config-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface ConfigSetup {
  formConfig: Config
  password: vue.Ref<string>
  savedSnapshot: vue.Ref<string>
  saving: vue.Ref<boolean>
  isDirty: vue.ComputedRef<boolean>
  loadConfig: () => Promise<void>
  handleSave: () => Promise<void>
  importExportPassword: vue.Ref<string>
  importing: vue.Ref<boolean>
  exporting: vue.Ref<boolean>
  handleExport: () => Promise<void>
  handleImportFile: (file: { raw: { size?: number; text: () => Promise<string> } }) => Promise<void>
  newNodeName: vue.Ref<string>
  newNodeAddr: vue.Ref<string>
  editNodeName: vue.Ref<string>
  editNodeAddr: vue.Ref<string>
  editNodeOriginalAddr: vue.Ref<string>
  addNode: () => void
  saveEditNode: () => void
  pingTargetList: vue.Ref<{ Addr: string; enabled: boolean }[]>
  editPingConfig: (row: { Name: string; Addr: string }) => void
  savePingConfig: () => void
  newProvinceName: vue.Ref<string>
  addProvince: () => void
}

function createView(t: test.TestContext) {
  const createObjectURL = t.mock.fn((_blob: Blob) => 'blob:config-export')
  const revokeObjectURL = t.mock.fn((_url: string) => {})
  const link = { href: '', download: '', click: t.mock.fn(() => {}) }
  const createElement = t.mock.fn((_tag: string) => link)
  const saveConfig = t.mock.fn(async (_config: Config): Promise<void> => {})
  const unmountCallbacks: Array<() => void> = []
  const showError = t.mock.fn((_message: string) => {})
  const showSuccess = t.mock.fn((_message: string) => {})
  const verifyConfigPassword = t.mock.fn(
    async (_password: string, _signal?: AbortSignal) => 'valid'
  )
  const dependencies: Record<string, unknown> = {
    vue: {
      ...vue,
      onMounted: () => {},
      onUnmounted: (fn: () => void) => unmountCallbacks.push(fn)
    },
    'vue-router': { onBeforeRouteLeave: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'element-plus': { ElMessage: { success: showSuccess, error: showError, warning: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusConfigStyles': {},
    '@/api': { isRequestCanceled },
    '@/api/config': { verifyConfigPassword, getConfigUrl: () => '/api/config.json' },
    '@/utils/configValidation': validation,
    '@/utils/format': { displayName: (name: string) => name },
    '@/stores/config': {
      useConfigStore: () => ({
        saveConfig,
        loadConfig: async () => ({
          Ver: 'test',
          Port: 8899,
          Name: 'local',
          Addr: '127.0.0.1',
          Mode: {},
          Base: { Timeout: 3, Refresh: 5, Archive: 30 },
          Topology: { Tsound: '', Tline: '2', Tsymbolsize: '50' },
          Network: {
            '127.0.0.1': {
              Name: 'local',
              Addr: '127.0.0.1',
              Smartping: true,
              Ping: [],
              Topology: []
            }
          },
          Chinamap: {},
          Toollimit: 0,
          Authiplist: ''
        })
      })
    }
  }
  const exports: { default?: { setup: (props: object, context: object) => ConfigSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    Blob,
    URL: { createObjectURL, revokeObjectURL },
    document: { createElement },
    URLSearchParams,
    window: { removeEventListener: () => {} },
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  return {
    view,
    saveConfig,
    showError,
    showSuccess,
    verifyConfigPassword,
    createObjectURL,
    revokeObjectURL,
    createElement,
    link,
    unmount: () => unmountCallbacks.forEach((fn) => fn())
  }
}

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
    view.newProvinceName.value = name
    view.addProvince()
    assert.ok(Object.prototype.hasOwnProperty.call(view.formConfig.Chinamap, name))
    view.formConfig.Chinamap[name]!.ctcc.push('192.0.2.1')
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
  view.editPingConfig(node)
  const self = view.pingTargetList.value.find((item) => item.Addr === node.Addr)
  assert.ok(self)
  assert.equal(self.enabled, true)
  view.savePingConfig()
  assert.equal(node.Ping.join(','), '127.0.0.1')
  view.editPingConfig(node)
  view.pingTargetList.value.find((item) => item.Addr === node.Addr)!.enabled = false
  view.savePingConfig()
  assert.equal(node.Ping.length, 0)
})

test('node dialogs reject whitespace-only names without changing configuration', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  view.newNodeName.value = ' \t '
  view.newNodeAddr.value = '192.0.2.1'
  view.addNode()
  assert.equal(view.formConfig.Network['192.0.2.1'], undefined)
  view.editNodeOriginalAddr.value = '127.0.0.1'
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
  view.editNodeOriginalAddr.value = originalAddress
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

  view.editNodeOriginalAddr.value = movedAddress
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
  view.editNodeOriginalAddr.value = '127.0.0.1'
  view.editNodeAddr.value = '192.0.2.1'
  view.editNodeName.value = 'conflicting edit'
  view.saveEditNode()
  assert.equal(JSON.stringify(view.formConfig), baseline)
})
