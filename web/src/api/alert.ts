import request from './index'
import type { AlertData } from '@/types'

export const getAlerts = (baseUrl: string, date?: string): Promise<AlertData> => {
	const target = new URL(`${baseUrl}/api/alert.json`)
	if (date) target.searchParams.set('date', date)
	const params = new URLSearchParams({ g: target.toString() })
	return request.get(`/proxy.json?${params.toString()}`)
}
