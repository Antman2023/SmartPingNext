import assert from 'node:assert/strict'
import test from 'node:test'
import axios from 'axios'
import { createRequestClientFixture } from './testFixtures/requestClient.js'
import { isRequestCanceled } from '../utils/requestCancellation.js'
import type { Config } from '../types/index.js'

test('tools API returns node rejection envelopes for validation after HTTP success', async (t) => {
  let payload: unknown
  const { runTools } = await createRequestClientFixture(t, (request, response) => {
    const url = new URL(request.url!, 'http://127.0.0.1')
    assert.equal(url.pathname, '/api/proxy.json')
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify(payload))
  })
  for (const error of ['Time Limit Exceeded!', undefined, null, 123, {}, '', '  ']) {
    payload = { status: 'false', ...error !== undefined ? { error } : {} }
    assert.deepEqual(await runTools('192.0.2.1:8899', 'example.test'), payload)
  }
  for (const status of [true, 200, 'unexpected', false, {}]) {
    payload = { status }
    assert.deepEqual(await runTools('192.0.2.1:8899', 'example.test'), payload)
  }
})

test('shared HTTP client still normalizes business failures and preserves password and save categories', async (t) => {
  let payload: unknown = { status: 'false', info: 'configuration rejected' }
  const client = await createRequestClientFixture(t, (request, response) => {
    request.resume()
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify(payload))
  })
  await assert.rejects(() => client.request.get('/config.json'), {
    name: 'ApiError', message: 'configuration rejected', status: 200
  })
  assert.equal(await client.verifyConfigPassword('test & password'), 'invalid')
  await assert.rejects(() => client.saveConfig({ Name: 'local' } as Config, 'test'), {
    name: 'ApiError', message: 'configuration rejected', status: 200
  })
  for (const status of ['true', true, 200]) {
    payload = { status, info: 'saved' }
    assert.deepEqual(await client.request.get<unknown, unknown>('/config.json'), payload)
    const saved = await client.saveConfig({ Name: 'local' } as Config, 'test') as { status: string; info: string }
    assert.equal(saved.status, 'true')
    assert.equal(saved.info, 'saved')
  }
})

test('tools requests preserve HTTP error categories despite accepting business rejection envelopes', async (t) => {
  let statusCode = 429
  const client = await createRequestClientFixture(t, (request, response) => {
    request.resume()
    response.statusCode = statusCode
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify({ status: 'false', error: 'remote diagnostic failed' }))
  })
  for (const [status, message] of [[429, 'common.tooManyRequests'], [503, 'common.serviceUnavailable'],
    [504, 'common.gatewayTimeout'], [401, 'common.unauthorized']] as const) {
    statusCode = status
    await assert.rejects(() => client.runTools('192.0.2.1:8899', 'example.test'), {
      name: 'ApiError', status, message
    })
  }
  statusCode = 429
  assert.equal(await client.verifyConfigPassword('test'), 'rate-limited')
})

test('tools HTTP cancellation retains its category and later requests recover', async (t) => {
  let hold = true
  let markStarted!: () => void
  const started = new Promise<void>((resolve) => { markStarted = resolve })
  const payload = { status: 'true', error: '', ip: '192.0.2.1',
    ping: { SendPk: 5, RevcPk: 5, LossPk: 0, MinDelay: 0, AvgDelay: 0, MaxDelay: 0 } }
  const client = await createRequestClientFixture(t, (_request, response) => {
    if (hold) {
      markStarted()
      return
    }
    response.setHeader('Content-Type', 'application/json')
    response.end(JSON.stringify(payload))
  })
  const controller = new AbortController()
  const pending = client.runTools('192.0.2.1:8899', 'example.test', controller.signal)
  await started
  controller.abort()
  await assert.rejects(pending, (error) => axios.isCancel(error) && isRequestCanceled(error))
  hold = false
  assert.deepEqual(await client.runTools('192.0.2.1:8899', 'example.test'), payload)
})
