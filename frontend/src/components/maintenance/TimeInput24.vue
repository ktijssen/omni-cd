<template>
  <span class="t24" role="group" :aria-label="label">
    <ScrollSelect
      :model-value="hour"
      :options="HOURS"
      :label="`${label} hour`"
      @update:model-value="h => update(h, minute)"
    />
    <span class="t24-sep">:</span>
    <ScrollSelect
      :model-value="minute"
      :options="minutes"
      :label="`${label} minute`"
      @update:model-value="m => update(hour, m)"
    />
  </span>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import ScrollSelect from './ScrollSelect.vue'

// A 24-hour "HH:MM" picker. Native <input type="time"> follows the browser
// locale and shows AM/PM in e.g. en-US, so it is not used here.
const props = withDefaults(defineProps<{ modelValue: string; label?: string; step?: number }>(), {
  label: 'Time',
  step: 5,
})
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const pad = (n: number) => String(n).padStart(2, '0')
const HOURS = Array.from({ length: 24 }, (_, i) => pad(i))

const valid = computed(() => /^\d{2}:\d{2}$/.test(props.modelValue))
const hour = computed(() => (valid.value ? props.modelValue.slice(0, 2) : '00'))
const minute = computed(() => (valid.value ? props.modelValue.slice(3, 5) : '00'))

// Minutes on the step grid, plus the current value if it is off-grid (e.g. a
// window saved via the API as 22:07) so it is shown rather than silently changed.
const minutes = computed(() => {
  const list = Array.from({ length: Math.ceil(60 / props.step) }, (_, i) => pad(i * props.step))
  if (!list.includes(minute.value)) list.push(minute.value)
  return list.sort()
})

function update(h: string, m: string) {
  emit('update:modelValue', `${h}:${m}`)
}
</script>

<style scoped>
.t24 { display: inline-flex; align-items: center; gap: 4px; }
.t24-sep { color: var(--text-muted); }
</style>
