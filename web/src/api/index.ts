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
    if (res.status && res.status !== 'true' && res.status !== 200) {
      throw handleNetworkError({ response: { status: res.status, data: res } })
    }
    return res
  },
  (error) => {
    return Promise.reject(handleNetworkError(error))
  }
)

export default instance
