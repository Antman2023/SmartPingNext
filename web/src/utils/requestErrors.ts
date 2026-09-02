const REQUEST_TIMEOUT_CODES = new Set(['ECONNABORTED', 'ETIMEDOUT'])

export const isRequestTimeout = (error: unknown): boolean => {
  if (typeof error !== 'object' || error === null || !('code' in error)) {
    return false
  }
  return typeof error.code === 'string' && REQUEST_TIMEOUT_CODES.has(error.code)
}
