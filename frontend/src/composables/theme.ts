import { computed, ref } from 'vue'

/** The user's choice; "system" follows the OS light/dark setting. */
export type ThemeMode = 'system' | 'light' | 'dark'

const STORAGE_KEY = 'omnicd-theme'

function readMode(): ThemeMode {
  try {
    const v = localStorage.getItem(STORAGE_KEY)
    if (v === 'light' || v === 'dark' || v === 'system') return v
  } catch { /* storage blocked */ }
  return 'system'
}

const lightQuery = typeof window !== 'undefined' && window.matchMedia
  ? window.matchMedia('(prefers-color-scheme: light)')
  : undefined

const mode = ref<ThemeMode>(readMode())
const systemLight = ref(lightQuery?.matches ?? false)
lightQuery?.addEventListener('change', e => { systemLight.value = e.matches; apply() })

/** The theme in effect: "light" or "dark". */
const resolved = computed<'light' | 'dark'>(() =>
  mode.value === 'system' ? (systemLight.value ? 'light' : 'dark') : mode.value)

function apply() {
  document.documentElement.setAttribute('data-theme', resolved.value)
}

function setMode(m: ThemeMode) {
  mode.value = m
  try { localStorage.setItem(STORAGE_KEY, m) } catch { /* storage blocked */ }
  apply()
}

apply()

/** Colour theme state shared by the whole app. */
export function useTheme() {
  return { mode, resolved, setMode }
}
