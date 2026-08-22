declare module 'china-map-geojson/lib/china' {
  const chinaMap: Parameters<typeof import('echarts').registerMap>[1]

  export default chinaMap
}
