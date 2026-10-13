import type { EChartsType } from 'echarts/core'

// Callers own readiness checks and user feedback; export failures propagate.
export function downloadChartImage(
  chart: Pick<EChartsType, 'getDataURL'>,
  filename: string,
  isDark: boolean
): void {
  const url = chart.getDataURL({
    type: 'png',
    pixelRatio: 2,
    backgroundColor: isDark ? '#1a1a1a' : '#fff'
  })
  const prefix = 'data:image/png;base64,'
  if (typeof url !== 'string' || !url.startsWith(prefix) || url.length === prefix.length) {
    throw new Error('Empty or invalid PNG export')
  }
  const link = document.createElement('a')
  link.download = filename
  link.href = url
  link.click()
}
