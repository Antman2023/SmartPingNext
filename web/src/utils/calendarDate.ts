const monthLengths = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31]

// Validate civil fields without parsing them in the browser's timezone.
export const isCalendarDate = (year: number, month: number, day: number): boolean => {
  const days = month === 2 && year % 4 === 0 && (year % 100 !== 0 || year % 400 === 0)
    ? 29 : (monthLengths[month - 1] ?? 0)
  return day >= 1 && day <= days
}

const minuteLabelPattern = /^(-?\d{4})-(\d{2})-(\d{2}) (\d{2}):(\d{2})$/

export const isMinuteLabel = (value: unknown): value is string => {
  if (typeof value !== 'string') return false
  const parts = minuteLabelPattern.exec(value)
  return parts !== null && parts[0] === value &&
    isCalendarDate(+parts[1]!, +parts[2]!, +parts[3]!) && +parts[4]! <= 23 && +parts[5]! <= 59
}
