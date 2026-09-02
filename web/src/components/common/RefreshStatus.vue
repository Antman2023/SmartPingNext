<template>
  <div class="refresh-status">
    <div class="refresh-status__copy">
      <span class="page-eyebrow">{{ $t('common.dataFreshness') }}</span>
      <strong aria-live="polite">
        {{
          lastUpdated ? $t('common.lastUpdatedAt', { time: lastUpdated }) : $t('common.notUpdated')
        }}
      </strong>
    </div>

    <el-tooltip :content="$t('common.refreshNow')" placement="bottom">
      <el-button
        circle
        :loading="loading"
        :aria-label="$t('common.refreshNow')"
        @click="$emit('refresh')"
      >
        <el-icon><RefreshRight /></el-icon>
      </el-button>
    </el-tooltip>
  </div>
</template>

<script setup lang="ts">
import { RefreshRight } from '@element-plus/icons-vue'
import { ElButton, ElIcon, ElTooltip } from 'element-plus'

defineProps<{
  loading: boolean
  lastUpdated: string
}>()

defineEmits<{
  refresh: []
}>()
</script>

<style scoped lang="scss">
.refresh-status {
  min-width: 190px;
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 18px;
}

.refresh-status__copy {
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 6px;

  strong {
    color: var(--color-text-primary);
    font-size: 12px;
    font-weight: 600;
    white-space: nowrap;
  }
}

:deep(.el-button) {
  width: 36px;
  height: 36px;
  min-height: 36px;
  flex-shrink: 0;
}

@media (max-width: 900px) {
  .refresh-status {
    width: 100%;
  }
}
</style>
