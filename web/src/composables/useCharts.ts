import { useThemeStore } from '@/stores/theme'
import type { EChartsOption } from 'echarts'
import { useI18n } from 'vue-i18n'

export function useCharts() {
  const themeStore = useThemeStore()
  const { t } = useI18n()

  const getChartTheme = () => {
    return themeStore.theme === 'dark' ? 'dark' : 'light'
  }

  const getAxisStyle = () => {
    const isDark = themeStore.theme === 'dark'

    return {
      axisLine: {
        lineStyle: {
          color: isDark ? '#4c4d4f' : '#dcdfe6'
        }
      },
      axisLabel: {
        color: isDark ? '#a3a6ad' : '#606266'
      },
      splitLine: {
        lineStyle: {
          color: isDark ? '#363637' : '#ebeef5'
        }
      }
    }
  }

  const getBaseChartOption = (): EChartsOption => {
    const isDark = themeStore.theme === 'dark'

    return {
      backgroundColor: 'transparent',
      textStyle: {
        color: isDark ? '#a3a6ad' : '#606266'
      },
      grid: {
        left: '3%',
        right: '4%',
        bottom: '3%',
        containLabel: true
      },
      tooltip: {
        backgroundColor: isDark ? '#252525' : '#fff',
        borderColor: isDark ? '#4c4d4f' : '#dcdfe6',
        textStyle: {
          color: isDark ? '#e5eaf3' : '#303133'
        }
      },
      legend: {
        textStyle: {
          color: isDark ? '#a3a6ad' : '#606266'
        }
      },
      xAxis: getAxisStyle(),
      yAxis: getAxisStyle()
    }
  }

  const getPingChartOption = (data: { time: string; value: number }[]): EChartsOption => {
    const baseOption = getBaseChartOption()

    return {
      ...baseOption,
      xAxis: {
        ...getAxisStyle(),
        type: 'category',
        data: data.map(item => item.time)
      },
      yAxis: {
        ...getAxisStyle(),
        type: 'value',
        name: `${t('charts.latency')} (ms)`
      },
      series: [{
        name: t('charts.latency'),
        type: 'line',
        smooth: true,
        data: data.map(item => item.value),
        lineStyle: {
          color: '#409eff',
          width: 2
        },
        areaStyle: {
          color: {
            type: 'linear',
            x: 0,
            y: 0,
            x2: 0,
            y2: 1,
            colorStops: [
              { offset: 0, color: 'rgba(64, 158, 255, 0.3)' },
              { offset: 1, color: 'rgba(64, 158, 255, 0.05)' }
            ]
          }
        },
        itemStyle: {
          color: '#409eff'
        }
      }]
    }
  }

  return {
    getChartTheme,
    getBaseChartOption,
    getPingChartOption
  }
}
