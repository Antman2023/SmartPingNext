import assert from 'node:assert/strict'
import test from 'node:test'
import { isRequestTimeout } from './requestErrors.js'

test('isRequestTimeout recognizes Axios timeout codes', () => {
  assert.equal(isRequestTimeout({ code: 'ECONNABORTED' }), true)
  assert.equal(isRequestTimeout({ code: 'ETIMEDOUT' }), true)
})

test('isRequestTimeout rejects cancellation and unrelated failures', () => {
  assert.equal(isRequestTimeout({ code: 'ERR_CANCELED' }), false)
  assert.equal(isRequestTimeout(new Error('timeout of 15000ms exceeded')), false)
  assert.equal(isRequestTimeout(null), false)
})
