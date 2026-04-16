<template>
  <div class="search-box" @keydown.escape="clearQuery">
    <input
      ref="inputRef"
      v-model="query"
      type="text"
      placeholder="Search nodes..."
      class="search-input"
      autocomplete="off"
      spellcheck="false"
    />
    <ul v-if="results.length > 0 && query" class="results-list">
      <li
        v-for="node in results"
        :key="node.key"
        class="result-item"
        @click="selectNode(node.key)"
      >
        <span class="result-addr">{{ node.address }}</span>
        <span v-if="node.name" class="result-name">{{ node.name }}</span>
      </li>
    </ul>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useGraphStore } from '../stores/graph'
import type { GraphNode } from '../lib/api'

const store = useGraphStore()
const query = ref('')
const inputRef = ref<HTMLInputElement | null>(null)

const results = computed<GraphNode[]>(() => {
  if (!query.value || !store.snapshot) return []
  const q = query.value.toLowerCase()
  const matches: GraphNode[] = []
  for (const node of Object.values(store.snapshot.nodes)) {
    if (
      node.address.toLowerCase().includes(q) ||
      node.key.toLowerCase().startsWith(q) ||
      (node.name && node.name.toLowerCase().includes(q))
    ) {
      matches.push(node)
      if (matches.length >= 10) break
    }
  }
  return matches
})

function selectNode(key: string) {
  store.selectNode(key)
  query.value = ''
}

function clearQuery() {
  query.value = ''
}

function onKeyDown(e: KeyboardEvent) {
  if (e.key === '/' && document.activeElement !== inputRef.value) {
    e.preventDefault()
    inputRef.value?.focus()
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKeyDown)
})

onUnmounted(() => {
  document.removeEventListener('keydown', onKeyDown)
})
</script>

<style scoped>
.search-box {
  position: relative;
  width: 100%;
}

.search-input {
  width: 100%;
  padding: 5px 8px;
  background: var(--input-bg);
  border: none;
  border-bottom: 1px solid var(--panel-border);
  color: var(--input-text);
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  font-size: 10px;
  outline: none;
}

.search-input::placeholder {
  color: var(--input-placeholder);
}

.results-list {
  position: absolute;
  top: 100%;
  left: 0;
  right: 0;
  background: var(--results-bg);
  border: none;
  list-style: none;
  margin: 0;
  padding: 0;
  z-index: 200;
  max-height: 260px;
  overflow-y: auto;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.2);
}

.result-item {
  display: flex;
  align-items: baseline;
  gap: 0.5rem;
  padding: 4px 8px;
  cursor: pointer;
  transition: background 0.1s;
}

.result-item:hover {
  background: var(--results-hover);
}

.result-addr {
  color: var(--results-text);
  font-size: 10px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
}

.result-name {
  color: var(--input-placeholder);
  font-size: 10px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

@media (max-width: 768px) {
  .search-input {
    min-height: 44px;
    font-size: 12px;
    padding: 6px 10px;
  }
}

@media (max-width: 480px) {
  .results-list {
    max-height: 30vh;
  }
}
</style>
