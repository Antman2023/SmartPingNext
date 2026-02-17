import type { EChartsOption } from 'echarts'
import type { PingLogData } from '@/types'

interface ChartTheme {
  isDark: boolean
  textColor: string
  textColorSecondary: string
  borderColor: string
  splitLineColor: string
  backgroundColor: string
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

export function getPingChartOption(data: PingLogData | null, isDark: boolean, showDataZoom = false): EChartsOption {
  const theme = getThemeConfig(isDark)

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
      formatter: (params: any) => {
        let result = params[0].name + '<br/>'
        params.forEach((item: any) => {
          let value = item.value
          if (item.seriesName === '丢包率') {
            value = parseFloat(value).toFixed(0) + '%'
          } else {
            value = parseFloat(value).toFixed(2) + 'ms'
          }
          result += item.marker + item.seriesName + ': ' + value + '<br/>'
        })
        return result
      }
    },
    legend: {
      data: ['最大延迟', '平均延迟', '最小延迟', '丢包率'],
      selected: {
        '最大延迟': false,
        '最小延迟': false
      },
      top: 0,
      textStyle: {
        color: theme.textColorSecondary
      }
    },
    grid: {
      left: '3%',
      right: '3%',
      top: 30,
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
        interval: 'auto',
        formatter: (value: string, index: number) => {
          if (!value) return ''
          const time = value.length >= 16 ? value.substring(11, 16) : value
          const date = value.length >= 10 ? value.substring(5, 10) : ''

          const lastcheck = data?.lastcheck || []
          const prevValue = index > 0 ? lastcheck[index - 1] : ''
          const prevDate = prevValue && prevValue.length >= 10 ? prevValue.substring(5, 10) : ''

          if (date && (index === 0 || date !== prevDate)) {
            return `{date|${date}}\n{time|${time}}`
          }
          return `{time|${time}}`
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
        name: '延迟 (ms)',
        position: 'left',
        nameTextStyle: { color: theme.textColorSecondary },
        axisLine: {
          lineStyle: { color: theme.borderColor }
        },
        axisLabel: {
          color: theme.textColorSecondary
        },
        splitLine: {
          lineStyle: { color: theme.splitLineColor }
        }
      },
      {
        type: 'value',
        name: '丢包率 (%)',
        min: 0,
        max: 100,
        position: 'right',
        nameTextStyle: { color: theme.textColorSecondary },
        axisLine: {
          lineStyle: { color: theme.borderColor }
        },
        axisLabel: {
          color: theme.textColorSecondary,
          formatter: '{value}%'
        },
        splitLine: { show: false }
      }
    ],
    series: [
      {
        name: '最大延迟',
        type: 'line',
        data: data?.maxdelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#e6a23c' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '平均延迟',
        type: 'line',
        data: data?.avgdelay || [],
        animation: false,
        lineStyle: { width: 2 },
        itemStyle: { color: '#00CC66' },
        areaStyle: { opacity: 0.2 }
      },
      {
        name: '最小延迟',
        type: 'line',
        data: data?.mindelay || [],
        animation: false,
        lineStyle: { width: 1 },
        itemStyle: { color: '#409eff' },
        areaStyle: { opacity: 0.1 }
      },
      {
        name: '丢包率',
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

export function getPingMiniChartOption(data: PingLogData | null, isDark: boolean): EChartsOption {
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
        name: '延迟',
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
        name: '丢包',
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
