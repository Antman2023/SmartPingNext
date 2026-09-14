const DEFAULT_TIME_RANGE_HOURS = 6
const MAX_TIME_RANGE_HOURS = 31 * 24

function parseTimeRangeMinute(value: unknown): number | null {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2} \d{2}:\d{2}$/.test(value)) return null
  // These are node-local wall-clock values; avoid browser timezone conversion.
  const date = new Date(value.replace(' ', 'T') + ':00Z')
  if (!Number.isFinite(date.getTime()) || date.toISOString().slice(0, 16).replace('T', ' ') !== value) return null
  return date.getTime()
}

export function isValidTimeRange(start: unknown, end: unknown): boolean {
  const from = parseTimeRangeMinute(start)
  const to = parseTimeRangeMinute(end)
  return from !== null && to !== null && to >= from && to - from <= MAX_TIME_RANGE_HOURS * 3600000
}

export function normalizeTimeRangeHours(value: unknown): number {
  const hours = Number(value)
  return Number.isFinite(hours) && hours >= 1 / 60 && hours <= MAX_TIME_RANGE_HOURS
    ? hours
    : DEFAULT_TIME_RANGE_HOURS
}
