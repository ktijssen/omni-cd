<script lang="ts">
import * as monaco from 'monaco-editor'

window.MonacoEnvironment = {
  getWorker() {
    return new Worker(new URL('./editor.worker.ts', import.meta.url), { type: 'module' })
  },
}

const OMNICD_THEME = 'omnicd-dark'

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
</script>

<script setup lang="ts">
import { onWatcherCleanup, useTemplateRef, watch } from 'vue'

const props = defineProps<{
  modelValue: string
  language?: string
  readOnly?: boolean
}>()

const editorEl = useTemplateRef<HTMLDivElement>('editorEl')

let instance: monaco.editor.IStandaloneCodeEditor | undefined

watch(
  editorEl,
  (el) => {
    if (!el) return

    const model = monaco.editor.createModel(props.modelValue, props.language ?? 'yaml')

    const created = monaco.editor.create(el, {
      model,
      theme: OMNICD_THEME,
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
