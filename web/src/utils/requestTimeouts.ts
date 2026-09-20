export const DEFAULT_API_REQUEST_TIMEOUT_MS = 15_000
export const MAX_PROXY_SERVER_TIMEOUT_SECONDS = 60
export const PROXY_RESPONSE_MARGIN_MS = 5_000
// Keep timeouts within the signed 32-bit range supported by timer-based adapters.
export const MAX_REQUEST_TIMEOUT_MS = 2_147_483_647

const isValidTimeout = (value: number): boolean =>
  Number.isInteger(value) && value > 0 && value <= MAX_REQUEST_TIMEOUT_MS

export const normalizeRequestTimeout = (
  value: unknown,
  fallback = DEFAULT_API_REQUEST_TIMEOUT_MS
): number => {
  const timeout = typeof value === 'number' || typeof value === 'string' ? Number(value) : NaN
  if (isValidTimeout(timeout)) return timeout
  return isValidTimeout(fallback) ? fallback : DEFAULT_API_REQUEST_TIMEOUT_MS
}

export const resolveProxyClientTimeout = (
  apiTimeoutMs: number,
  serverTimeoutSeconds = MAX_PROXY_SERVER_TIMEOUT_SECONDS
): number => {
  const normalizedApiTimeout = normalizeRequestTimeout(apiTimeoutMs)
  const normalizedServerTimeout =
    Number.isFinite(serverTimeoutSeconds) && serverTimeoutSeconds > 0
      ? Math.min(serverTimeoutSeconds, MAX_PROXY_SERVER_TIMEOUT_SECONDS)
      : MAX_PROXY_SERVER_TIMEOUT_SECONDS

  return Math.max(normalizedApiTimeout, normalizedServerTimeout * 1_000 + PROXY_RESPONSE_MARGIN_MS)
}
