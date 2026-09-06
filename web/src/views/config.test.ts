import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import type { Config } from '../types/index.js'
import * as validation from '../utils/configValidation.js'

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
}

function createView(t: test.TestContext) {
  const saveConfig = t.mock.fn(async (_config: Config): Promise<void> => {})
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-router': { onBeforeRouteLeave: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'element-plus': { ElMessage: { success: () => {}, error: () => {}, warning: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusConfigStyles': {},
    '@/api': { isRequestCanceled: () => false },
    '@/utils/configValidation': validation,
    '@/utils/format': { displayName: (name: string) => name },
    '@/stores/config': { useConfigStore: () => ({
      saveConfig,
      loadConfig: async () => ({
        Ver: 'test', Port: 8899, Name: 'local', Addr: '127.0.0.1', Mode: {},
        Base: { Timeout: 3, Refresh: 5, Archive: 30 },
        Topology: { Tsound: '', Tline: '2', Tsymbolsize: '50' },
        Network: { '127.0.0.1': { Name: 'local', Addr: '127.0.0.1', Smartping: true, Ping: [], Topology: [] } },
        Chinamap: {}, Toollimit: 0, Authiplist: ''
      })
    }) }
  }
  const exports: { default?: { setup: (props: object, context: object) => ConfigSetup } } = {}
  runInNewContext(compiled, {
    exports, AbortController, console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  return { view, saveConfig }
}

test('config save retains edits made while the submitted snapshot is pending', async (t) => {
  const { view, saveConfig } = createView(t)
  await view.loadConfig()
  view.formConfig.Base.Refresh = 10
  view.password.value = 'password'
  let finish!: () => void
  saveConfig.mock.mockImplementation(() => new Promise<void>((resolve) => { finish = resolve }))
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
  saveConfig.mock.mockImplementationOnce(async () => { throw new Error('offline') })
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
