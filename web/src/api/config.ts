import request, { proxyRequestConfig } from './index'
import type { Config } from '@/types'
import { ApiError } from '@/utils/error'
import { normalizeConfigResponse } from '@/utils/configResponse'
import i18n from '@/locales'

export type PasswordVerificationResult = 'valid' | 'invalid' | 'rate-limited'

export const getConfigUrl = (): string => request.getUri({ url: '/config.json' })

export const verifyConfigPassword = async (
  password: string,
  signal?: AbortSignal
): Promise<PasswordVerificationResult> => {
  try {
    const result = await request.post<unknown, unknown>(
      '/verify-password.json',
      new URLSearchParams({ password }),
      { signal }
    )
    return typeof result === 'object' &&
      result !== null &&
      'status' in result &&
      result.status === 'true'
      ? 'valid'
      : 'invalid'
  } catch (error) {
    // The shared interceptor rejects business failures even when HTTP is 200.
    if (error instanceof ApiError) {
      if (error.status === 429) return 'rate-limited'
      if (error.status === 200) return 'invalid'
    }
    throw error
  }
}

const validateConfigResponse = (value: unknown): Config => {
  const config = normalizeConfigResponse(value)
  if (!config) throw new Error(i18n.global.t('common.invalidConfigResponse'))
  return config
}

export const fetchConfig = async (signal?: AbortSignal): Promise<Config> => {
  return validateConfigResponse(await request.get<unknown, unknown>('/config.json', { signal }))
}

export const fetchProxyConfig = async (url: string, signal?: AbortSignal): Promise<Config> => {
  const params = new URLSearchParams({ g: `${url}/api/config.json` })
  return validateConfigResponse(await request.get<unknown, unknown>(`/proxy.json?${params.toString()}`, proxyRequestConfig(signal)))
}

export const saveConfig = async (
  config: Config,
  password: string,
  signal?: AbortSignal
): Promise<{ status: string; info?: string }> => {
  const data = new URLSearchParams()
  data.append('config', JSON.stringify(config))
  data.append('password', password)
  const result = await request.post<unknown, unknown>('/saveconfig.json', data, { signal })
  if (typeof result !== 'object' || result === null || Array.isArray(result) ||
    !('status' in result) || ![true, 'true', 200].includes(result.status as string | number | boolean)) {
    throw new Error(i18n.global.t('common.configSaveUnconfirmed'))
  }
  return {
    status: 'true',
    ...('info' in result && typeof result.info === 'string' ? { info: result.info } : {})
  }
}
