export function escapeTooltipText(value: unknown): string {
  return String(value ?? '').replace(/[&<>"']/g, (character) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'
  })[character]!)
}

export function tooltipNumber(value: unknown): number | null {
  if (typeof value !== 'number' && typeof value !== 'string') return null
  if (typeof value === 'string' && value.trim() === '') return null
  const number = Number(value)
  return Number.isFinite(number) ? number : null
}

export function formatMappingTooltip(params: { name?: unknown; seriesName?: unknown; value?: unknown }): string {
  const delay = tooltipNumber(params.value)
  const delayText = delay === null ? '--' : `${delay.toFixed(2)}ms`
  return `${escapeTooltipText(params.name)}<br/>${escapeTooltipText(params.seriesName)}: ${delayText}`
}
