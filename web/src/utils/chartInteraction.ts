import type { EChartsOption } from 'echarts'

interface NamedSeries { id: string; name: string }

// Match by stable ID because localized names and responsive legend types can change.
export function preserveLegendSelection(option: EChartsOption, previous?: Record<string, unknown>): void {
  if (!previous || !option.legend || Array.isArray(option.legend)) return
  const legends = previous.legend as Array<{ selected?: Record<string, boolean> }> | undefined
  const previousSeries = previous.series as NamedSeries[] | undefined
  const nextSeries = option.series as NamedSeries[] | undefined
  if (!legends?.length || !previousSeries?.length || !nextSeries) return
  const selected = legends[0]!.selected ?? {}
  option.legend.selected = Object.fromEntries(nextSeries.map((series) => {
    const oldSeries = previousSeries.find((item) => item.id === series.id)
    return [series.name, oldSeries ? selected[oldSeries.name] !== false : true]
  }))
}
