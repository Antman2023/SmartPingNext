import { ElMessage } from 'element-plus'

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
  data?: { message?: string }
}

interface AxiosError {
  response?: AxiosErrorResponse
  request?: unknown
  message?: string
}

function isAxiosError(error: unknown): error is AxiosError {
  return typeof error === 'object' && error !== null && ('response' in error || 'request' in error)
}

export function handleError(error: unknown, defaultMessage = '操作失败'): void {
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
  if (isAxiosError(error)) {
    if (error.response) {
      const status = error.response.status
      const data = error.response.data

      switch (status) {
        case 400:
          return new ApiError(data?.message || '请求参数错误', status)
        case 401:
          return new ApiError('未授权，请登录', status)
        case 403:
          return new ApiError('拒绝访问', status)
        case 404:
          return new ApiError('请求资源不存在', status)
        case 500:
          return new ApiError(data?.message || '服务器内部错误', status)
        case 502:
          return new ApiError('网关错误', status)
        case 503:
          return new ApiError('服务不可用', status)
        case 504:
          return new ApiError('网关超时', status)
        default:
          return new ApiError(data?.message || `请求失败 (${status})`, status)
      }
    } else if (error.request) {
      return new ApiError('网络连接失败，请检查网络', 0)
    }
  }

  return new ApiError(error instanceof Error ? error.message || '请求失败' : '请求失败', 0)
}
