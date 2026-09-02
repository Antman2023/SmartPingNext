import axios from 'axios'

export const isRequestCanceled = (error: unknown): boolean => {
  return (
    axios.isCancel(error) ||
    (typeof DOMException !== 'undefined' &&
      error instanceof DOMException &&
      error.name === 'AbortError')
  )
}

export const normalizeRejectedRequest = <T>(
  error: unknown,
  normalize: (error: unknown) => T
): unknown | T => (isRequestCanceled(error) ? error : normalize(error))
