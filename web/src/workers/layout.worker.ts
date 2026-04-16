/**
 * Web Worker: ForceAtlas2 layout computation.
 *
 * Runs the layout algorithm off the main thread so the UI stays responsive
 * while processing large graphs (thousands of nodes).
 *
 * Protocol:
 *   In:  { type: 'compute', nodes: WorkerNode[], edges: WorkerEdge[],
 *           iterations: number, barnesHut: boolean }
 *   Out: { type: 'done', positions: Record<string, { x: number; y: number }> }
 *        { type: 'error', message: string }
 */

import Graph from 'graphology'
import forceAtlas2 from 'graphology-layout-forceatlas2'

export interface WorkerNode {
  key: string
  x: number
  y: number
}

export interface WorkerEdge {
  src: string
  dst: string
}

export interface ComputeMessage {
  type: 'compute'
  nodes: WorkerNode[]
  edges: WorkerEdge[]
  iterations: number
  barnesHut: boolean
}

export interface DoneMessage {
  type: 'done'
  positions: Record<string, { x: number; y: number }>
}

export interface ErrorMessage {
  type: 'error'
  message: string
}

self.onmessage = (event: MessageEvent<ComputeMessage>) => {
  const msg = event.data
  if (msg.type !== 'compute') return

  try {
    const graph = new Graph({ multi: false, type: 'undirected' })

    for (const n of msg.nodes) {
      graph.addNode(n.key, { x: n.x, y: n.y })
    }

    for (const e of msg.edges) {
      if (!graph.hasNode(e.src) || !graph.hasNode(e.dst)) continue
      if (!graph.hasEdge(e.src, e.dst) && !graph.hasEdge(e.dst, e.src)) {
        graph.addEdge(e.src, e.dst)
      }
    }

    if (graph.order > 1) {
      forceAtlas2.assign(graph, {
        iterations: msg.iterations,
        settings: {
          barnesHutOptimize: msg.barnesHut,
          barnesHutTheta: 0.5,
          // Weak gravity + high scaling ratio spreads nodes further apart so
          // individual nodes can be inspected without overlap when zoomed in.
          strongGravityMode: false,
          gravity: 0.05,
          scalingRatio: 10,
          linLogMode: false,
          outboundAttractionDistribution: false,
          adjustSizes: false,
          edgeWeightInfluence: 0,
          slowDown: 2,
        }
      })
    }

    const positions: Record<string, { x: number; y: number }> = {}
    graph.forEachNode((key, attrs) => {
      positions[key] = { x: attrs.x as number, y: attrs.y as number }
    })

    const reply: DoneMessage = { type: 'done', positions }
    self.postMessage(reply)
  } catch (err) {
    const reply: ErrorMessage = {
      type: 'error',
      message: err instanceof Error ? err.message : String(err)
    }
    self.postMessage(reply)
  }
}
