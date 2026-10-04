import type { PingLogData } from '../types/index.js'
import { isMinuteLabel } from './calendarDate.js'

const decimalNumberPattern = /^[+-]?(?:\d+(?:\.\d*)?|\.\d+)(?:[eE][+-]?\d+)?$/
const metricFields = ['maxdelay', 'mindelay', 'avgdelay', 'losspk']

export const isPingLogData = (value: unknown): value is PingLogData => {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  // Minute labels come from the node's elapsed-time timeline. Clock rollbacks
  // can repeat or reverse labels, so validation must not reorder them.
  if (!Array.isArray(data.lastcheck) || !data.lastcheck.every(isMinuteLabel)) return false
  const size = data.lastcheck.length
  return metricFields.every((key) => {
    const values = data[key]
    return Array.isArray(values) && values.length === size && values.every((sample) => {
      if (typeof sample !== 'string') return false
      if (sample === '-') return true
      if (!decimalNumberPattern.test(sample)) return false
      const numeric = Number(sample)
      return Number.isFinite(numeric) && numeric >= 0 && (key !== 'losspk' || numeric <= 100)
    })
  })
}
