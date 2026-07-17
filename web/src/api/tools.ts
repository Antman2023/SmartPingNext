import request from './index'
import type { ToolsResult } from '@/types'

export const runTools = (baseUrl: string, target: string): Promise<ToolsResult> => {
	const remote = new URL(`http://${baseUrl}/api/tools.json`)
	remote.searchParams.set('t', target)
	const params = new URLSearchParams({ t: '10', g: remote.toString() })
	return request.get(`/proxy.json?${params.toString()}`)
}
