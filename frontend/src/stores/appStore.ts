import { defineStore } from 'pinia'
import { ref, shallowRef } from 'vue'
import type { SnapshotData } from '@/types'

export const useAppStore = defineStore('app', () => {
  const state = shallowRef<SnapshotData | null>(null)
  const wsConnected = ref(false)
  let ws: WebSocket | null = null
  let reconnectTimer: ReturnType<typeof setTimeout> | null = null
  let reconnectDelay = 3000

  function connect() {
    if (ws && (ws.readyState === WebSocket.OPEN || ws.readyState === WebSocket.CONNECTING)) {
      return
    }
    const protocol = location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${protocol}//${location.host}/ws`)

    ws.onopen = () => {
      wsConnected.value = true
      reconnectDelay = 3000
      if (reconnectTimer) {
        clearTimeout(reconnectTimer)
        reconnectTimer = null
      }
    }

    ws.onmessage = (event) => {
      try {
        state.value = JSON.parse(event.data) as SnapshotData
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      wsConnected.value = false
      scheduleReconnect()
    }

    ws.onerror = () => {
      ws?.close()
    }
  }

  function scheduleReconnect() {
    if (reconnectTimer) return
    reconnectTimer = setTimeout(() => {
      reconnectTimer = null
      connect()
    }, reconnectDelay)
    reconnectDelay = Math.min(reconnectDelay * 1.5, 10000)
  }

  function disconnect() {
    if (reconnectTimer) {
      clearTimeout(reconnectTimer)
      reconnectTimer = null
    }
    ws?.close()
    ws = null
  }

  async function fetchState() {
    try {
      const res = await fetch('/api/state')
      if (res.ok) {
        state.value = await res.json()
      }
    } catch {
      // ignore
    }
  }

  return { state, wsConnected, connect, disconnect, fetchState }
})
