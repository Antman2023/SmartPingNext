export async function mapWithConcurrency<T, R>(
  items: readonly T[],
  concurrency: number,
  mapper: (item: T, index: number) => Promise<R>
): Promise<R[]> {
  const results = new Array<R>(items.length)
  const workerCount = Math.min(Math.max(Math.floor(concurrency), 1), items.length)
  let nextIndex = 0

  const workers = Array.from({ length: workerCount }, async () => {
    while (nextIndex < items.length) {
      const index = nextIndex
      nextIndex += 1
      results[index] = await mapper(items[index] as T, index)
    }
  })

  await Promise.all(workers)
  return results
}
