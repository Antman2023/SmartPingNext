import type { AlertData, AlertLog } from '../types/index.js'

const alertLogFields = ['Logtime', 'Targetip', 'Targetname', 'Tracert', 'Fromip', 'Fromname']
const calendarDatePattern = /^(-?\d{4})-(\d{2})-(\d{2})$/
// Keep stored civil labels, seconds/fractions, SQLite's 24-hour forms and
// timezone suffixes. Do not parse in the browser's timezone or rewrite labels.
const alertTimestampPattern = /^(-?\d{4})-(\d{2})-(\d{2})(?:[T \t\r\n\f\v]*(\d{2}):(\d{2})(?::(\d{2})(?:\.(\d+))?)?(?:[ \t\r\n\f\v]*(?:[Zz]|[+-](\d{2}):(\d{2})))?)?[ \t\r\n\f\v]*$/
const monthLengths = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]

const isCalendarDate = (year: number, month: number, day: number): boolean => {
  const days = month === 2 && year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
    ? 29 : (monthLengths[month - 1] ?? 0)
  return day >= 1 && day <= days
}

const isAlertDate = (value: unknown): value is string => {
  if (typeof value !== 'string') return false
  const parts = calendarDatePattern.exec(value)
  // JavaScript's $ also matches before a final newline; archive labels must
  // match the entire string so they can be used unchanged as query dates.
  return parts !== null && parts[0] === value && isCalendarDate(+parts[1]!, +parts[2]!, +parts[3]!)
}

const parseAlertTimestamp = (value: string): RegExpExecArray | null => {
  const parts = alertTimestampPattern.exec(value)
  if (parts !== null && parts[0] === value &&
    isCalendarDate(+parts[1]!, +parts[2]!, +parts[3]!) &&
    (parts[4] === undefined || (+parts[4] <= 24 && +parts[5]! <= 59 && +(parts[6] ?? 0) <= 59)) &&
    (parts[8] === undefined || (+parts[8] <= 14 && +parts[9]! <= 59))) return parts
  return null
}

export const isAlertData = (value: unknown): value is AlertData => {
  if (typeof value !== 'object' || value === null) return false
  const data = value as Record<string, unknown>
  return (
    Array.isArray(data.dates) &&
    data.dates.every(isAlertDate) &&
    Array.isArray(data.logs) &&
    data.logs.every((log: unknown) => {
      if (typeof log !== 'object' || log === null) return false
      const record = log as Record<string, unknown>
      return alertLogFields.every((key) => typeof record[key] === 'string') &&
        parseAlertTimestamp(record.Logtime as string) !== null
    })
  )
}

const sortAlertTimes = <T>(values: readonly T[], label: (value: T) => string): T[] => {
  // Parse each label once, not once per comparison. UTC arithmetic compares
  // stored civil fields without the browser's timezone. Node timezone metadata
  // is unavailable, so offset suffixes do not convert other nodes' local clocks.
  const entries = values.map((value, index) => {
    const parts = parseAlertTimestamp(label(value))
    if (!parts) throw new Error('Invalid alert timestamp')
    const year = +parts[1]!
    const earlyYear = year >= 0 && year < 100
    let time = Date.UTC(earlyYear ? year + 400 : year, +parts[2]! - 1, +parts[3]!,
      +(parts[4] ?? 0), +(parts[5] ?? 0), +(parts[6] ?? 0))
    // Date.UTC treats years 0..99 as 1900..1999. A Gregorian 400-year cycle
    // preserves leap days while moving these years outside that special case.
    if (earlyYear) time -= 146097 * 24 * 60 * 60 * 1000
    const fraction = parts[7] ?? ''
    let end = fraction.length
    while (end > 0 && fraction.charCodeAt(end - 1) === 48) end--
    return { value, index, time, fraction: fraction.slice(0, end) }
  })
  entries.sort((a, b) => b.time - a.time || (
    a.fraction === b.fraction ? a.index - b.index : a.fraction < b.fraction ? 1 : -1
  ))
  return entries.map(({ value }) => value)
}

export const sortAlertDates = (dates: readonly string[]): string[] =>
  sortAlertTimes(dates, (date) => date)

export const sortAlertRecords = <T extends AlertLog>(records: readonly T[]): T[] =>
  sortAlertTimes(records, (record) => record.Logtime)
