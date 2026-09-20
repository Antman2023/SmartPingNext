import assert from 'node:assert/strict'
import test from 'node:test'
import { setImmediate } from 'node:timers/promises'
import { mapWithConcurrency } from './concurrency.js'

const delay = (milliseconds: number) =>
  new Promise<void>((resolve) => {
    setTimeout(resolve, milliseconds)
  })

test('mapWithConcurrency stops dispatching queued tasks after an asynchronous failure', async () => {
  const failure = new Error('failed request')
  const started: number[] = []
  let releaseSlow!: () => void
  const slow = new Promise<void>((resolve) => { releaseSlow = resolve })
  const result = mapWithConcurrency([0, 1, 2, 3, 4], 2, async (value) => {
    started.push(value)
    if (value === 0) throw failure
    await slow
    return value
  })
  try {
    await assert.rejects(result, (error) => error === failure)
    assert.deepEqual(started, [0, 1])
  } finally {
    releaseSlow()
  }
  // Let the other active worker resume and attempt to take its next item.
  await setImmediate()
  assert.deepEqual(started, [0, 1])
  assert.deepEqual(await mapWithConcurrency([2, 3], 2, async (value) => value), [2, 3])
})

test('mapWithConcurrency does not start other workers after a synchronous mapper failure', async () => {
  const failure = new Error('invalid task')
  const started: number[] = []
  await assert.rejects(mapWithConcurrency([0, 1, 2], 3, (value) => {
    started.push(value)
    throw failure
  }), (error) => error === failure)
  assert.deepEqual(started, [0])
})

test('mapWithConcurrency starts queued work while another worker is slow', async () => {
  let releaseSlow!: () => void
  const slow = new Promise<void>((resolve) => {
    releaseSlow = resolve
  })
  let thirdStarted!: () => void
  const third = new Promise<void>((resolve) => {
    thirdStarted = resolve
  })
  const result = mapWithConcurrency([0, 1, 2], 2, async (value) => {
    if (value === 0) await slow
    if (value === 2) thirdStarted()
    return value
  })
  try {
    await Promise.race([
      third,
      delay(1000).then(() => {
        throw new Error('queued work stalled')
      })
    ])
  } finally {
    releaseSlow()
  }
  assert.deepEqual(await result, [0, 1, 2])
})

test('mapWithConcurrency does not start queued work after cancellation', async () => {
  const controller = new AbortController()
  const started: number[] = []
  await assert.rejects(
    mapWithConcurrency(
      [0, 1, 2, 3],
      2,
      async (value) => {
        started.push(value)
        if (value === 0) controller.abort()
        return value
      },
      controller.signal
    ),
    { name: 'AbortError' }
  )
  assert.deepEqual(started, [0])
})

test('mapWithConcurrency rejects cancellation before starting and after the last item', async () => {
  const controller = new AbortController()
  controller.abort()
  await assert.rejects(
    mapWithConcurrency(
      [1],
      1,
      async () => {
        assert.fail('canceled work must not start')
      },
      controller.signal
    ),
    { name: 'AbortError' }
  )
  const last = new AbortController()
  await assert.rejects(
    mapWithConcurrency(
      [1],
      1,
      async (value) => {
        last.abort()
        return value
      },
      last.signal
    ),
    { name: 'AbortError' }
  )
})

test('mapWithConcurrency stops queued work when active requests swallow cancellation', async () => {
  const controller = new AbortController()
  const started: number[] = []
  let active = 0
  const result = mapWithConcurrency(
    [0, 1, 2, 3, 4],
    2,
    async (value) => {
      started.push(value)
      active += 1
      try {
        await new Promise<void>((_resolve, reject) => {
          controller.signal.addEventListener('abort', () => reject(controller.signal.reason), {
            once: true
          })
        })
      } catch {
        // Views handle request cancellation locally before the worker takes another item.
        return null
      } finally {
        active -= 1
      }
      return value
    },
    controller.signal
  )

  assert.deepEqual(started, [0, 1])
  const rejected = assert.rejects(result, { name: 'AbortError' })
  controller.abort()
  await rejected
  assert.equal(active, 0)
  assert.deepEqual(started, [0, 1])

  const refreshed = await mapWithConcurrency([2, 3, 4], 2, async (value) => value)
  assert.deepEqual(refreshed, [2, 3, 4])
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
