<template>
  <div class="stats-bar">
    <span class="stats-text">
      {{ store.nodeCount }} nodes / {{ store.edgeCount }} edges
      <template v-if="updatedAgo">/ Updated {{ updatedAgo }}</template>
      <template v-if="crawlLabel">/ Crawl: {{ crawlLabel }}</template>
    </span>
    <span class="ws-dot" :class="wsClass" :title="store.wsStatus"></span>
  </div>
</template>

<script setup lang="ts">
import { computed, ref, onMounted, onUnmounted } from 'vue'
import { useGraphStore } from '../stores/graph'

const store = useGraphStore()

// Tick every second to update "X ago" display
const now = ref(Date.now())
let timer: ReturnType<typeof setInterval> | null = null

onMounted(() => {
  timer = setInterval(() => { now.value = Date.now() }, 1000)
})

onUnmounted(() => {
  if (timer !== null) clearInterval(timer)
})

const wsClass = computed(() => ({
  'ws-dot--open': store.wsStatus === 'OPEN',
  'ws-dot--connecting': store.wsStatus === 'CONNECTING',
  'ws-dot--closed': store.wsStatus === 'CLOSED' || store.wsStatus === 'ERROR',
}))

function formatDuration(s?: string): string {
  if (!s) return '...'
  return s
    .replace(/(\d+)h/, '$1 hr ')
    .replace(/(\d+)m/, '$1 min ')
    .replace(/(\d+)s/, '$1 sec')
    .trim()
}

const crawlLabel = computed<string | null>(() => {
  if (!store.config) return null
  return formatDuration(store.config.crawler_interval)
})

const updatedAgo = computed<string | null>(() => {
  if (!store.snapshot?.timestamp) return null
  const ts = new Date(store.snapshot.timestamp).getTime()
  if (isNaN(ts)) return null
  const seconds = Math.floor((now.value - ts) / 1000)
  if (seconds < 5) return 'just now'
  if (seconds < 60) return `${seconds}s ago`
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes}m ago`
  return `${Math.floor(minutes / 60)}h ago`
})
</script>

<style scoped>
.stats-bar {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 10px;
  font-size: 10px;
  line-height: 1.5;
  color: var(--overlay-text);
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  background: var(--panel-bg);
  border-top: 1px solid var(--panel-border);
  border-radius: 0 4px 0 0;
}

.stats-text {
  white-space: nowrap;
}

/* WS status dot only */
.ws-dot {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  flex-shrink: 0;
  background: #999;
}

.ws-dot--open {
  background: #3fb950;
}

.ws-dot--connecting {
  background: #f5a623;
  animation: pulse 1.2s ease-in-out infinite;
}

.ws-dot--closed {
  background: #f85149;
}

@keyframes pulse {
  0%, 100% { opacity: 1; }
  50%       { opacity: 0.35; }
}

@media (max-width: 768px) {
  .stats-bar {
    padding: 4px 8px;
    gap: 4px;
  }
}
</style>
