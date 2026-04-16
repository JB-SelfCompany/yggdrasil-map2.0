// Types matching Go backend

export interface GraphNode {
  key: string
  address: string
  name: string
  os: string
  build_ver: string
  in_tree: boolean
  first_seen: string
  last_seen: string
  latency_ns: number
}

export interface GraphEdge {
  src_key: string
  dst_key: string
  is_tree_edge: boolean
  up: boolean
  cost: number
  inbound: boolean
}

export interface GraphSnapshot {
  timestamp: string
  nodes: Record<string, GraphNode>
  edges: GraphEdge[]
  crawl_duration_ms: number
}

export interface StatusResponse {
  last_crawl_time: string
  node_count: number
  edge_count: number
  crawl_running: boolean
}

export interface CrawlProgress {
  running: boolean
  visited: number
  duration_ms: number
}

export interface WSMessage {
  type: 'snapshot' | 'crawl_progress'
  data: GraphSnapshot | CrawlProgress
}

export interface AppConfig {
  crawler_interval: string
  bfs_progress_sec: number
}

const BASE = '/api'

async function request<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, options)
  if (!res.ok) {
    throw new Error(`API ${path} returned ${res.status}: ${res.statusText}`)
  }
  return res.json() as Promise<T>
}

export async function fetchGraph(): Promise<GraphSnapshot> {
  return request<GraphSnapshot>('/graph')
}

export async function fetchStatus(): Promise<StatusResponse> {
  return request<StatusResponse>('/status')
}

export async function triggerCrawl(): Promise<void> {
  await request<unknown>('/crawl', { method: 'POST' })
}

export async function fetchConfig(): Promise<AppConfig> {
  const res = await fetch(`${BASE}/config`)
  if (!res.ok) throw new Error('config fetch failed')
  return res.json() as Promise<AppConfig>
}
