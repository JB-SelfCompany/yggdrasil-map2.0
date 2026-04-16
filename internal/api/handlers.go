package api

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/JB-SelfCompany/yggmap/internal/graph"
)

// hexKeyRe matches a valid 64-character lowercase or uppercase hex node public key.
var hexKeyRe = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

// jsonError writes a JSON error body with the given HTTP status code.
func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg}) //nolint:errcheck
}

// GraphHandler serves GET /api/graph — the full current snapshot as JSON.
// Supports gzip encoding (Accept-Encoding: gzip) and conditional requests
// (If-None-Match) to minimise transfer size for large graphs.
func GraphHandler(store *graph.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		snap := store.Get()
		if snap == nil {
			jsonError(w, "no snapshot available yet", http.StatusServiceUnavailable)
			return
		}

		// ETag derived from snapshot timestamp — cheap to compute, stable per crawl.
		etag := fmt.Sprintf(`"%d"`, snap.Timestamp.UnixNano())
		w.Header().Set("ETag", etag)
		// Cache for up to 60 s; proxies and browsers may serve stale until revalidated.
		w.Header().Set("Cache-Control", "public, max-age=60, must-revalidate")
		w.Header().Set("Last-Modified", snap.Timestamp.UTC().Format(http.TimeFormat))
		// Tell caches that the response varies by encoding.
		w.Header().Set("Vary", "Accept-Encoding")

		// Conditional GET: client already holds this version.
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		// Gzip encoding: a 3 000-node graph is typically 2–5 MB plain JSON but
		// only 200–400 KB gzipped — a 10× reduction that matters on Yggdrasil.
		if strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			w.Header().Set("Content-Encoding", "gzip")
			gz := gzip.NewWriter(w)
			defer gz.Close()
			json.NewEncoder(gz).Encode(snap) //nolint:errcheck
		} else {
			json.NewEncoder(w).Encode(snap) //nolint:errcheck
		}
	}
}

// NodeHandler serves GET /api/graph/node/{key} — a single node by public key.
// The key is extracted by stripping the known prefix from the URL path.
func NodeHandler(store *graph.Store) http.HandlerFunc {
	const prefix = "/api/graph/node/"
	return func(w http.ResponseWriter, r *http.Request) {
		key := strings.TrimPrefix(r.URL.Path, prefix)
		key = strings.TrimSuffix(key, "/")
		if key == "" {
			jsonError(w, "missing node key", http.StatusBadRequest)
			return
		}
		if !hexKeyRe.MatchString(key) {
			jsonError(w, "invalid node key", http.StatusBadRequest)
			return
		}

		snap := store.Get()
		if snap == nil {
			jsonError(w, "no snapshot available yet", http.StatusServiceUnavailable)
			return
		}

		node, ok := snap.Nodes[key]
		if !ok {
			jsonError(w, "node not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(node) //nolint:errcheck
	}
}

// StatusResponse is the body of GET /api/status.
type StatusResponse struct {
	LastCrawlTime string `json:"last_crawl_time"` // RFC3339; empty if no snapshot yet
	NodeCount     int    `json:"node_count"`
	EdgeCount     int    `json:"edge_count"`
	CrawlRunning  bool   `json:"crawl_running"`
}

// StatusHandler serves GET /api/status.
func StatusHandler(store *graph.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		resp := StatusResponse{}

		snap := store.Get()
		if snap != nil {
			resp.LastCrawlTime = snap.Timestamp.UTC().Format(time.RFC3339)
			resp.NodeCount = snap.NodeCount()
			resp.EdgeCount = snap.EdgeCount()
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

// TriggerCrawlHandler serves POST /api/crawl — requests an immediate crawl.
// triggerFn is called synchronously but should be non-blocking (e.g. send on a buffered channel).
// Repeated calls within the cooldown window (60 s) return 429 Too Many Requests.
func TriggerCrawlHandler(triggerFn func()) http.HandlerFunc {
	var mu sync.Mutex
	var lastTrigger time.Time
	const cooldown = 60 * time.Second

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		mu.Lock()
		if time.Since(lastTrigger) < cooldown {
			mu.Unlock()
			w.Header().Set("Retry-After", "60")
			jsonError(w, "crawl cooldown active — try again later", http.StatusTooManyRequests)
			return
		}
		lastTrigger = time.Now()
		mu.Unlock()

		if triggerFn != nil {
			triggerFn()
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "crawl triggered"}) //nolint:errcheck
	}
}

// HealthHandler serves GET /api/health — always returns 200 OK.
func HealthHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"}) //nolint:errcheck
	}
}

// ConfigResponse is the body of GET /api/config.
type ConfigResponse struct {
	CrawlerInterval string `json:"crawler_interval"`
	BFSProgressSec  int    `json:"bfs_progress_sec"`
}

// ConfigHandler serves GET /api/config — exposes runtime configuration to the frontend.
// crawlerInterval is the raw duration string from config (e.g. "10m").
// bfsProgressSec is the mid-crawl update interval from config (0 = disabled).
func ConfigHandler(crawlerInterval string, bfsProgressSec int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := ConfigResponse{
			CrawlerInterval: crawlerInterval,
			BFSProgressSec:  bfsProgressSec,
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}
