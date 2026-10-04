import request, { proxyRequestConfig } from './index'
import type { ChinaMapData } from '@/types'
import i18n from '@/locales'
import { isMinuteLabel } from '@/utils/calendarDate'

const isMappingData = (value: unknown): value is ChinaMapData => {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  if (
    typeof data.text !== 'string' ||
    !isMinuteLabel(data.subtext) ||
    typeof data.avgdelay !== 'object' ||
    data.avgdelay === null
  )
    return false
  const carriers = data.avgdelay as Record<string, unknown>
  return ['ctcc', 'cucc', 'cmcc'].every((carrier) => {
    const values = carriers[carrier]
    return (
      Array.isArray(values) &&
      values.every((point: unknown) => {
        if (typeof point !== 'object' || point === null) return false
        const entry = point as Record<string, unknown>
        return (
          typeof entry.name === 'string' &&
          typeof entry.value === 'number' &&
          Number.isFinite(entry.value) && entry.value >= 0
        )
      })
    )
  })
}

const validateMappingData = (value: unknown): ChinaMapData => {
  if (!isMappingData(value)) throw new Error(i18n.global.t('common.invalidMappingResponse'))
  return value
}

export const getMapping = async (d?: string, signal?: AbortSignal): Promise<ChinaMapData> => {
  let url = '/mapping.json'
  if (d) url += `?d=${encodeURIComponent(d)}`
  return validateMappingData(await request.get<unknown, unknown>(url, { signal }))
}

export const getProxyMapping = async (
  baseUrl: string,
  d?: string,
  signal?: AbortSignal
): Promise<ChinaMapData> => {
  const target = new URL(`${baseUrl}/api/mapping.json`)
  if (d) {
    target.searchParams.set('d', d)
  }

  const params = new URLSearchParams({ g: target.toString() })
  return validateMappingData(
    await request.get<unknown, unknown>(
      `/proxy.json?${params.toString()}`,
      proxyRequestConfig(signal)
    )
  )
}
