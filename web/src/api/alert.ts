import request, { proxyRequestConfig } from './index'
import type { AlertData, AlertLog } from '@/types'

type AlertApiResponse = [string[], AlertLog[]]

export const getAlerts = async (
  baseUrl: string,
  date?: string,
  signal?: AbortSignal
): Promise<AlertData> => {
  const target = new URL(`${baseUrl}/api/alert.json`)
  if (date) target.searchParams.set('date', date)
  const params = new URLSearchParams({ g: target.toString() })
  const [dates, logs] = await request.get<AlertApiResponse, AlertApiResponse>(
    `/proxy.json?${params.toString()}`,
    proxyRequestConfig(signal)
  )

  return { dates, logs }
}
