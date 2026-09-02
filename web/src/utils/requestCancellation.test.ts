import assert from 'node:assert/strict'
import test from 'node:test'
import axios from 'axios'
import { isRequestCanceled, normalizeRejectedRequest } from './requestCancellation.js'

test('isRequestCanceled recognizes Axios cancellation errors', () => {
  assert.equal(isRequestCanceled(new axios.CanceledError('superseded')), true)
  assert.equal(isRequestCanceled(new DOMException('unmounted', 'AbortError')), true)
  assert.equal(isRequestCanceled(new Error('network failed')), false)
  assert.equal(isRequestCanceled(null), false)
})

test('normalizeRejectedRequest preserves cancellation without invoking the normalizer', () => {
  const cancellation = new axios.CanceledError('route changed')
  let normalizationCalls = 0

  const normalized = normalizeRejectedRequest(cancellation, () => {
    normalizationCalls += 1
    return new Error('unexpected')
  })

  assert.equal(normalized, cancellation)
  assert.equal(normalizationCalls, 0)
})

test('normalizeRejectedRequest normalizes non-cancellation failures', () => {
  const source = new Error('connection reset')
  const replacement = new Error('localized network error')

  const normalized = normalizeRejectedRequest(source, (error) => {
    assert.equal(error, source)
    return replacement
  })

  assert.equal(normalized, replacement)
})
