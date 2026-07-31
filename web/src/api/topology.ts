import request from './index'

export const getTopology = (
  addr: string,
  port: number,
  localAddr: string
): Promise<Record<string, string>> => {
  if (addr === localAddr) {
    return request.get('/topology.json')
  }
  const params = new URLSearchParams({ g: `http://${addr}:${port}/api/topology.json` })
  return request.get(`/proxy.json?${params.toString()}`)
}
