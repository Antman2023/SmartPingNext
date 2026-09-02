import request, { proxyRequestConfig } from './index'

export const getTopology = (
  addr: string,
  port: number,
  localAddr: string,
  signal?: AbortSignal
): Promise<Record<string, string>> => {
  if (addr === localAddr) {
    return request.get('/topology.json', { signal })
  }
  const params = new URLSearchParams({ g: `http://${addr}:${port}/api/topology.json` })
  return request.get(`/proxy.json?${params.toString()}`, proxyRequestConfig(signal))
}
