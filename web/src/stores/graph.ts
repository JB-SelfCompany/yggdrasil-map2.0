import { ref, computed, watch } from 'vue'
import { defineStore } from 'pinia'
import { useWebSocket } from '@vueuse/core'
import { triggerCrawl as apiTriggerCrawl, fetchConfig, fetchGraph } from '../lib/api'
import type { GraphSnapshot, CrawlProgress, WSMessage, AppConfig } from '../lib/api'

// Resolve the WebSocket URL relative to the current page origin so the
// store works both with the Vite dev-proxy (/ws) and with production
// (same host as the page).
function resolveWsUrl(): string {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  const host = window.location.host
  return `${proto}//${host}/ws`
}

// Type guards for WebSocket message parsing
function isGraphSnapshot(data: unknown): data is GraphSnapshot {
  return (
    typeof data === 'object' &&
    data !== null &&
    'timestamp' in data &&
    'nodes' in data &&
    'edges' in data
  )
}

function isCrawlProgress(data: unknown): data is CrawlProgress {
  return (
    typeof data === 'object' &&
    data !== null &&
    'running' in data
  )
}

export const useGraphStore = defineStore('graph', () => {
  // ── State ──────────────────────────────────────────────────────────────────
  const snapshot = ref<GraphSnapshot | null>(null)
  const crawlRunning = ref(false)
  const crawlProgress = ref(0)
  const selectedKey = ref<string | null>(null)
  const wsStatus = ref<'CONNECTING' | 'OPEN' | 'CLOSED' | 'ERROR'>('CONNECTING')
  const config = ref<AppConfig | null>(null)

  // ── Getters ────────────────────────────────────────────────────────────────
  const nodeCount = computed(() =>
    snapshot.value ? Object.keys(snapshot.value.nodes).length : 0
  )

  const edgeCount = computed(() =>
    snapshot.value ? snapshot.value.edges.length : 0
  )

  const selectedNode = computed(() =>
    snapshot.value && selectedKey.value
      ? (snapshot.value.nodes[selectedKey.value] ?? null)
      : null
  )

  // ── Actions ────────────────────────────────────────────────────────────────
  function connect() {
    // Fetch app config once at startup; failure is non-fatal.
    fetchConfig()
      .then((cfg) => { config.value = cfg })
      .catch((err) => { console.warn('yggmap: fetchConfig failed:', err) })

    // Pre-populate snapshot from REST before WS delivers — eliminates "Waiting for data"
    // on page refresh when the backend already has data from SQLite or a previous crawl.
    fetchGraph()
      .then((data) => { if (!snapshot.value) snapshot.value = data })
      .catch(() => { /* no data yet, WS will provide */ })

    const { status, data } = useWebSocket(resolveWsUrl(), {
      autoReconnect: {
        retries: 30,
        delay: 3000,
        onFailed() {
          wsStatus.value = 'ERROR'
        }
      },
      onConnected() {
        wsStatus.value = 'OPEN'
      },
      onDisconnected() {
        wsStatus.value = 'CLOSED'
      },
      onError() {
        wsStatus.value = 'ERROR'
      }
    })

    // Mirror VueUse status string onto our typed ref
    // VueUse uses 'OPEN' | 'CONNECTING' | 'CLOSED'
    // We handle 'ERROR' ourselves via onError callback above
    watch(
      status,
      (s) => {
        if (s === 'OPEN' || s === 'CONNECTING' || s === 'CLOSED') {
          wsStatus.value = s
        }
      },
      { immediate: true }
    )

    // Process incoming WebSocket messages
    watch(data, (raw: string | null) => {
      if (!raw) return
      try {
        const msg = JSON.parse(raw) as WSMessage
        if (msg.type === 'snapshot') {
          if (isGraphSnapshot(msg.data)) {
            snapshot.value = msg.data
          }
        } else if (msg.type === 'crawl_progress') {
          if (isCrawlProgress(msg.data)) {
            crawlRunning.value = msg.data.running
            crawlProgress.value = msg.data.visited
          }
        }
      } catch {
        // Ignore malformed messages
      }
    })
  }

  function selectNode(key: string | null) {
    selectedKey.value = key
  }

  async function triggerCrawl() {
    try {
      await apiTriggerCrawl()
      crawlRunning.value = true
    } catch (err) {
      console.error('Failed to trigger crawl:', err)
    }
  }

  return {
    // state
    snapshot,
    crawlRunning,
    crawlProgress,
    selectedKey,
    wsStatus,
    config,
    // getters
    nodeCount,
    edgeCount,
    selectedNode,
    // actions
    connect,
    selectNode,
    triggerCrawl
  }
})
