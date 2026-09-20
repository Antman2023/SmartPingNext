import type { Config } from '../types/index.js'

const isRecord = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
const isString = (value: unknown): value is string => typeof value === 'string'
const isNumber = (value: unknown): value is number =>
  typeof value === 'number' && Number.isFinite(value)
const isStringMap = (value: unknown): value is Record<string, string> =>
  isRecord(value) && Object.values(value).every(isString)
const stringList = (value: unknown): string[] | null =>
  value == null ? [] : Array.isArray(value) && value.every(isString) ? value : null

// Validate structure before publishing remote data into shared stores or views.
// Nil Go slices/maps represent empty collections, not malformed responses.
export const normalizeConfigResponse = (value: unknown): Config | null => {
  if (!isRecord(value) ||
    !['Ver', 'Name', 'Addr', 'Authiplist'].every((key) => isString(value[key])) ||
    !isNumber(value.Port) || !isNumber(value.Toollimit) ||
    !isRecord(value.Base) || !Object.values(value.Base).every(isNumber) ||
    !['Timeout', 'Refresh', 'Archive'].every((key) => isNumber((value.Base as Record<string, unknown>)[key])) ||
    !isStringMap(value.Topology) ||
    !['Tline', 'Tsymbolsize'].every((key) => isString((value.Topology as Record<string, unknown>)[key])) ||
    (value.Mode != null && !isStringMap(value.Mode)) ||
    !isRecord(value.Network) || (value.Chinamap != null && !isRecord(value.Chinamap))) {
    return null
  }

  const network: Config['Network'] = Object.create(null)
  for (const [key, member] of Object.entries(value.Network)) {
    if (!isRecord(member) || !isString(member.Name) || !isString(member.Addr) ||
      typeof member.Smartping !== 'boolean') return null
    const ping = stringList(member.Ping)
    const topology = member.Topology ?? []
    if (!ping || !Array.isArray(topology) || !topology.every((rule) =>
      isStringMap(rule) && ['Name', 'Addr', 'Thdchecksec', 'Thdoccnum', 'Thdavgdelay', 'Thdloss']
        .every((field) => isString(rule[field])))) return null
    network[key] = { ...member, Name: member.Name, Addr: member.Addr,
      Smartping: member.Smartping, Ping: ping, Topology: topology }
  }

  const chinamap: Config['Chinamap'] = Object.create(null)
  for (const [province, providers] of Object.entries(value.Chinamap ?? {})) {
    if (providers != null && !isRecord(providers)) return null
    const ctcc = stringList(providers?.ctcc)
    const cucc = stringList(providers?.cucc)
    const cmcc = stringList(providers?.cmcc)
    if (!ctcc || !cucc || !cmcc) return null
    chinamap[province] = { ...providers, ctcc, cucc, cmcc }
  }

  return { ...value, Mode: value.Mode ?? {},
    Topology: { Tsound: '', ...value.Topology }, Network: network, Chinamap: chinamap } as Config
}
