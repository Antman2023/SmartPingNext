import request from './index'

export const getTopology = (addr: string, port: number): Promise<Record<string, string>> => {
	const params = new URLSearchParams({ g: `http://${addr}:${port}/api/topology.json` })
	return request.get(`/proxy.json?${params.toString()}`)
}
