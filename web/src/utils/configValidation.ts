import type { Config, TopologyConfig } from '@/types'

export const CONFIG_LIMITS = {
  timeout: { min: 1, max: 60 },
  archive: { min: 1, max: 36500 },
  refresh: { min: 1, max: 1440 },
  topologyLine: { min: 0, max: 20 },
  topologySymbol: { min: 0, max: 500 },
  toolLimit: { min: 0, max: 86400 },
  networkNodes: 1024,
  targetsPerNode: 1024,
  mappingTargets: 8192,
  authorizedIps: 4096
} as const

const OPTIONAL_BASE_LIMITS = {
  PingCount: { min: 1, max: 120 },
  PingIntervalMs: { min: 100, max: 60000 },
  PingTimeoutMs: { min: 100, max: 60000 },
  PingStaggerMs: { min: 0, max: 60000 },
  MappingConcurrency: { min: 1, max: 64 },
  MappingProbeCount: { min: 1, max: 20 }
} as const

export interface ConfigValidationIssue {
  key: string
  params?: Record<string, number | string>
}

const issue = (key: string, params?: Record<string, number | string>): ConfigValidationIssue => ({
  key,
  params
})

const isIntegerInRange = (value: number, min: number, max: number) =>
  Number.isInteger(value) && value >= min && value <= max

const isPositiveFloatInRange = (value: string, max: number) => {
  if (value.trim() === '' || value !== value.trim()) {
    return false
  }
  const parsed = Number(value)
  return Number.isFinite(parsed) && parsed > 0 && parsed <= max
}

const parseInteger = (value: string): number | null => {
  if (!/^[+-]?\d+$/.test(value)) {
    return null
  }
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) ? parsed : null
}

export const isValidIPv4 = (value: string): boolean => {
  if (value !== value.trim()) {
    return false
  }
  const parts = value.split('.')
  return (
    parts.length === 4 &&
    parts.every(
      (part) => /^\d{1,3}$/.test(part) && Number(part) <= 255 && String(Number(part)) === part
    )
  )
}

const isValidIPv6 = (value: string): boolean => {
  if (value !== value.trim() || !value.includes(':')) {
    return false
  }

  let normalized = value
  if (normalized.includes('.')) {
    const lastColon = normalized.lastIndexOf(':')
    const ipv4 = normalized.slice(lastColon + 1)
    if (lastColon < 0 || !isValidIPv4(ipv4)) {
      return false
    }
    normalized = `${normalized.slice(0, lastColon)}:0:0`
  }

  if (!/^[0-9a-f:]+$/i.test(normalized) || (normalized.match(/::/g) ?? []).length > 1) {
    return false
  }

  const hasCompression = normalized.includes('::')
  const [left = '', right = ''] = normalized.split('::')
  const leftParts = left ? left.split(':') : []
  const rightParts = right ? right.split(':') : []
  const parts = [...leftParts, ...rightParts]
  if (parts.some((part) => !/^[0-9a-f]{1,4}$/i.test(part))) {
    return false
  }
  return hasCompression ? parts.length < 8 : parts.length === 8
}

const isValidIPAddress = (value: string) => isValidIPv4(value) || isValidIPv6(value)

const validateTopologyRule = (
  source: string,
  rule: TopologyConfig,
  config: Config
): ConfigValidationIssue | null => {
  if (!isValidIPv4(rule.Addr) || !config.Network[rule.Addr] || !rule.Name.trim()) {
    return issue('config.validationTopologyTarget', { source, target: rule.Addr || '-' })
  }

  const checkSeconds = parseInteger(rule.Thdchecksec)
  const loss = parseInteger(rule.Thdloss)
  const avgDelay = parseInteger(rule.Thdavgdelay)
  const occurrences = parseInteger(rule.Thdoccnum)
  if (
    checkSeconds === null ||
    !isIntegerInRange(checkSeconds, 1, 86400) ||
    checkSeconds % 60 !== 0 ||
    loss === null ||
    !isIntegerInRange(loss, 0, 100) ||
    avgDelay === null ||
    !isIntegerInRange(avgDelay, 1, 60000) ||
    occurrences === null ||
    !isIntegerInRange(occurrences, 1, 10000) ||
    occurrences > checkSeconds / 60
  ) {
    return issue('config.validationTopologyRule', { source, target: rule.Addr })
  }
  return null
}

export const validateConfigForEdit = (config: Config): ConfigValidationIssue | null => {
  if (!config.Name.trim()) {
    return issue('config.validationNodeName')
  }
  if (!isIntegerInRange(config.Port, 1, 65535)) {
    return issue('config.validationPort')
  }
  if (!isValidIPv4(config.Addr)) {
    return issue('config.validationLocalAddress')
  }

  const baseRules = [
    ['config.validationTimeout', config.Base.Timeout, CONFIG_LIMITS.timeout],
    ['config.validationArchive', config.Base.Archive, CONFIG_LIMITS.archive],
    ['config.validationRefresh', config.Base.Refresh, CONFIG_LIMITS.refresh]
  ] as const
  for (const [key, value, limits] of baseRules) {
    if (!isIntegerInRange(value, limits.min, limits.max)) {
      return issue(key, limits)
    }
  }

  for (const [key, limits] of Object.entries(OPTIONAL_BASE_LIMITS)) {
    const value = config.Base[key as keyof typeof OPTIONAL_BASE_LIMITS]
    if (value !== undefined && !isIntegerInRange(value, limits.min, limits.max)) {
      return issue('config.validationBaseParameter', { name: key, ...limits })
    }
  }

  if (!isPositiveFloatInRange(config.Topology.Tline, CONFIG_LIMITS.topologyLine.max)) {
    return issue('config.validationLineWidth')
  }
  if (!isPositiveFloatInRange(config.Topology.Tsymbolsize, CONFIG_LIMITS.topologySymbol.max)) {
    return issue('config.validationSymbolSize')
  }
  if (
    !isIntegerInRange(config.Toollimit, CONFIG_LIMITS.toolLimit.min, CONFIG_LIMITS.toolLimit.max)
  ) {
    return issue('config.validationToolLimit')
  }

  const networkEntries = Object.entries(config.Network)
  if (networkEntries.length === 0) {
    return issue('config.validationNetworkEmpty')
  }
  if (networkEntries.length > CONFIG_LIMITS.networkNodes) {
    return issue('config.validationNetworkLimit', { max: CONFIG_LIMITS.networkNodes })
  }
  if (!config.Network[config.Addr]) {
    return issue('config.validationLocalNodeMissing')
  }

  for (const [address, member] of networkEntries) {
    if (!isValidIPv4(address) || !isValidIPv4(member.Addr) || address !== member.Addr) {
      return issue('config.validationNodeAddress', { address })
    }
    if (!member.Name.trim()) {
      return issue('config.validationNamedNode', { address })
    }
    if (
      member.Ping.length > CONFIG_LIMITS.targetsPerNode ||
      member.Topology.length > CONFIG_LIMITS.targetsPerNode
    ) {
      return issue('config.validationTargetLimit', {
        address,
        max: CONFIG_LIMITS.targetsPerNode
      })
    }

    const seenTargets = new Set<string>()
    for (const target of member.Ping) {
      if (!isValidIPv4(target) || !config.Network[target]) {
        return issue('config.validationPingTarget', { source: address, target })
      }
      if (seenTargets.has(target)) {
        return issue('config.validationDuplicateTarget', { source: address, target })
      }
      seenTargets.add(target)
    }

    for (const topologyRule of member.Topology) {
      const topologyIssue = validateTopologyRule(address, topologyRule, config)
      if (topologyIssue) {
        return topologyIssue
      }
    }
  }

  let mappingTargetCount = 0
  for (const providers of Object.values(config.Chinamap)) {
    for (const addresses of [providers.ctcc, providers.cucc, providers.cmcc]) {
      mappingTargetCount += addresses.length
      if (mappingTargetCount > CONFIG_LIMITS.mappingTargets) {
        return issue('config.validationMappingLimit', { max: CONFIG_LIMITS.mappingTargets })
      }
      const invalidAddress = addresses.find((address) => address !== '' && !isValidIPv4(address))
      if (invalidAddress) {
        return issue('config.validationMappingAddress', { address: invalidAddress })
      }
    }
  }

  const authorizedAddresses = config.Authiplist.replace(/ /g, '').split(',')
  if (authorizedAddresses.length > CONFIG_LIMITS.authorizedIps) {
    return issue('config.validationAuthLimit', { max: CONFIG_LIMITS.authorizedIps })
  }
  const invalidAuthorizedAddress = authorizedAddresses.find(
    (address) => address !== '' && !isValidIPAddress(address)
  )
  if (invalidAuthorizedAddress) {
    return issue('config.validationAuthAddress', { address: invalidAuthorizedAddress })
  }

  const modeType = config.Mode.Type ?? ''
  if (modeType !== '' && modeType !== 'local' && modeType !== 'cloud') {
    return issue('config.validationMode')
  }
  if (modeType === 'cloud') {
    try {
      if (!/^https?:\/\//.test(config.Mode.Endpoint ?? '')) {
        return issue('config.validationCloudEndpoint')
      }
      const endpoint = new URL(config.Mode.Endpoint ?? '')
      if (!['http:', 'https:'].includes(endpoint.protocol) || !endpoint.hostname) {
        return issue('config.validationCloudEndpoint')
      }
    } catch {
      return issue('config.validationCloudEndpoint')
    }
  }

  return null
}
