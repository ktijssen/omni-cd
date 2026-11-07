<template>
  <div class="theme-toggle" role="radiogroup" aria-label="Colour theme">
    <button
      v-for="o in options"
      :key="o.mode"
      type="button"
      role="radio"
      class="theme-toggle-btn"
      :class="{ active: mode === o.mode }"
      :aria-checked="mode === o.mode"
      :title="o.label"
      :aria-label="o.label"
      @click="setMode(o.mode)"
      v-html="o.icon"
    ></button>
  </div>
</template>

<script setup lang="ts">
import { useTheme, type ThemeMode } from '@/composables/theme'

const { mode, setMode } = useTheme()

const sun = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M4.93 4.93l1.41 1.41M17.66 17.66l1.41 1.41M2 12h2M20 12h2M4.93 19.07l1.41-1.41M17.66 6.34l1.41-1.41"/></svg>`
const moon = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><path d="M21 12.79A9 9 0 1 1 11.21 3 7 7 0 0 0 21 12.79z"/></svg>`
const monitor = `<svg width="15" height="15" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="2" y="3" width="20" height="14" rx="2"/><path d="M8 21h8M12 17v4"/></svg>`

const options: { mode: ThemeMode; label: string; icon: string }[] = [
  { mode: 'light', label: 'Light', icon: sun },
  { mode: 'dark', label: 'Dark', icon: moon },
  { mode: 'system', label: 'System', icon: monitor },
]
</script>

<style scoped>
.theme-toggle {
  display: inline-flex; gap: 2px; padding: 2px;
  border: 1px solid var(--border); border-radius: 8px; background: var(--surface);
}
.theme-toggle-btn {
  display: flex; align-items: center; justify-content: center;
  width: 28px; height: 24px; border: none; border-radius: 6px;
  background: none; color: var(--text-faint); cursor: pointer;
  transition: background 0.15s, color 0.15s;
}
.theme-toggle-btn:hover { color: var(--text); }
.theme-toggle-btn.active { background: var(--surface-3); color: var(--accent); }
.theme-toggle-btn:focus-visible { outline: 2px solid var(--accent); outline-offset: 1px; }
</style>
