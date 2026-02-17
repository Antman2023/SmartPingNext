export default {
  // Theme
  theme: {
    lightMode: 'Light Mode',
    darkMode: 'Dark Mode'
  },
  // Common
  common: {
    save: 'Save',
    cancel: 'Cancel',
    confirm: 'Confirm',
    loading: 'Loading...',
    loadFailed: 'Load Failed',
    query: 'Query',
    saveImage: 'Save Image',
    delete: 'Delete',
    add: 'Add',
    edit: 'Edit',
    enable: 'Enable',
    operation: 'Operation',
    status: 'Status',
    name: 'Name',
    ip: 'IP',
    node: 'Node',
    nodeList: 'Node List',
    pleaseInput: 'Please input',
    pleaseSelect: 'Please select',
    password: 'Password',
    success: 'Success',
    failed: 'Failed',
    saveSuccess: 'Saved successfully',
    saveFailed: 'Save failed',
    pleaseEnterPassword: 'Please enter password',
    passwordError: 'Password error',
    high: 'High',
    low: 'Low',
    configLoadFailed: 'Failed to load config',
    configSaveFailed: 'Failed to save config',
    configLoadFailedNetwork: 'Failed to load config, please check network connection',
    chartLoadFailed: 'Failed to load chart data',
    dataLoadFailed: 'Failed to load data'
  },
  // Navigation menu
  nav: {
    dashboard: 'Dashboard',
    reverse: 'Reverse',
    topology: 'Topology',
    mapping: 'Latency Map',
    tools: 'Tools',
    alerts: 'Alerts',
    config: 'Settings',
    language: 'Language',
    zhCN: '简体中文',
    enUS: 'English'
  },
  // Dashboard page
  dashboard: {
    title: 'Dashboard',
    timeRanges: {
      hour1: '1 Hour',
      hour3: '3 Hours',
      hour6: '6 Hours',
      hour12: '12 Hours',
      day1: '1 Day',
      day3: '3 Days',
      day7: '7 Days'
    },
    startTime: 'Start Time',
    endTime: 'End Time'
  },
  // Reverse page
  reverse: {
    title: 'Reverse Monitor'
  },
  // Topology page
  topology: {
    title: 'PING Topology',
    viewAlerts: 'View Alerts',
    topologyList: 'Topology List'
  },
  // Mapping page
  mapping: {
    title: 'Latency Map',
    selectTime: 'Select Time',
    telecom: 'Telecom',
    unicom: 'Unicom',
    mobile: 'Mobile'
  },
  // Tools page
  tools: {
    title: 'Check Tools',
    check: 'Check',
    enterTarget: 'Enter target address',
    resolvedIP: 'Resolved IP',
    sent: 'Sent',
    received: 'Received',
    packetLoss: 'Loss',
    latency: 'Latency',
    requestFailed: 'Request failed'
  },
  // Alerts page
  alerts: {
    title: 'Alert Records',
    alertArchive: 'Alert Archive',
    alertHistory: 'Alert History',
    alertDate: 'Alert Date',
    sourceNode: 'Source Node',
    sourceIP: 'Source IP',
    targetNode: 'Target Node',
    targetIP: 'Target IP',
    backToTopology: 'Back to Topology',
    mtrResult: 'MTR Result',
    host: 'Host',
    packetLossRate: 'Loss Rate',
    latest: 'Latest',
    average: 'Avg',
    best: 'Best',
    worst: 'Worst',
    standardDeviation: 'StdDev'
  },
  // Config page
  config: {
    title: 'System Settings',
    saveConfig: 'Save Config',
    importExport: 'Import/Export',
    exportConfig: 'Export',
    importConfig: 'Import',
    baseConfig: 'Basic Config',
    base: 'Basic',
    pingTopology: 'Ping Topology',
    checkTools: 'Check Tools',
    authManagement: 'Authorization',
    timeout: 'Timeout (sec)',
    pageRefresh: 'Page Refresh (min)',
    dataArchive: 'Data Archive (days)',
    alertSound: 'Alert Sound',
    lineWidth: 'Line Width',
    symbolSize: 'Symbol Size',
    rateLimit: 'Rate Limit (sec)',
    ipWhitelist: 'IP Whitelist (comma separated)',
    pingNetwork: 'Ping Node Network',
    nodeName: 'Node Name',
    nodeIP: 'Node IP',
    addNode: 'Add Node',
    newNode: 'New Node',
    pleaseEnterNodeName: 'Please enter node name',
    pleaseEnterIPv4: 'Enter IPv4 address, e.g. 192.168.1.1',
    tempSave: 'Save',
    pingConfig: 'Ping Config',
    topoConfig: 'Topology Config',
    selectPingTargets: 'Select target nodes to ping from {name}:',
    selectTopoTargets: 'Select target nodes to monitor in topology for {name}:',
    pingConfigUpdated: 'Ping config updated',
    topoConfigUpdated: 'Topology config updated',
    chinaMapNetwork: 'Latency Test Network',
    chinaMapConfig: '{province} Latency Config',
    addProvince: 'Add Province',
    pleaseEnterProvince: 'Please enter province name',
    provinceExample: 'e.g. Beijing, Shanghai, Guangdong',
    provinceExists: 'Province already exists',
    provinceDeleted: 'Province deleted',
    provinceAdded: 'Province added',
    deleteProvince: 'Delete Province',
    eachLineOneIP: 'One IP per line',
    delayConfigUpdated: 'Latency config updated',
    configExported: 'Config exported',
    configImported: 'Config imported, please save to apply',
    configInvalid: 'Invalid config file format',
    configParseFailed: 'Config file parse failed',
    nodeAdded: 'Node added, please save to apply',
    pleaseEnterNodeAndIP: 'Please enter node name and IP',
    pleaseEnterValidIPv4: 'Please enter a valid IPv4 address',
    nodeIPExists: 'Node IP already exists',
    pleaseEnterProvinceName: 'Please enter province name',
    perLineIP: 'One IP per line'
  },
  // Page titles
  pageTitle: {
    dashboard: 'Dashboard',
    reverse: 'Reverse Monitor',
    topology: 'Topology',
    mapping: 'Latency Map',
    tools: 'Check Tools',
    alerts: 'Alert Records',
    config: 'Settings'
  }
}
