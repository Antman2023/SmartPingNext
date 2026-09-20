import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_API_REQUEST_TIMEOUT_MS,
  MAX_REQUEST_TIMEOUT_MS,
  normalizeRequestTimeout,
  resolveProxyClientTimeout
} from './requestTimeouts.js'

test('normalizeRequestTimeout accepts positive values and rejects invalid input', () => {
  assert.equal(normalizeRequestTimeout('30000'), 30_000)
  assert.equal(normalizeRequestTimeout(1), 1)
  assert.equal(normalizeRequestTimeout(0), DEFAULT_API_REQUEST_TIMEOUT_MS)
  assert.equal(normalizeRequestTimeout(Number.NaN), DEFAULT_API_REQUEST_TIMEOUT_MS)
})

test('normalizeRequestTimeout rejects unsafe timer values without coercing objects', () => {
  const invalidValues = [
    0.5, 1.5, -1, Infinity, -Infinity, MAX_REQUEST_TIMEOUT_MS + 1,
    '4294967296', '', ' ', 'invalid', null, undefined, true, false, [], [1000],
    Symbol('timeout'), { valueOf: () => { throw new Error('must not coerce objects') } }
  ]
  for (const value of invalidValues) {
    assert.equal(normalizeRequestTimeout(value), DEFAULT_API_REQUEST_TIMEOUT_MS)
  }
  assert.equal(normalizeRequestTimeout(String(MAX_REQUEST_TIMEOUT_MS)), MAX_REQUEST_TIMEOUT_MS)
  assert.equal(normalizeRequestTimeout(' 30000 '), 30_000)
})

test('normalizeRequestTimeout validates fallback values too', () => {
  assert.equal(normalizeRequestTimeout(undefined, 20_000), 20_000)
  for (const fallback of [0, -1, 0.5, NaN, Infinity, MAX_REQUEST_TIMEOUT_MS + 1]) {
    assert.equal(normalizeRequestTimeout(undefined, fallback), DEFAULT_API_REQUEST_TIMEOUT_MS)
  }
})

test('resolveProxyClientTimeout leaves time for the backend proxy response', () => {
  assert.equal(resolveProxyClientTimeout(15_000), 65_000)
  assert.equal(resolveProxyClientTimeout(15_000, 10), 15_000)
  assert.equal(resolveProxyClientTimeout(90_000), 90_000)
  assert.equal(resolveProxyClientTimeout(15_000, 600), 65_000)
  assert.equal(resolveProxyClientTimeout(4_294_967_296), 65_000)
})
