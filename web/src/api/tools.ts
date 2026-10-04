import request, { proxyRequestConfig, type ApiRequestConfig } from './index'
import type { ToolsResult } from '@/types'

export const runTools = (
  baseUrl: string,
  target: string,
  signal?: AbortSignal
): Promise<ToolsResult> => {
  const remote = new URL(`http://${baseUrl}/api/tools.json`)
  remote.searchParams.set('t', target)
  const params = new URLSearchParams({ t: '10', g: remote.toString() })
  // The tools view validates both success statistics and node rejection bodies.
  // HTTP errors and cancellation still use the shared response interceptor.
  const options: ApiRequestConfig = { ...proxyRequestConfig(signal, 10), validateBusinessStatus: false }
  return request.get(`/proxy.json?${params.toString()}`, options)
}
