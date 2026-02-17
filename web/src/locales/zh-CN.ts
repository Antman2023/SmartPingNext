export default {
  // 主题
  theme: {
    lightMode: '浅色模式',
    darkMode: '深色模式'
  },
  // 通用
  common: {
    save: '保存',
    cancel: '取消',
    confirm: '确定',
    loading: '加载中...',
    loadFailed: '加载失败',
    query: '查询',
    saveImage: '保存图片',
    delete: '删除',
    add: '添加',
    edit: '编辑',
    enable: '启用',
    operation: '操作',
    status: '状态',
    name: '名称',
    ip: 'IP',
    node: '节点',
    nodeList: '节点列表',
    pleaseInput: '请输入',
    pleaseSelect: '请选择',
    password: '密码',
    success: '操作成功',
    failed: '操作失败',
    saveSuccess: '保存成功',
    saveFailed: '保存失败',
    pleaseEnterPassword: '请输入密码',
    passwordError: '密码错误',
    high: '高',
    low: '低',
    configLoadFailed: '加载配置失败',
    configSaveFailed: '保存配置失败',
    configLoadFailedNetwork: '加载配置失败，请检查网络连接',
    chartLoadFailed: '加载图表数据失败',
    dataLoadFailed: '加载数据失败'
  },
  // 导航菜单
  nav: {
    dashboard: '正向监控',
    reverse: '反向监控',
    topology: '拓扑图',
    mapping: '延迟地图',
    tools: '检测工具',
    alerts: '报警记录',
    config: '系统配置',
    language: '语言',
    zhCN: '简体中文',
    enUS: 'English'
  },
  // 正向监控页面
  dashboard: {
    title: '正向监控',
    timeRanges: {
      hour1: '1小时',
      hour3: '3小时',
      hour6: '6小时',
      hour12: '12小时',
      day1: '1天',
      day3: '3天',
      day7: '7天'
    },
    startTime: '开始时间',
    endTime: '结束时间'
  },
  // 反向监控页面
  reverse: {
    title: '反向监控'
  },
  // 拓扑图页面
  topology: {
    title: 'PING 拓扑',
    viewAlerts: '查看报警记录',
    topologyList: '拓扑列表'
  },
  // 延迟地图页面
  mapping: {
    title: '全国延迟地图',
    selectTime: '选择时间',
    telecom: '电信',
    unicom: '联通',
    mobile: '移动'
  },
  // 检测工具页面
  tools: {
    title: '检测工具',
    check: '检测',
    enterTarget: '输入目标地址',
    resolvedIP: '解析IP',
    sent: '发送',
    received: '接收',
    packetLoss: '丢包',
    latency: '延迟',
    requestFailed: '请求失败'
  },
  // 报警记录页面
  alerts: {
    title: '报警记录',
    alertArchive: '报警存档',
    alertHistory: '报警历史',
    alertDate: '报警日期',
    sourceNode: '源节点',
    sourceIP: '源IP',
    targetNode: '目标节点',
    targetIP: '目标IP',
    backToTopology: '返回拓扑图',
    mtrResult: 'MTR 结果',
    host: '主机',
    packetLossRate: '丢包率',
    latest: '最近',
    average: '平均',
    best: '最好',
    worst: '最差',
    standardDeviation: '标准差'
  },
  // 系统配置页面
  config: {
    title: '系统配置',
    saveConfig: '保存配置',
    importExport: '导入/导出',
    exportConfig: '导出配置',
    importConfig: '导入配置',
    baseConfig: '基础配置',
    base: '基础',
    pingTopology: 'Ping拓扑',
    checkTools: '检测工具',
    authManagement: '授权管理',
    timeout: '接口超时(秒)',
    pageRefresh: '页面刷新(分钟)',
    dataArchive: '数据存档(天)',
    alertSound: '报警声音',
    lineWidth: '连线粗细',
    symbolSize: '形状大小',
    rateLimit: '限定频率(秒)',
    ipWhitelist: '用户IP列表(逗号隔开)',
    pingNetwork: 'Ping节点测试网络',
    nodeName: '节点名称',
    nodeIP: '节点IP',
    addNode: '添加节点',
    newNode: '新增节点',
    pleaseEnterNodeName: '请输入节点名称',
    pleaseEnterIPv4: '请输入IPv4地址，如 192.168.1.1',
    tempSave: '暂存',
    pingConfig: 'Ping配置',
    topoConfig: '拓扑配置',
    selectPingTargets: '选择要从 {name} 发起 Ping 检测的目标节点：',
    selectTopoTargets: '选择 {name} 在拓扑图中需要监控的目标节点：',
    pingConfigUpdated: 'Ping配置已更新',
    topoConfigUpdated: '拓扑配置已更新',
    chinaMapNetwork: '全国延迟测试网络',
    chinaMapConfig: '{province} 延迟配置',
    addProvince: '添加省份',
    pleaseEnterProvince: '请输入省份名称',
    provinceExample: '如：北京、上海、广东',
    provinceExists: '该省份已存在',
    provinceDeleted: '省份已删除',
    provinceAdded: '省份已添加',
    deleteProvince: '删除省份',
    eachLineOneIP: '每行一个IP地址',
    delayConfigUpdated: '延迟配置已更新',
    configExported: '配置已导出',
    configImported: '配置已导入，请点击保存按钮保存配置',
    configInvalid: '配置文件格式无效',
    configParseFailed: '配置文件解析失败',
    nodeAdded: '节点已添加，请点击保存按钮保存配置',
    pleaseEnterNodeAndIP: '请填写节点名称和IP',
    pleaseEnterValidIPv4: '请输入有效的IPv4地址',
    nodeIPExists: '该IP节点已存在',
    pleaseEnterProvinceName: '请输入省份名称',
    perLineIP: '每行一个IP地址'
  },
  // 页面标题
  pageTitle: {
    dashboard: '正向监控',
    reverse: '反向监控',
    topology: '拓扑图',
    mapping: '延迟地图',
    tools: '检测工具',
    alerts: '报警记录',
    config: '系统配置'
  }
}
