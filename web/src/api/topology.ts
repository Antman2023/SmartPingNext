import request, { proxyRequestConfig } from './index'
import i18n from '@/locales'

const validateTopologyData = (value: unknown): Record<string, string> => {
  if (
    typeof value !== 'object' || value === null || Array.isArray(value) ||
    !Object.values(value).every((status) =>
      status === 'true' || status === 'false' || status === 'unknown'
    )
  ) {
    throw new Error(i18n.global.t('common.invalidTopologyResponse'))
  }
  return value as Record<string, string>
}

export const getTopology = async (
  addr: string,
  port: number,
  localAddr: string,
  signal?: AbortSignal
): Promise<Record<string, string>> => {
  if (addr === localAddr) {
    return validateTopologyData(await request.get<unknown, unknown>('/topology.json', { signal }))
  }
  const params = new URLSearchParams({ g: `http://${addr}:${port}/api/topology.json` })
  return validateTopologyData(
    await request.get<unknown, unknown>(`/proxy.json?${params.toString()}`, proxyRequestConfig(signal))
  )
}
