import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'
import { runInNewContext } from 'node:vm'
import ts from 'typescript'
import { isRequestTimeout } from './requestErrors.js'

const source = readFileSync(new URL('../../src/utils/error.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }
}).outputText
const exports = {} as { handleNetworkError: (error: unknown) => Error & { status?: number } }
runInNewContext(compiled, {
  exports,
  require: (name: string) => {
    if (name === 'element-plus') return { ElMessage: {} }
    if (name === '@/locales') return { default: { global: { t: (key: string) => key } } }
    if (name === '@/utils/requestErrors') return { isRequestTimeout }
    throw new Error(`Unexpected dependency: ${name}`)
  }
})

test('network errors ignore malformed remote messages without throwing during coercion', () => {
  for (const [status, fallback] of [
    [400, 'common.badRequest'],
    [500, 'common.serverError'],
    [200, 'common.requestFailedWithStatus']
  ] as const) {
    for (const data of [
      null,
      [],
      'html error body',
      42,
      ...[{}, [], true, 123, '   ', { toString: null, valueOf: null }].map((value) => ({
        info: value,
        message: value,
        error: value
      }))
    ]) {
      const error = exports.handleNetworkError({ response: { status, data } })
      assert.equal(error.message, fallback)
      assert.equal(error.status, status)
      assert.equal(error.name, 'ApiError')
    }
  }
})

test('network errors keep valid message priority and skip malformed higher-priority fields', () => {
  for (const status of [200, 400, 500]) {
    assert.equal(
      exports.handleNetworkError({
        response: { status, data: { info: 'details', message: 'summary' } }
      }).message,
      'details'
    )
    assert.equal(
      exports.handleNetworkError({ response: { status, data: { info: {}, message: 'summary' } } })
        .message,
      'summary'
    )
  }
  assert.equal(
    exports.handleNetworkError({
      response: { status: 200, data: { info: ' ', message: null, error: 'probe failed' } }
    }).message,
    'probe failed'
  )
  assert.equal(
    exports.handleNetworkError({ response: { status: 429, data: { info: 'remote' } } }).message,
    'common.tooManyRequests'
  )
})
