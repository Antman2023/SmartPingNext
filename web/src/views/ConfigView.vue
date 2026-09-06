<template>
  <div
    v-loading="loadingConfig || saving"
    :element-loading-text="saving ? $t('common.saving') : $t('common.loading')"
    class="page-shell config-view"
  >
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

    <section
      v-if="configError && !configLoaded"
      class="surface-panel empty-state config-view__load-error"
    >
      <el-icon class="config-view__load-error-icon text-danger"><Warning /></el-icon>
      <span>{{ $t('common.configLoadFailedNetwork') }}</span>
      <el-button size="small" @click="loadConfig">{{ $t('common.retry') }}</el-button>
    </section>

    <div v-else class="config-view__grid">
      <div class="config-view__rail">
        <section class="surface-panel">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.saveConfig') }}</h2>
              <p class="surface-panel__description">{{ $t('config.subtitle') }}</p>
            </div>
            <div class="config-view__save-meta">
              <span
                v-if="configLoaded"
                class="config-view__save-state"
                :class="{ 'is-dirty': isDirty }"
                aria-live="polite"
              >
                <el-icon>
                  <EditPen v-if="isDirty" />
                  <CircleCheck v-else />
                </el-icon>
                {{ isDirty ? $t('config.unsaved') : $t('config.saved') }}
              </span>
              <el-tooltip :content="$t('config.viewRawConfig')">
                <el-link
                  type="primary"
                  href="/api/config.json"
                  target="_blank"
                  :aria-label="$t('config.viewRawConfig')"
                  :title="$t('config.viewRawConfig')"
                >
                  <el-icon><Document /></el-icon>
                </el-link>
              </el-tooltip>
            </div>
          </div>

          <div class="control-row">
            <el-input
              v-model="password"
              type="password"
              :disabled="saving || importExportBusy || !isDirty"
              :placeholder="$t('common.password')"
              @keyup.enter="handleSave"
            />
            <el-button
              type="primary"
              :loading="saving"
              :disabled="importExportBusy || !isDirty"
              @click="handleSave"
              >{{ $t('common.save') }}</el-button
            >
          </div>
          <div
            v-if="currentValidationIssue"
            id="config-validation-summary"
            class="config-view__validation"
            role="alert"
            aria-live="assertive"
            tabindex="-1"
          >
            <el-icon><Warning /></el-icon>
            <span>{{ validationIssueMessage }}</span>
          </div>
        </section>

        <section id="config-base-settings" class="surface-panel" tabindex="-1">
          <div class="surface-panel__header">
            <div>
              <h2 class="surface-panel__title">{{ $t('config.importExport') }}</h2>
              <p class="surface-panel__description">{{ $t('config.importExportHint') }}</p>
            </div>
          </div>

          <div class="config-view__stack">
            <el-input
              v-model="importExportPassword"
              type="password"
              :disabled="importExportBusy || saving"
              :placeholder="$t('common.password')"
            />
            <div class="control-row">
              <el-button
                :loading="exporting"
                :disabled="importing || saving"
                @click="handleExport"
                >{{ $t('config.exportConfig') }}</el-button
              >
              <el-upload
                :auto-upload="false"
                :show-file-list="false"
                :disabled="importExportBusy || saving"
                accept=".json"
                :on-change="handleImportFile"
              >
                <el-button :loading="importing" :disabled="exporting || saving">{{
                  $t('config.importConfig')
                }}</el-button>
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
                <el-form-item
                  :label="$t('config.timeout')"
                  :error="validationErrorFor('config.validationTimeout')"
                >
                  <el-input-number
                    id="config-timeout"
                    v-model="formConfig.Base.Timeout"
                    class="config-view__number-input"
                    :min="CONFIG_LIMITS.timeout.min"
                    :max="CONFIG_LIMITS.timeout.max"
                    controls-position="right"
                  />
                </el-form-item>
                <el-form-item
                  :label="$t('config.pageRefresh')"
                  :error="validationErrorFor('config.validationRefresh')"
                >
                  <el-input-number
                    id="config-refresh"
                    v-model="formConfig.Base.Refresh"
                    class="config-view__number-input"
                    :min="CONFIG_LIMITS.refresh.min"
                    :max="CONFIG_LIMITS.refresh.max"
                    controls-position="right"
                  />
                </el-form-item>
                <el-form-item
                  :label="$t('config.dataArchive')"
                  :error="validationErrorFor('config.validationArchive')"
                >
                  <el-input-number
                    id="config-archive"
                    v-model="formConfig.Base.Archive"
                    class="config-view__number-input"
                    :min="CONFIG_LIMITS.archive.min"
                    :max="CONFIG_LIMITS.archive.max"
                    controls-position="right"
                  />
                </el-form-item>
              </div>
            </div>

            <div class="config-view__group">
              <h3>{{ $t('config.pingTopology') }}</h3>
              <div class="grid-3">
                <el-form-item :label="$t('config.alertSound')">
                  <el-input v-model="formConfig.Topology.Tsound" />
                </el-form-item>
                <el-form-item
                  :label="$t('config.lineWidth')"
                  :error="validationErrorFor('config.validationLineWidth')"
                >
                  <el-input-number
                    id="config-topology-line"
                    :model-value="Number(formConfig.Topology.Tline)"
                    class="config-view__number-input"
                    :min="CONFIG_LIMITS.topologyLine.min"
                    :max="CONFIG_LIMITS.topologyLine.max"
                    :step="0.5"
                    controls-position="right"
                    @update:model-value="updateTopologyNumber('Tline', $event)"
                  />
                </el-form-item>
                <el-form-item
                  :label="$t('config.symbolSize')"
                  :error="validationErrorFor('config.validationSymbolSize')"
                >
                  <el-input-number
                    id="config-topology-symbol"
                    :model-value="Number(formConfig.Topology.Tsymbolsize)"
                    class="config-view__number-input"
                    :min="CONFIG_LIMITS.topologySymbol.min"
                    :max="CONFIG_LIMITS.topologySymbol.max"
                    controls-position="right"
                    @update:model-value="updateTopologyNumber('Tsymbolsize', $event)"
                  />
                </el-form-item>
              </div>
            </div>

            <div class="config-view__group">
              <h3>{{ $t('config.checkTools') }}</h3>
              <el-form-item
                :label="$t('config.rateLimit')"
                :error="validationErrorFor('config.validationToolLimit')"
              >
                <el-input-number
                  id="config-tool-limit"
                  v-model="formConfig.Toollimit"
                  class="config-view__number-input"
                  :min="CONFIG_LIMITS.toolLimit.min"
                  :max="CONFIG_LIMITS.toolLimit.max"
                  controls-position="right"
                />
              </el-form-item>
            </div>

            <div id="config-auth-settings" class="config-view__group" tabindex="-1">
              <h3>{{ $t('config.authManagement') }}</h3>
              <el-form-item
                :label="$t('config.ipWhitelist')"
                :error="
                  validationErrorFor('config.validationAuthAddress', 'config.validationAuthLimit')
                "
              >
                <el-input id="config-auth-list" v-model="formConfig.Authiplist" />
              </el-form-item>
            </div>
          </el-form>
        </section>
      </div>

      <div class="config-view__main">
        <section id="config-network-settings" class="surface-panel" tabindex="-1">
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
                  <el-checkbox
                    v-model="row._original.Smartping"
                    :disabled="row.isSelf"
                    :aria-label="$t('config.toggleSmartping', { name: displayName(row.Name) })"
                  />
                </template>
              </el-table-column>
              <el-table-column :label="$t('common.operation')" min-width="260">
                <template #default="{ row }">
                  <div class="control-row">
                    <el-button size="small" @click="showEditNode(row as NetworkListItem)">{{
                      $t('common.edit')
                    }}</el-button>
                    <el-button
                      size="small"
                      :disabled="!row.isSelf && !row._original.Smartping"
                      @click="editPingConfig(row as NetworkListItem)"
                    >
                      {{ $t('config.pingConfig') }}
                    </el-button>
                    <el-button
                      size="small"
                      :disabled="!row.isSelf && !row._original.Smartping"
                      @click="editTopoConfig(row as NetworkListItem)"
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
                    :aria-label="$t('config.deleteNodeLabel', { name: displayName(row.Name) })"
                    :title="$t('config.deleteNodeLabel', { name: displayName(row.Name) })"
                    @click="deleteNode(row as NetworkListItem)"
                  />
                </template>
              </el-table-column>
            </el-table>
          </div>
        </section>

        <section id="config-mapping-settings" class="surface-panel" tabindex="-1">
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
                :step-strictly="true"
                controls-position="right"
                size="small"
                @change="
                  row.occurrenceCount = Math.min(
                    row.occurrenceCount,
                    maxOccurrenceCount(row.checkSeconds)
                  )
                "
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
                :max="maxOccurrenceCount(row.checkSeconds)"
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
        <el-tab-pane :label="`${$t('mapping.telecom')} (CTCC)`" name="ctcc">
          <el-form-item class="config-view__ip-editor" :error="chinaMapValidationError('ctcc')">
            <el-input
              v-model="chinaMapIps.ctcc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
              @input="clearChinaMapValidation('ctcc')"
            />
          </el-form-item>
        </el-tab-pane>
        <el-tab-pane :label="`${$t('mapping.unicom')} (CUCC)`" name="cucc">
          <el-form-item class="config-view__ip-editor" :error="chinaMapValidationError('cucc')">
            <el-input
              v-model="chinaMapIps.cucc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
              @input="clearChinaMapValidation('cucc')"
            />
          </el-form-item>
        </el-tab-pane>
        <el-tab-pane :label="`${$t('mapping.mobile')} (CMCC)`" name="cmcc">
          <el-form-item class="config-view__ip-editor" :error="chinaMapValidationError('cmcc')">
            <el-input
              v-model="chinaMapIps.cmcc"
              type="textarea"
              :rows="6"
              :placeholder="$t('config.eachLineOneIP')"
              @input="clearChinaMapValidation('cmcc')"
            />
          </el-form-item>
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
import { computed, nextTick, onMounted, onUnmounted, reactive, ref } from 'vue'
import { onBeforeRouteLeave } from 'vue-router'
import { useI18n } from 'vue-i18n'
import '@/plugins/elementPlusConfigStyles'
import { CircleCheck, Delete, Document, EditPen, Warning } from '@element-plus/icons-vue'
import {
  ElCheckbox,
  ElDialog,
  ElForm,
  ElFormItem,
  ElInput,
  ElInputNumber,
  ElLink,
  ElMessage,
  ElMessageBox,
  ElTabPane,
  ElTable,
  ElTableColumn,
  ElTabs,
  ElTooltip,
  ElUpload,
  type UploadFile
} from 'element-plus'
import { useConfigStore } from '@/stores/config'
import { isRequestCanceled } from '@/api'
import {
  CONFIG_LIMITS,
  isValidIPv4,
  validateConfigForEdit,
  type ConfigValidationIssue
} from '@/utils/configValidation'
import { displayName } from '@/utils/format'
import type { Config, NetworkMember, TopologyConfig } from '@/types'

const { t } = useI18n()
const configStore = useConfigStore()
const password = ref('')
const importExportPassword = ref('')
const loadingConfig = ref(false)
const configError = ref(false)
const configLoaded = ref(false)
const savedSnapshot = ref('')
const saving = ref(false)
const exporting = ref(false)
const importing = ref(false)
const importExportBusy = computed(() => exporting.value || importing.value)
let isUnmounted = false
let configRequestId = 0
let saveAbortController: AbortController | null = null
let passwordVerificationAbortController: AbortController | null = null

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

const serializeConfig = (value: Config): string =>
  JSON.stringify(value, (_key, item: unknown) => {
    if (typeof item === 'object' && item !== null && !Array.isArray(item)) {
      return Object.fromEntries(
        Object.entries(item as Record<string, unknown>).sort(([left], [right]) =>
          left.localeCompare(right)
        )
      )
    }
    return item
  })

const currentSnapshot = computed(() => serializeConfig(formConfig))
const isDirty = computed(() => configLoaded.value && currentSnapshot.value !== savedSnapshot.value)
const validationAttempted = ref(false)
const currentValidationIssue = computed(() =>
  validationAttempted.value ? validateConfigForEdit(formConfig) : null
)
const validationIssueMessage = computed(() => {
  const issue = currentValidationIssue.value
  return issue ? t(issue.key, issue.params ?? {}) : ''
})

const validationErrorFor = (...keys: string[]) => {
  const issue = currentValidationIssue.value
  return issue && keys.includes(issue.key) ? t(issue.key, issue.params ?? {}) : ''
}

const validationTargetIds: Record<string, string> = {
  'config.validationTimeout': 'config-timeout',
  'config.validationArchive': 'config-archive',
  'config.validationRefresh': 'config-refresh',
  'config.validationLineWidth': 'config-topology-line',
  'config.validationSymbolSize': 'config-topology-symbol',
  'config.validationToolLimit': 'config-tool-limit',
  'config.validationAuthAddress': 'config-auth-list',
  'config.validationAuthLimit': 'config-auth-list',
  'config.validationMappingAddress': 'config-mapping-settings',
  'config.validationMappingProvider': 'config-mapping-settings',
  'config.validationMappingLimit': 'config-mapping-settings'
}

const networkValidationKeys = new Set([
  'config.validationNodeName',
  'config.validationPort',
  'config.validationLocalAddress',
  'config.validationNetworkEmpty',
  'config.validationNetworkLimit',
  'config.validationLocalNodeMissing',
  'config.validationNodeAddress',
  'config.validationNamedNode',
  'config.validationTargetLimit',
  'config.validationPingTarget',
  'config.validationDuplicateTarget',
  'config.validationDuplicateTopologyTarget',
  'config.validationTopologyTarget',
  'config.validationTopologyRule'
])

const focusValidationIssue = async (validationIssue: ConfigValidationIssue) => {
  await nextTick()
  const targetId = networkValidationKeys.has(validationIssue.key)
    ? 'config-network-settings'
    : (validationTargetIds[validationIssue.key] ?? 'config-validation-summary')
  const target = document.getElementById(targetId)
  if (!target) {
    return
  }

  target.scrollIntoView({
    behavior: window.matchMedia('(prefers-reduced-motion: reduce)').matches ? 'auto' : 'smooth',
    block: 'center'
  })
  target.focus({ preventScroll: true })
}

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

const maxOccurrenceCount = (checkSeconds: number) => {
  return Math.max(Math.floor(checkSeconds / 60), 1)
}

const updateTopologyNumber = (field: 'Tline' | 'Tsymbolsize', value: number | undefined) => {
  if (value !== undefined && Number.isFinite(value)) {
    formConfig.Topology[field] = String(value)
  }
}

const showValidationIssue = (validationIssue: ConfigValidationIssue) => {
  ElMessage.error(t(validationIssue.key, validationIssue.params ?? {}))
}

const chinaMapVisible = ref(false)
type ChinaMapProviderKey = 'ctcc' | 'cucc' | 'cmcc'

const chinaMapTab = ref<ChinaMapProviderKey>('ctcc')
const currentProvince = ref('')
const chinaMapIps = reactive({
  ctcc: '',
  cucc: '',
  cmcc: ''
})
const chinaMapValidationIssue = ref<{
  provider: ChinaMapProviderKey
  address: string
} | null>(null)
const chinaMapValidationError = (provider: ChinaMapProviderKey) => {
  const issue = chinaMapValidationIssue.value
  return issue?.provider === provider
    ? t('config.validationMappingAddress', { address: issue.address })
    : ''
}
const clearChinaMapValidation = (provider: ChinaMapProviderKey) => {
  if (chinaMapValidationIssue.value?.provider === provider) {
    chinaMapValidationIssue.value = null
  }
}
const addProvinceVisible = ref(false)
const newProvinceName = ref('')

const loadConfig = async () => {
  const requestId = ++configRequestId
  loadingConfig.value = true
  configError.value = false
  try {
    const cfg = await configStore.loadConfig()
    if (isUnmounted || requestId !== configRequestId) {
      return
    }

    if (!cfg) {
      configError.value = true
      return
    }

    Object.assign(formConfig, JSON.parse(JSON.stringify(cfg)) as Config)
    configLoaded.value = true
    validationAttempted.value = false
    savedSnapshot.value = serializeConfig(formConfig)
  } finally {
    if (!isUnmounted && requestId === configRequestId) {
      loadingConfig.value = false
    }
  }
}

const isRecord = (value: unknown): value is Record<string, unknown> => {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

const normalizeImportedConfig = (value: unknown): Record<string, unknown> | null => {
  if (
    !isRecord(value) ||
    typeof value.Name !== 'string' ||
    typeof value.Addr !== 'string' ||
    !isRecord(value.Base) ||
    Object.values(value.Base).some((item) => typeof item !== 'number' || !Number.isFinite(item)) ||
    !isRecord(value.Topology) ||
    Object.values(value.Topology).some((item) => typeof item !== 'string') ||
    !isRecord(value.Network) ||
    (value.Mode !== undefined &&
      value.Mode !== null &&
      (!isRecord(value.Mode) ||
        Object.values(value.Mode).some((item) => typeof item !== 'string'))) ||
    (value.Chinamap !== undefined && value.Chinamap !== null && !isRecord(value.Chinamap)) ||
    (value.Toollimit !== undefined &&
      (typeof value.Toollimit !== 'number' || !Number.isFinite(value.Toollimit))) ||
    (value.Authiplist !== undefined && typeof value.Authiplist !== 'string')
  ) {
    return null
  }

  const network: Record<string, unknown> = {}
  for (const [addr, rawMember] of Object.entries(value.Network)) {
    if (
      !isRecord(rawMember) ||
      typeof rawMember.Name !== 'string' ||
      typeof rawMember.Addr !== 'string' ||
      (rawMember.Smartping !== undefined && typeof rawMember.Smartping !== 'boolean') ||
      (rawMember.Ping !== undefined &&
        (!Array.isArray(rawMember.Ping) ||
          rawMember.Ping.some((target) => typeof target !== 'string'))) ||
      (rawMember.Topology !== undefined &&
        (!Array.isArray(rawMember.Topology) ||
          rawMember.Topology.some(
            (rule) =>
              !isRecord(rule) || Object.values(rule).some((item) => typeof item !== 'string')
          )))
    ) {
      return null
    }
    network[addr] = {
      ...rawMember,
      Ping: Array.isArray(rawMember.Ping) ? rawMember.Ping : [],
      Topology: Array.isArray(rawMember.Topology) ? rawMember.Topology : []
    }
  }

  const chinaMap: Record<string, unknown> = {}
  if (isRecord(value.Chinamap)) {
    for (const [province, rawProviders] of Object.entries(value.Chinamap)) {
      if (!isRecord(rawProviders)) {
        return null
      }
      for (const provider of ['ctcc', 'cucc', 'cmcc']) {
        const addresses = rawProviders[provider]
        if (
          addresses !== undefined &&
          (!Array.isArray(addresses) || addresses.some((address) => typeof address !== 'string'))
        ) {
          return null
        }
      }
      chinaMap[province] = {
        ...rawProviders,
        ctcc: Array.isArray(rawProviders.ctcc) ? rawProviders.ctcc : [],
        cucc: Array.isArray(rawProviders.cucc) ? rawProviders.cucc : [],
        cmcc: Array.isArray(rawProviders.cmcc) ? rawProviders.cmcc : []
      }
    }
  }

  return {
    Name: value.Name,
    Addr: value.Addr,
    Base: { ...value.Base },
    Topology: { ...value.Topology },
    Mode: isRecord(value.Mode) ? value.Mode : {},
    Network: network,
    Chinamap: chinaMap,
    Toollimit: typeof value.Toollimit === 'number' ? value.Toollimit : 0,
    Authiplist: typeof value.Authiplist === 'string' ? value.Authiplist : ''
  }
}

const handleSave = async () => {
  if (saving.value || importExportBusy.value) {
    return
  }
  validationAttempted.value = true
  const validationIssue = validateConfigForEdit(formConfig)
  if (validationIssue) {
    void focusValidationIssue(validationIssue)
    return
  }
  if (!password.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  const controller = new AbortController()
  saveAbortController = controller
  saving.value = true
  const submittedConfig = JSON.parse(JSON.stringify(formConfig)) as Config
  const submittedSnapshot = serializeConfig(submittedConfig)
  try {
    await configStore.saveConfig(submittedConfig, password.value, controller.signal)
    if (!isUnmounted) {
      savedSnapshot.value = submittedSnapshot
      validationAttempted.value = false
      ElMessage.success(t('common.saveSuccess'))
    }
  } catch (error: unknown) {
    if (!isUnmounted && !isRequestCanceled(error)) {
      ElMessage.error(t('common.saveFailed') + ': ' + (error instanceof Error ? error.message : ''))
    }
    if (!isRequestCanceled(error)) {
      console.error('保存失败', error)
    }
  } finally {
    if (saveAbortController === controller) {
      saveAbortController = null
    }
    if (!isUnmounted) {
      password.value = ''
      saving.value = false
    }
  }
}

type PasswordVerificationResult = 'valid' | 'invalid' | 'rate-limited'

const verifyPassword = async (
  pwd: string,
  signal: AbortSignal
): Promise<PasswordVerificationResult> => {
  const response = await fetch('/api/verify-password.json', {
    method: 'POST',
    headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
    body: new URLSearchParams({ password: pwd }),
    signal
  })
  if (response.status === 429) {
    return 'rate-limited'
  }
  if (!response.ok) {
    throw new Error(`HTTP ${response.status}`)
  }
  const result: unknown = await response.json()
  return isRecord(result) && result.status === 'true' ? 'valid' : 'invalid'
}

const handleExport = async () => {
  if (importExportBusy.value || saving.value) {
    return
  }
  if (!importExportPassword.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  const controller = new AbortController()
  passwordVerificationAbortController = controller
  exporting.value = true
  try {
    const verification = await verifyPassword(importExportPassword.value, controller.signal)
    if (isUnmounted) {
      return
    }
    if (verification === 'rate-limited') {
      ElMessage.error(t('common.tooManyRequests'))
      return
    }
    if (verification !== 'valid') {
      ElMessage.error(t('common.passwordError'))
      return
    }

    const exportConfig = JSON.parse(JSON.stringify(formConfig)) as Record<string, unknown>
    delete exportConfig.Password
    delete exportConfig.Ver

    const blob = new Blob([JSON.stringify(exportConfig, null, 2)], { type: 'application/json' })
    const url = URL.createObjectURL(blob)
    const link = document.createElement('a')
    link.href = url
    link.download = `smartping-config-${new Date().toISOString().slice(0, 10)}.json`
    link.click()
    URL.revokeObjectURL(url)

    ElMessage.success(t('config.configExported'))
  } catch (error) {
    if (!isUnmounted && !isRequestCanceled(error)) {
      ElMessage.error(t('config.passwordVerifyFailed'))
    }
    if (!isRequestCanceled(error)) {
      console.error('密码验证失败', error)
    }
  } finally {
    if (passwordVerificationAbortController === controller) {
      passwordVerificationAbortController = null
    }
    if (!isUnmounted) {
      importExportPassword.value = ''
      exporting.value = false
    }
  }
}

const handleImportFile = async (file: UploadFile) => {
  if (importExportBusy.value || saving.value) {
    return
  }
  if (!file.raw) {
    ElMessage.error(t('config.configInvalid'))
    return
  }
  const rawFile = file.raw
  if (!importExportPassword.value) {
    ElMessage.warning(t('common.pleaseEnterPassword'))
    return
  }

  const controller = new AbortController()
  passwordVerificationAbortController = controller
  importing.value = true
  try {
    let verification: PasswordVerificationResult
    try {
      verification = await verifyPassword(importExportPassword.value, controller.signal)
    } catch (error) {
      if (!isUnmounted && !isRequestCanceled(error)) {
        ElMessage.error(t('config.passwordVerifyFailed'))
      }
      if (!isRequestCanceled(error)) {
        console.error('密码验证失败', error)
      }
      return
    }

    if (isUnmounted) {
      return
    }
    if (verification === 'rate-limited') {
      ElMessage.error(t('common.tooManyRequests'))
      return
    }
    if (verification !== 'valid') {
      ElMessage.error(t('common.passwordError'))
      return
    }

    try {
      const importedConfig = normalizeImportedConfig(JSON.parse(await rawFile.text()))
      if (isUnmounted) {
        return
      }

      if (!importedConfig) {
        ElMessage.error(t('config.configInvalid'))
        return
      }

      const currentName = formConfig.Name
      const currentAddr = formConfig.Addr
      const currentPort = formConfig.Port

      const candidate = {
        ...JSON.parse(JSON.stringify(formConfig)),
        ...importedConfig,
        Name: currentName,
        Addr: currentAddr,
        Port: currentPort
      } as Config
      const validationIssue = validateConfigForEdit(candidate)
      if (validationIssue) {
        showValidationIssue(validationIssue)
        return
      }

      Object.assign(formConfig, candidate)
      validationAttempted.value = false

      ElMessage.success(t('config.configImported'))
    } catch (error) {
      ElMessage.error(t('config.configParseFailed'))
      console.error('配置文件解析失败', error)
    }
  } finally {
    if (passwordVerificationAbortController === controller) {
      passwordVerificationAbortController = null
    }
    if (!isUnmounted) {
      importExportPassword.value = ''
      importing.value = false
    }
  }
}

const showAddNode = () => {
  newNodeName.value = ''
  newNodeAddr.value = ''
  addNodeVisible.value = true
}

const addNode = () => {
  if (!newNodeName.value.trim() || !newNodeAddr.value.trim()) {
    ElMessage.warning(t('config.pleaseEnterNodeAndIP'))
    return
  }

  if (!isValidIPv4(newNodeAddr.value.trim())) {
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

const deleteNode = async (row: NetworkListItem) => {
  try {
    await ElMessageBox.confirm(
      t('config.deleteNodeConfirm', { name: displayName(row.Name) }),
      t('config.deleteNode'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
  } catch {
    return
  }

  const addr = row.Addr
  delete formConfig.Network[addr]

  for (const [, member] of Object.entries(formConfig.Network)) {
    const pingIndex = member.Ping.indexOf(addr)
    if (pingIndex !== -1) {
      member.Ping.splice(pingIndex, 1)
    }
    member.Topology = member.Topology.filter((topology) => topology.Addr !== addr)
  }
  ElMessage.success(t('config.nodeDeleted'))
}

const showEditNode = (row: NetworkListItem) => {
  editNodeOriginalAddr.value = row.Addr
  editNodeName.value = row.Name
  editNodeAddr.value = row.Addr
  editNodeVisible.value = true
}

const saveEditNode = () => {
  if (!editNodeName.value.trim() || !editNodeAddr.value.trim()) {
    ElMessage.warning(t('config.pleaseEnterNodeAndIP'))
    return
  }

  if (!isValidIPv4(editNodeAddr.value.trim())) {
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
      const checkSeconds = ruleNumber(current?.Thdchecksec, 900, 60, 86400)
      return {
        Name: network.Name,
        Addr: addr,
        enabled: !!current,
        checkSeconds,
        occurrenceCount: Math.min(
          ruleNumber(current?.Thdoccnum, 3, 1, 10000),
          maxOccurrenceCount(checkSeconds)
        ),
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
      Thdoccnum: String(Math.min(item.occurrenceCount, maxOccurrenceCount(item.checkSeconds))),
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
  chinaMapValidationIssue.value = null

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

  const providers = {
    ctcc: parseIps(chinaMapIps.ctcc),
    cucc: parseIps(chinaMapIps.cucc),
    cmcc: parseIps(chinaMapIps.cmcc)
  }
  for (const [provider, addresses] of Object.entries(providers) as Array<
    [ChinaMapProviderKey, string[]]
  >) {
    const invalidAddress = addresses.find((address) => !isValidIPv4(address))
    if (invalidAddress) {
      chinaMapValidationIssue.value = { provider, address: invalidAddress }
      chinaMapTab.value = provider
      return
    }
  }

  const mappingTargetCount = Object.entries(formConfig.Chinamap).reduce(
    (count, [province, currentProviders]) =>
      count +
      Object.values(province === currentProvince.value ? providers : currentProviders).reduce(
        (providerCount, addresses) => providerCount + addresses.length,
        0
      ),
    0
  )
  if (mappingTargetCount > CONFIG_LIMITS.mappingTargets) {
    showValidationIssue({
      key: 'config.validationMappingLimit',
      params: { max: CONFIG_LIMITS.mappingTargets }
    })
    return
  }

  formConfig.Chinamap[currentProvince.value] = providers
  chinaMapValidationIssue.value = null

  chinaMapVisible.value = false
  ElMessage.success(t('config.delayConfigUpdated'))
}

const deleteChinaMap = async () => {
  if (!currentProvince.value) {
    return
  }

  try {
    await ElMessageBox.confirm(
      t('config.deleteProvinceConfirm', { province: currentProvince.value }),
      t('config.deleteProvince'),
      {
        confirmButtonText: t('common.confirm'),
        cancelButtonText: t('common.cancel'),
        type: 'warning'
      }
    )
  } catch {
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

const confirmDiscardChanges = async (): Promise<boolean> => {
  if (!isDirty.value) {
    return true
  }

  try {
    await ElMessageBox.confirm(t('config.unsavedChangesMessage'), t('config.unsavedChanges'), {
      confirmButtonText: t('config.discardChanges'),
      cancelButtonText: t('common.cancel'),
      type: 'warning'
    })
    return true
  } catch {
    return false
  }
}

const handleBeforeUnload = (event: BeforeUnloadEvent) => {
  if (!isDirty.value) {
    return
  }
  event.preventDefault()
  event.returnValue = ''
}

onBeforeRouteLeave(async () => {
  if (saving.value || importExportBusy.value) {
    return false
  }
  return confirmDiscardChanges()
})

onMounted(() => {
  window.addEventListener('beforeunload', handleBeforeUnload)
  loadConfig()
})

onUnmounted(() => {
  isUnmounted = true
  configRequestId++
  saveAbortController?.abort()
  passwordVerificationAbortController?.abort()
  window.removeEventListener('beforeunload', handleBeforeUnload)
})
</script>

<style scoped lang="scss">
.config-view__grid {
  display: grid;
  grid-template-columns: minmax(340px, 0.9fr) minmax(0, 1.5fr);
  gap: 20px;
}

.config-view__load-error {
  min-height: 320px;
}

.config-view__load-error-icon {
  font-size: 30px;
}

.config-view__save-meta,
.config-view__save-state {
  display: inline-flex;
  align-items: center;
}

.config-view__validation {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  margin-top: 12px;
  padding: 10px 12px;
  border-left: 3px solid var(--color-danger);
  background: color-mix(in srgb, var(--color-danger) 8%, transparent);
  color: var(--color-danger);
  font-size: 13px;
  line-height: 1.5;

  .el-icon {
    flex: 0 0 auto;
    margin-top: 2px;
  }
}

.config-view__save-meta {
  gap: 14px;
}

.config-view__save-state {
  gap: 6px;
  color: var(--color-success);
  font-size: 12px;
  font-weight: 600;
  white-space: nowrap;

  &.is-dirty {
    color: var(--color-warning);
  }
}

.config-view__rail,
.config-view__main {
  min-width: 0;
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

.config-view__ip-editor {
  margin-bottom: 0;
}

.config-view__number-input {
  width: 100%;
}

@media (max-width: 1320px) {
  .config-view__grid {
    grid-template-columns: minmax(0, 1fr);
  }
}
</style>
