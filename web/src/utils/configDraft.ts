import { reactive, ref, shallowReactive, type Ref } from 'vue'
import type { Config, NetworkMember, TopologyConfig } from '../types/index.js'

const observedRecords = new WeakSet<object>()
const observedMembers = new WeakSet<object>()
const observedRules = new WeakSet<object>()

interface RecordState {
  revision: Ref<number>
  version: number
  observe: (value: unknown) => unknown
}
const recordStates = new WeakMap<object, RecordState>()
const identity = (value: unknown) => value
const trackRecord = (target: object) => { void recordStates.get(target)!.revision.value }
const updateRecord = (state: RecordState) => { state.revision.value = ++state.version }
const recordHandler: ProxyHandler<object> = {
  get(target, key, receiver) {
    trackRecord(target)
    return Reflect.get(target, key, receiver)
  },
  set(target, key, value) {
    const state = recordStates.get(target)!
    const hadKey = Object.prototype.hasOwnProperty.call(target, key)
    const previous = Reflect.get(target, key)
    const observed = typeof key === 'string' ? state.observe(value) : value
    if (!Reflect.set(target, key, observed)) return false
    if (!hadKey || !Object.is(previous, observed)) updateRecord(state)
    return true
  },
  deleteProperty(target, key) {
    const hadKey = Object.prototype.hasOwnProperty.call(target, key)
    if (!Reflect.deleteProperty(target, key)) return false
    if (hadKey) updateRecord(recordStates.get(target)!)
    return true
  },
  defineProperty(target, key, descriptor) {
    if (!Reflect.defineProperty(target, key, descriptor)) return false
    updateRecord(recordStates.get(target)!)
    return true
  },
  has(target, key) {
    trackRecord(target)
    return Reflect.has(target, key)
  },
  ownKeys(target) {
    trackRecord(target)
    return Reflect.ownKeys(target)
  },
  getOwnPropertyDescriptor(target, key) {
    trackRecord(target)
    return Reflect.getOwnPropertyDescriptor(target, key)
  }
}

// Dictionary keys are data, including names reserved by Vue's object proxy.
// Keep the revision outside the data object and share proxy handlers, so each
// rule needs neither a Map nor a separate set of handler closures.
function reactiveRecord<T extends object>(record: T, observeValue?: (value: T[keyof T]) => T[keyof T]): T {
  if (observedRecords.has(record)) return record
  const target = Object.create(null) as T
  for (const key of Object.keys(record)) {
    // A supplied Vue proxy may shadow a data key in its getter. The own data
    // descriptor still contains the stored configuration value.
    const descriptor = Reflect.getOwnPropertyDescriptor(record, key)!
    const value = ('value' in descriptor ? descriptor.value : Reflect.get(record, key)) as T[keyof T]
    Reflect.set(target, key, observeValue ? observeValue(value) : value)
  }
  recordStates.set(target, {
    revision: ref(0), version: 0,
    observe: observeValue ? (value) => observeValue(value as T[keyof T]) : identity
  })
  const observed = new Proxy(target, recordHandler as ProxyHandler<T>)
  observedRecords.add(observed)
  return observed
}

function observeField<T extends object, K extends keyof T>(target: T, key: K, observe: (value: T[K]) => T[K]) {
  let value = observe(target[key])
  Object.defineProperty(target, key, {
    enumerable: true,
    configurable: true,
    get: () => value,
    set: (replacement: T[K]) => { value = observe(replacement) }
  })
}

function reactiveRules(rules: TopologyConfig[]): TopologyConfig[] {
  if (observedRules.has(rules)) return rules
  const observed = new Proxy(shallowReactive(Array.from(rules, (rule) => reactiveRecord(rule))), {
    set(target, key, value, receiver) {
      const index = typeof key === 'string' ? Number(key) : NaN
      const isIndex = Number.isInteger(index) && index >= 0 && index < 0xffffffff && String(index) === key
      return Reflect.set(target, key, isIndex ? reactiveRecord(value) : value, receiver)
    }
  })
  observedRules.add(observed)
  return observed
}

function reactiveMember(member: NetworkMember): NetworkMember {
  if (observedMembers.has(member)) return member
  const draft = { ...member }
  observeField(draft, 'Ping', (value) => reactive(value))
  observeField(draft, 'Topology', reactiveRules)
  const observed = shallowReactive(draft)
  observedMembers.add(observed)
  return observed
}

export function createConfigDraft(config: Config): Config {
  const draft = { ...config }
  observeField(draft, 'Mode', (value) => reactiveRecord(value))
  observeField(draft, 'Base', (value) => reactiveRecord(value))
  observeField(draft, 'Topology', (value) => reactiveRecord(value))
  observeField(draft, 'Network', (value) => reactiveRecord(value, reactiveMember))
  observeField(draft, 'Chinamap', (value) => reactiveRecord(value, (providers) => reactive(providers)))
  return shallowReactive(draft)
}
