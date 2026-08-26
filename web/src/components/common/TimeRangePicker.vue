<template>
  <div class="time-range-picker">
    <el-date-picker
      v-model="startTime"
      type="datetime"
      :placeholder="startPlaceholder"
      :format="format"
      :value-format="valueFormat"
    />
    <el-date-picker
      v-model="endTime"
      type="datetime"
      :placeholder="endPlaceholder"
      :format="format"
      :value-format="valueFormat"
    />
    <el-button-group>
      <el-button
        v-for="range in ranges"
        :key="range.hours"
        size="small"
        @click="setRange(range.hours)"
      >
        {{ range.label }}
      </el-button>
    </el-button-group>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue'
import { useI18n } from 'vue-i18n'
import { ElDatePicker } from 'element-plus'

const props = defineProps<{
  modelValue?: { start: string; end: string }
  format?: string
  valueFormat?: string
  startPlaceholder?: string
  endPlaceholder?: string
}>()

const emit = defineEmits<{
  'update:modelValue': [{ start: string; end: string }]
}>()

const { t } = useI18n()

const format = props.format || 'YYYY-MM-DD HH:mm'
const valueFormat = props.valueFormat || 'YYYY-MM-DD HH:mm'

const startTime = ref(props.modelValue?.start || '')
const endTime = ref(props.modelValue?.end || '')

const ranges = computed(() => [
  { label: t('dashboard.timeRanges.hour1'), hours: 1 },
  { label: t('dashboard.timeRanges.hour3'), hours: 3 },
  { label: t('dashboard.timeRanges.hour6'), hours: 6 },
  { label: t('dashboard.timeRanges.hour12'), hours: 12 },
  { label: t('dashboard.timeRanges.day1'), hours: 24 },
  { label: t('dashboard.timeRanges.day3'), hours: 72 },
  { label: t('dashboard.timeRanges.day7'), hours: 168 }
])

const setRange = (hours: number) => {
  const end = new Date()
  const start = new Date(end.getTime() - hours * 60 * 60 * 1000)

  const formatDate = (d: Date) => {
    const pad = (n: number) => (n < 10 ? '0' + n : n)
    return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}`
  }

  startTime.value = formatDate(start)
  endTime.value = formatDate(end)
}

watch([startTime, endTime], () => {
  emit('update:modelValue', {
    start: startTime.value,
    end: endTime.value
  })
})
</script>

<style scoped lang="scss">
.time-range-picker {
  display: flex;
  align-items: center;
  gap: 12px;
  flex-wrap: wrap;
}
</style>
