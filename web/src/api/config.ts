import request from './index'
import type { Config } from '@/types'

export const fetchConfig = (): Promise<Config> => {
  return request.get('/config.json')
}

export const fetchProxyConfig = (url: string): Promise<Config> => {
  return request.get(`/proxy.json?g=${url}/api/config.json`)
}

export const saveConfig = (config: Config, password: string): Promise<{ status: string; info?: string }> => {
  const data = new URLSearchParams()
  data.append('config', JSON.stringify(config))
  data.append('password', password)
  return request.post('/saveconfig.json', data)
}
