import request from './index'
import type { AlertData } from '@/types'

export const getAlerts = (baseUrl: string, date?: string): Promise<AlertData> => {
  let url = `/proxy.json?g=${baseUrl}/api/alert.json`
  if (date) url += `?date=${encodeURIComponent(date)}`
  return request.get(url)
}
