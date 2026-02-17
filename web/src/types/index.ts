// 配置相关类型
export interface NetworkMember {
  Name: string
  Addr: string
  Smartping: boolean
  Ping: string[]
  Topology: TopologyConfig[]
}

export interface TopologyConfig {
  Name: string
  Addr: string
  Thdchecksec: string
  Thdoccnum: string
  Thdavgdelay: string
  Thdloss: string
}

export interface BaseConfig {
  Timeout: number
  Refresh: number
  Archive: number
}

export interface TopologySettings {
  Tsound: string
  Tline: string
  Tsymbolsize: string
}

export interface ChinaMapProvider {
  ctcc: string[]
  cucc: string[]
  cmcc: string[]
}

export interface Config {
  Ver: string
  Port: number
  Name: string
  Addr: string
  Mode: Record<string, string>
  Base: BaseConfig
  Topology: TopologySettings
  Network: Record<string, NetworkMember>
  Chinamap: Record<string, ChinaMapProvider>
  Toollimit: number
  Authiplist: string
}

// PING 数据类型
export interface PingSt {
  SendPk: number
  RevcPk: number
  LossPk: number
  MinDelay: number
  AvgDelay: number
  MaxDelay: number
}

export interface PingLogData {
  lastcheck: string[]
  maxdelay: string[]
  mindelay: string[]
  avgdelay: string[]
  losspk: string[]
}

export interface ToolsResult {
  status: string
  error: string
  ip: string
  ping: PingSt
}

// 地图数据
export interface MapVal {
  value: number
  name: string
}

export interface ChinaMapData {
  text: string
  subtext: string
  avgdelay: {
    ctcc: MapVal[]
    cucc: MapVal[]
    cmcc: MapVal[]
  }
}

// 报警记录
export interface AlertLog {
  Logtime: string
  Targetip: string
  Targetname: string
  Tracert: string
  Fromip: string
  Fromname: string
}

export interface AlertData {
  dates: string[]
  logs: AlertLog[]
}

export interface MtrResult {
  Host: string
  Send: number
  Loss: number
  Last: number
  Avg: number
  Best: number
  Wrst: number
  StDev: number
}
