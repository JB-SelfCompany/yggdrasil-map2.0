<template>
  <transition name="slide">
    <aside v-if="store.selectedNode" class="node-panel">
      <button class="node-panel__close" @click="store.selectNode(null)">&#x2715;</button>

      <h2 class="node-panel__title">{{ store.selectedNode.name || store.selectedNode.address.slice(-16) }}</h2>

      <table class="node-panel__table">
        <tbody>
          <tr>
            <td class="field-label">Address</td>
            <td class="field-value mono">{{ store.selectedNode.address }}</td>
          </tr>
          <tr>
            <td class="field-label">Key</td>
            <td class="field-value mono truncate" :title="store.selectedNode.key">
              {{ store.selectedNode.key.slice(0, 16) }}...
            </td>
          </tr>
          <tr v-if="store.selectedNode.name">
            <td class="field-label">Name</td>
            <td class="field-value">{{ store.selectedNode.name }}</td>
          </tr>
          <tr v-if="store.selectedNode.os">
            <td class="field-label">OS</td>
            <td class="field-value">{{ store.selectedNode.os }}</td>
          </tr>
          <tr v-if="store.selectedNode.build_ver">
            <td class="field-label">Version</td>
            <td class="field-value">{{ store.selectedNode.build_ver }}</td>
          </tr>
          <tr>
            <td class="field-label">Peers</td>
            <td class="field-value">{{ peerDegree }}</td>
          </tr>
          <tr v-if="store.selectedNode.latency_ns > 0">
            <td class="field-label">Latency</td>
            <td class="field-value">{{ formatLatency(store.selectedNode.latency_ns) }}</td>
          </tr>
          <tr>
            <td class="field-label">Type</td>
            <td class="field-value">
              <span v-if="store.selectedNode.in_tree">Tree</span>
              <span v-else>Peer</span>
            </td>
          </tr>
          <tr>
            <td class="field-label">First seen</td>
            <td class="field-value">{{ formatDate(store.selectedNode.first_seen) }}</td>
          </tr>
          <tr>
            <td class="field-label">Last seen</td>
            <td class="field-value">{{ formatDate(store.selectedNode.last_seen) }}</td>
          </tr>
        </tbody>
      </table>
    </aside>
  </transition>
</template>

<script setup lang="ts">
import { computed } from 'vue'
import { useGraphStore } from '../stores/graph'

const store = useGraphStore()

const peerDegree = computed<number>(() => {
  if (!store.snapshot || !store.selectedKey) return 0
  const key = store.selectedKey
  let count = 0
  for (const edge of store.snapshot.edges) {
    if (edge.src_key === key || edge.dst_key === key) count++
  }
  return count
})

function formatLatency(ns: number): string {
  if (ns <= 0) return '—'
  if (ns < 1_000) return `${ns} ns`
  if (ns < 1_000_000) return `${(ns / 1_000).toFixed(1)} µs`
  if (ns < 1_000_000_000) return `${(ns / 1_000_000).toFixed(1)} ms`
  return `${(ns / 1_000_000_000).toFixed(2)} s`
}

function formatDate(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (isNaN(d.getTime())) return iso
  return d.toLocaleString()
}
</script>

<style scoped>
.node-panel {
  position: relative;
  background: var(--panel-bg);
  min-width: 250px;
  overflow-y: auto;
  padding: 10px 12px 12px;
}

.node-panel__close {
  position: absolute;
  top: 6px;
  right: 8px;
  background: none;
  border: none;
  color: #29BBFF;
  cursor: pointer;
  font-size: 0.875rem;
  padding: 2px 4px;
  line-height: 1;
  transition: color 0.15s;
}

.node-panel__close:hover {
  color: #0099DD;
}

.node-panel__title {
  margin: 0 0 8px 0;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 13px;
  font-weight: 400;
  color: #29BBFF;
  padding-right: 20px;
  word-break: break-all;
}

.node-panel__table {
  width: 100%;
  border-collapse: collapse;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 12px;
}

.node-panel__table tbody tr {
  border: none;
}

.field-label {
  color: #29BBFF;
  font-weight: 400;
  letter-spacing: 1px;
  text-transform: uppercase;
  font-size: 10px;
  white-space: nowrap;
  padding: 2px 8px 2px 0;
  vertical-align: top;
}

.field-value {
  color: var(--text);
  text-align: right;
  padding: 2px 0;
  word-break: break-all;
  vertical-align: top;
}

.mono {
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
}

.truncate {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 140px;
  display: block;
}

/* Slide-in from right */
.slide-enter-active,
.slide-leave-active {
  transition: transform 0.2s ease, opacity 0.2s ease;
}

.slide-enter-from,
.slide-leave-to {
  transform: translateX(100%);
  opacity: 0;
}

@media (max-width: 768px) {
  .node-panel {
    min-width: unset;
    max-width: 100%;
    padding: 8px 10px 10px;
  }
  .truncate {
    max-width: 100%;
  }
  /* IPv6 addresses wrap instead of overflow */
  .node-info td {
    word-break: break-all;
  }
  /* Slide from bottom on mobile */
  .slide-enter-from,
  .slide-leave-to {
    transform: translateY(100%);
  }
}
</style>
