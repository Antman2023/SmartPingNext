import request, { proxyRequestConfig } from './index'
import type { PingLogData } from '@/types'
import i18n from '@/locales'

const isPingLogData = (value: unknown): value is PingLogData => {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  if (!Array.isArray(data.lastcheck) || !data.lastcheck.every((time) => typeof time === 'string')) {
    return false
  }
  const size = data.lastcheck.length
  return ['maxdelay', 'mindelay', 'avgdelay', 'losspk'].every((key) => {
    const values = data[key]
    return (
      Array.isArray(values) &&
      values.length === size &&
      values.every((sample) => {
        if (typeof sample !== 'string') return false
        if (sample === '-') return true
        if (!/^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/.test(sample)) return false
        const value = Number(sample)
        return Number.isFinite(value) && value >= 0 && (key !== 'losspk' || value <= 100)
      })
    )
  })
}

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
