import assert from 'node:assert/strict'
import test from 'node:test'
import { mapWithConcurrency } from './concurrency.js'

const delay = (milliseconds: number) =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, milliseconds)
  })

test('mapWithConcurrency preserves input order and caps active work', async () => {
  const items = [30, 5, 20, 1, 10]
  let active = 0
  let maxActive = 0

  const results = await mapWithConcurrency(items, 2, async (milliseconds, index) => {
    active += 1
    maxActive = Math.max(maxActive, active)
    await delay(milliseconds)
    active -= 1
    return `${index}:${milliseconds}`
  })

  assert.deepEqual(results, ['0:30', '1:5', '2:20', '3:1', '4:10'])
  assert.equal(maxActive, 2)
})

test('mapWithConcurrency normalizes invalid worker counts', async () => {
  for (const concurrency of [
    Number.NaN,
    Number.POSITIVE_INFINITY,
    Number.NEGATIVE_INFINITY,
    0,
    -2
  ]) {
    let active = 0
    let maxActive = 0
    const results = await mapWithConcurrency([1, 2, 3], concurrency, async (value) => {
      active += 1
      maxActive = Math.max(maxActive, active)
      await delay(1)
      active -= 1
      return value * 2
    })

    assert.deepEqual(results, [2, 4, 6])
    assert.equal(maxActive, 1)
  }
})

test('mapWithConcurrency does not invoke the mapper for an empty input', async () => {
  let calls = 0
  const results = await mapWithConcurrency([], 4, async () => {
    calls += 1
    return 'unexpected'
  })

  assert.deepEqual(results, [])
  assert.equal(calls, 0)
})
