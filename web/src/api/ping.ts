import request, { proxyRequestConfig } from './index'
import type { PingLogData } from '@/types'
import i18n from '@/locales'
import { isPingLogData } from '@/utils/pingData'

const validatePingData = (value: unknown): PingLogData => {
  if (!isPingLogData(value)) throw new Error(i18n.global.t('common.invalidPingResponse'))
  return value
}

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

export const getPingData = async (
  ip: string,
  starttime?: string,
  endtime?: string,
  signal?: AbortSignal
): Promise<PingLogData> => {
  let url = `/ping.json?ip=${encodeURIComponent(ip)}`
  if (starttime) url += `&starttime=${encodeURIComponent(starttime)}`
  if (endtime) url += `&endtime=${encodeURIComponent(endtime)}`
  return validatePingData(await request.get<unknown, unknown>(url, { signal }))
}

export const getProxyPingData = async (
  baseUrl: string,
  ip: string,
  starttime?: string,
  endtime?: string,
  signal?: AbortSignal
): Promise<PingLogData> => {
  const target = new URL('/api/ping.json', `${baseUrl}/`)
  appendPingQuery(target.searchParams, ip, starttime, endtime)
  const params = new URLSearchParams({ g: target.toString() })
  return validatePingData(
    await request.get<unknown, unknown>(
      `/proxy.json?${params.toString()}`,
      proxyRequestConfig(signal)
    )
  )
}
