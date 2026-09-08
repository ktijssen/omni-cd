<template>
  <div class="diff-viewer">
    <div
      v-for="(line, i) in lines"
      :key="i"
      class="diff-line"
      :class="lineClass(line)"
    >{{ line }}</div>
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
  if (line.startsWith('#')) return 'diff-comment'
  return ''
}
</script>
