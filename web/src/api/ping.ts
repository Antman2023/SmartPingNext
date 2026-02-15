import request from './index'
import type { PingLogData } from '@/types'

export const getPingData = (ip: string, starttime?: string, endtime?: string): Promise<PingLogData> => {
  let url = `/ping.json?ip=${encodeURIComponent(ip)}`
  if (starttime) url += `&starttime=${encodeURIComponent(starttime)}`
  if (endtime) url += `&endtime=${encodeURIComponent(endtime)}`
  return request.get(url)
}
