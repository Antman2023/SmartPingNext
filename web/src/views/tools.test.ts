import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import * as vue from 'vue'
import { compileScript, parse } from 'vue/compiler-sfc'
import { mapWithConcurrency } from '../utils/concurrency.js'
import type { ToolsResult } from '../types/index.js'
import { createRequestClientFixture } from '../api/testFixtures/requestClient.js'

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
  configLoading: vue.Ref<boolean>
  target: vue.Ref<string>
  lastRunAt: vue.Ref<Date | null>
  runCheck: () => Promise<void>
  loadConfig: () => Promise<void>
}

function createView(t: test.TestContext, requestTools?: (base: string, target: string, signal?: AbortSignal) => Promise<ToolsResult>) {
  const runTools = t.mock.fn(
    async (_base: string, _target: string, _signal?: AbortSignal): Promise<ToolsResult> => ({
      status: 'true',
      error: '',
      ip: '192.0.2.1',
      ping: { SendPk: 4, RevcPk: 4, LossPk: 0, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 }
    })
  )
  const fetchConfig = t.mock.fn(async (_signal?: AbortSignal) => ({
    Addr: '127.0.0.1',
    Port: 8899,
    Network: Object.fromEntries(
      Array.from({ length: 6 }, (_, i) => [
        `192.0.2.${i + 1}`,
        { Name: `probe-${i}`, Addr: `192.0.2.${i + 1}`, Smartping: true }
      ])
    )
  }))
  let unmount!: () => void
  const dependencies: Record<string, unknown> = {
    vue: { ...vue, onMounted: () => {}, onUnmounted: (callback: () => void) => { unmount = callback } },
    'vue-i18n': { useI18n: () => ({ t: (key: string) => key }) },
    'element-plus': { ElMessage: { error: () => {}, warning: () => {} } },
    '@element-plus/icons-vue': {},
    '@/plugins/elementPlusToolsStyles': {},
    '@/api': {
      isRequestCanceled: (error: unknown) =>
        error instanceof DOMException && error.name === 'AbortError'
    },
    '@/api/config': { fetchConfig },
    '@/api/tools': { runTools: requestTools ?? runTools },
    '@/utils/concurrency': { mapWithConcurrency },
    '@/utils/format': { formatTime: () => '12:00' }
  }
  const exports: { default?: { setup: (props: object, context: object) => ToolsSetup } } = {}
  runInNewContext(compiled, {
    exports,
    Error,
    AbortController,
    console: { error: () => {} },
    require: (name: string) => {
      assert.ok(name in dependencies, `Unexpected dependency: ${name}`)
      return dependencies[name]
    }
  })
  const scope = vue.effectScope()
  t.after(() => { unmount(); scope.stop() })
  const view = scope.run(() => exports.default!.setup({}, { expose: () => {} }))!
  return { view, runTools, fetchConfig, unmount: () => unmount() }
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

test('tools rejects contradictory counters and measured delay summaries without blocking other nodes', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  const sample = await runTools('node', 'target')
  const invalidStats = [
    { SendPk: 0, RevcPk: 0, LossPk: 0, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 },
    { SendPk: 0, RevcPk: 0, LossPk: 100 },
    { LossPk: 1 },
    { SendPk: 5, RevcPk: 5, LossPk: 100 },
    { SendPk: 5, RevcPk: 4, LossPk: 0 },
    { SendPk: 5, RevcPk: 0, LossPk: 0 },
    { SendPk: 100, RevcPk: 71, LossPk: 28 },
    { SendPk: 7, RevcPk: 5, LossPk: 29 },
    { SendPk: Number.MAX_SAFE_INTEGER, RevcPk: 8106479329266892, LossPk: 10 },
    { MinDelay: 2.01, AvgDelay: 2, MaxDelay: 3 },
    { MinDelay: 1, AvgDelay: 3.01, MaxDelay: 3 },
    { MinDelay: 4, AvgDelay: 2, MaxDelay: 3 },
    { SendPk: 5, RevcPk: 1, LossPk: 80, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 }
  ]
  for (const stats of invalidStats) {
    runTools.mock.mockImplementation(async (base) => base.startsWith('192.0.2.1:')
      ? { ...sample, ping: { ...sample.ping, ...stats } } : sample)
    await view.runCheck()
    assert.equal(view.results.value[0]!.result, null, JSON.stringify(stats))
    assert.equal(view.results.value[0]!.error, 'tools.invalidResponse')
    assert.equal(view.results.value.filter((row) => row.result?.status === 'true').length, 5)
    assert.ok(view.results.value.every((row) => !row.loading))
    assert.equal(view.checking.value, false)
  }
  runTools.mock.mockImplementation(async () => sample)
  await view.runCheck()
  assert.ok(view.results.value.every((row) => row.result && !row.error))
})

test('tools preserves consistent integer loss, zero delays and all-loss placeholders', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  const sample = await runTools('node', 'target')
  const validStats = [
    { SendPk: 5, RevcPk: 5, LossPk: 0, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 },
    { SendPk: 5, RevcPk: 4, LossPk: 20, MinDelay: 0, AvgDelay: 0.01, MaxDelay: 0.02 },
    { SendPk: 3, RevcPk: 2, LossPk: 33, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 },
    { SendPk: 7, RevcPk: 5, LossPk: 28, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 },
    { SendPk: 100, RevcPk: 71, LossPk: 29, MinDelay: 1, AvgDelay: 2, MaxDelay: 3 },
    { SendPk: 1, RevcPk: 1, LossPk: 0, MinDelay: 7.25, AvgDelay: 7.25, MaxDelay: 7.25 },
    { SendPk: 5, RevcPk: 1, LossPk: 80, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 },
    { SendPk: 5, RevcPk: 0, LossPk: 100, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 },
    { SendPk: 5, RevcPk: 0, LossPk: 100, MinDelay: 3000, AvgDelay: 3000, MaxDelay: 3000 },
    { SendPk: Number.MAX_SAFE_INTEGER, RevcPk: 8106479329266892, LossPk: 9,
      MinDelay: 1, AvgDelay: 2, MaxDelay: 3 }
  ]
  for (const stats of validStats) {
    const result = { ...sample, ping: stats }
    runTools.mock.mockImplementation(async () => result)
    await view.runCheck()
    assert.ok(view.results.value.every((row) => row.result && !row.error), JSON.stringify(stats))
    for (const row of view.results.value) {
      assert.deepEqual(row.result!.ping, stats)
    }
  }
})

test('tools classifies invalid node rejection bodies through the actual HTTP client', async (t) => {
  let reply: unknown
  const client = await createRequestClientFixture(t, (_request, response) => {
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify(reply))
  })
  const { view } = createView(t, client.runTools)
  await view.loadConfig()
  for (const error of [undefined, null, 123, {}, '', '  ']) {
    reply = { status: 'false', error, ip: '', ping: { SendPk: 0, RevcPk: 0, LossPk: 0, MinDelay: -1, AvgDelay: 0, MaxDelay: 0 } }
    await view.runCheck()
    for (const row of view.results.value) {
      assert.equal(row.result, null)
      assert.equal(row.error, 'tools.invalidResponse', JSON.stringify(reply))
    }
    assert.ok(view.results.value.every((row) => !row.loading))
  }
})

test('tools preserves rejection reasons, isolates failures and recovers through the actual HTTP client', async (t) => {
  const valid: ToolsResult = { status: 'true', error: '', ip: '192.0.2.1',
    ping: { SendPk: 5, RevcPk: 5, LossPk: 0, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 } }
  let reason: string | null = 'Time Limit Exceeded!'
  const client = await createRequestClientFixture(t, (request, response) => {
    const query = new URL(request.url!, 'http://127.0.0.1').searchParams
    const remote = new URL(query.get('g')!)
    assert.equal(remote.pathname, '/api/tools.json')
    assert.equal(query.get('t'), '10')
    assert.equal(remote.searchParams.get('t'), 'https://example.test/a?q=a&b=1')
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify(remote.hostname === '192.0.2.1' && reason !== null
      ? { status: 'false', error: reason, ip: '', ping: { ...valid.ping, MinDelay: -1 } } : valid))
  })
  const { view } = createView(t, client.runTools)
  await view.loadConfig()
  view.target.value = 'https://example.test/a?q=a&b=1'
  for (const message of ['Time Limit Exceeded!', 'Unable to resolve destination host', 'target empty!']) {
    reason = message
    await view.runCheck()
    assert.equal(view.results.value[0]!.error, message)
    assert.equal(view.results.value[0]!.result, null)
    assert.equal(view.results.value.filter((row) => row.result?.status === 'true').length, 5)
    assert.ok(view.results.value.every((row) => !row.loading))
  }
  reason = null
  await view.runCheck()
  assert.ok(view.results.value.every((row) => !row.error && row.result?.ping.AvgDelay === 0))
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

test('tools preserves node rejection messages without publishing unmeasured statistics', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  const sample = await runTools('node', 'target')
  for (const message of ['Time Limit Exceeded!', 'Unable to resolve destination host', 'target empty!']) {
    runTools.mock.mockImplementation(async (base) => base.startsWith('192.0.2.1:')
      ? { ...sample, status: 'false', error: message, ping: { ...sample.ping, MinDelay: -1 } }
      : sample)
    await view.runCheck()
    assert.equal(view.results.value[0]!.error, message)
    assert.equal(view.results.value[0]!.result, null)
    assert.equal(view.results.value.filter((row) => row.result).length, 5)
    assert.ok(view.results.value.every((row) => !row.loading))
  }
  runTools.mock.mockImplementation(async () => sample)
  await view.runCheck()
  assert.ok(view.results.value.every((row) => !row.error && row.result))
})

test('tools rejects malformed node rejection messages', async (t) => {
  const { view, runTools } = createView(t)
  await view.loadConfig()
  const sample = await runTools('node', 'target')
  for (const error of [undefined, null, 123, {}, '', '  ']) {
    runTools.mock.mockImplementation(async () => ({ ...sample, status: 'false', error } as unknown as ToolsResult))
    await view.runCheck()
    assert.ok(view.results.value.every((row) => row.result === null && row.error === 'tools.invalidResponse'))
  }
})

test('tools suspends checks during config reload and resumes with the new endpoints', async (t) => {
  const { view, runTools, fetchConfig } = createView(t)
  await view.loadConfig()
  await view.runCheck()
  const oldRows = view.results.value
  const oldResults = oldRows.map((row) => row.result)
  const cfg = await fetchConfig()
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  t.after(() => release())
  fetchConfig.mock.mockImplementation(async () => {
    await gate
    return { ...cfg, Port: 9000, Addr: '127.0.0.2' }
  })
  runTools.mock.resetCalls()
  const pending = view.loadConfig()
  await view.runCheck()
  try {
    assert.equal(runTools.mock.callCount(), 0)
    assert.equal(view.checking.value, false)
    assert.equal(view.results.value, oldRows)
    assert.deepEqual(oldRows.map((row) => row.result), oldResults)
  } finally {
    release()
    await pending
  }
  await view.runCheck()
  assert.equal(runTools.mock.callCount(), 6)
  assert.ok(runTools.mock.calls.every((call) => call.arguments[0].endsWith(':9000') && call.arguments[1] === '127.0.0.2'))
})

test('tools unmount prevents late config loads and new check requests', async (t) => {
  const { view, runTools, fetchConfig, unmount } = createView(t)
  await view.loadConfig()
  await view.runCheck()
  const lastRunAt = view.lastRunAt.value
  const rows = view.results.value
  const calls = runTools.mock.callCount()
  const configCalls = fetchConfig.mock.callCount()
  unmount()
  await view.loadConfig()
  await view.runCheck()
  assert.equal(fetchConfig.mock.callCount(), configCalls)
  assert.equal(runTools.mock.callCount(), calls)
  assert.equal(view.lastRunAt.value, lastRunAt)
  assert.equal(view.results.value, rows)
  assert.equal(view.configLoading.value, false)
  assert.equal(view.checking.value, false)
})

test('tools unmount cancels active probes and discards queued work and late failures', async (t) => {
  const { view, runTools, unmount } = createView(t)
  await view.loadConfig()
  let release!: () => void
  const gate = new Promise<void>((resolve) => { release = resolve })
  t.after(() => release())
  runTools.mock.mockImplementation(async () => {
    await gate
    throw new Error('late failure')
  })
  const pending = view.runCheck()
  assert.equal(runTools.mock.callCount(), 4)
  const signal = runTools.mock.calls[0]!.arguments[2]!
  unmount()
  assert.equal(signal.aborted, true)
  release()
  await pending
  assert.equal(runTools.mock.callCount(), 4)
  assert.equal(view.lastRunAt.value, null)
  assert.ok(view.results.value.every((row) => !row.result && !row.error))
})
