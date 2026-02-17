<template>
  <div class="config-view">
    <div class="config-header">
      <h2>{{ $t('config.title') }}</h2>
    </div>

    <div class="config-content">
      <el-row :gutter="20">
        <!-- 左侧配置 -->
        <el-col :span="8">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>{{ $t('config.saveConfig') }}</span>
                <el-link type="primary" href="/api/config.json" target="_blank">
                  <el-icon><Document /></el-icon>
                </el-link>
              </div>
            </template>
            <div class="save-config">
              <el-input v-model="password" type="password" :placeholder="$t('common.password')" style="width: 200px;" />
              <el-button type="primary" @click="handleSave">{{ $t('common.save') }}</el-button>
            </div>
          </el-card>

          <el-card class="mt-2">
            <template #header>
              <span>{{ $t('config.importExport') }}</span>
            </template>
            <div class="import-export">
              <div class="import-export-row">
                <el-input v-model="importExportPassword" type="password" :placeholder="$t('common.password')" style="width: 150px;" />
                <el-button @click="handleExport">{{ $t('config.exportConfig') }}</el-button>
                <el-upload
                  ref="uploadRef"
                  :auto-upload="false"
                  :show-file-list="false"
                  accept=".json"
                  :on-change="handleImportFile"
                >
                  <el-button>{{ $t('config.importConfig') }}</el-button>
                </el-upload>
              </div>
            </div>
          </el-card>

          <el-card class="mt-2">
            <template #header>
              <span>{{ $t('config.baseConfig') }}</span>
            </template>
            <el-form label-width="120px" label-position="top">
              <h4>{{ $t('config.base') }}</h4>
              <el-row :gutter="12">
                <el-col :span="8">
                  <el-form-item :label="$t('config.timeout')">
                    <el-input v-model.number="formConfig.Base.Timeout" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item :label="$t('config.pageRefresh')">
                    <el-input v-model.number="formConfig.Base.Refresh" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item :label="$t('config.dataArchive')">
                    <el-input v-model.number="formConfig.Base.Archive" />
                  </el-form-item>
                </el-col>
              </el-row>

              <h4>{{ $t('config.pingTopology') }}</h4>
              <el-row :gutter="12">
                <el-col :span="8">
                  <el-form-item :label="$t('config.alertSound')">
                    <el-input v-model="formConfig.Topology.Tsound" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item :label="$t('config.lineWidth')">
                    <el-input v-model="formConfig.Topology.Tline" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item :label="$t('config.symbolSize')">
                    <el-input v-model="formConfig.Topology.Tsymbolsize" />
                  </el-form-item>
                </el-col>
              </el-row>

              <h4>{{ $t('config.checkTools') }}</h4>
              <el-form-item :label="$t('config.rateLimit')">
                <el-input v-model.number="formConfig.Toollimit" />
              </el-form-item>

              <h4>{{ $t('config.authManagement') }}</h4>
              <el-form-item :label="$t('config.ipWhitelist')">
                <el-input v-model="formConfig.Authiplist" />
              </el-form-item>
            </el-form>
          </el-card>
        </el-col>

        <!-- 右侧网络配置 -->
        <el-col :span="16">
          <el-card>
            <template #header>
              <span>{{ $t('config.pingNetwork') }}</span>
            </template>
            <el-table :data="networkList" stripe style="width: 100%">
              <el-table-column prop="Name" :label="$t('config.nodeName')" min-width="120" />
              <el-table-column prop="Addr" :label="$t('config.nodeIP')" min-width="140" />
              <el-table-column label="SmartPing" width="100">
                <template #default="{ row }">
                  <el-checkbox v-model="row._original.Smartping" :disabled="row.isSelf" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('common.operation')" min-width="180">
                <template #default="{ row }">
                  <el-button-group>
                    <el-button size="small" :disabled="!row.isSelf && !row._original.Smartping" @click="editPingConfig(row)">{{ $t('config.pingConfig') }}</el-button>
                    <el-button size="small" :disabled="!row.isSelf && !row._original.Smartping" @click="editTopoConfig(row)">{{ $t('config.topoConfig') }}</el-button>
                  </el-button-group>
                </template>
              </el-table-column>
              <el-table-column width="60">
                <template #default="{ row }">
                  <el-button
                    v-if="!row.isSelf"
                    type="danger"
                    size="small"
                    :icon="Delete"
                    circle
                    @click="deleteNode(row)"
                  />
                </template>
              </el-table-column>
            </el-table>
            <div style="margin-top: 12px; text-align: center;">
              <el-button @click="showAddNode">{{ $t('config.addNode') }}</el-button>
            </div>
          </el-card>

          <el-card class="mt-2">
            <template #header>
              <span>{{ $t('config.chinaMapNetwork') }}</span>
            </template>
            <div class="chinamap-list">
              <el-button
                v-for="(provData, prov) in formConfig.Chinamap"
                :key="prov"
                @click="editChinaMap(prov as string)"
              >
                {{ prov }} ({{ $t('mapping.telecom') }}{{ provData.ctcc?.length || 0 }}, {{ $t('mapping.unicom') }}{{ provData.cucc?.length || 0 }}, {{ $t('mapping.mobile') }}{{ provData.cmcc?.length || 0 }})
              </el-button>
              <el-button @click="showAddChinaMap">+</el-button>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 添加节点弹窗 -->
    <el-dialog v-model="addNodeVisible" :title="$t('config.newNode')" width="400px">
      <el-form label-width="80px">
        <el-form-item :label="$t('config.nodeName')">
          <el-input v-model="newNodeName" :placeholder="$t('config.pleaseEnterNodeName')" />
        </el-form-item>
        <el-form-item :label="$t('config.nodeIP')">
          <el-input v-model="newNodeAddr" :placeholder="$t('config.pleaseEnterIPv4')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addNodeVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="addNode">{{ $t('config.tempSave') }}</el-button>
      </template>
    </el-dialog>

    <!-- Ping配置弹窗 -->
    <el-dialog v-model="pingConfigVisible" :title="$t('config.pingConfig')" width="500px">
      <p class="dialog-tip">{{ $t('config.selectPingTargets', { name: currentEditNode?.Name }) }}</p>
      <el-table :data="pingTargetList" stripe max-height="400">
        <el-table-column prop="Name" :label="$t('config.nodeName')" width="150" />
        <el-table-column prop="Addr" :label="$t('config.nodeIP')" width="150" />
        <el-table-column :label="$t('common.enable')" width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.enabled" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="pingConfigVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="savePingConfig">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 拓扑配置弹窗 -->
    <el-dialog v-model="topoConfigVisible" :title="$t('config.topoConfig')" width="500px">
      <p class="dialog-tip">{{ $t('config.selectTopoTargets', { name: currentEditNode?.Name }) }}</p>
      <el-table :data="topoTargetList" stripe max-height="400">
        <el-table-column prop="Name" :label="$t('config.nodeName')" width="150" />
        <el-table-column prop="Addr" :label="$t('config.nodeIP')" width="150" />
        <el-table-column :label="$t('common.enable')" width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.enabled" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="topoConfigVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTopoConfig">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 全国延迟配置弹窗 -->
    <el-dialog v-model="chinaMapVisible" :title="currentProvince ? $t('config.chinaMapConfig', { province: currentProvince }) : $t('config.chinaMapConfig', { province: '' })" width="600px">
      <el-tabs v-model="chinaMapTab">
        <el-tab-pane label="电信(CTCC)" name="ctcc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.ctcc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="联通(CUCC)" name="cucc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.cucc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="移动(CMCC)" name="cmcc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.cmcc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="chinaMapVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button v-if="currentProvince" type="danger" @click="deleteChinaMap">{{ $t('config.deleteProvince') }}</el-button>
        <el-button type="primary" @click="saveChinaMap">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <!-- 添加省份弹窗 -->
    <el-dialog v-model="addProvinceVisible" :title="$t('config.addProvince')" width="400px">
      <el-form label-width="80px">
        <el-form-item :label="$t('config.nodeName')">
          <el-input v-model="newProvinceName" :placeholder="$t('config.provinceExample')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addProvinceVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="addProvince">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { useI18n } from 'vue-i18n'
import { Document, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { fetchConfig, saveConfig } from '@/api/config'
import type { Config, NetworkMember, TopologyConfig } from '@/types'

const { t } = useI18n()
const config = ref<Config | null>(null)
const password = ref('')
const importExportPassword = ref('')
const uploadRef = ref()

const formConfig = reactive<Config>({
  Ver: '',
  Port: 8899,
  Name: '',
  Addr: '',
  Mode: {},
  Base: { Timeout: 3, Refresh: 5, Archive: 30 },
  Topology: { Tsound: '', Tline: '2', Tsymbolsize: '50' },
  Network: {},
  Chinamap: {},
  Toollimit: 0,
  Authiplist: ''
})

type NetworkListItem = {
  _original: NetworkMember
  Name: string
  Addr: string
  Smartping: boolean
  Ping: string[]
  Topology: TopologyConfig[]
  isSelf: boolean
}

const networkList = computed(() => {
  if (!formConfig.Network) return []
  return Object.entries(formConfig.Network).map(([addr, network]) => ({
    _original: network,
    Name: network.Name,
    Addr: addr,
    Smartping: network.Smartping,
    Ping: network.Ping,
    Topology: network.Topology,
    isSelf: addr === formConfig.Addr
  }))
})

const addNodeVisible = ref(false)
const newNodeName = ref('')
const newNodeAddr = ref('')

// Ping 配置相关
const pingConfigVisible = ref(false)
const currentEditNode = ref<{ Name: string; Addr: string } | null>(null)
const pingTargetList = ref<Array<{ Name: string; Addr: string; enabled: boolean }>>([])

// 拓扑配置相关
const topoConfigVisible = ref(false)
const topoTargetList = ref<Array<{ Name: string; Addr: string; enabled: boolean }>>([])

// 全国延迟配置相关
const chinaMapVisible = ref(false)
const chinaMapTab = ref('ctcc')
const currentProvince = ref('')
const chinaMapIps = reactive({
  ctcc: '',
  cucc: '',
  cmcc: ''
})
const addProvinceVisible = ref(false)
const newProvinceName = ref('')

const loadConfig = async () => {
  try {
    const cfg = await fetchConfig()
    config.value = cfg

    // 复制配置到表单
    Object.assign(formConfig, cfg)
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const handleSave = async () => {
  if (!password.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  try {
    const result = await saveConfig(formConfig, password.value)
    if (result.status === 'true') {
      ElMessage.success(t('common.saveSuccess'))
    } else {
      ElMessage.error(result.info || t('common.saveFailed'))
      console.error('保存失败:', result.info)
    }
} catch (e: unknown) {
    ElMessage.error(t('common.saveFailed') + ': ' + (e instanceof Error ? e.message : ''))
    console.error('保存失败', e)
  }
}

// 验证密码
const verifyPassword = async (pwd: string): Promise<boolean> => {
  try {
    const response = await fetch('/api/verify-password.json', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body: `password=${encodeURIComponent(pwd)}`
    })
    const result = await response.json()
    return result.status === 'true'
  } catch {
    return false
  }
}

// 导出配置
const handleExport = async () => {
  if (!importExportPassword.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  const valid = await verifyPassword(importExportPassword.value)
  if (!valid) {
    ElMessage.error(t('common.passwordError'))
    return
  }

  // 导出配置（移除敏感信息）
  const exportConfig = {
    ...formConfig,
    Password: undefined,
    Ver: undefined
  }

  const blob = new Blob([JSON.stringify(exportConfig, null, 2)], { type: 'application/json' })
  const url = URL.createObjectURL(blob)
  const link = document.createElement('a')
  link.href = url
  link.download = `smartping-config-${new Date().toISOString().slice(0, 10)}.json`
  link.click()
  URL.revokeObjectURL(url)

  ElMessage.success(t('config.configExported'))
}

// 导入配置文件选择
const handleImportFile = async (file: { raw: File }) => {
  if (!importExportPassword.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  const valid = await verifyPassword(importExportPassword.value)
  if (!valid) {
    ElMessage.error(t('common.passwordError'))
    return
  }

  const reader = new FileReader()
  reader.onload = (e) => {
    try {
      const importedConfig = JSON.parse(e.target?.result as string)

      // 验证配置结构
      if (!importedConfig.Name || !importedConfig.Addr || !importedConfig.Network) {
        ElMessage.error(t('config.configInvalid'))
        return
      }

      // 保留当前节点的关键信息
      const currentName = formConfig.Name
      const currentAddr = formConfig.Addr
      const currentPort = formConfig.Port

      // 应用导入的配置
      Object.assign(formConfig, importedConfig, {
        Name: currentName,
        Addr: currentAddr,
        Port: currentPort
      })

      ElMessage.success(t('config.configImported'))
    } catch {
      ElMessage.error(t('config.configParseFailed'))
    }
  }
  reader.readAsText(file.raw)
}

const showAddNode = () => {
  newNodeName.value = ''
  newNodeAddr.value = ''
  addNodeVisible.value = true
}

const addNode = () => {
  if (!newNodeName.value || !newNodeAddr.value) {
    ElMessage.warning(t('config.pleaseEnterNodeAndIP'))
    return
  }

  // 验证 IP 格式
  const ipRegex = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/
  if (!ipRegex.test(newNodeAddr.value.trim())) {
    ElMessage.warning(t('config.pleaseEnterValidIPv4'))
    return
  }

  const addr = newNodeAddr.value.trim()
  if (formConfig.Network[addr]) {
    ElMessage.warning(t('config.nodeIPExists'))
    return
  }

  formConfig.Network[addr] = {
    Name: newNodeName.value.trim(),
    Addr: addr,
    Smartping: false,
    Ping: [],
    Topology: []
  }

  addNodeVisible.value = false
  ElMessage.success(t('config.nodeAdded'))
}

const deleteNode = (row: NetworkListItem) => {
  delete formConfig.Network[row.Addr]
}

const editPingConfig = (row: NetworkListItem) => {
  currentEditNode.value = { Name: row.Name, Addr: row.Addr }

  // 获取当前节点的 Ping 配置
  const currentPingList = formConfig.Network[row.Addr]?.Ping || []

  // 构建可选目标列表（排除自己）
  pingTargetList.value = Object.entries(formConfig.Network)
    .filter(([addr]) => addr !== row.Addr)
    .map(([addr, network]) => ({
      Name: network.Name,
      Addr: addr,
      enabled: currentPingList.includes(addr)
    }))

  pingConfigVisible.value = true
}

const savePingConfig = () => {
  if (!currentEditNode.value) return

  // 收集选中的目标 IP
  const selectedAddrs = pingTargetList.value
    .filter(item => item.enabled)
    .map(item => item.Addr)

  // 更新配置
  if (formConfig.Network[currentEditNode.value.Addr]) {
    formConfig.Network[currentEditNode.value.Addr].Ping = selectedAddrs
  }

  pingConfigVisible.value = false
  ElMessage.success(t('config.pingConfigUpdated'))
}

const editTopoConfig = (row: NetworkListItem) => {
  currentEditNode.value = { Name: row.Name, Addr: row.Addr }

  // 获取当前节点的拓扑配置
  const currentTopoList = formConfig.Network[row.Addr]?.Topology?.map(t => t.Addr) || []

  // 构建可选目标列表（排除自己）
  topoTargetList.value = Object.entries(formConfig.Network)
    .filter(([addr]) => addr !== row.Addr)
    .map(([addr, network]) => ({
      Name: network.Name,
      Addr: addr,
      enabled: currentTopoList.includes(addr)
    }))

  topoConfigVisible.value = true
}

const saveTopoConfig = () => {
  if (!currentEditNode.value) return

  // 收集选中的目标，构造拓扑配置格式
  const selectedTopos = topoTargetList.value
    .filter(item => item.enabled)
    .map(item => ({
      Name: item.Name,
      Addr: item.Addr,
      Thdchecksec: '1',
      Thdoccnum: '5',
      Thdavgdelay: '1000',
      Thdloss: '100'
    }))

  // 更新配置
  if (formConfig.Network[currentEditNode.value.Addr]) {
    formConfig.Network[currentEditNode.value.Addr].Topology = selectedTopos
  }

  topoConfigVisible.value = false
  ElMessage.success(t('config.topoConfigUpdated'))
}

const editChinaMap = (prov: string) => {
  currentProvince.value = prov
  chinaMapTab.value = 'ctcc'

  const provData = formConfig.Chinamap[prov] || {}

  chinaMapIps.ctcc = (provData.ctcc || []).join('\n')
  chinaMapIps.cucc = (provData.cucc || []).join('\n')
  chinaMapIps.cmcc = (provData.cmcc || []).join('\n')

  chinaMapVisible.value = true
}

const saveChinaMap = () => {
  if (!currentProvince.value) return

  // 解析 IP 列表
  const parseIps = (text: string) => {
    return text.split('\n')
      .map(ip => ip.trim())
      .filter(ip => ip.length > 0)
  }

  formConfig.Chinamap[currentProvince.value] = {
    ctcc: parseIps(chinaMapIps.ctcc),
    cucc: parseIps(chinaMapIps.cucc),
    cmcc: parseIps(chinaMapIps.cmcc)
  }

  chinaMapVisible.value = false
  ElMessage.success(t('config.delayConfigUpdated'))
}

const deleteChinaMap = () => {
  if (!currentProvince.value) return

  delete formConfig.Chinamap[currentProvince.value]
  chinaMapVisible.value = false
  ElMessage.success(t('config.provinceDeleted'))
}

const showAddChinaMap = () => {
  newProvinceName.value = ''
  addProvinceVisible.value = true
}

const addProvince = () => {
  if (!newProvinceName.value.trim()) {
    ElMessage.warning(t('config.pleaseEnterProvinceName'))
    return
  }

  const provName = newProvinceName.value.trim()

  if (formConfig.Chinamap[provName]) {
    ElMessage.warning(t('config.provinceExists'))
    return
  }

  formConfig.Chinamap[provName] = {
    ctcc: [],
    cucc: [],
    cmcc: []
  }

  addProvinceVisible.value = false
  ElMessage.success(t('config.provinceAdded'))
}

onMounted(() => {
  loadConfig()
})
</script>

<style scoped lang="scss">
.config-view {
  height: 100%;
}

.config-header {
  margin-bottom: 20px;

  h2 {
    margin: 0;
    font-size: 18px;
    color: var(--color-text-primary);
  }
}

.config-content {
  .card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }

  .save-config {
    display: flex;
    gap: 12px;
    align-items: center;
  }

  .import-export {
    display: flex;
    gap: 12px;
  }

  .import-export-row {
    display: flex;
    gap: 12px;
    align-items: center;
    flex-wrap: wrap;
  }

  h4 {
    margin: 16px 0 12px 0;
    color: var(--color-text-primary);
    font-size: 14px;
  }

  .chinamap-list {
    display: flex;
    flex-wrap: wrap;
    gap: 10px;
  }

  // 深色模式下卡片标题颜色
  :deep(.el-card__header) {
    span {
      color: var(--color-text-primary);
    }
  }
}

.dialog-tip {
  margin: 0 0 16px 0;
  color: var(--color-text-regular);

  strong {
    color: var(--color-primary);
  }
}

.ip-list-editor {
  :deep(.el-textarea__inner) {
    font-family: monospace;
  }
}
</style>
