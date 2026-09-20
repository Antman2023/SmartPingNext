import { CONFIG_LIMITS } from './configValidation.js'

export const resolveRefreshInterval = (minutes: unknown): number => {
  const { min, max } = CONFIG_LIMITS.refresh
  const value = typeof minutes === 'number' && Number.isFinite(minutes) ? minutes : min
  // Bound before converting to milliseconds so malformed remote values cannot overflow timers.
  return Math.min(Math.max(value, min), max) * 60_000
}
