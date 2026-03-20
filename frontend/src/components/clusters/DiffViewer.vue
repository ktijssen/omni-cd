<template>
  <div class="diff-viewer">
    <span
      v-for="(line, i) in lines"
      :key="i"
      :class="lineClass(line)"
    >{{ line }}<br /></span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue'

const props = defineProps<{
  diff: string
}>()

const lines = computed(() => props.diff.split('\n'))

function lineClass(line: string) {
  if (line.startsWith('+') && !line.startsWith('+++')) return 'diff-add'
  if (line.startsWith('-') && !line.startsWith('---')) return 'diff-del'
  if (line.startsWith('@@') || line.startsWith('---') || line.startsWith('+++')) return 'diff-hdr'
  return ''
}
</script>
