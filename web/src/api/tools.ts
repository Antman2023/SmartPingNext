import request from './index'
import type { ToolsResult } from '@/types'

export const runTools = (baseUrl: string, target: string): Promise<ToolsResult> => {
  return request.get(`/proxy.json?t=10&g=http://${baseUrl}/api/tools.json?t=${encodeURIComponent(target)}`)
}
