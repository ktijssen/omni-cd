<template>
  <button
    ref="buttonEl"
    type="button"
    class="repo-form-input ss-button"
    aria-haspopup="listbox"
    :aria-expanded="open"
    :aria-label="label"
    @click="toggle"
    @keydown="onKeydown"
  >
    <span>{{ modelValue }}</span>
    <svg class="ss-caret" :class="{ up: open }" width="10" height="10" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true"><path fill-rule="evenodd" clip-rule="evenodd" d="M3.14645 5.14645C3.34171 4.95118 3.65829 4.95118 3.85355 5.14645L8 9.29289L12.1464 5.14645C12.3417 4.95118 12.6583 4.95118 12.8536 5.14645C13.0488 5.34171 13.0488 5.65829 12.8536 5.85355L8.35355 10.3536C8.15829 10.5488 7.84171 10.5488 7.64645 10.3536L3.14645 5.85355C2.95118 5.65829 2.95118 5.34171 3.14645 5.14645Z"/></svg>
  </button>

  <!-- Teleported so the list is not clipped by the modal's scroll area. -->
  <Teleport to="body">
    <ul
      v-if="open"
      ref="listEl"
      class="ss-list"
      role="listbox"
      :aria-label="label"
      :style="listStyle"
      @mousedown.prevent
    >
      <li
        v-for="(opt, i) in options"
        :key="opt"
        role="option"
        class="ss-option"
        :class="{ selected: opt === modelValue, active: i === activeIndex }"
        :aria-selected="opt === modelValue"
        @mouseenter="activeIndex = i"
        @click="choose(opt)"
      >{{ opt }}</li>
    </ul>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, onBeforeUnmount, ref } from 'vue'

// A compact dropdown whose list shows VISIBLE_ROWS entries and scrolls. The
// native <select> popup height is decided by the browser and cannot be styled.
const VISIBLE_ROWS = 6
const ROW_HEIGHT = 28

const props = withDefaults(defineProps<{ modelValue: string; options: string[]; label?: string }>(), {
  label: 'Select',
})
const emit = defineEmits<{ 'update:modelValue': [value: string] }>()

const open = ref(false)
const activeIndex = ref(-1)
const buttonEl = ref<HTMLButtonElement | null>(null)
const listEl = ref<HTMLUListElement | null>(null)
const pos = ref({ top: 0, left: 0, width: 0 })

const listStyle = computed(() => ({
  top: `${pos.value.top}px`,
  left: `${pos.value.left}px`,
  minWidth: `${pos.value.width}px`,
  maxHeight: `${VISIBLE_ROWS * ROW_HEIGHT + 2}px`, // + 2 for the 1px borders
  '--ss-row': `${ROW_HEIGHT}px`,
}))

function place() {
  const r = buttonEl.value?.getBoundingClientRect()
  if (!r) return
  const listHeight = Math.min(props.options.length, VISIBLE_ROWS) * ROW_HEIGHT + 2 // + borders
  // Open upwards when there is not enough room below the button.
  const below = window.innerHeight - r.bottom
  const top = below < listHeight + 8 && r.top > below ? r.top - listHeight - 4 : r.bottom + 4
  pos.value = { top, left: r.left, width: r.width }
}

function scrollActiveIntoView(center: boolean) {
  const list = listEl.value
  if (!list || activeIndex.value < 0) return
  const rowTop = activeIndex.value * ROW_HEIGHT
  if (center) {
    // Whole rows only, with the selected one third from the top, so no row is cut off.
    list.scrollTop = Math.max(0, activeIndex.value - Math.floor((VISIBLE_ROWS - 1) / 2)) * ROW_HEIGHT
  } else if (rowTop < list.scrollTop) {
    list.scrollTop = rowTop
  } else if (rowTop + ROW_HEIGHT > list.scrollTop + list.clientHeight) {
    list.scrollTop = rowTop + ROW_HEIGHT - list.clientHeight
  }
}

async function openList() {
  if (open.value) return
  place()
  activeIndex.value = Math.max(0, props.options.indexOf(props.modelValue))
  open.value = true
  document.addEventListener('mousedown', onOutside, true)
  window.addEventListener('resize', close)
  window.addEventListener('scroll', onScroll, true)
  await nextTick()
  scrollActiveIntoView(true)
}

function close() {
  if (!open.value) return
  open.value = false
  document.removeEventListener('mousedown', onOutside, true)
  window.removeEventListener('resize', close)
  window.removeEventListener('scroll', onScroll, true)
}

function toggle() {
  if (open.value) close()
  else openList()
}

function choose(opt: string) {
  emit('update:modelValue', opt)
  close()
  buttonEl.value?.focus()
}

function onOutside(e: MouseEvent) {
  const t = e.target as Node
  if (!buttonEl.value?.contains(t) && !listEl.value?.contains(t)) close()
}

// Close when the page or modal scrolls, but not when the list itself scrolls.
function onScroll(e: Event) {
  if (e.target !== listEl.value) close()
}

function move(to: number) {
  activeIndex.value = Math.min(props.options.length - 1, Math.max(0, to))
  scrollActiveIntoView(false)
}

function onKeydown(e: KeyboardEvent) {
  if (!open.value) {
    if (['ArrowDown', 'ArrowUp', 'Enter', ' '].includes(e.key)) {
      e.preventDefault()
      openList()
    }
    return
  }
  switch (e.key) {
    case 'ArrowDown': e.preventDefault(); move(activeIndex.value + 1); break
    case 'ArrowUp': e.preventDefault(); move(activeIndex.value - 1); break
    case 'PageDown': e.preventDefault(); move(activeIndex.value + VISIBLE_ROWS); break
    case 'PageUp': e.preventDefault(); move(activeIndex.value - VISIBLE_ROWS); break
    case 'Home': e.preventDefault(); move(0); break
    case 'End': e.preventDefault(); move(props.options.length - 1); break
    case 'Enter':
    case ' ':
      e.preventDefault()
      if (activeIndex.value >= 0) choose(props.options[activeIndex.value])
      break
    case 'Escape':
      e.preventDefault()
      e.stopPropagation()
      close()
      break
    case 'Tab':
      close()
      break
  }
}

onBeforeUnmount(close)
</script>

<style scoped>
.ss-button {
  display: inline-flex; align-items: center; justify-content: space-between; gap: 6px;
  width: 62px; padding: 7px 8px; cursor: pointer; text-align: left;
  font-family: inherit; font-variant-numeric: tabular-nums;
}
.ss-button:focus-visible { outline: none; border-color: var(--accent); }
.ss-caret { color: var(--text-faint); flex-shrink: 0; transition: transform 0.15s; }
.ss-caret.up { transform: rotate(180deg); }
</style>

<style>
/* Unscoped: the list is teleported to <body>. */
.ss-list {
  position: fixed; z-index: 3100; margin: 0; padding: 0; list-style: none; box-sizing: border-box;
  overflow-y: auto; background: var(--bg-inset); border: 1px solid var(--border); border-radius: 6px;
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.45); scrollbar-width: thin; scrollbar-color: var(--border) transparent;
}
.ss-option {
  height: var(--ss-row); line-height: var(--ss-row); padding: 0 12px;
  font-size: 13px; color: var(--text-2); cursor: pointer; font-variant-numeric: tabular-nums;
}
.ss-option.active { background: var(--surface-2); color: var(--text-strong); }
.ss-option.selected { color: var(--accent); font-weight: 600; }
</style>
