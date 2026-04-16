<template>
  <div v-if="store.crawlRunning" class="crawl-status">
    <span class="spinner"></span>
    <span>Crawling network...</span>
    <span v-if="store.crawlProgress > 0" class="crawl-count">{{ store.crawlProgress }} visited</span>
  </div>
</template>

<script setup lang="ts">
import { useGraphStore } from '../stores/graph'

const store = useGraphStore()
</script>

<style scoped>
.crawl-status {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 5px 10px;
  background: var(--panel-bg);
  border-top: 1px solid var(--panel-border);
  font-size: 10px;
  font-family: 'Ubuntu Mono', 'Consolas', monospace;
  color: var(--overlay-text);
}

.crawl-count {
  color: var(--input-placeholder);
}

.spinner {
  display: inline-block;
  width: 10px;
  height: 10px;
  border: 2px solid rgba(41, 187, 255, 0.25);
  border-top-color: #29BBFF;
  border-radius: 50%;
  animation: spin 0.7s linear infinite;
  flex-shrink: 0;
}

@keyframes spin {
  to { transform: rotate(360deg); }
}

@media (max-width: 768px) {
  .crawl-status {
    padding: 4px 8px;
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}
</style>
