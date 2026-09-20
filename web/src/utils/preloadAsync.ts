// Start loading immediately, share in-flight/successful work, and allow retry after failure.
export function preloadAsync<T>(loader: () => Promise<T>): () => Promise<T> {
  let pending: Promise<T> | undefined
  const load = (): Promise<T> => {
    if (!pending) {
      const request = Promise.resolve().then(loader)
      pending = request
      // Preloading can fail before any component has requested this promise.
      // Attach a handler without swallowing the rejection returned to callers.
      void request.catch(() => {
        if (pending === request) pending = undefined
      })
    }
    return pending
  }
  void load()
  return load
}
