import assert from 'node:assert/strict'
import test from 'node:test'
import type { Config } from '../types/index.js'
import { CONFIG_LIMITS, isValidIPv4, validateConfigForEdit } from './configValidation.js'

const makeConfig = (): Config => ({
  Ver: 'test',
  Port: 8899,
  Name: 'Local',
  Addr: '192.0.2.1',
  Mode: { Type: 'local', Endpoint: '' },
  Base: {
    Timeout: 5,
    Archive: 10,
    Refresh: 1,
    PingCount: 20,
    PingIntervalMs: 3000,
    PingTimeoutMs: 3000,
    PingStaggerMs: 100,
    MappingConcurrency: 8,
    MappingProbeCount: 3
  },
  Topology: { Tline: '1', Tsymbolsize: '70', Tsound: '/alert.mp3' },
  Network: {
    '192.0.2.1': {
      Name: 'Local',
      Addr: '192.0.2.1',
      Smartping: true,
      Ping: ['198.51.100.2'],
      Topology: [
        {
          Name: 'Remote',
          Addr: '198.51.100.2',
          Thdchecksec: '900',
          Thdoccnum: '3',
          Thdavgdelay: '200',
          Thdloss: '30'
        }
      ]
    },
    '198.51.100.2': {
      Name: 'Remote',
      Addr: '198.51.100.2',
      Smartping: true,
      Ping: [],
      Topology: []
    }
  },
  Chinamap: {
    Shanghai: {
      ctcc: ['203.0.113.1'],
      cucc: [],
      cmcc: []
    }
  },
  Toollimit: 0,
  Authiplist: '127.0.0.1,2001:db8::1'
})

test('validateConfigForEdit accepts a complete valid configuration', () => {
  assert.equal(validateConfigForEdit(makeConfig()), null)
})

test('isValidIPv4 rejects ambiguous and malformed addresses', () => {
  for (const address of ['127.0.0.1', '0.0.0.0', '255.255.255.255']) {
    assert.equal(isValidIPv4(address), true, address)
  }
  for (const address of ['127.0.0.01', '256.0.0.1', '127.0.0', ' 127.0.0.1', '::1']) {
    assert.equal(isValidIPv4(address), false, address)
  }
})

test('validateConfigForEdit rejects invalid optional base values', () => {
  const config = makeConfig()
  config.Base.MappingConcurrency = Number.NaN

  assert.equal(validateConfigForEdit(config)?.key, 'config.validationBaseParameter')
})

test('missing topology fields return validation issues instead of throwing', () => {
  for (const [field, key] of [['Tline', 'config.validationLineWidth'], ['Tsymbolsize', 'config.validationSymbolSize']] as const) {
    const config = makeConfig()
    delete (config.Topology as unknown as Record<string, unknown>)[field]
    assert.equal(validateConfigForEdit(config)?.key, key)
  }
  for (const field of ['Name', 'Addr', 'Thdchecksec', 'Thdoccnum', 'Thdavgdelay', 'Thdloss']) {
    const config = makeConfig()
    delete (config.Network[config.Addr]!.Topology[0] as unknown as Record<string, unknown>)[field]
    const key = field === 'Name' || field === 'Addr' ? 'config.validationTopologyTarget' : 'config.validationTopologyRule'
    assert.equal(validateConfigForEdit(config)?.key, key, field)
    const wrongType = makeConfig()
    const rule = wrongType.Network[wrongType.Addr]!.Topology[0] as unknown as Record<string, unknown>
    rule[field] = field === 'Name' || field === 'Addr' ? 123 : Number(rule[field])
    assert.equal(validateConfigForEdit(wrongType)?.key, key, `${field} numeric value`)
  }
  for (const value of [undefined, null, 1, {}, []]) {
    const config = makeConfig()
    ;(config.Topology as unknown as Record<string, unknown>).Tline = value
    assert.equal(validateConfigForEdit(config)?.key, 'config.validationLineWidth')
    const ruleConfig = makeConfig()
    ;(ruleConfig.Network[ruleConfig.Addr]!.Topology[0] as unknown as Record<string, unknown>).Name = value
    assert.equal(validateConfigForEdit(ruleConfig)?.key, 'config.validationTopologyTarget')
    assert.equal(isValidIPv4(value), false)
  }
})

test('base extensions require exactly representable integers and keep known limit messages', () => {
  for (const name of ['custom', '__proto__', '__v_isReactive']) {
    for (const value of [0.5, -0.5, Number.MAX_SAFE_INTEGER + 1, -Number.MAX_SAFE_INTEGER - 1, Number.NaN, Number.POSITIVE_INFINITY]) {
      const config = makeConfig()
      Object.defineProperty(config.Base, name, { value, enumerable: true, configurable: true })
      const issue = validateConfigForEdit(config)
      assert.equal(issue?.key, 'config.validationBaseInteger', `${name}/${value}`)
      assert.equal(issue?.params?.name, name)
    }
    for (const value of [0, 1, -1, Number.MAX_SAFE_INTEGER, -Number.MAX_SAFE_INTEGER]) {
      const config = makeConfig()
      Object.defineProperty(config.Base, name, { value, enumerable: true, configurable: true })
      assert.equal(validateConfigForEdit(config), null, `${name}/${value}`)
    }
  }
  const config = makeConfig()
  config.Base.Timeout = 1.5
  assert.equal(validateConfigForEdit(config)?.key, 'config.validationTimeout')
  config.Base.Timeout = 5
  config.Base.PingCount = 1.5
  assert.equal(validateConfigForEdit(config)?.key, 'config.validationBaseParameter')
})

test('topology display controls do not offer values rejected by validation', () => {
  assert.equal(CONFIG_LIMITS.topologyLine.min > 0, true)
  assert.equal(CONFIG_LIMITS.topologySymbol.min > 0, true)

  const zeroLine = makeConfig()
  zeroLine.Topology.Tline = '0'
  assert.equal(validateConfigForEdit(zeroLine)?.key, 'config.validationLineWidth')

  const zeroSymbol = makeConfig()
  zeroSymbol.Topology.Tsymbolsize = '0'
  assert.equal(validateConfigForEdit(zeroSymbol)?.key, 'config.validationSymbolSize')
})

test('validateConfigForEdit rejects non-decimal topology values', () => {
  for (const value of ['0x10', '0b10', '0o10', '0x1p4']) {
    const config = makeConfig()
    config.Topology.Tline = value
    assert.equal(validateConfigForEdit(config)?.key, 'config.validationLineWidth', value)
  }

  for (const value of ['.5', '1.', '+1', '1e1']) {
    const config = makeConfig()
    config.Topology.Tline = value
    assert.equal(validateConfigForEdit(config), null, value)
  }
})

test('validateConfigForEdit rejects duplicate ping targets', () => {
  const config = makeConfig()
  config.Network[config.Addr]!.Ping.push('198.51.100.2')

  assert.equal(validateConfigForEdit(config)?.key, 'config.validationDuplicateTarget')
})

test('validateConfigForEdit enforces topology sample-window constraints', () => {
  const config = makeConfig()
  config.Network[config.Addr]!.Topology[0]!.Thdchecksec = '120'
  config.Network[config.Addr]!.Topology[0]!.Thdoccnum = '3'

  assert.equal(validateConfigForEdit(config)?.key, 'config.validationTopologyRule')
})

test('validateConfigForEdit rejects unsafe cloud endpoints', () => {
  const endpoints = [
    'file:///tmp/config.json',
    'https://user:secret@example.com/config.json',
    'https://@example.com/config.json',
    'https://:@example.com/config.json',
    'https://example.com/config.json#private',
    'https://example.com:65536/config.json'
  ]

  for (const endpoint of endpoints) {
    const config = makeConfig()
    config.Mode = { Type: 'cloud', Endpoint: endpoint }
    assert.equal(validateConfigForEdit(config)?.key, 'config.validationCloudEndpoint', endpoint)
  }
})

test('validateConfigForEdit rejects unsupported mapping carriers', () => {
  const config = makeConfig()
  const providers = config.Chinamap.Shanghai as unknown as Record<string, string[]>
  providers.unknown = ['203.0.113.2']

  assert.equal(validateConfigForEdit(config)?.key, 'config.validationMappingProvider')
})
