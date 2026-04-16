<template>
  <div class="graph-canvas-wrapper">
    <div ref="containerRef" class="graph-canvas"></div>
    <div v-if="layoutPending" class="layout-overlay">
      <span class="layout-label">Computing layout… {{ layoutNodeCount }} nodes</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, watch, onMounted, onUnmounted } from 'vue'
import Sigma from 'sigma'
import Graph from 'graphology'
import { useGraphStore } from '../stores/graph'
import { useThemeStore } from '../stores/theme'
import type { GraphSnapshot } from '../lib/api'
import {
  snapshotToGraph,
  applySnapshotDiff,
  applyDepthColors,
  DEFAULT_NODE_COLOR,
  getLodEdgeDisplayData,
  getThemeEdgeColor,
  getThemeLabelColor,
  getBaseNodeSize,
  LOD_CLOSE,
  LOD_MID,
} from '../lib/sigma'
import type { ComputeMessage, DoneMessage, ErrorMessage } from '../workers/layout.worker'

const POSITIONS_KEY = 'yggmap_positions'
const NODE_KEYS_KEY  = 'yggmap_node_keys'

const store = useGraphStore()
const themeStore = useThemeStore()
const containerRef = ref<HTMLElement | null>(null)

let sigmaInstance: Sigma | null = null
let graph: Graph | null = null
let worker: Worker | null = null

let hoveredNode: string | null = null
let currentCameraRatio = 1.0

const layoutPending = ref(false)
const layoutNodeCount = ref(0)

function getWorker(): Worker {
  if (!worker) {
    worker = new Worker(
      new URL('../workers/layout.worker.ts', import.meta.url),
      { type: 'module' }
    )
  }
  return worker
}

function handleResize() {
  sigmaInstance?.refresh()
}

onMounted(() => {
  window.addEventListener('resize', handleResize)
})

onUnmounted(() => {
  window.removeEventListener('resize', handleResize)
  sigmaInstance?.kill()
  sigmaInstance = null
  graph = null
  worker?.terminate()
  worker = null
})

watch(
  () => store.selectedKey,
  (selectedKey) => {
    if (!graph || !sigmaInstance) return
    applyDepthColors(graph, selectedKey, themeStore.isDark)
    sigmaInstance.refresh()
  }
)

// When theme toggles, update sigma edge color and node label colors
watch(
  () => themeStore.isDark,
  (isDark) => {
    if (!sigmaInstance || !graph) return
    sigmaInstance.setSetting('defaultEdgeColor', getThemeEdgeColor(isDark))
    const labelColor = getThemeLabelColor(isDark)
    graph.forEachNode((node) => {
      graph!.setNodeAttribute(node, 'labelColor', labelColor)
    })
    sigmaInstance.refresh()
  }
)

function savePositions(
  positions: Record<string, { x: number; y: number }>,
  nodeKeys: string[]
): void {
  try {
    localStorage.setItem(POSITIONS_KEY, JSON.stringify(positions))
    localStorage.setItem(NODE_KEYS_KEY, JSON.stringify(nodeKeys.slice().sort()))
  } catch {
    // ignore quota errors
  }
}

function loadPositions(
  snapKeys: string[]
): Record<string, { x: number; y: number }> | null {
  try {
    const raw = localStorage.getItem(POSITIONS_KEY)
    const keysRaw = localStorage.getItem(NODE_KEYS_KEY)
    if (!raw || !keysRaw) return null

    const savedKeys: string[] = JSON.parse(keysRaw)
    const positions: Record<string, { x: number; y: number }> = JSON.parse(raw)

    // Count how many of the SAVED nodes still appear in the current snapshot.
    // Using savedKeys.length as denominator answers: "are most of our saved
    // positions still relevant?" rather than "does this snapshot have positions
    // for every node?". This way a crawl that discovers new nodes does not
    // invalidate the existing layout — new nodes get placed in the bounding
    // box of the existing layout by snapshotToGraph (x/y not in savedPositions
    // fall back to random within the existing spread).
    const snapSet = new Set(snapKeys)
    let matches = 0
    for (const k of savedKeys) {
      if (snapSet.has(k) && positions[k]) matches++
    }

    const denominator = savedKeys.length > 0 ? savedKeys.length : snapKeys.length
    const overlapRatio = denominator > 0 ? matches / denominator : 0
    if (overlapRatio < 0.8) return null

    return positions
  } catch {
    return null
  }
}

function mountSigma(g: Graph): void {
  if (!containerRef.value) return

  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }
  hoveredNode = null
  currentCameraRatio = 1.0

  try {
    sigmaInstance = new Sigma(g, containerRef.value, {
      renderEdgeLabels: false,
      labelRenderedSizeThreshold: 4,
      labelFont: '"Ubuntu Mono", "Consolas", monospace',
      labelSize: 11,
      labelWeight: '400',
      labelColor: { attribute: 'labelColor' },
      defaultEdgeColor: getThemeEdgeColor(themeStore.isDark),
      defaultNodeColor: DEFAULT_NODE_COLOR,
      minCameraRatio: 0.01,
      maxCameraRatio: 100,

      nodeReducer: (node, data) => {
        const degree = g.degree(node)
        return { ...data, size: getBaseNodeSize(degree) }
      },

      edgeReducer: (edge, data) => {
        const isTreeEdge = (g.getEdgeAttribute(edge, 'is_tree_edge') as boolean | undefined) ?? false
        const src = g.source(edge)
        const dst = g.target(edge)
        const isHov = hoveredNode !== null && (hoveredNode === src || hoveredNode === dst)
        const lod = getLodEdgeDisplayData(isTreeEdge, currentCameraRatio, isHov, themeStore.isDark)
        return { ...data, color: lod.color, size: lod.size, hidden: lod.hidden }
      },
    })

    sigmaInstance.getCamera().on('updated', (state) => {
      currentCameraRatio = state.ratio
      if (!sigmaInstance) return
      const threshold = state.ratio < LOD_CLOSE ? 0 : state.ratio < LOD_MID ? 4 : 7
      sigmaInstance.setSetting('labelRenderedSizeThreshold', threshold)
    })

    sigmaInstance.on('clickNode', ({ node }) => store.selectNode(node))
    sigmaInstance.on('clickStage', () => store.selectNode(null))

    sigmaInstance.on('enterNode', ({ node }) => {
      hoveredNode = node
      sigmaInstance?.refresh()
    })

    sigmaInstance.on('leaveNode', () => {
      hoveredNode = null
      sigmaInstance?.refresh()
    })

  } catch (e) {
    console.error('yggmap: Sigma initialization failed:', e)
    sigmaInstance = null
    return
  }

  // Apply coloring: depth-based from selected node, degree-based when nothing selected
  applyDepthColors(g, store.selectedKey, themeStore.isDark)
  sigmaInstance?.refresh()
}

function mountGraph(snap: GraphSnapshot): void {
  if (sigmaInstance) {
    sigmaInstance.kill()
    sigmaInstance = null
  }
  graph = null
  hoveredNode = null

  const snapKeys = Object.keys(snap.nodes)
  const savedPositions = loadPositions(snapKeys)
  const newGraph = snapshotToGraph(snap, savedPositions ?? undefined)

  // Saved positions cover >=80% of nodes: skip ForceAtlas2 for stable reload
  if (savedPositions !== null) {
    graph = newGraph
    mountSigma(graph)
    return
  }

  const nodeCount = newGraph.order
  if (nodeCount <= 1) {
    graph = newGraph
    mountSigma(graph)
    return
  }

  const iterations = nodeCount < 200 ? 300 : nodeCount < 800 ? 220 : nodeCount < 2000 ? 150 : 100
  const barnesHut = nodeCount > 300

  const workerNodes: { key: string; x: number; y: number }[] = []
  newGraph.forEachNode((key, attrs) => {
    workerNodes.push({ key, x: attrs.x as number, y: attrs.y as number })
  })

  const workerEdges: { src: string; dst: string }[] = []
  newGraph.forEachEdge((_key, _attrs, src, dst) => {
    workerEdges.push({ src, dst })
  })

  layoutPending.value = true
  layoutNodeCount.value = nodeCount

  const w = getWorker()

  const onMessage = (event: MessageEvent<DoneMessage | ErrorMessage>) => {
    w.removeEventListener('message', onMessage)
    layoutPending.value = false

    if (event.data.type === 'error') {
      console.error('yggmap: layout worker error:', event.data.message)
      graph = newGraph
      mountSigma(graph)
      return
    }

    const positions = (event.data as DoneMessage).positions

    for (const [key, pos] of Object.entries(positions)) {
      if (newGraph.hasNode(key)) {
        newGraph.setNodeAttribute(key, 'x', pos.x)
        newGraph.setNodeAttribute(key, 'y', pos.y)
      }
    }

    // Persist so next reload skips ForceAtlas2 entirely
    savePositions(positions, snapKeys)
    graph = newGraph
    mountSigma(graph)
  }

  w.addEventListener('message', onMessage)

  const msg: ComputeMessage = {
    type: 'compute',
    nodes: workerNodes,
    edges: workerEdges,
    iterations,
    barnesHut
  }
  w.postMessage(msg)
}

function needsFullRebuild(prevCount: number, nextCount: number): boolean {
  if (prevCount === 0) return true
  if (nextCount > prevCount * 5 && nextCount > 50) return true
  return false
}

watch(
  () => store.snapshot,
  (snap) => {
    if (!snap || !containerRef.value) return
    if (layoutPending.value) return

    const firstLoad = !sigmaInstance || !graph

    const prevOrder = graph?.order ?? 0
    const nextOrder = Object.keys(snap.nodes).length

    if (firstLoad) {
      // Very first snapshot received — always do a full mount
      mountGraph(snap)
    } else if (!store.crawlRunning && needsFullRebuild(prevOrder, nextOrder)) {
      // Huge structural change (e.g. network reset) — full rebuild
      mountGraph(snap)
    } else {
      // Incremental update: works both during BFS and for periodic re-crawl diffs.
      // applySnapshotDiff places new nodes within the existing bounding box,
      // so there is no scatter even while crawlRunning is true.
      const changed = applySnapshotDiff(graph!, snap)
      if (changed) {
        applyDepthColors(graph!, store.selectedKey, themeStore.isDark)
        sigmaInstance?.refresh()
      }
    }
  },
  { immediate: true }
)

// When BFS finishes, do a final diff merge and persist positions for next reload.
watch(
  () => store.crawlRunning,
  (running, wasRunning) => {
    if (!running && wasRunning === true && !layoutPending.value && store.snapshot) {
      if (!sigmaInstance || !graph) {
        // No graph yet (e.g. crawl finished before first snapshot watcher fired)
        mountGraph(store.snapshot)
      } else {
        // Graph already mounted: final patch + always persist positions
        const changed = applySnapshotDiff(graph, store.snapshot)
        if (changed) {
          applyDepthColors(graph, store.selectedKey, themeStore.isDark)
          sigmaInstance?.refresh()
        }
        // Always persist so next reload skips ForceAtlas2 entirely
        const positions: Record<string, { x: number; y: number }> = {}
        graph.forEachNode((key, attrs) => {
          positions[key] = { x: attrs.x as number, y: attrs.y as number }
        })
        savePositions(positions, Object.keys(store.snapshot.nodes))
      }
    }
  }
)
</script>

<style scoped>
.graph-canvas-wrapper {
  position: relative;
  width: 100%;
  height: 100%;
}

.graph-canvas {
  width: 100%;
  height: 100%;
  background: var(--bg);
}

.layout-overlay {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--layout-overlay-bg);
  pointer-events: none;
}

.layout-label {
  font-family: "Ubuntu Mono", "Consolas", monospace;
  font-size: 14px;
  color: var(--layout-label-text);
  background: var(--layout-label-bg);
  padding: 8px 18px;
  border-radius: 6px;
  border: 1px solid var(--layout-label-border);
}
</style>
