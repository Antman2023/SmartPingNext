import { ElMessage } from 'element-plus'
import i18n from '@/locales'
import { isRequestTimeout } from '@/utils/requestErrors'

export class ApiError extends Error {
  constructor(
    message: string,
    public status?: number,
    public code?: string
  ) {
    super(message)
    this.name = 'ApiError'
  }
}

interface AxiosErrorResponse {
  status: number
  data?: unknown
}

function responseMessage(data: unknown, fields: string[]): string | undefined {
  if (typeof data !== 'object' || data === null || Array.isArray(data)) return undefined
  const record = data as Record<string, unknown>
  for (const field of fields) {
    const value = record[field]
    if (typeof value === 'string' && value.trim()) return value
  }
  return undefined
}

interface AxiosError {
  response?: AxiosErrorResponse
  request?: unknown
  message?: string
}

function isAxiosError(error: unknown): error is AxiosError {
  return typeof error === 'object' && error !== null && ('response' in error || 'request' in error)
}

export function handleError(error: unknown, defaultMessage = i18n.global.t('common.failed')): void {
  let message = defaultMessage

  if (error instanceof ApiError) {
    message = error.message
  } else if (error instanceof Error) {
    message = error.message || defaultMessage
  } else if (typeof error === 'string') {
    message = error
  }

  console.error(error)
  ElMessage.error(message)
}

export function handleNetworkError(error: unknown): ApiError {
  if (isRequestTimeout(error)) {
    return new ApiError(i18n.global.t('common.requestTimeout'), 0, 'request_timeout')
  }
  if (isAxiosError(error)) {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data

      switch (status) {
        case 400:
          return new ApiError(
            responseMessage(data, ['info', 'message']) || i18n.global.t('common.badRequest'),
            status
          )
        case 401:
          return new ApiError(i18n.global.t('common.unauthorized'), status)
        case 403:
          return new ApiError(i18n.global.t('common.forbidden'), status)
        case 404:
          return new ApiError(i18n.global.t('common.notFound'), status)
        case 429:
          return new ApiError(i18n.global.t('common.tooManyRequests'), status)
        case 500:
          return new ApiError(
            responseMessage(data, ['info', 'message']) || i18n.global.t('common.serverError'),
            status
          )
        case 502:
          return new ApiError(i18n.global.t('common.gatewayError'), status)
        case 503:
          return new ApiError(i18n.global.t('common.serviceUnavailable'), status)
        case 504:
          return new ApiError(i18n.global.t('common.gatewayTimeout'), status)
        default:
          return new ApiError(
            responseMessage(data, ['info', 'message', 'error']) ||
              i18n.global.t('common.requestFailedWithStatus', { status }),
            status
          )
      }
    } else if (error.request) {
      return new ApiError(i18n.global.t('common.networkFailed'), 0)
    }
  }

  return new ApiError(
    error instanceof Error
      ? error.message || i18n.global.t('common.requestFailed')
      : i18n.global.t('common.requestFailed'),
    0
  )
}
