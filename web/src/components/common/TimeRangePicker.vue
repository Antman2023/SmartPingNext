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
import { ref, watch } from 'vue'

const props = defineProps<{
  modelValue?: { start: string; end: string }
  format?: string
  valueFormat?: string
  startPlaceholder?: string
  endPlaceholder?: string
}>()

const emit = defineEmits<{
  (e: 'update:modelValue', value: { start: string; end: string }): void
}>()

const format = props.format || 'YYYY-MM-DD HH:mm'
const valueFormat = props.valueFormat || 'YYYY-MM-DD HH:mm'

const startTime = ref(props.modelValue?.start || '')
const endTime = ref(props.modelValue?.end || '')

const ranges = [
  { label: '1小时', hours: 1 },
  { label: '3小时', hours: 3 },
  { label: '6小时', hours: 6 },
  { label: '12小时', hours: 12 },
  { label: '1天', hours: 24 },
  { label: '3天', hours: 72 },
  { label: '7天', hours: 168 }
]

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
