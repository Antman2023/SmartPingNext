export function formatDateTime(date: Date): string {
  const pad = (n: number) => (n < 10 ? '0' + n : n)
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function formatTime(date: Date): string {
  const pad = (n: number) => (n < 10 ? '0' + n : n)
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function formatDate(date: Date): string {
  const pad = (n: number) => (n < 10 ? '0' + n : n)
  return `${pad(date.getMonth() + 1)}-${pad(date.getDate())}`
}

export function extractTime(dateTimeStr: string): string {
  if (!dateTimeStr || dateTimeStr.length < 16) return dateTimeStr || ''
  return dateTimeStr.substring(11, 16)
}

export function extractDate(dateTimeStr: string): string {
  if (!dateTimeStr || dateTimeStr.length < 10) return dateTimeStr || ''
  return dateTimeStr.substring(5, 10)
}
