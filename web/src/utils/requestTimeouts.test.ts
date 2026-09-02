import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_API_REQUEST_TIMEOUT_MS,
  normalizeRequestTimeout,
  resolveProxyClientTimeout
} from './requestTimeouts.js'

test('normalizeRequestTimeout accepts positive values and rejects invalid input', () => {
  assert.equal(normalizeRequestTimeout('30000'), 30_000)
  assert.equal(normalizeRequestTimeout(1), 1)
  assert.equal(normalizeRequestTimeout(0), DEFAULT_API_REQUEST_TIMEOUT_MS)
  assert.equal(normalizeRequestTimeout(Number.NaN), DEFAULT_API_REQUEST_TIMEOUT_MS)
})

test('resolveProxyClientTimeout leaves time for the backend proxy response', () => {
  assert.equal(resolveProxyClientTimeout(15_000), 65_000)
  assert.equal(resolveProxyClientTimeout(15_000, 10), 15_000)
  assert.equal(resolveProxyClientTimeout(90_000), 90_000)
  assert.equal(resolveProxyClientTimeout(15_000, 600), 65_000)
})
