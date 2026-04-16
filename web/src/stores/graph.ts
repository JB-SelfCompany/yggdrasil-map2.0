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

  // ── WebSocket — initialized once at store creation, lives for app lifetime ──
  // Must NOT be inside connect() / onMounted: VueUse's useWebSocket registers
  // tryOnScopeDispose cleanup, which would close the socket whenever MapPage
  // unmounts, causing "Waiting" on every return to the Network tab.
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

  watch(
    status,
    (s) => {
      if (s === 'OPEN' || s === 'CONNECTING' || s === 'CLOSED') {
        wsStatus.value = s
      }
    },
    { immediate: true }
  )

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
    // Fetch config and snapshot via REST on first visit; both are no-ops if
    // data is already present (store is a singleton).
    if (!config.value) {
      fetchConfig()
        .then((cfg) => { config.value = cfg })
        .catch((err) => { console.warn('yggmap: fetchConfig failed:', err) })
    }
    if (!snapshot.value) {
      fetchGraph()
        .then((d) => { if (!snapshot.value) snapshot.value = d })
        .catch(() => { /* no data yet, WS will provide */ })
    }
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
