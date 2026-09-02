import axios from 'axios'
import { handleNetworkError } from '@/utils/error'

const instance = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  timeout: Number(import.meta.env.VITE_API_TIMEOUT) || 15000
})

// 请求拦截器
instance.interceptors.request.use(
  (config) => {
    return config
  },
  (error) => {
    return Promise.reject(error)
  }
)

// 响应拦截器
instance.interceptors.response.use(
  (response) => {
    const res = response.data
    const responseStatus =
      typeof res === 'object' && res !== null && 'status' in res ? res.status : undefined
    // 统一处理业务状态：'true' 字符串、true 或 200 数字都视为成功。
    const isSuccess = responseStatus === 'true' || responseStatus === true || responseStatus === 200
    if (responseStatus !== undefined && !isSuccess) {
      throw handleNetworkError({ response: { status: response.status, data: res } })
    }
    return res
  },
  (error) => {
    return Promise.reject(handleNetworkError(error))
  }
)

export default instance
