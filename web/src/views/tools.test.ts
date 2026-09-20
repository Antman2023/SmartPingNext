import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import type { ToolsResult } from '../types/index.js'

const source = readFileSync(new URL('../../src/views/ToolsView.vue', import.meta.url), 'utf8')
const { descriptor } = parse(source)
const compiled = ts.transpileModule(compileScript(descriptor, { id: 'tools-test' }).content, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText

interface ToolsSetup {
  results: vue.Ref<
    {
      checked: boolean
      loading: boolean
      result: ToolsResult | null
      error: string | null
    }[]
  >
  checking: vue.Ref<boolean>
  lastRunAt: vue.Ref<Date | null>
  runCheck: () => Promise<void>
  loadConfig: () => Promise<void>
}

function createView(t: test.TestContext) {
  const runTools = t.mock.fn(
    async (_base: string, _target: string, _signal?: AbortSignal): Promise<ToolsResult> => ({
      status: 'true',
      error: '',
      ip: '192.0.2.1',
      ping: { SendPk: 4, RevcPk: 4, LossPk: 0, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 }
    })
  )
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: () => {} },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'element-plus': { ElMessage: { error: () => {}, warning: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusToolsStyles': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': {
      fetchConfig: async () => ({
        Addr: '127.0.0.1',
        Port: 8899,
        Network: Object.fromEntries(
          Array.from({ length: 6 }, (_, i) => [
            `192.0.2.${i + 1}`,
            { Name: `probe-${i}`, Addr: `192.0.2.${i + 1}`, Smartping: true }
          ])
        )
      })
    },
    '@/api/tools': { runTools },
    '@/utils/concurrency': { mapWithConcurrency },
    '@/utils/format': { formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => ToolsSetup } } = {}
  runInNewContext(compiled, {
    exports,
    AbortController,
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => scope.stop())
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  return { view, runTools }
}

test('tools marks queued probes as loading while respecting the concurrency limit', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  view.results.value[5]!.checked = false
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const sample = await runTools('node', 'target')
  runTools.mock.resetCalls()
  runTools.mock.mockImplementation(async () => {
    await gate
    return sample
  })
  const pending = view.runCheck()
  assert.equal(runTools.mock.callCount(), 4)
  assert.equal(view.checking.value, true)
  assert.equal(view.results.value.filter((row) => row.loading).length, 5)
  assert.equal(view.results.value[5]!.loading, false)
  release()
  await pending
  assert.equal(runTools.mock.callCount(), 5)
  assert.equal(
    view.results.value.some((row) => row.loading),
    false
  )
  assert.equal(view.checking.value, false)
})

test('tools configuration reload resets the previous run timestamp', async (t) => {
  const { view } = createView(t)
  await view.loadConfig()
  await view.runCheck()
  assert.ok(view.lastRunAt.value)
  await view.loadConfig()
  assert.equal(view.lastRunAt.value, null)
  assert.equal(
    view.results.value.some((row) => row.result),
    false
  )
})

test('tools isolates malformed success payloads and accepts valid zero measurements', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  const sample = await runTools('node', 'target')
  const malformed: unknown[] = [
    null,
    {},
    { ...sample, ip: 42 },
    { ...sample, error: {} },
    { ...sample, status: 'unexpected' },
    { ...sample, ping: null },
    ...Object.keys(sample.ping).flatMap((key) =>
      [undefined, '1.2', NaN, Infinity, -1].map((value) => ({
        ...sample,
        ping: { ...sample.ping, [key]: value }
      }))
    ),
    ...[
      { SendPk: 1.5 }, { RevcPk: 0.5 }, { SendPk: Number.MAX_SAFE_INTEGER + 1 },
      { RevcPk: sample.ping.SendPk + 1 }, { LossPk: 101 }, { LossPk: 0.5 }
    ].map((fields) => ({ ...sample, ping: { ...sample.ping, ...fields } }))
  ]
  for (const response of malformed) {
    runTools.mock.mockImplementation(async (base) =>
      base.startsWith('192.0.2.1:') ? (response as ToolsResult) : sample
    )
    await view.runCheck()
    assert.equal(view.checking.value, false)
    assert.equal(view.results.value[0]!.result, null)
    assert.ok(view.results.value[0]!.error)
    assert.equal(view.results.value.filter((row) => row.result?.status === 'true').length, 5)
    assert.ok(view.results.value.every((row) => !row.loading))
  }
  runTools.mock.mockImplementation(async () => ({
    ...sample,
    ping: { ...sample.ping, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 }
  }))
  await view.runCheck()
  assert.ok(view.results.value.every((row) => !row.error && row.result?.ping.AvgDelay === 0))
  runTools.mock.mockImplementation(async () => ({
    ...sample,
    ping: { SendPk: 5, RevcPk: 0, LossPk: 100, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 }
  }))
  await view.runCheck()
  assert.ok(view.results.value.every((row) => !row.error && row.result?.ping.LossPk === 100))
})

test('tools configuration reload cancels queued work and ignores late responses', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  let release!: () => void
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const sample = await runTools('node', 'target')
  runTools.mock.resetCalls()
  runTools.mock.mockImplementation(async () => {
    await gate
    return sample
  })
  const pending = view.runCheck()
  const signal = runTools.mock.calls[0]!.arguments[2]!
  await view.loadConfig()
  assert.equal(signal.aborted, true)
  release()
  await pending
  assert.equal(runTools.mock.callCount(), 4)
  assert.equal(view.checking.value, false)
  assert.equal(view.lastRunAt.value, null)
  assert.ok(view.results.value.every((row) => !row.loading && !row.result && !row.error))
})
