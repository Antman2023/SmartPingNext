<template>
  <div class="monitor-refresh-control">
    <div class="monitor-refresh-control__copy">
      <span class="page-eyebrow">{{ $t('common.autoRefresh') }}</span>
      <strong>{{ modelValue ? $t('common.enabled') : $t('common.paused') }}</strong>
      <span class="monitor-refresh-control__updated" aria-live="polite">
        {{
          lastUpdated ? $t('common.lastUpdatedAt', { time: lastUpdated }) : $t('common.notUpdated')
        }}
      </span>
    </div>

    <div class="monitor-refresh-control__actions">
      <el-tooltip :content="$t('common.refreshNow')" placement="bottom">
        <el-button
          circle
          :loading="refreshing"
          :aria-label="$t('common.refreshNow')"
          @click="$emit('refresh')"
        >
          <el-icon><RefreshRight /></el-icon>
        </el-button>
      </el-tooltip>
      <el-switch
        :model-value="modelValue"
        size="small"
        :aria-label="$t('common.autoRefresh')"
        @update:model-value="updateModelValue"
      />
    </div>
  </div>
</template>

<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue'
import { ElButton, ElIcon, ElSwitch, ElTooltip } from 'element-plus'

defineProps<{
  modelValue: boolean
  refreshing: boolean
  lastUpdated: string
}>()

const emit = defineEmits<{
  refresh: []
  'update:modelValue': [value: boolean]
}>()

const updateModelValue = (value: string | number | boolean) => {
  if (typeof value === 'boolean') {
    emit('update:modelValue', value)
  }
}
</script>

<style scoped lang="scss">
.monitor-refresh-control {
  min-width: 228px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.monitor-refresh-control__copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 5px;

  strong {
    font-size: 14px;
    color: var(--color-text-primary);
  }
}

.monitor-refresh-control__updated {
  color: var(--color-text-secondary);
  font-size: 11px;
  white-space: nowrap;
}

.monitor-refresh-control__actions {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-shrink: 0;

  :deep(.el-button) {
    width: 36px;
    height: 36px;
    min-height: 36px;
  }
}

@media (max-width: 900px) {
  .monitor-refresh-control {
    width: 100%;
  }
}
</style>
