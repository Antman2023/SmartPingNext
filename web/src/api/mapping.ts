import request from './index'
import type { ChinaMapData } from '@/types'

export const getMapping = (d?: string): Promise<ChinaMapData> => {
  let url = '/mapping.json'
  if (d) url += `?d=${encodeURIComponent(d)}`
  return request.get(url)
}

export const getProxyMapping = (baseUrl: string, d?: string): Promise<ChinaMapData> => {
  let url = `/proxy.json?g=${baseUrl}/api/mapping.json`
  if (d) url += `?d=${encodeURIComponent(d)}`
  return request.get(url)
}
