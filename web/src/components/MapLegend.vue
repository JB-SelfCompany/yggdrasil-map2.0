<template>
  <Transition name="legend-fade">
    <div v-if="!store.selectedKey" class="map-legend">
      <div class="map-legend__title">Node connectivity</div>
      <ul class="map-legend__list">
        <li v-for="item in LEGEND" :key="item.label" class="map-legend__item">
          <span class="map-legend__dot" :style="{ background: item.color }" />
          <span class="map-legend__label">{{ item.label }}</span>
          <span class="map-legend__range">{{ item.range }}</span>
        </li>
      </ul>
    </div>
  </Transition>
</template>

<script setup lang="ts">
import { useGraphStore } from '../stores/graph'

const store = useGraphStore()

const LEGEND = [
  { color: '#AAAAAA', label: 'Isolated',   range: '≤ 1 peer'  },
  { color: '#3A7BD5', label: 'Leaf',        range: '2–5'       },
  { color: '#0891B2', label: 'Ordinary',    range: '6–15'      },
  { color: '#059669', label: 'Connected',   range: '16–40'     },
  { color: '#D97706', label: 'Hub',         range: '41–100'    },
  { color: '#DC2626', label: 'Super-hub',   range: '100+'      },
]
</script>

<style scoped>
.map-legend {
  background: var(--panel-bg, rgba(255, 255, 255, 0.92));
  border: 1px solid var(--border, rgba(0, 0, 0, 0.10));
  border-radius: 8px;
  padding: 8px 10px;
  backdrop-filter: blur(6px);
  box-shadow: 0 2px 8px rgba(0, 0, 0, 0.10);
  min-width: 160px;
  pointer-events: none; /* non-interactive — doesn't block map clicks */
}

.map-legend__title {
  font-size: 0.65rem;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  text-transform: uppercase;
  letter-spacing: 0.06em;
  color: var(--text-muted, #888);
  margin-bottom: 6px;
}

.map-legend__list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.map-legend__item {
  display: flex;
  align-items: center;
  gap: 7px;
}

.map-legend__dot {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}

.map-legend__label {
  font-size: 0.72rem;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  color: var(--text, #333);
  flex: 1;
}

.map-legend__range {
  font-size: 0.65rem;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  color: var(--text-muted, #999);
  white-space: nowrap;
}

/* Fade in/out when a node is selected / deselected */
.legend-fade-enter-active,
.legend-fade-leave-active {
  transition: opacity 0.2s ease, transform 0.2s ease;
}
.legend-fade-enter-from,
.legend-fade-leave-to {
  opacity: 0;
  transform: translateY(4px);
}
</style>
