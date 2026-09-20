import request, { proxyRequestConfig } from './index'
import type { AlertData } from '@/types'
import { isAlertData } from '@/utils/alertData'
import i18n from '@/locales'

export const getAlerts = async (
  baseUrl: string,
  date?: string,
  signal?: AbortSignal
): Promise<AlertData> => {
  const dateQuery = new URLSearchParams()
  if (date) dateQuery.set('date', date)
  let url = `/alert.json${dateQuery.size ? `?${dateQuery.toString()}` : ''}`
  if (baseUrl) {
    const target = new URL(`${baseUrl}/api/alert.json`)
    target.search = dateQuery.toString()
    url = `/proxy.json?${new URLSearchParams({ g: target.toString() }).toString()}`
  }
  const response = await request.get<unknown, unknown>(
    url,
    baseUrl ? proxyRequestConfig(signal) : { signal }
  )

  if (Array.isArray(response) && response.length === 2) {
    const data = { dates: response[0], logs: response[1] }
    if (isAlertData(data)) return data
  }
  throw new Error(i18n.global.t('common.invalidAlertResponse'))
}
