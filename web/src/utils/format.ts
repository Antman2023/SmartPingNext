import i18n from '@/locales'

const pad = (n: number) => (n < 10 ? '0' + n : n)

export function formatDateTime(date: Date): string {
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function formatTime(date: Date): string {
  return `${pad(date.getHours())}:${pad(date.getMinutes())}`
}

export function formatDate(date: Date): string {
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

/**
 * 将节点名按当前 locale 翻译（如英文模式下 "本机" → "Local"）
 */
export function displayName(name: string): string {
  const t = i18n.global.t as (key: string) => string
  const translated = t(`nodeName.${name}`)
  // t() 在找不到 key 时返回 key 本身（含前缀），此时保持原名
  if (translated === `nodeName.${name}`) {
    return name
  }
  return translated
}
