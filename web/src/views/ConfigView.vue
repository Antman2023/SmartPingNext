<template>
  <div class="page-shell config-view">
    <div class="page-header">
      <div class="page-heading">
        <span class="page-eyebrow">{{ $t('config.title') }}</span>
        <h1 class="page-title">{{ $t('config.title') }}</h1>
        <p class="page-subtitle">{{ $t('config.subtitle') }}</p>
      </div>

      <div class="page-actions">
        <div class="page-kpis">
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.node') }}</span>
            <strong class="page-kpi__value">{{ networkList.length }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.probes') }}</span>
            <strong class="page-kpi__value">{{ smartpingCount }}</strong>
          </div>
          <div class="page-kpi">
            <span class="page-kpi__label">{{ $t('common.provinces') }}</span>
            <strong class="page-kpi__value">{{ provinceCount }}</strong>
          </div>
        </div>
      </div>
    </div>

    <div class="config-view__grid">
      <div class="config-view__rail">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.saveConfig') }}</h2>
              <p class="surface-panel__description">{{ $t('config.subtitle') }}</p>
            </div>
            <el-link type="primary" href="/api/config.json" target="_blank">
              <el-icon><Document /></el-icon>
            </el-link>
          </div>

          <div class="control-row">
            <el-input v-model="password" type="password" :placeholder="$t('common.password')" />
            <el-button type="primary" @click="handleSave">{{ $t('common.save') }}</el-button>
          </div>
        </section>

        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.importExport') }}</h2>
              <p class="surface-panel__description">{{ $t('config.configImported') }}</p>
            </div>
          </div>

          <div class="config-view__stack">
            <el-input
              v-model="importExportPassword"
              type="password"
              :placeholder="$t('common.password')"
            />
            <div class="control-row">
              <el-button @click="handleExport">{{ $t('config.exportConfig') }}</el-button>
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                accept=".json"
                :on-change="handleImportFile"
              >
                <el-button>{{ $t('config.importConfig') }}</el-button>
              </el-upload>
            </div>
          </div>
        </section>

        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.baseConfig') }}</h2>
              <p class="surface-panel__description">
                {{ $t('config.base') }} / {{ $t('config.pingTopology') }}
              </p>
            </div>
          </div>

          <el-form label-position="top" class="config-view__form">
            <div class="config-view__group">
              <h3>{{ $t('config.base') }}</h3>
              <div class="grid-3">
                <el-form-item :label="$t('config.timeout')">
                  <el-input v-model.number="formConfig.Base.Timeout" />
                </el-form-item>
                <el-form-item :label="$t('config.pageRefresh')">
                  <el-input v-model.number="formConfig.Base.Refresh" />
                </el-form-item>
                <el-form-item :label="$t('config.dataArchive')">
                  <el-input v-model.number="formConfig.Base.Archive" />
                </el-form-item>
              </div>
            </div>

            <div class="config-view__group">
              <h3>{{ $t('config.pingTopology') }}</h3>
              <div class="grid-3">
                <el-form-item :label="$t('config.alertSound')">
                  <el-input v-model="formConfig.Topology.Tsound" />
                </el-form-item>
                <el-form-item :label="$t('config.lineWidth')">
                  <el-input v-model="formConfig.Topology.Tline" />
                </el-form-item>
                <el-form-item :label="$t('config.symbolSize')">
                  <el-input v-model="formConfig.Topology.Tsymbolsize" />
                </el-form-item>
              </div>
            </div>

            <div class="config-view__group">
              <h3>{{ $t('config.checkTools') }}</h3>
              <el-form-item :label="$t('config.rateLimit')">
                <el-input v-model.number="formConfig.Toollimit" />
              </el-form-item>
            </div>

            <div class="config-view__group">
              <h3>{{ $t('config.authManagement') }}</h3>
              <el-form-item :label="$t('config.ipWhitelist')">
                <el-input v-model="formConfig.Authiplist" />
              </el-form-item>
            </div>
          </el-form>
        </section>
      </div>

      <div class="config-view__main">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.pingNetwork') }}</h2>
              <p class="surface-panel__description">
                {{ networkList.length }} {{ $t('common.node') }}
              </p>
            </div>
            <el-button @click="showAddNode">{{ $t('config.addNode') }}</el-button>
          </div>

          <div class="table-scroll">
            <el-table :data="networkList" stripe style="width: 100%">
              <el-table-column prop="Name" :label="$t('config.nodeName')" min-width="140" />
              <el-table-column prop="Addr" :label="$t('config.nodeIP')" min-width="150" />
              <el-table-column label="SmartPing" width="110">
                <template #default="{ row }">
                  <el-checkbox v-model="row._original.Smartping" :disabled="row.isSelf" />
                </template>
              </el-table-column>
              <el-table-column :label="$t('common.operation')" min-width="260">
                <template #default="{ row }">
                  <div class="control-row">
                    <el-button size="small" @click="showEditNode(row)">{{
                      $t('common.edit')
                    }}</el-button>
                    <el-button
                      size="small"
                      :disabled="!row.isSelf && !row._original.Smartping"
                      @click="editPingConfig(row)"
                    >
                      {{ $t('config.pingConfig') }}
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!row.isSelf && !row._original.Smartping"
                      @click="editTopoConfig(row)"
                    >
                      {{ $t('config.topoConfig') }}
                    </el-button>
                  </div>
                </template>
              </el-table-column>
              <el-table-column width="70">
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
          </div>
        </section>

        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.chinaMapNetwork') }}</h2>
              <p class="surface-panel__description">
                {{ provinceCount }} {{ $t('common.provinces') }}
              </p>
            </div>
            <el-button @click="showAddChinaMap">{{ $t('config.addProvince') }}</el-button>
          </div>

          <div class="config-view__province-grid">
            <button
              v-for="(provData, prov) in formConfig.Chinamap"
              :key="prov"
              type="button"
              class="surface-panel surface-panel--tight config-view__province"
              @click="editChinaMap(prov as string)"
            >
              <strong>{{ prov }}</strong>
              <span>{{ $t('mapping.telecom') }} {{ provData.ctcc?.length || 0 }}</span>
              <span>{{ $t('mapping.unicom') }} {{ provData.cucc?.length || 0 }}</span>
              <span>{{ $t('mapping.mobile') }} {{ provData.cmcc?.length || 0 }}</span>
            </button>
          </div>
        </section>
      </div>
    </div>

    <el-dialog v-model="addNodeVisible" :title="$t('config.newNode')" width="420px">
      <el-form label-position="top">
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

    <el-dialog v-model="editNodeVisible" :title="$t('config.editNode')" width="420px">
      <el-form label-position="top">
        <el-form-item :label="$t('config.nodeName')">
          <el-input v-model="editNodeName" :placeholder="$t('config.pleaseEnterNodeName')" />
        </el-form-item>
        <el-form-item :label="$t('config.nodeIP')">
          <el-input v-model="editNodeAddr" :placeholder="$t('config.pleaseEnterIPv4')" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="editNodeVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveEditNode">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="pingConfigVisible" :title="$t('config.pingConfig')" width="560px">
      <p class="config-view__dialog-tip">
        {{ $t('config.selectPingTargets', { name: currentEditNode?.Name }) }}
      </p>
      <div class="table-scroll">
        <el-table :data="pingTargetList" stripe max-height="420">
          <el-table-column prop="Name" :label="$t('config.nodeName')" min-width="150" />
          <el-table-column prop="Addr" :label="$t('config.nodeIP')" min-width="150" />
          <el-table-column :label="$t('common.enable')" width="90" align="center">
            <template #default="{ row }">
              <el-checkbox v-model="row.enabled" />
            </template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="pingConfigVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="savePingConfig">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="topoConfigVisible"
      :title="$t('config.topoConfig')"
      width="min(960px, 94vw)"
    >
      <p class="config-view__dialog-tip">
        {{ $t('config.selectTopoTargets', { name: currentEditNode?.Name }) }}
      </p>
      <div class="table-scroll">
        <el-table :data="topoTargetList" stripe max-height="460">
          <el-table-column prop="Name" :label="$t('config.nodeName')" min-width="130" />
          <el-table-column prop="Addr" :label="$t('config.nodeIP')" width="140" />
          <el-table-column :label="$t('common.enable')" width="75" align="center">
            <template #default="{ row }">
              <el-checkbox v-model="row.enabled" />
            </template>
          </el-table-column>
          <el-table-column :label="$t('config.checkWindow')" width="145">
            <template #default="{ row }">
              <el-input-number
                v-model="row.checkSeconds"
                class="config-view__number-input"
                :disabled="!row.enabled"
                :min="60"
                :max="86400"
                :step="60"
                controls-position="right"
                size="small"
              />
            </template>
          </el-table-column>
          <el-table-column :label="$t('config.occurrenceThreshold')" width="125">
            <template #default="{ row }">
              <el-input-number
                v-model="row.occurrenceCount"
                class="config-view__number-input"
                :disabled="!row.enabled"
                :min="1"
                :max="10000"
                controls-position="right"
                size="small"
              />
            </template>
          </el-table-column>
          <el-table-column :label="$t('config.avgDelayThreshold')" width="135">
            <template #default="{ row }">
              <el-input-number
                v-model="row.avgDelay"
                class="config-view__number-input"
                :disabled="!row.enabled"
                :min="1"
                :max="60000"
                controls-position="right"
                size="small"
              />
            </template>
          </el-table-column>
          <el-table-column :label="$t('config.lossThreshold')" width="115">
            <template #default="{ row }">
              <el-input-number
                v-model="row.lossPercent"
                class="config-view__number-input"
                :disabled="!row.enabled"
                :min="0"
                :max="100"
                controls-position="right"
                size="small"
              />
            </template>
          </el-table-column>
        </el-table>
      </div>
      <template #footer>
        <el-button @click="topoConfigVisible = false">{{ $t('common.cancel') }}</el-button>
        <el-button type="primary" @click="saveTopoConfig">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog
      v-model="chinaMapVisible"
      :title="
        currentProvince
          ? $t('config.chinaMapConfig', { province: currentProvince })
          : $t('config.chinaMapConfig', { province: '' })
      "
      width="640px"
    >
      <el-tabs v-model="chinaMapTab">
        <el-tab-pane label="电信(CTCC)" name="ctcc">
          <div class="config-view__ip-editor">
            <el-input
              v-model="chinaMapIps.ctcc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="联通(CUCC)" name="cucc">
          <div class="config-view__ip-editor">
            <el-input
              v-model="chinaMapIps.cucc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="移动(CMCC)" name="cmcc">
          <div class="config-view__ip-editor">
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
        <el-button v-if="currentProvince" type="danger" @click="deleteChinaMap">{{
          $t('config.deleteProvince')
        }}</el-button>
        <el-button type="primary" @click="saveChinaMap">{{ $t('common.confirm') }}</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="addProvinceVisible" :title="$t('config.addProvince')" width="420px">
      <el-form label-position="top">
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
import { computed, onMounted, reactive, ref } from 'vue'
import { useI18n } from 'vue-i18n'
import { Delete, Document } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { fetchConfig, saveConfig } from '@/api/config'
import type { Config, NetworkMember, TopologyConfig } from '@/types'

const { t } = useI18n()
const config = ref<Config | null>(null)
const password = ref('')
const importExportPassword = ref('')

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
  if (!formConfig.Network) {
    return []
  }

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

const smartpingCount = computed(() => networkList.value.filter((node) => node.Smartping).length)
const provinceCount = computed(() => Object.keys(formConfig.Chinamap || {}).length)

const addNodeVisible = ref(false)
const newNodeName = ref('')
const newNodeAddr = ref('')

const editNodeVisible = ref(false)
const editNodeName = ref('')
const editNodeAddr = ref('')
const editNodeOriginalAddr = ref('')

const pingConfigVisible = ref(false)
const currentEditNode = ref<{ Name: string; Addr: string } | null>(null)
const pingTargetList = ref<Array<{ Name: string; Addr: string; enabled: boolean }>>([])

interface TopologyTargetItem {
  Name: string
  Addr: string
  enabled: boolean
  checkSeconds: number
  occurrenceCount: number
  avgDelay: number
  lossPercent: number
}

const topoConfigVisible = ref(false)
const topoTargetList = ref<TopologyTargetItem[]>([])

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
    Object.assign(formConfig, cfg)
  } catch (error) {
    console.error('加载配置失败', error)
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
  } catch (error: unknown) {
    ElMessage.error(t('common.saveFailed') + ': ' + (error instanceof Error ? error.message : ''))
    console.error('保存失败', error)
  }
}

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
  reader.onload = (event) => {
    try {
      const importedConfig = JSON.parse(event.target?.result as string)

      if (!importedConfig.Name || !importedConfig.Addr || !importedConfig.Network) {
        ElMessage.error(t('config.configInvalid'))
        return
      }

      const currentName = formConfig.Name
      const currentAddr = formConfig.Addr
      const currentPort = formConfig.Port

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

  const ipRegex =
    /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/
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
  const addr = row.Addr
  delete formConfig.Network[addr]

  for (const [, member] of Object.entries(formConfig.Network)) {
    const pingIndex = member.Ping.indexOf(addr)
    if (pingIndex !== -1) {
      member.Ping.splice(pingIndex, 1)
    }
    member.Topology = member.Topology.filter((topology) => topology.Addr !== addr)
  }
}

const showEditNode = (row: NetworkListItem) => {
  editNodeOriginalAddr.value = row.Addr
  editNodeName.value = row.Name
  editNodeAddr.value = row.Addr
  editNodeVisible.value = true
}

const saveEditNode = () => {
  if (!editNodeName.value || !editNodeAddr.value) {
    ElMessage.warning(t('config.pleaseEnterNodeAndIP'))
    return
  }

  const ipRegex =
    /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/
  if (!ipRegex.test(editNodeAddr.value.trim())) {
    ElMessage.warning(t('config.pleaseEnterValidIPv4'))
    return
  }

  const oldAddr = editNodeOriginalAddr.value
  const newAddr = editNodeAddr.value.trim()
  const newName = editNodeName.value.trim()

  if (oldAddr !== newAddr && formConfig.Network[newAddr]) {
    ElMessage.warning(t('config.nodeIPExists'))
    return
  }

  const nodeData = formConfig.Network[oldAddr]
  if (!nodeData) {
    return
  }

  nodeData.Name = newName
  nodeData.Addr = newAddr

  if (oldAddr !== newAddr) {
    formConfig.Network[newAddr] = nodeData
    delete formConfig.Network[oldAddr]

    for (const [, member] of Object.entries(formConfig.Network)) {
      const pingIndex = member.Ping.indexOf(oldAddr)
      if (pingIndex !== -1) {
        member.Ping[pingIndex] = newAddr
      }

      for (const topology of member.Topology) {
        if (topology.Addr === oldAddr) {
          topology.Addr = newAddr
          topology.Name = newName
        }
      }
    }

    if (oldAddr === formConfig.Addr) {
      formConfig.Addr = newAddr
      formConfig.Name = newName
    }
  } else {
    for (const [, member] of Object.entries(formConfig.Network)) {
      for (const topology of member.Topology) {
        if (topology.Addr === oldAddr) {
          topology.Name = newName
        }
      }
    }

    if (oldAddr === formConfig.Addr) {
      formConfig.Name = newName
    }
  }

  editNodeVisible.value = false
  ElMessage.success(t('config.nodeUpdated'))
}

const editPingConfig = (row: NetworkListItem) => {
  currentEditNode.value = { Name: row.Name, Addr: row.Addr }
  const currentPingList = formConfig.Network[row.Addr]?.Ping || []

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
  if (!currentEditNode.value) {
    return
  }

  const selectedAddrs = pingTargetList.value.filter((item) => item.enabled).map((item) => item.Addr)

  if (formConfig.Network[currentEditNode.value.Addr]) {
    formConfig.Network[currentEditNode.value.Addr].Ping = selectedAddrs
  }

  pingConfigVisible.value = false
  ElMessage.success(t('config.pingConfigUpdated'))
}

const editTopoConfig = (row: NetworkListItem) => {
  currentEditNode.value = { Name: row.Name, Addr: row.Addr }
  const currentTopologies = new Map(
    (formConfig.Network[row.Addr]?.Topology || []).map((topology) => [topology.Addr, topology])
  )

  const ruleNumber = (value: string | undefined, fallback: number, min: number, max: number) => {
    const parsed = Number(value)
    return Number.isFinite(parsed) ? Math.min(Math.max(parsed, min), max) : fallback
  }

  topoTargetList.value = Object.entries(formConfig.Network)
    .filter(([addr]) => addr !== row.Addr)
    .map(([addr, network]) => {
      const current = currentTopologies.get(addr)
      return {
        Name: network.Name,
        Addr: addr,
        enabled: !!current,
        checkSeconds: ruleNumber(current?.Thdchecksec, 900, 60, 86400),
        occurrenceCount: ruleNumber(current?.Thdoccnum, 3, 1, 10000),
        avgDelay: ruleNumber(current?.Thdavgdelay, 200, 1, 60000),
        lossPercent: ruleNumber(current?.Thdloss, 30, 0, 100)
      }
    })

  topoConfigVisible.value = true
}

const saveTopoConfig = () => {
  if (!currentEditNode.value) {
    return
  }

  const selectedTopologies = topoTargetList.value
    .filter((item) => item.enabled)
    .map((item) => ({
      Name: item.Name,
      Addr: item.Addr,
      Thdchecksec: String(item.checkSeconds),
      Thdoccnum: String(item.occurrenceCount),
      Thdavgdelay: String(item.avgDelay),
      Thdloss: String(item.lossPercent)
    }))

  if (formConfig.Network[currentEditNode.value.Addr]) {
    formConfig.Network[currentEditNode.value.Addr].Topology = selectedTopologies
  }

  topoConfigVisible.value = false
  ElMessage.success(t('config.topoConfigUpdated'))
}

const editChinaMap = (province: string) => {
  currentProvince.value = province
  chinaMapTab.value = 'ctcc'

  const provinceData = formConfig.Chinamap[province] || {}
  chinaMapIps.ctcc = (provinceData.ctcc || []).join('\n')
  chinaMapIps.cucc = (provinceData.cucc || []).join('\n')
  chinaMapIps.cmcc = (provinceData.cmcc || []).join('\n')

  chinaMapVisible.value = true
}

const saveChinaMap = () => {
  if (!currentProvince.value) {
    return
  }

  const parseIps = (text: string) => {
    return text
      .split('\n')
      .map((ip) => ip.trim())
      .filter((ip) => ip.length > 0)
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
  if (!currentProvince.value) {
    return
  }

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

  const provinceName = newProvinceName.value.trim()
  if (formConfig.Chinamap[provinceName]) {
    ElMessage.warning(t('config.provinceExists'))
    return
  }

  formConfig.Chinamap[provinceName] = {
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
.config-view__grid {
  display: grid;
  grid-template-columns: minmax(340px, 0.9fr) minmax(0, 1.5fr);
  gap: 20px;
}

.config-view__rail,
.config-view__main {
  display: flex;
  flex-direction: column;
  gap: 20px;
}

.config-view__stack {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.config-view__form {
  display: flex;
  flex-direction: column;
  gap: 18px;
}

.config-view__group {
  padding-top: 4px;

  h3 {
    margin: 0 0 12px;
    font-size: 14px;
    font-weight: 600;
    letter-spacing: -0.01em;
    color: var(--color-text-primary);
  }
}

.config-view__province-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
  gap: 14px;
}

.config-view__province {
  width: 100%;
  text-align: left;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  gap: 8px;
  font: inherit;
  color: var(--color-text-primary);

  strong {
    font-size: 16px;
    font-weight: 700;
    letter-spacing: -0.03em;
  }

  span {
    font-size: 13px;
    color: var(--color-text-secondary);
  }
}

.config-view__dialog-tip {
  margin: 0 0 16px;
  color: var(--color-text-secondary);
  line-height: 1.7;
}

.config-view__ip-editor :deep(.el-textarea__inner) {
  font-family: 'JetBrains Mono', 'Cascadia Code', 'SFMono-Regular', Consolas, monospace;
}

.config-view__number-input {
  width: 100%;
}

@media (max-width: 1320px) {
  .config-view__grid {
    grid-template-columns: 1fr;
  }
}
</style>
