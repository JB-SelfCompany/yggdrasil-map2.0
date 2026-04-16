<template>
  <div class="about-page">
    <div class="card">
      <h2>Network Information</h2>
      <table class="info-table">
        <tr>
          <th>Nodes</th>
          <td>{{ store.nodeCount }}</td>
        </tr>
        <tr>
          <th>Edges</th>
          <td>{{ store.edgeCount }}</td>
        </tr>
        <tr>
          <th>Crawl interval</th>
          <td>{{ formatDuration(store.config?.crawler_interval) }}</td>
        </tr>
        <tr>
          <th>Map update interval</th>
          <td>{{ formatDuration(store.config?.crawler_interval) }} <span class="hint">(refreshes after each crawl)</span></td>
        </tr>
        <tr v-if="store.snapshot">
          <th>Last update</th>
          <td>{{ formatTime(store.snapshot.timestamp) }}</td>
        </tr>
      </table>

      <h2>License</h2>
      <table class="info-table">
        <tr>
          <th>License</th>
          <td><a class="mono" href="https://www.gnu.org/licenses/gpl-3.0.html" target="_blank" rel="noopener">GNU General Public License v3.0</a></td>
        </tr>
        <tr>
          <th>Source</th>
          <td><a class="mono" href="https://github.com/JB-SelfCompany/yggdrasil-map2.0" target="_blank" rel="noopener">github.com/JB-SelfCompany/yggmap</a></td>
        </tr>
      </table>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useGraphStore } from '../stores/graph'

const store = useGraphStore()

function formatDuration(s?: string): string {
  if (!s) return '...'
  return s
    .replace(/(\d+)h/, '$1 hr ')
    .replace(/(\d+)m/, '$1 min ')
    .replace(/(\d+)s/, '$1 sec')
    .trim()
}

function formatTime(iso: string): string {
  try {
    return new Date(iso).toLocaleString()
  } catch {
    return iso
  }
}
</script>

<style scoped>
.about-page {
  width: 100%;
  height: 100%;
  overflow-y: auto;
  background: #F5F5F5;
  padding: 24px 16px;
  box-sizing: border-box;
}

.card {
  max-width: 640px;
  margin: 0 auto;
  background: #FFFFFF;
  border: 1px solid #E5E7EB;
  border-radius: 8px;
  padding: 24px 28px;
}

h2 {
  margin: 0 0 16px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 15px;
  font-weight: 600;
  color: #29BBFF;
  letter-spacing: 0.04em;
  text-transform: uppercase;
}

h2 + h2,
table + h2 {
  margin-top: 28px;
}

.info-table {
  width: 100%;
  border-collapse: collapse;
  font-size: 13px;
  color: #333333;
}

.info-table tr {
  border-bottom: 1px solid #E5E7EB;
}

.info-table tr:last-child {
  border-bottom: none;
}

.info-table th {
  width: 38%;
  padding: 8px 4px 8px 0;
  font-weight: 500;
  color: #888888;
  text-align: left;
  white-space: nowrap;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
}

.info-table td {
  padding: 8px 4px;
  color: #333333;
  word-break: break-all;
}

.mono {
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 12px;
  color: #29BBFF;
}

.hint {
  color: #999999;
  font-size: 11px;
}

</style>
