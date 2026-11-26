<template>
  <div class="target-selection-toolbar">
    <el-input
      v-model="query"
      class="target-selection-toolbar__search"
      :aria-label="$t('config.searchTargets')"
      :placeholder="$t('config.searchTargets')"
      :disabled="disabled"
      clearable
    />
    <div class="target-selection-toolbar__actions">
      <el-button size="small" :disabled="disabled || !hasMatches" @click="$emit('select', 'all')">
        {{ $t(query.trim() ? 'config.selectMatchingTargets' : 'config.selectAllTargets') }}
      </el-button>
      <el-button size="small" :disabled="disabled || !hasMatches" @click="$emit('select', 'invert')">
        {{ $t(query.trim() ? 'config.invertMatchingTargets' : 'config.invertTargetSelection') }}
      </el-button>
      <span class="target-selection-toolbar__count" role="status">
        {{ $t('config.selectedTargetCount', { count: selectedCount }) }}
      </span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ElButton, ElInput } from 'element-plus'

const query = defineModel<string>({ required: true })
defineProps<{ disabled: boolean; hasMatches: boolean; selectedCount: number }>()
defineEmits<{ select: [mode: 'all' | 'invert'] }>()
</script>

<style scoped lang="scss">
.target-selection-toolbar,
.target-selection-toolbar__actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.target-selection-toolbar {
  margin-bottom: 12px;
}

.target-selection-toolbar__search {
  flex: 1 1 180px;
  max-width: 280px;
}

.target-selection-toolbar__actions :deep(.el-button + .el-button) {
  margin-left: 0;
}

.target-selection-toolbar__count {
  color: var(--color-text-secondary);
  font-size: 12px;
}
</style>
