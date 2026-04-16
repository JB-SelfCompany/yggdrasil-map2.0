// Package graph defines the in-memory topology model for yggmap.
// Types in this package are independent of the admin API wire format.
package graph

import "time"

// GraphNode represents a single Yggdrasil node discovered during a crawl.
type GraphNode struct {
	Key      string `json:"key"`       // hex ed25519 public key (canonical ID)
	Address  string `json:"address"`   // 200::/7 IPv6 address
	Name     string `json:"name"`      // from getNodeInfo, may be empty
	OS       string `json:"os"`        // from getNodeInfo
	BuildVer string `json:"build_ver"` // from getSelf or debug_remoteGetSelf

	InTree bool `json:"in_tree"` // appeared in getTree response

	FirstSeen time.Time     `json:"first_seen"`
	LastSeen  time.Time     `json:"last_seen"`
	Latency   time.Duration `json:"latency_ns"` // 0 if unknown; from getPeers
}

// GraphEdge represents a directed connection between two nodes.
// Edges are stored with canonical ordering (SrcKey < DstKey) to avoid
// duplicates, except for tree edges where direction is significant.
type GraphEdge struct {
	SrcKey     string `json:"src_key"`
	DstKey     string `json:"dst_key"`
	IsTreeEdge bool   `json:"is_tree_edge"` // true = parent-child in spanning tree
	Up         bool   `json:"up"`
	Cost       uint64 `json:"cost"`
	Inbound    bool   `json:"inbound"`
}

// GraphSnapshot is an immutable point-in-time view of the network topology
// produced by a single crawl cycle.
type GraphSnapshot struct {
	Timestamp       time.Time             `json:"timestamp"`
	Nodes           map[string]*GraphNode `json:"nodes"` // key → node
	Edges           []GraphEdge           `json:"edges"`
	CrawlDurationMS int64                 `json:"crawl_duration_ms"`
}

// NodeCount returns the number of nodes in the snapshot.
func (s *GraphSnapshot) NodeCount() int {
	return len(s.Nodes)
}

// EdgeCount returns the number of edges in the snapshot.
func (s *GraphSnapshot) EdgeCount() int {
	return len(s.Edges)
}
