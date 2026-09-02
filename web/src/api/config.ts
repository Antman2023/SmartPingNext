import request, { proxyRequestConfig } from './index'
import type { Config } from '@/types'

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
