<template>
  <div class="map-page">
    <!-- Canvas fills the entire area -->
    <GraphCanvas />

    <!-- Loading overlay before data arrives -->
    <div v-if="statusLabel" class="canvas-overlay">
      <div class="canvas-overlay__content">
        <span v-if="store.wsStatus === 'CONNECTING'" class="spinner spinner--lg" />
        <p class="canvas-overlay__text">{{ statusLabel }}</p>
      </div>
    </div>

    <!-- Floating sidebar (top-right) -->
    <div class="map-sidebar">
      <SearchBox />
      <NodePanel />
      <CrawlStatus />
    </div>

    <!-- Stats + legend — bottom-left -->
    <div class="map-stats">
      <StatsBar />
      <MapLegend />
    </div>

  </div>
</template>

<script setup lang="ts">
import { onMounted, computed } from 'vue'
import { useGraphStore } from '../stores/graph'
import GraphCanvas from '../components/GraphCanvas.vue'
import NodePanel from '../components/NodePanel.vue'
import StatsBar from '../components/StatsBar.vue'
import SearchBox from '../components/SearchBox.vue'
import CrawlStatus from '../components/CrawlStatus.vue'
import MapLegend from '../components/MapLegend.vue'

const store = useGraphStore()

onMounted(() => {
  store.connect()
})

const statusLabel = computed<string | null>(() => {
  // Show map immediately as soon as we have data — regardless of WS state.
  // REST pre-fetch in connect() populates snapshot before WS handshake completes,
  // so we must not let the CONNECTING status gate the render.
  if (store.snapshot) return null
  if (store.wsStatus === 'CONNECTING') return 'Connecting...'
  if (store.wsStatus === 'CLOSED') return 'Disconnected'
  if (store.wsStatus === 'ERROR') return 'Connection error'
  return 'Waiting for data...'
})
</script>

<style scoped>
/* Full area, canvas as background */
.map-page {
  position: relative;
  width: 100%;
  height: 100%;
  overflow: hidden;
  background: var(--bg);
}

/* Loading overlay */
.canvas-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--overlay-bg);
  z-index: 10;
}

.canvas-overlay__content {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
}

.canvas-overlay__text {
  margin: 0;
  color: var(--overlay-text);
  font-size: 0.875rem;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
}

/* Floating sidebar — top-right */
.map-sidebar {
  position: absolute;
  top: 0;
  right: 0;
  min-width: 250px;
  max-width: 320px;
  z-index: 50;
  display: flex;
  flex-direction: column;
}

/* Stats panel — bottom-left */
.map-stats {
  position: absolute;
  bottom: 0;
  left: 0;
  z-index: 50;
}

/* Spinner for overlay */
.spinner {
  display: inline-block;
  width: 14px;
  height: 14px;
  border: 2px solid rgba(41, 187, 255, 0.25);
  border-top-color: #29BBFF;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}

.spinner--lg {
  width: 32px;
  height: 32px;
  border-width: 3px;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  /* Sidebar moves to bottom, full width */
  .map-sidebar {
    top: auto;
    bottom: 0;
    right: 0;
    left: 0;
    min-width: unset;
    max-width: 100%;
    max-height: 50vh;
    overflow-y: auto;
  }
  /* Stats move to top-left (below header) */
  .map-stats {
    bottom: auto;
    top: 0;
    left: 0;
  }
}

@media (max-width: 480px) {
  .map-sidebar {
    max-height: 55vh;
  }
}
</style>
