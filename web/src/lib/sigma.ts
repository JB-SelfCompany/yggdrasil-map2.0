import Graph from 'graphology'
import type { GraphSnapshot } from './api'

// Depth-based color palette — original yggdrasil-map colors
export const DEPTH_COLORS: string[] = [
  '#000000', // depth 0  — selected node
  '#096EE8', // depth 1
  '#09E8B8', // depth 2
  '#36E809', // depth 3
  '#ADE809', // depth 4
  '#E8B809', // depth 5
  '#E87509', // depth 6
  '#E83A09', // depth 7
  '#E86946', // depth 8
  '#E8AC9B', // depth 9
  '#E8C9C1', // depth 10+
]

export const DEFAULT_NODE_COLOR = '#3A7BD5'    // blue — default leaf
export const UNREACHABLE_COLOR = '#CCCCCC'     // light gray — unreachable in depth view

// ── Edge color constants (light theme) ──────────────────────────────────────
export const TREE_EDGE_COLOR_CLOSE = 'rgba(0, 0, 0, 0.15)'
export const TREE_EDGE_COLOR_MID   = 'rgba(0, 0, 0, 0.15)'
export const TREE_EDGE_COLOR_FAR   = 'rgba(0, 0, 0, 0.15)'
export const PEER_EDGE_COLOR_CLOSE = 'rgba(0, 0, 0, 0.15)'
export const PEER_EDGE_COLOR_MID   = 'rgba(0, 0, 0, 0.15)'
export const PEER_EDGE_COLOR_FAR   = 'rgba(0, 0, 0, 0.00)'
export const DEFAULT_EDGE_COLOR    = 'rgba(0, 0, 0, 0.15)'
export const HOVER_EDGE_COLOR      = 'rgba(0, 0, 0, 0.5)'

// ── Theme-aware color helpers ────────────────────────────────────────────────

export function getThemeEdgeColor(isDark: boolean): string {
  return isDark ? 'rgba(255, 255, 255, 0.25)' : 'rgba(0, 0, 0, 0.15)'
}

export function getThemeHoverEdgeColor(isDark: boolean): string {
  return isDark ? 'rgba(255, 255, 255, 0.70)' : 'rgba(0, 0, 0, 0.50)'
}

export function getThemeLabelColor(isDark: boolean): string {
  return isDark ? '#CBD5E1' : '#333333'
}

// ── LOD camera thresholds ────────────────────────────────────────────────────
// Sigma camera.ratio: small = zoomed in, large = zoomed out
//   ratio < 0.3  — close zoom (~30–50 nodes visible)
//   ratio ~ 1.0  — default view
//   ratio > 1.5  — overview (all nodes)
export const LOD_CLOSE = 0.3
export const LOD_MID   = 1.2

export const TREE_EDGE_SIZE_CLOSE = 1.0
export const TREE_EDGE_SIZE_MID   = 1.0
export const TREE_EDGE_SIZE_FAR   = 1.0
export const PEER_EDGE_SIZE_CLOSE = 1.0
export const PEER_EDGE_SIZE_MID   = 1.0
export const PEER_EDGE_SIZE_FAR   = 1.0

/**
 * Returns edge display attributes based on LOD zoom level and edge type.
 * Called from Sigma's edgeReducer — must be pure and fast (~22k calls/frame).
 * All edges now use uniform thin lines; peer edges are only hidden at far zoom.
 */
export function getLodEdgeDisplayData(
  isTreeEdge: boolean,
  cameraRatio: number,
  isHovered: boolean,
  isDark = false
): { color: string; size: number; hidden: boolean } {
  if (isHovered) {
    return { color: getThemeHoverEdgeColor(isDark), size: 1.5, hidden: false }
  }
  // Hide peer edges at far zoom only
  if (!isTreeEdge && cameraRatio >= LOD_MID) {
    return { color: getThemeEdgeColor(isDark), size: 1.0, hidden: true }
  }
  return { color: getThemeEdgeColor(isDark), size: 1.0, hidden: false }
}

/**
 * Returns node display size boosted at close zoom to compensate for Sigma scaling.
 * @deprecated Kept for backwards compatibility — use getBaseNodeSize for new call sites.
 */
export function getLodNodeSize(baseSize: number, cameraRatio: number): number {
  if (cameraRatio <= 0) return baseSize * 3
  const zoomFactor = Math.max(0.3, Math.min(3.0, 1.0 / cameraRatio))
  return Math.max(2, Math.min(40, baseSize * zoomFactor))
}

function nodeLabel(address: string, name: string): string {
  return name || address.slice(-8)
}

/**
 * Light-theme degree-based color palette for nodes.
 */
export function getNodeColor(degree: number): string {
  if (degree <= 1)    return '#AAAAAA'  // gray   — isolated/leaf
  if (degree <= 5)    return '#3A7BD5'  // blue   — leaf
  if (degree <= 15)   return '#0891B2'  // cyan   — ordinary
  if (degree <= 40)   return '#059669'  // green  — connected
  if (degree <= 100)  return '#D97706'  // amber  — hub
  return '#DC2626'                      // red    — super-hub
}

// Keep an unexported alias so existing internal call sites are unaffected.
function nodeColor(degree: number): string {
  return getNodeColor(degree)
}

/**
 * Fixed-size formula: small dots for leaves, slightly larger for hubs.
 * leaf(1)~4px, hub(100+)~6px. No zoom scaling.
 */
export function getBaseNodeSize(degree: number): number {
  return degree > 20 ? 6 : 4
}

/**
 * Zoom-dependent node size.
 * @deprecated Zoom scaling removed — delegates to getBaseNodeSize for compatibility.
 */
export function getZoomedNodeSize(degree: number, _cameraRatio: number): number {
  return getBaseNodeSize(degree)
}

/**
 * Logarithmic size scale — kept for backwards compatibility with snapshotToGraph /
 * applySnapshotDiff which store a base_size attribute.
 */
function nodeSize(degree: number): number {
  return getBaseNodeSize(degree)
}

/**
 * BFS from sourceKey. Returns a Map<nodeKey, depth>.
 * Nodes unreachable from the source are not included in the map.
 */
export function computeDepths(graph: Graph, sourceKey: string): Map<string, number> {
  const depths = new Map<string, number>()
  if (!graph.hasNode(sourceKey)) return depths

  const queue: string[] = [sourceKey]
  depths.set(sourceKey, 0)

  while (queue.length > 0) {
    const current = queue.shift()!
    const currentDepth = depths.get(current)!

    for (const neighbor of graph.neighbors(current)) {
      if (!depths.has(neighbor)) {
        depths.set(neighbor, currentDepth + 1)
        queue.push(neighbor)
      }
    }
  }

  return depths
}

/**
 * Apply depth-based colors to all nodes in the graph from the selected node,
 * or degree-based colors when no node is selected.
 */
export function applyDepthColors(
  graph: Graph,
  selectedKey: string | null,
  isDark = false,
): void {
  const labelColor = getThemeLabelColor(isDark)

  if (!selectedKey || !graph.hasNode(selectedKey)) {
    // No selection: degree-based palette — shows network topology at a glance
    graph.forEachNode((key) => {
      graph.setNodeAttribute(key, 'color', getNodeColor(graph.degree(key)))
      graph.setNodeAttribute(key, 'labelColor', labelColor)
    })
    return
  }

  const depths = computeDepths(graph, selectedKey)

  graph.forEachNode((key) => {
    if (depths.has(key)) {
      const depth = Math.min(depths.get(key)!, 10)
      graph.setNodeAttribute(key, 'color', DEPTH_COLORS[depth])
    } else {
      graph.setNodeAttribute(key, 'color', UNREACHABLE_COLOR)
    }
    graph.setNodeAttribute(key, 'labelColor', labelColor)
  })
}

/**
 * Build a new graphology Graph from a snapshot.
 * If savedPositions is provided and matches nodes, those positions are used
 * instead of random placement — ForceAtlas2 then converges to a stable layout.
 */
export function snapshotToGraph(
  snap: GraphSnapshot,
  savedPositions?: Record<string, { x: number; y: number }>
): Graph {
  const graph = new Graph({ multi: false, type: 'undirected' })

  for (const [key, node] of Object.entries(snap.nodes)) {
    const saved = savedPositions?.[key]
    graph.addNode(key, {
      label: nodeLabel(node.address, node.name),
      color: DEFAULT_NODE_COLOR,
      labelColor: '#333333',
      size: 3,
      x: saved ? saved.x : Math.random() * 1000,
      y: saved ? saved.y : Math.random() * 1000,
      address: node.address,
      name: node.name,
      os: node.os,
      build_ver: node.build_ver,
      in_tree: node.in_tree,
      first_seen: node.first_seen,
      last_seen: node.last_seen,
      latency_ns: node.latency_ns
    })
  }

  for (const edge of snap.edges) {
    if (!graph.hasNode(edge.src_key) || !graph.hasNode(edge.dst_key)) continue
    if (graph.hasEdge(edge.src_key, edge.dst_key)) continue

    graph.addEdge(edge.src_key, edge.dst_key, {
      color: DEFAULT_EDGE_COLOR,
      size: 1,
      is_tree_edge: edge.is_tree_edge,
      up: edge.up,
      cost: edge.cost
    })
  }

  // Apply degree-based color and size now that all edges are in place.
  graph.forEachNode((key) => {
    const deg = graph.degree(key)
    graph.setNodeAttribute(key, 'color', nodeColor(deg))
    const sz = nodeSize(deg)
    graph.setNodeAttribute(key, 'size', sz)
    graph.setNodeAttribute(key, 'base_size', sz)
  })

  return graph
}

/**
 * Apply a new snapshot onto an existing graphology Graph in-place.
 * Preserves nodes/edges that haven't changed so Sigma camera and layout
 * positions are maintained. Returns true if any changes were made.
 */
export function applySnapshotDiff(
  graph: Graph,
  snap: GraphSnapshot
): boolean {
  let changed = false

  // Compute the bounding box of currently-positioned nodes so new nodes can
  // be placed within the visible area instead of at random 0–1000 coords.
  let bbMinX = 0, bbMaxX = 1000, bbMinY = 0, bbMaxY = 1000
  if (graph.order > 0) {
    bbMinX = Infinity; bbMaxX = -Infinity
    bbMinY = Infinity; bbMaxY = -Infinity
    graph.forEachNode((_key, attrs) => {
      const nx = (attrs.x as number) ?? 0
      const ny = (attrs.y as number) ?? 0
      if (nx < bbMinX) bbMinX = nx
      if (nx > bbMaxX) bbMaxX = nx
      if (ny < bbMinY) bbMinY = ny
      if (ny > bbMaxY) bbMaxY = ny
    })
    if (!isFinite(bbMinX)) { bbMinX = 0; bbMaxX = 1000; bbMinY = 0; bbMaxY = 1000 }
  }
  const bbW = Math.max(bbMaxX - bbMinX, 10)
  const bbH = Math.max(bbMaxY - bbMinY, 10)

  // --- Nodes: add new, update attributes, remove stale ---
  const incomingKeys = new Set(Object.keys(snap.nodes))

  graph.forEachNode((key) => {
    if (!incomingKeys.has(key)) {
      graph.dropNode(key)
      changed = true
    }
  })

  for (const [key, node] of Object.entries(snap.nodes)) {
    const attrs = {
      label: nodeLabel(node.address, node.name),
      labelColor: '#333333',
      address: node.address,
      name: node.name,
      os: node.os,
      build_ver: node.build_ver,
      in_tree: node.in_tree,
      first_seen: node.first_seen,
      last_seen: node.last_seen,
      latency_ns: node.latency_ns
    }

    if (!graph.hasNode(key)) {
      graph.addNode(key, {
        ...attrs,
        color: DEFAULT_NODE_COLOR,
        size: 3,
        x: bbMinX + Math.random() * bbW,
        y: bbMinY + Math.random() * bbH
      })
      changed = true
    } else {
      // Always skip color/labelColor — depth colors are applied externally via applyDepthColors.
      const skipAttrs = new Set(['color', 'labelColor'])
      for (const [attr, val] of Object.entries(attrs)) {
        if (skipAttrs.has(attr)) continue
        if (graph.getNodeAttribute(key, attr) !== val) {
          graph.setNodeAttribute(key, attr, val)
          changed = true
        }
      }
    }
  }

  // --- Edges: add new, remove stale ---
  const incomingEdges = new Set(
    snap.edges.map((e) => `${e.src_key}--${e.dst_key}`)
  )

  graph.forEachEdge((edgeKey, _attrs, src, dst) => {
    const fwd = `${src}--${dst}`
    const rev = `${dst}--${src}`
    if (!incomingEdges.has(fwd) && !incomingEdges.has(rev)) {
      graph.dropEdge(edgeKey)
      changed = true
    }
  })

  for (const edge of snap.edges) {
    if (!graph.hasNode(edge.src_key) || !graph.hasNode(edge.dst_key)) continue
    if (graph.hasEdge(edge.src_key, edge.dst_key)) continue

    graph.addEdge(edge.src_key, edge.dst_key, {
      color: DEFAULT_EDGE_COLOR,
      size: 1,
      is_tree_edge: edge.is_tree_edge,
      up: edge.up,
      cost: edge.cost
    })
    changed = true
  }

  // Update sizes for all nodes after structural changes (colors applied externally).
  if (changed) {
    graph.forEachNode((key) => {
      const deg = graph.degree(key)
      const sz = nodeSize(deg)
      graph.setNodeAttribute(key, 'size', sz)
      graph.setNodeAttribute(key, 'base_size', sz)
    })
  }

  return changed
}
