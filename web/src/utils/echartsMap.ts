import * as echarts from 'echarts/core'
import { MapChart } from 'echarts/charts'
import {
  GeoComponent,
  LegendComponent,
  TitleComponent,
  TooltipComponent,
  VisualMapComponent
} from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
  MapChart,
  GeoComponent,
  TitleComponent,
  TooltipComponent,
  LegendComponent,
  VisualMapComponent,
  CanvasRenderer
])

export { echarts }
