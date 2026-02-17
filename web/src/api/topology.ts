import request from './index'

export const getTopology = (addr: string, port: number): Promise<Record<string, string>> => {
  return request.get(`/proxy.json?g=http://${addr}:${port}/api/topology.json`)
}
