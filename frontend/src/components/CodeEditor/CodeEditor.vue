<script lang="ts">
import * as monaco from 'monaco-editor'

window.MonacoEnvironment = {
  getWorker() {
    return new Worker(new URL('./editor.worker.ts', import.meta.url), { type: 'module' })
  },
}

const OMNICD_THEME = 'omnicd-dark'
const OMNICD_THEME_LIGHT = 'omnicd-light'

monaco.editor.defineTheme(OMNICD_THEME, {
  base: 'vs-dark',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#101118',
    'editor.lineHighlightBackground': '#101118',
    'editorLineNumber.foreground': '#5b5c64',
    'editorLineNumber.activeForeground': '#9fa1a6',
    'editorGutter.background': '#101118',
    'editorIndentGuide.background1': '#2c2e38',
    'editorIndentGuide.activeBackground1': '#3a3d4a',
    'editorWidget.background': '#1f222e',
    'editorWidget.border': '#2c2e38',
    'editorHoverWidget.background': '#1f222e',
    'editorHoverWidget.border': '#2c2e38',
    'editorStickyScroll.background': '#101118',
    'scrollbarSlider.background': '#2c2e3899',
    'scrollbarSlider.hoverBackground': '#3a3d4a99',
  },
})

// Monaco needs literal colours; these match the light tokens in main.css.
monaco.editor.defineTheme(OMNICD_THEME_LIGHT, {
  base: 'vs',
  inherit: true,
  rules: [],
  colors: {
    'editor.background': '#f4f5f7',
    'editor.lineHighlightBackground': '#f4f5f7',
    'editorLineNumber.foreground': '#888b94',
    'editorLineNumber.activeForeground': '#4f525b',
    'editorGutter.background': '#f4f5f7',
    'editorIndentGuide.background1': '#dcdfe5',
    'editorIndentGuide.activeBackground1': '#c3c7d0',
    'editorWidget.background': '#ffffff',
    'editorWidget.border': '#dcdfe5',
    'editorHoverWidget.background': '#ffffff',
    'editorHoverWidget.border': '#dcdfe5',
    'editorStickyScroll.background': '#f4f5f7',
    'scrollbarSlider.background': '#c3c7d099',
    'scrollbarSlider.hoverBackground': '#888b9499',
  },
})
</script>

<script setup lang="ts">
import { onWatcherCleanup, useTemplateRef, watch } from 'vue'
import { useTheme } from '@/composables/theme'

const props = defineProps<{
  modelValue: string
  language?: string
  readOnly?: boolean
}>()

const editorEl = useTemplateRef<HTMLDivElement>('editorEl')

let instance: monaco.editor.IStandaloneCodeEditor | undefined

// Monaco's theme is global; follow the app's colour theme.
const { resolved } = useTheme()
const editorTheme = () => (resolved.value === 'light' ? OMNICD_THEME_LIGHT : OMNICD_THEME)
watch(resolved, () => monaco.editor.setTheme(editorTheme()))

watch(
  editorEl,
  (el) => {
    if (!el) return

    const model = monaco.editor.createModel(props.modelValue, props.language ?? 'yaml')

    const created = monaco.editor.create(el, {
      model,
      theme: editorTheme(),
      readOnly: props.readOnly ?? true,
      domReadOnly: props.readOnly ?? true,
      automaticLayout: true,
      fontSize: 13,
      fontFamily: "'SF Mono','Fira Code',monospace",
      lineNumbersMinChars: 3,
      lineDecorationsWidth: 5,
      minimap: { enabled: false },
      scrollBeyondLastLine: false,
      renderLineHighlight: 'none',
      contextmenu: false,
    })

    instance = created

    onWatcherCleanup(() => {
      created.dispose()
      model.dispose()
      instance = undefined
    })
  },
  { immediate: true },
)

watch(
  () => props.modelValue,
  (val) => {
    const model = instance?.getModel()
    if (model && model.getValue() !== val) model.setValue(val)
  },
)
</script>

<template>
  <div ref="editorEl" style="width:100%;height:100%;"></div>
</template>
