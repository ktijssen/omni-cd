/// <reference types="vite/client" />

declare module '*.css' {
  const content: string
  export default content
}

declare module 'monaco-editor/esm/vs/editor/editor.worker'
