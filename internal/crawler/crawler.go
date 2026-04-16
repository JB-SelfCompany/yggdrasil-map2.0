// Package crawler implements Yggdrasil topology discovery.
// The primary method is a concurrent BFS via debug_remoteGetPeers, identical
// to the approach used by the original yggdrasil-map crawler.go.
package crawler

import (
	"context"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/JB-SelfCompany/yggmap/internal/admin"
	"github.com/JB-SelfCompany/yggmap/internal/config"
	"github.com/JB-SelfCompany/yggmap/internal/graph"
)

// bfsConcurrency is the number of parallel debug_remoteGetPeers requests.
// The original yggdrasil-map used 32; 16 is a safe default.
const bfsConcurrency = 16

// Crawler performs topology discovery against the local Yggdrasil admin API.
type Crawler struct {
	admin    *admin.Client
	bfsAdmin *admin.Client // separate client with shorter timeout for BFS remote calls
	cfg      *config.CrawlerConfig
	logger   *log.Logger

	// OnProgress, if set, is called with a preliminary snapshot after
	// getSelf/getTree/getPeers complete — before the slower BFS phase.
	// This lets the UI show data immediately without waiting for BFS.
	OnProgress func(*graph.GraphSnapshot)

	// OnBFSProgress, if set, is called approximately every 5 seconds during
	// BFS with an intermediate snapshot of all nodes discovered so far.
	// This allows the UI to progressively update the map as the crawl proceeds
	// rather than waiting for the full BFS to complete.
	OnBFSProgress func(*graph.GraphSnapshot)
}

// New creates a Crawler. bfsAdmin is used for debug_remoteGetPeers calls and
// should have a shorter timeout than the main admin client (e.g. 5s vs 8s).
func New(a *admin.Client, bfsAdmin *admin.Client, cfg *config.CrawlerConfig, logger *log.Logger) *Crawler {
	return &Crawler{admin: a, bfsAdmin: bfsAdmin, cfg: cfg, logger: logger}
}

// Crawl performs a single full crawl and returns a GraphSnapshot.
// Algorithm (mirrors original yggdrasil-map crawler.go):
//  1. getSelf           — own node, hard fail if unavailable
//  2. getTree           — adds tree topology (soft fail)
//  3. getPeers          — adds direct peers with metrics (soft fail)
//  4. BFS via debug_remoteGetPeers — recursively discovers the full network
//  5. getNodeInfo       — enriches nodes with names/OS (optional, best-effort)
func (c *Crawler) Crawl(ctx context.Context) (*graph.GraphSnapshot, error) {
	start := time.Now()

	var mu sync.Mutex
	nodes := make(map[string]*graph.GraphNode)
	var edges []graph.GraphEdge

	// -------------------------------------------------------------------------
	// Step 1: getSelf
	// -------------------------------------------------------------------------
	self, err := c.admin.GetSelf()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	nodes[self.Key] = &graph.GraphNode{
		Key:       self.Key,
		Address:   self.Address,
		BuildVer:  self.BuildVersion,
		FirstSeen: now,
		LastSeen:  now,
	}

	if ctx.Err() != nil {
		return buildSnapshot(start, nodes, edges), nil
	}

	// -------------------------------------------------------------------------
	// Step 2: getTree — partial spanning tree known to local node
	// -------------------------------------------------------------------------
	tree, err := c.admin.GetTree()
	if err != nil {
		c.logger.Printf("crawler: getTree error: %v", err)
	} else {
		for _, entry := range tree.Tree {
			if entry.Key == "" {
				continue
			}
			n := upsertNode(nodes, entry.Key, now)
			if entry.Address != "" {
				n.Address = entry.Address
			}
			n.InTree = true
		}
		for _, entry := range tree.Tree {
			if entry.Parent == "" || entry.Parent == entry.Key {
				continue
			}
			edges = appendEdge(edges, graph.GraphEdge{
				SrcKey:     entry.Parent,
				DstKey:     entry.Key,
				IsTreeEdge: true,
				Up:         true,
			})
		}
	}

	if ctx.Err() != nil {
		return buildSnapshot(start, nodes, edges), nil
	}

	// -------------------------------------------------------------------------
	// Step 3: getPeers — direct connections with metrics
	// -------------------------------------------------------------------------
	peers, err := c.admin.GetPeers()
	if err != nil {
		c.logger.Printf("crawler: getPeers error: %v", err)
	} else {
		for _, p := range peers.Peers {
			if p.Key == "" {
				continue
			}
			n := upsertNode(nodes, p.Key, now)
			if p.Address != "" {
				n.Address = p.Address
			}
			if p.Latency > 0 {
				n.Latency = time.Duration(p.Latency)
			}
			edges = appendEdge(edges, graph.GraphEdge{
				SrcKey:  self.Key,
				DstKey:  p.Key,
				Up:      p.Up,
				Cost:    p.Cost,
				Inbound: p.Inbound,
			})
		}
	}

	if ctx.Err() != nil {
		return buildSnapshot(start, nodes, edges), nil
	}

	// -------------------------------------------------------------------------
	// Preliminary snapshot — publish before BFS so the UI shows data immediately.
	// BFS can take tens of seconds when remote nodes timeout; without this the
	// map stays blank until the entire crawl finishes.
	// -------------------------------------------------------------------------
	if c.OnProgress != nil {
		c.OnProgress(buildSnapshot(start, nodes, edges))
	}

	// -------------------------------------------------------------------------
	// Step 4: BFS via debug_remoteGetPeers
	// Seeds with all keys discovered so far, then recursively expands.
	// -------------------------------------------------------------------------
	c.bfsCrawl(ctx, start, nodes, &edges, &mu, now)

	// Signal BFS completion so the UI hides the crawl spinner immediately.
	// fetchNodeInfo can take tens of seconds; without this the spinner stays up
	// long after the graph stops changing.
	if c.OnProgress != nil {
		mu.Lock()
		bfsSnap := buildSnapshot(start, nodes, edges)
		mu.Unlock()
		c.OnProgress(bfsSnap)
	}

	// -------------------------------------------------------------------------
	// Step 5: getNodeInfo (optional, best-effort, parallel)
	// -------------------------------------------------------------------------
	if c.cfg.EnableNodeInfo {
		c.fetchNodeInfo(ctx, nodes, now)
	}

	return buildSnapshot(start, nodes, edges), nil
}

// bfsCrawl performs a concurrent BFS starting from all nodes already in `nodes`.
// For each node it calls debug_remoteGetPeers, adds discovered peers to the graph,
// and enqueues newly-seen peers for further expansion.
//
// This is the core discovery mechanism — without it the map only shows the tiny
// slice of the network visible from the local node's getTree/getPeers.
func (c *Crawler) bfsCrawl(
	ctx context.Context,
	start time.Time,
	nodes map[string]*graph.GraphNode,
	edges *[]graph.GraphEdge,
	mu *sync.Mutex,
	now time.Time,
) {
	// visited tracks keys that have been enqueued (not necessarily processed yet).
	visited := newSyncSet()

	// Seed with all keys already known from getSelf/getTree/getPeers.
	mu.Lock()
	seedKeys := make([]string, 0, len(nodes))
	for k := range nodes {
		visited.add(k)
		seedKeys = append(seedKeys, k)
	}
	mu.Unlock()

	c.logger.Printf("crawler: bfs starting with %d seed nodes", len(seedKeys))

	// work is a buffered channel carrying keys to process.
	// 100k buffer is plenty for any real Yggdrasil network.
	work := make(chan string, 100_000)
	var wg sync.WaitGroup

	// Enqueue seed keys.
	for _, k := range seedKeys {
		wg.Add(1)
		work <- k
	}

	// done is closed by the drainer goroutine after wg reaches zero,
	// signalling that all BFS work is complete.
	done := make(chan struct{})

	// Drainer: closes work (and done) once all in-flight items finish.
	// Must be started before workers so they see work close cleanly.
	go func() {
		wg.Wait()
		close(work) // unblocks workers' range loop
		close(done) // signals bfsCrawl to return
	}()

	// Start bfsConcurrency workers.
	for i := 0; i < bfsConcurrency; i++ {
		go func() {
			for key := range work {
				c.processKey(ctx, key, nodes, edges, mu, now, visited, work, &wg)
			}
		}()
	}

	// Start a progress reporter goroutine while BFS runs.
	// BFSProgressSec == 0 disables mid-crawl updates.
	bfsProgressInterval := time.Duration(c.cfg.BFSProgressSec) * time.Second
	if c.OnBFSProgress != nil && bfsProgressInterval > 0 {
		go func() {
			ticker := time.NewTicker(bfsProgressInterval)
			defer ticker.Stop()
			for {
				select {
				case <-done:
					return
				case <-ctx.Done():
					return
				case <-ticker.C:
					mu.Lock()
					nodesCopy := make(map[string]*graph.GraphNode, len(nodes))
					for k, v := range nodes {
						n := *v
						nodesCopy[k] = &n
					}
					edgesCopy := make([]graph.GraphEdge, len(*edges))
					copy(edgesCopy, *edges)
					mu.Unlock()
					snap := buildSnapshot(start, nodesCopy, edgesCopy)
					c.OnBFSProgress(snap)
				}
			}
		}()
	}

	// Block until all work is done or context is cancelled.
	select {
	case <-ctx.Done():
		// Drain remaining items so workers can exit their range loop cleanly.
		for range work {
		}
	case <-done:
	}

	mu.Lock()
	finalCount := len(nodes)
	mu.Unlock()
	c.logger.Printf("crawler: bfs complete, visited=%d nodes", finalCount)
}

// processKey queries debug_remoteGetPeers for key, records discovered peers as
// nodes and edges, and enqueues any newly-seen peer keys for BFS expansion.
func (c *Crawler) processKey(
	ctx context.Context,
	key string,
	nodes map[string]*graph.GraphNode,
	edges *[]graph.GraphEdge,
	mu *sync.Mutex,
	now time.Time,
	visited *syncSet,
	work chan<- string,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	if ctx.Err() != nil {
		return
	}

	peerKeys, err := c.bfsAdmin.DebugRemoteGetPeers(key)
	if err != nil {
		// Most nodes respond; failures are transient or the node is unreachable.
		// Don't spam the log — only log non-timeout errors.
		if !isTimeoutErr(err) {
			c.logger.Printf("crawler: debug_remoteGetPeers(%s): %v", shortKey(key), err)
		}
		return
	}

	for _, pk := range peerKeys {
		if pk == "" || pk == key {
			continue
		}

		mu.Lock()
		upsertNode(nodes, pk, now)
		*edges = appendEdge(*edges, graph.GraphEdge{
			SrcKey: key,
			DstKey: pk,
			Up:     true,
		})
		mu.Unlock()

		// Enqueue only if not yet seen.
		if visited.add(pk) {
			wg.Add(1)
			select {
			case work <- pk:
			default:
				// Buffer full (shouldn't happen with 100k buffer).
				wg.Done()
			}
		}
	}
}

// fetchNodeInfo concurrently enriches nodes with name/OS from getNodeInfo.
// Errors are silently ignored — nodeinfo is best-effort.
func (c *Crawler) fetchNodeInfo(ctx context.Context, nodes map[string]*graph.GraphNode, now time.Time) {
	concurrency := c.cfg.NodeInfoConcurrency
	if concurrency <= 0 {
		concurrency = 4
	}

	keys := make([]string, 0, len(nodes))
	for k := range nodes {
		keys = append(keys, k)
	}

	sem := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	for _, key := range keys {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		go func(k string) {
			defer wg.Done()
			defer func() { <-sem }()

			if ctx.Err() != nil {
				return
			}
			info, err := c.admin.GetNodeInfo(k)
			if err != nil {
				return // silently skip — timeouts are expected
			}
			applyNodeInfo(nodes[k], info)
		}(key)
	}

	wg.Wait()
}

// applyNodeInfo extracts name/OS/version from a nodeinfo map.
func applyNodeInfo(n *graph.GraphNode, info map[string]interface{}) {
	if n == nil || info == nil {
		return
	}
	if v, ok := stringField(info, "name"); ok {
		n.Name = v
	} else if v, ok := stringField(info, "hostname"); ok {
		n.Name = v
	}
	if v, ok := stringField(info, "os"); ok {
		n.OS = v
	}
	if v, ok := stringField(info, "buildversion"); ok && n.BuildVer == "" {
		n.BuildVer = v
	}
}

// -------------------------------------------------------------------------
// Helpers
// -------------------------------------------------------------------------

func stringField(m map[string]interface{}, key string) (string, bool) {
	v, ok := m[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

func upsertNode(nodes map[string]*graph.GraphNode, key string, now time.Time) *graph.GraphNode {
	if n, ok := nodes[key]; ok {
		n.LastSeen = now
		return n
	}
	n := &graph.GraphNode{Key: key, FirstSeen: now, LastSeen: now}
	nodes[key] = n
	return n
}

func appendEdge(edges []graph.GraphEdge, e graph.GraphEdge) []graph.GraphEdge {
	if !e.IsTreeEdge {
		if e.SrcKey > e.DstKey {
			e.SrcKey, e.DstKey = e.DstKey, e.SrcKey
			e.Inbound = !e.Inbound
		}
		for _, ex := range edges {
			if !ex.IsTreeEdge && ex.SrcKey == e.SrcKey && ex.DstKey == e.DstKey {
				return edges
			}
		}
	}
	return append(edges, e)
}

func buildSnapshot(start time.Time, nodes map[string]*graph.GraphNode, edges []graph.GraphEdge) *graph.GraphSnapshot {
	return &graph.GraphSnapshot{
		Timestamp:       start,
		Nodes:           nodes,
		Edges:           edges,
		CrawlDurationMS: time.Since(start).Milliseconds(),
	}
}

// shortKey returns the first 8 chars of a hex key for log messages.
func shortKey(k string) string {
	if len(k) > 8 {
		return k[:8]
	}
	return k
}

// isTimeoutErr returns true for errors that represent expected unreachable-node
// conditions. Yggdrasil admin socket returns "timeout" (not "timed out") for
// remote nodes that don't respond; a short bfsAdmin deadline produces "i/o timeout"
// or "deadline exceeded" instead.
func isTimeoutErr(err error) bool {
	if err == nil {
		return false
	}
	s := err.Error()
	return strings.Contains(s, "timeout") ||
		strings.Contains(s, "timed out") ||
		strings.Contains(s, "deadline exceeded")
}

// min returns the smaller of a and b.
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// -------------------------------------------------------------------------
// syncSet — concurrent-safe set of strings
// -------------------------------------------------------------------------

type syncSet struct {
	mu    sync.Mutex
	items map[string]struct{}
}

func newSyncSet() *syncSet {
	return &syncSet{items: make(map[string]struct{})}
}

// add adds key to the set. Returns true if key was new, false if already present.
func (s *syncSet) add(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.items[key]; ok {
		return false
	}
	s.items[key] = struct{}{}
	return true
}
