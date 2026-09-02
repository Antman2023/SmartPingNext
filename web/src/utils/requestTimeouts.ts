export const DEFAULT_API_REQUEST_TIMEOUT_MS = 15_000
export const MAX_PROXY_SERVER_TIMEOUT_SECONDS = 60
export const PROXY_RESPONSE_MARGIN_MS = 5_000

export const normalizeRequestTimeout = (
  value: unknown,
  fallback = DEFAULT_API_REQUEST_TIMEOUT_MS
): number => {
  const timeout = typeof value === 'number' ? value : Number(value)
  return Number.isFinite(timeout) && timeout > 0 ? timeout : fallback
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
