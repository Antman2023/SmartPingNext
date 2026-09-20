import type { AlertData } from '../types/index.js'

export const isAlertData = (value: unknown): value is AlertData => {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  return (
    Array.isArray(data.dates) &&
    data.dates.every((date) => typeof date === 'string') &&
    Array.isArray(data.logs) &&
    data.logs.every(
      (log: unknown) =>
        typeof log === 'object' &&
        log !== null &&
        ['Logtime', 'Targetip', 'Targetname', 'Tracert', 'Fromip', 'Fromname'].every(
          (key) => typeof (log as Record<string, unknown>)[key] === 'string'
        )
    )
  )
}
