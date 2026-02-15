<template>
  <div class="config-view">
    <div class="config-header">
      <h2>系统配置</h2>
    </div>

    <div class="config-content">
      <el-row :gutter="20">
        <!-- 左侧配置 -->
        <el-col :span="8">
          <el-card>
            <template #header>
              <div class="card-header">
                <span>保存配置</span>
                <el-link type="primary" href="/api/config.json" target="_blank">
                  <el-icon><Document /></el-icon>
                </el-link>
              </div>
            </template>
            <div class="save-config">
              <el-input v-model="password" type="password" placeholder="密码" style="width: 200px;" />
              <el-button type="primary" @click="handleSave">保存</el-button>
            </div>
          </el-card>

          <el-card class="mt-2">
            <template #header>
              <span>基础配置</span>
            </template>
            <el-form label-width="120px" label-position="top">
              <h4>基础</h4>
              <el-row :gutter="12">
                <el-col :span="8">
                  <el-form-item label="接口超时(秒)">
                    <el-input v-model.number="formConfig.Base.Timeout" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="页面刷新(分钟)">
                    <el-input v-model.number="formConfig.Base.Refresh" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="数据存档(天)">
                    <el-input v-model.number="formConfig.Base.Archive" />
                  </el-form-item>
                </el-col>
              </el-row>

              <h4>Ping拓扑</h4>
              <el-row :gutter="12">
                <el-col :span="8">
                  <el-form-item label="报警声音">
                    <el-input v-model="formConfig.Topology.Tsound" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="连线粗细">
                    <el-input v-model="formConfig.Topology.Tline" />
                  </el-form-item>
                </el-col>
                <el-col :span="8">
                  <el-form-item label="形状大小">
                    <el-input v-model="formConfig.Topology.Tsymbolsize" />
                  </el-form-item>
                </el-col>
              </el-row>

              <h4>报警邮件</h4>
              <el-form-item label="邮件服务器">
                <el-input v-model="formConfig.Alert.EmailHost" />
              </el-form-item>
              <el-row :gutter="12">
                <el-col :span="12">
                  <el-form-item label="发件邮箱">
                    <el-input v-model="formConfig.Alert.SendEmailAccount" />
                  </el-form-item>
                </el-col>
                <el-col :span="12">
                  <el-form-item label="发件邮箱密码">
                    <el-input v-model="formConfig.Alert.SendEmailPassword" type="password" />
                  </el-form-item>
                </el-col>
              </el-row>
              <el-form-item label="收件邮箱列表(分号隔开)">
                <el-input v-model="formConfig.Alert.RevcEmailList" />
              </el-form-item>

              <h4>检测工具</h4>
              <el-form-item label="限定频率(秒)">
                <el-input v-model.number="formConfig.Toollimit" />
              </el-form-item>

              <h4>授权管理</h4>
              <el-form-item label="用户IP列表(逗号隔开)">
                <el-input v-model="formConfig.Authiplist" />
              </el-form-item>
            </el-form>
          </el-card>
        </el-col>

        <!-- 右侧网络配置 -->
        <el-col :span="16">
          <el-card>
            <template #header>
              <span>Ping节点测试网络</span>
            </template>
            <el-table :data="networkList" stripe style="width: 100%">
              <el-table-column prop="Name" label="节点名称" min-width="120" />
              <el-table-column prop="Addr" label="节点IP" min-width="140" />
              <el-table-column label="SmartPing" width="100">
                <template #default="{ row }">
                  <el-checkbox v-model="row._original.Smartping" :disabled="row.isSelf" />
                </template>
              </el-table-column>
              <el-table-column label="操作" min-width="180">
                <template #default="{ row }">
                  <el-button-group>
                    <el-button size="small" :disabled="!row.isSelf && !row._original.Smartping" @click="editPingConfig(row)">Ping配置</el-button>
                    <el-button size="small" :disabled="!row.isSelf && !row._original.Smartping" @click="editTopoConfig(row)">拓扑配置</el-button>
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
              <el-button @click="showAddNode">添加节点</el-button>
            </div>
          </el-card>

          <el-card class="mt-2">
            <template #header>
              <span>全国延迟测试网络</span>
            </template>
            <div class="chinamap-list">
              <el-button
                v-for="(provData, prov) in formConfig.Chinamap"
                :key="prov"
                @click="editChinaMap(prov as string)"
              >
                {{ prov }} (电信{{ provData.ctcc?.length || 0 }}, 联通{{ provData.cucc?.length || 0 }}, 移动{{ provData.cmcc?.length || 0 }})
              </el-button>
              <el-button @click="showAddChinaMap">+</el-button>
            </div>
          </el-card>
        </el-col>
      </el-row>
    </div>

    <!-- 添加节点弹窗 -->
    <el-dialog v-model="addNodeVisible" title="新增节点" width="400px">
      <el-form label-width="80px">
        <el-form-item label="节点名称">
          <el-input v-model="newNodeName" placeholder="请输入节点名称" />
        </el-form-item>
        <el-form-item label="节点IP">
          <el-input v-model="newNodeAddr" placeholder="请输入IPv4地址，如 192.168.1.1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addNodeVisible = false">取消</el-button>
        <el-button type="primary" @click="addNode">暂存</el-button>
      </template>
    </el-dialog>

    <!-- Ping配置弹窗 -->
    <el-dialog v-model="pingConfigVisible" title="Ping配置" width="500px">
      <p class="dialog-tip">选择要从 <strong>{{ currentEditNode?.Name }}</strong> 发起 Ping 检测的目标节点：</p>
      <el-table :data="pingTargetList" stripe max-height="400">
        <el-table-column prop="Name" label="节点名称" width="150" />
        <el-table-column prop="Addr" label="节点IP" width="150" />
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.enabled" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="pingConfigVisible = false">取消</el-button>
        <el-button type="primary" @click="savePingConfig">确定</el-button>
      </template>
    </el-dialog>

    <!-- 拓扑配置弹窗 -->
    <el-dialog v-model="topoConfigVisible" title="拓扑配置" width="500px">
      <p class="dialog-tip">选择 <strong>{{ currentEditNode?.Name }}</strong> 在拓扑图中需要监控的目标节点：</p>
      <el-table :data="topoTargetList" stripe max-height="400">
        <el-table-column prop="Name" label="节点名称" width="150" />
        <el-table-column prop="Addr" label="节点IP" width="150" />
        <el-table-column label="启用" width="80" align="center">
          <template #default="{ row }">
            <el-checkbox v-model="row.enabled" />
          </template>
        </el-table-column>
      </el-table>
      <template #footer>
        <el-button @click="topoConfigVisible = false">取消</el-button>
        <el-button type="primary" @click="saveTopoConfig">确定</el-button>
      </template>
    </el-dialog>

    <!-- 全国延迟配置弹窗 -->
    <el-dialog v-model="chinaMapVisible" :title="currentProvince ? `${currentProvince} 延迟配置` : '全国延迟配置'" width="600px">
      <el-tabs v-model="chinaMapTab">
        <el-tab-pane label="电信(CTCC)" name="ctcc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.ctcc"
              type="textarea"
              :rows="6"
              placeholder="每行一个IP地址"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="联通(CUCC)" name="cucc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.cucc"
              type="textarea"
              :rows="6"
              placeholder="每行一个IP地址"
            />
          </div>
        </el-tab-pane>
        <el-tab-pane label="移动(CMCC)" name="cmcc">
          <div class="ip-list-editor">
            <el-input
              v-model="chinaMapIps.cmcc"
              type="textarea"
              :rows="6"
              placeholder="每行一个IP地址"
            />
          </div>
        </el-tab-pane>
      </el-tabs>
      <template #footer>
        <el-button @click="chinaMapVisible = false">取消</el-button>
        <el-button type="danger" @click="deleteChinaMap" v-if="currentProvince">删除省份</el-button>
        <el-button type="primary" @click="saveChinaMap">确定</el-button>
      </template>
    </el-dialog>

    <!-- 添加省份弹窗 -->
    <el-dialog v-model="addProvinceVisible" title="添加省份" width="400px">
      <el-form label-width="80px">
        <el-form-item label="省份名称">
          <el-input v-model="newProvinceName" placeholder="如：北京、上海、广东" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="addProvinceVisible = false">取消</el-button>
        <el-button type="primary" @click="addProvince">确定</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed } from 'vue'
import { Document, Delete } from '@element-plus/icons-vue'
import { ElMessage } from 'element-plus'
import { getConfig, saveConfig } from '@/api/topology'
import type { Config } from '@/types'

const config = ref<Config | null>(null)
const password = ref('')

const formConfig = reactive<Config>({
  Ver: '',
  Port: 18899,
  Name: '',
  Addr: '',
  Mode: {},
  Base: { Timeout: 3, Refresh: 5, Archive: 30 },
  Topology: { Tsound: '', Tline: '2', Tsymbolsize: '50' },
  Alert: { EmailHost: '', SendEmailAccount: '', SendEmailPassword: '', RevcEmailList: '' },
  Network: {},
  Chinamap: {},
  Toollimit: 0,
  Authiplist: ''
})

const networkList = computed(() => {
  if (!formConfig.Network) return []
  return Object.entries(formConfig.Network).map(([addr, network]) => ({
    // 保留原始对象的引用
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
    const cfg = await getConfig()
    config.value = cfg

    // 复制配置到表单
    Object.assign(formConfig, cfg)
  } catch (e) {
    console.error('加载配置失败', e)
  }
}

const handleSave = async () => {
  if (!password.value) {
    ElMessage.warning('请输入密码')
    return
  }

  try {
    const result = await saveConfig(formConfig, password.value)
    if (result.status === 'true') {
      ElMessage.success('保存成功')
    } else {
      ElMessage.error(result.info || '保存失败')
      console.error('保存失败:', result.info)
    }
  } catch (e: any) {
    ElMessage.error('保存失败: ' + (e.message || '未知错误'))
    console.error('保存失败', e)
  }
}

const showAddNode = () => {
  newNodeName.value = ''
  newNodeAddr.value = ''
  addNodeVisible.value = true
}

const addNode = () => {
  if (!newNodeName.value || !newNodeAddr.value) {
    ElMessage.warning('请填写节点名称和IP')
    return
  }

  // 验证 IP 格式
  const ipRegex = /^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$/
  if (!ipRegex.test(newNodeAddr.value.trim())) {
    ElMessage.warning('请输入有效的IPv4地址')
    return
  }

  const addr = newNodeAddr.value.trim()
  if (formConfig.Network[addr]) {
    ElMessage.warning('该IP节点已存在')
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
  ElMessage.success('节点已添加，请点击保存按钮保存配置')
}

const deleteNode = (row: any) => {
  delete formConfig.Network[row.Addr]
}

const editPingConfig = (row: any) => {
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
  ElMessage.success('Ping配置已更新')
}

const editTopoConfig = (row: any) => {
  currentEditNode.value = { Name: row.Name, Addr: row.Addr }

  // 获取当前节点的拓扑配置
  const currentTopoList = formConfig.Network[row.Addr]?.Topology?.map((t: any) => t.Addr) || []

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
  ElMessage.success('拓扑配置已更新')
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
  ElMessage.success('延迟配置已更新')
}

const deleteChinaMap = () => {
  if (!currentProvince.value) return

  delete formConfig.Chinamap[currentProvince.value]
  chinaMapVisible.value = false
  ElMessage.success('省份已删除')
}

const showAddChinaMap = () => {
  newProvinceName.value = ''
  addProvinceVisible.value = true
}

const addProvince = () => {
  if (!newProvinceName.value.trim()) {
    ElMessage.warning('请输入省份名称')
    return
  }

  const provName = newProvinceName.value.trim()

  if (formConfig.Chinamap[provName]) {
    ElMessage.warning('该省份已存在')
    return
  }

  formConfig.Chinamap[provName] = {
    ctcc: [],
    cucc: [],
    cmcc: []
  }

  addProvinceVisible.value = false
  ElMessage.success('省份已添加')
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
