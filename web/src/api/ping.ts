import request, { proxyRequestConfig } from './index'
import type { PingLogData } from '@/types'

const appendPingQuery = (
  target: URLSearchParams,
  ip: string,
  starttime?: string,
  endtime?: string
) => {
  target.set('ip', ip)
  if (starttime) target.set('starttime', starttime)
  if (endtime) target.set('endtime', endtime)
}

export const getPingData = (
  ip: string,
  starttime?: string,
  endtime?: string,
  signal?: AbortSignal
): Promise<PingLogData> => {
  let url = `/ping.json?ip=${encodeURIComponent(ip)}`
  if (starttime) url += `&starttime=${encodeURIComponent(starttime)}`
  if (endtime) url += `&endtime=${encodeURIComponent(endtime)}`
  return request.get(url, { signal })
}

export const getProxyPingData = (
  baseUrl: string,
  ip: string,
  starttime?: string,
  endtime?: string,
  signal?: AbortSignal
): Promise<PingLogData> => {
  const target = new URL('/api/ping.json', `${baseUrl}/`)
  appendPingQuery(target.searchParams, ip, starttime, endtime)
  const params = new URLSearchParams({ g: target.toString() })
  return request.get(`/proxy.json?${params.toString()}`, proxyRequestConfig(signal))
}
