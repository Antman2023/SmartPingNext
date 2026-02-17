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
  const anyError = error as any

  if (anyError.response) {
    const status = anyError.response.status
    const data = anyError.response.data

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
  } else if (anyError.request) {
    return new ApiError('网络连接失败，请检查网络', 0)
  } else {
    return new ApiError((error as Error)?.message || '请求失败', 0)
  }
}
