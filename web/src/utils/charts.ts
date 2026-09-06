import type { EChartsOption } from 'echarts'
import type { TooltipComponentFormatterCallbackParams } from 'echarts'
import type { PingLogData } from '@/types'
import { escapeTooltipText, tooltipNumber } from './chartTooltip.js'

// ECharts tooltip formatter 参数类型 (trigger: 'axis' 时为数组)
interface TooltipFormatterParam {
  name: string
  value: number | string
  seriesName: string
  marker: string
  dataIndex: number
}

interface ChartTheme {
  isDark: boolean
  textColor: string
  textColorSecondary: string
  borderColor: string
  splitLineColor: string
  backgroundColor: string
}

export interface PingChartLabels {
  maxDelay: string
  averageDelay: string
  minDelay: string
  lossRate: string
  latency: string
  loss: string
}

function getThemeConfig(isDark: boolean): ChartTheme {
  return {
    isDark,
    textColor: isDark ? '#e5eaf3' : '#303133',
    textColorSecondary: isDark ? '#a3a6ad' : '#606266',
    borderColor: isDark ? '#4c4d4f' : '#dcdfe6',
    splitLineColor: isDark ? '#363637' : '#ebeef5',
    backgroundColor: isDark ? '#1a1a1a' : '#fff'
  }
}

export function getPingChartOption(
  data: PingLogData | null,
  isDark: boolean,
  labels: PingChartLabels,
  showDataZoom = false,
  compact = false
): EChartsOption {
  const theme = getThemeConfig(isDark)
  let lastRenderedIndex = -1
  let lastRenderedDate = ''

  return {
    backgroundColor: 'transparent',
    title: {
      text: '',
      left: 'center',
      textStyle: {
        color: theme.textColor
      }
    },
    tooltip: {
      trigger: 'axis',
      backgroundColor: theme.backgroundColor,
      borderColor: theme.borderColor,
      textStyle: {
        color: theme.textColor
      },
      formatter: (params: TooltipComponentFormatterCallbackParams) => {
        const items = params as TooltipFormatterParam[]
        if (!items || items.length === 0) return ''
        let result = escapeTooltipText(items[0].name) + '<br/>'
        items.forEach((item) => {
          const numericValue = tooltipNumber(item.value)
          let value = '-'
          if (numericValue !== null) {
            value = item.seriesName === labels.lossRate
              ? numericValue.toFixed(0) + '%'
              : numericValue.toFixed(2) + 'ms'
          }
          result += item.marker + escapeTooltipText(item.seriesName) + ': ' + value + '<br/>'
        })
        return result
      }
    },
    legend: {
      type: compact ? 'scroll' : 'plain',
      data: [labels.maxDelay, labels.averageDelay, labels.minDelay, labels.lossRate],
      selected: {
        [labels.maxDelay]: false,
        [labels.minDelay]: false
      },
      top: 0,
      left: compact ? 0 : 'center',
      right: compact ? 0 : undefined,
      itemWidth: compact ? 12 : 25,
      itemHeight: compact ? 8 : 14,
      itemGap: compact ? 8 : 10,
      textStyle: {
        color: theme.textColorSecondary,
        fontSize: compact ? 10 : 12
      }
    },
    grid: {
      left: compact ? 0 : '3%',
      right: compact ? 0 : '3%',
      top: compact ? 48 : 30,
      bottom: showDataZoom ? 50 : 50,
      containLabel: true
    },
    dataZoom: showDataZoom ? [{
      type: 'slider',
      bottom: 10,
      borderColor: theme.borderColor,
      backgroundColor: isDark ? 'rgba(30,30,30,0.9)' : 'rgba(248,248,248,0.9)',
      fillerColor: isDark ? 'rgba(64,158,255,0.2)' : 'rgba(64,158,255,0.15)',
      handleStyle: {
        color: isDark ? '#4c4d4f' : '#fff',
        borderColor: isDark ? '#6c6e72' : '#909399'
      },
      moveHandleStyle: {
        color: isDark ? '#4c4d4f' : '#ddd'
      },
      emphasis: {
        handleStyle: {
          borderColor: '#409eff',
          color: isDark ? '#4c4d4f' : '#fff'
        },
        moveHandleStyle: {
          color: '#409eff'
        },
        handleLabel: {
          show: true
        }
      },
      handleLabel: {
        show: true
      },
      selectedDataBackground: {
        lineStyle: { color: '#409eff' },
        areaStyle: { color: 'rgba(64,158,255,0.2)' }
      },
      dataBackground: {
        lineStyle: { color: isDark ? '#6c6e72' : '#909399' },
        areaStyle: { color: isDark ? 'rgba(255,255,255,0.1)' : 'rgba(64,158,255,0.1)' }
      },
      textStyle: {
        color: theme.textColorSecondary
      }
    }] : undefined,
    xAxis: {
      type: 'category',
      data: data?.lastcheck || [],
      axisLine: {
        lineStyle: { color: theme.borderColor }
      },
      axisLabel: {
        color: theme.textColorSecondary,
        rotate: 0,
        hideOverlap: true,
        interval: 'auto',
        formatter: (value: string, index: number) => {
          if (!value) return ''
          const time = value.length >= 16 ? value.substring(11, 16) : value
          const date = value.length >= 10 ? value.substring(5, 10) : ''

          if (!date) return `{time|${time}}`
          // 只在“已显示标签”的日期发生变化时显示日期，不新增额外刻度
          if (index <= lastRenderedIndex) {
            lastRenderedIndex = -1
            lastRenderedDate = ''
          }
          const showDate = lastRenderedDate === '' || lastRenderedDate !== date
          lastRenderedIndex = index
          lastRenderedDate = date
          return showDate ? `{date|${date}}\n{time|${time}}` : `{time|${time}}`
        },
        rich: {
          date: {
            color: theme.textColorSecondary,
            fontSize: 11,
            padding: [0, 0, 2, 0]
          },
          time: {
            color: isDark ? '#cfd3dc' : '#606266',
            fontSize: 12
          }
        }
      },
      axisTick: {
        alignWithLabel: true
      }
    },
    yAxis: [
      {
        type: 'value',
        name: compact ? '' : `${labels.latency} (ms)`,
        position: 'left',
        nameTextStyle: { color: theme.textColorSecondary },
        axisLine: {
          lineStyle: { color: theme.borderColor }
        },
        axisLabel: {
          color: theme.textColorSecondary,
          fontSize: compact ? 10 : 12
        },
        splitLine: {
          lineStyle: { color: theme.splitLineColor }
        }
      },
      {
        type: 'value',
        name: compact ? '' : `${labels.lossRate} (%)`,
        min: 0,
        max: 100,
        position: 'right',
        nameTextStyle: { color: theme.textColorSecondary },
        axisLine: {
          lineStyle: { color: theme.borderColor }
        },
        axisLabel: {
          color: theme.textColorSecondary,
          fontSize: compact ? 10 : 12,
          formatter: '{value}%'
        },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: labels.maxDelay,
        type: 'line',
        data: data?.maxdelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#e6a23c' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: labels.averageDelay,
        type: 'line',
        data: data?.avgdelay || [],
        animation: false,
        lineStyle: { width: 2 },
        itemStyle: { color: '#00CC66' },
        areaStyle: { opacity: 0.2 }
      },
      {
        name: labels.minDelay,
        type: 'line',
        data: data?.mindelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#409eff' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: labels.lossRate,
        type: 'line',
        yAxisIndex: 1,
        data: data?.losspk || [],
        animation: false,
        lineStyle: { width: 2 },
        itemStyle: { color: '#f56c6c' },
        areaStyle: { opacity: 0.2 }
      }
    ]
  }
}

export function getPingMiniChartOption(
  data: PingLogData | null,
  isDark: boolean,
  labels: PingChartLabels
): EChartsOption {
  const labelColor = isDark ? '#6c6e72' : '#909399'

  return {
    backgroundColor: 'transparent',
    grid: {
      left: 35,
      right: 35,
      top: 18,
      bottom: 18,
      containLabel: false
    },
    xAxis: {
      type: 'category',
      data: data?.lastcheck || [],
      axisLine: { show: false },
      axisTick: { show: false },
      axisLabel: {
        show: true,
        color: labelColor,
        fontSize: 10,
        interval: 'auto',
        formatter: (value: string) => {
          if (!value || value.length < 16) return value || ''
          return value.substring(11, 16)
        }
      }
    },
    yAxis: [
      {
        type: 'value',
        position: 'left',
        name: labels.latency,
        nameTextStyle: {
          color: labelColor,
          fontSize: 10
        },
        nameGap: 5,
        min: 0,
        max: function(value: { max: number }) {
          return Math.max(value.max * 1.1, 10)
        },
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
          show: true,
          color: labelColor,
          fontSize: 9,
          formatter: (value: number) => Math.round(value).toString()
        },
        splitLine: { show: false }
      },
      {
        type: 'value',
        position: 'right',
        name: labels.loss,
        nameTextStyle: {
          color: labelColor,
          fontSize: 10
        },
        nameGap: 5,
        min: 0,
        max: 100,
        axisLine: { show: false },
        axisTick: { show: false },
        axisLabel: {
          show: true,
          color: labelColor,
          fontSize: 9,
          formatter: '{value}%'
        },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        type: 'line',
        data: data?.avgdelay || [],
        yAxisIndex: 0,
        smooth: false,
        symbol: 'none',
        lineStyle: {
          color: '#00CC66',
          width: 1.5
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(0, 204, 102, 0.3)' },
              { offset: 1, color: 'rgba(0, 204, 102, 0.05)' }
            ]
          }
        }
      },
      {
        type: 'line',
        data: data?.losspk || [],
        yAxisIndex: 1,
        smooth: false,
        symbol: 'none',
        lineStyle: {
          color: '#f56c6c',
          width: 1.5
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(245, 108, 108, 0.3)' },
              { offset: 1, color: 'rgba(245, 108, 108, 0.05)' }
            ]
          }
        }
      }
    ]
  }
}
