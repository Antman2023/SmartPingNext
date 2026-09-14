import request, { proxyRequestConfig } from './index'
import axios from 'axios'
import type { Config } from '@/types'

export type PasswordVerificationResult = 'valid' | 'invalid' | 'rate-limited'

export const verifyPassword = async (
  password: string,
  signal: AbortSignal
): Promise<PasswordVerificationResult> => {
  // Credential failures are expected results here, so bypass the shared
  // business-error interceptor while retaining the application's API settings.
  const response = await axios.post<unknown>(
    '/verify-password.json',
    new URLSearchParams({ password }),
    {
      baseURL: request.defaults.baseURL,
      timeout: request.defaults.timeout,
      signal,
      validateStatus: (status) => (status >= 200 && status < 300) || status === 429
    }
  )
  if (response.status === 429) return 'rate-limited'
  const result = response.data
  return typeof result === 'object' && result !== null && 'status' in result && result.status === 'true'
    ? 'valid'
    : 'invalid'
}

export const fetchConfig = (signal?: AbortSignal): Promise<Config> => {
  return request.get('/config.json', { signal })
}

export const fetchProxyConfig = (url: string, signal?: AbortSignal): Promise<Config> => {
  const params = new URLSearchParams({ g: `${url}/api/config.json` })
  return request.get(`/proxy.json?${params.toString()}`, proxyRequestConfig(signal))
}

export const saveConfig = (
  config: Config,
  password: string,
  signal?: AbortSignal
): Promise<{ status: string; info?: string }> => {
  const data = new URLSearchParams()
  data.append('config', JSON.stringify(config))
  data.append('password', password)
  return request.post('/saveconfig.json', data, { signal })
}
