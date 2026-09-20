import assert from 'node:assert/strict'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { preloadAsync } from './preloadAsync.js'

test('preloading starts before demand and shares pending and successful results', async () => {
  let calls = 0
  let resolve!: (value: object) => void
  const module = {}
  const load = preloadAsync(() => {
    calls++
    return new Promise<object>((done) => { resolve = done })
  })
  await setImmediate()
  assert.equal(calls, 1)
  const first = load()
  assert.equal(load(), first)
  resolve(module)
  assert.equal(await first, module)
  assert.equal(await load(), module)
  assert.equal(calls, 1)
})

test('an unused rejected preload is handled and a later request can retry', async () => {
  for (const synchronous of [false, true]) {
    let calls = 0
    const failure = new Error('chunk unavailable')
    const load = preloadAsync(() => {
      if (++calls === 1) {
        if (synchronous) throw failure
        return Promise.reject(failure)
      }
      return Promise.resolve('loaded')
    })
    // An unhandled preload rejection would fail this test through the Node runner.
    await setImmediate()
    assert.equal(calls, 1)
    assert.equal(await load(), 'loaded')
    assert.equal(calls, 2)
  }
})

test('a requested preload still reports its original failure to the component', async () => {
  const failure = new Error('load failed')
  let calls = 0
  const load = preloadAsync(async () => {
    calls++
    throw failure
  })
  await assert.rejects(load(), (error) => error === failure)
  await assert.rejects(load(), (error) => error === failure)
  assert.equal(calls, 2)
})
