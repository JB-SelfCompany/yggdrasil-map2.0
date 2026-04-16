package api

import (
	"bufio"
	"crypto/subtle"
	"fmt"
	"log"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	"github.com/JB-SelfCompany/yggmap/internal/config"
	"github.com/JB-SelfCompany/yggmap/internal/graph"
	"github.com/JB-SelfCompany/yggmap/internal/web"
)

// responseWriter wraps http.ResponseWriter to capture the status code for logging.
// It forwards http.Hijacker so WebSocket upgrades work through the middleware chain.
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

// Hijack forwards to the underlying ResponseWriter if it implements http.Hijacker.
// Required for WebSocket upgrades via gorilla/websocket.
func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	h, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("api: underlying ResponseWriter does not implement http.Hijacker")
	}
	return h.Hijack()
}

// ---------------------------------------------------------------------------
// Rate limiter — fixed-window per /64 IPv6 prefix (stdlib only, no external deps)
// ---------------------------------------------------------------------------

type rateBucket struct {
	count    int
	windowAt time.Time
}

type ipRateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*rateBucket
	limit   int // requests per minute
}

func newIPRateLimiter(perMin int) *ipRateLimiter {
	return &ipRateLimiter{
		buckets: make(map[string]*rateBucket),
		limit:   perMin,
	}
}

// allow returns true if the request from ip is within the rate limit.
func (rl *ipRateLimiter) allow(ip string) bool {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[ip]
	if !ok || now.Sub(b.windowAt) >= time.Minute {
		rl.buckets[ip] = &rateBucket{count: 1, windowAt: now}
		return true
	}
	if b.count >= rl.limit {
		return false
	}
	b.count++
	return true
}

// cleanup removes expired buckets. Call periodically in a background goroutine.
func (rl *ipRateLimiter) cleanup() {
	now := time.Now()
	rl.mu.Lock()
	defer rl.mu.Unlock()
	for ip, b := range rl.buckets {
		if now.Sub(b.windowAt) >= time.Minute {
			delete(rl.buckets, ip)
		}
	}
}

// clientIP extracts the request IP and, for IPv6 addresses, buckets it by /64
// prefix to prevent per-address evasion of rate limits.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return host
	}
	ip4 := ip.To4()
	if ip4 != nil {
		return ip4.String()
	}
	// IPv6: mask to /64 prefix.
	mask := net.CIDRMask(64, 128)
	return ip.Mask(mask).String()
}

// ---------------------------------------------------------------------------
// Server
// ---------------------------------------------------------------------------

// Server holds the HTTP mux, hub, and dependencies for the yggmap API.
type Server struct {
	mux    *http.ServeMux
	hub    *Hub
	store  *graph.Store
	cfg    *config.ServerConfig
	sec    *config.SecurityConfig
	rl     *ipRateLimiter
	logger *log.Logger
}

// NewServer creates a Server. Call Setup before using the server.
func NewServer(store *graph.Store, hub *Hub, cfg *config.ServerConfig, sec *config.SecurityConfig, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.Default()
	}
	var rl *ipRateLimiter
	if sec != nil && sec.RateLimitPerMin > 0 {
		rl = newIPRateLimiter(sec.RateLimitPerMin)
	}
	s := &Server{
		mux:    http.NewServeMux(),
		hub:    hub,
		store:  store,
		cfg:    cfg,
		sec:    sec,
		rl:     rl,
		logger: logger,
	}
	// Background goroutine to clean expired rate-limit buckets every minute.
	if rl != nil {
		go func() {
			t := time.NewTicker(time.Minute)
			defer t.Stop()
			for range t.C {
				rl.cleanup()
			}
		}()
	}
	return s
}

// Setup registers all routes and wraps the mux with middleware.
// triggerCrawl is called when POST /api/crawl is received; it may be nil.
// crawlerInterval is the raw duration string from config (e.g. "10m"), forwarded to /api/config.
// bfsProgressSec is the mid-crawl update interval in seconds (0 = disabled), forwarded to /api/config.
func (s *Server) Setup(triggerCrawl func(), crawlerInterval string, bfsProgressSec int) {
	s.mux.HandleFunc("/api/graph", GraphHandler(s.store))
	s.mux.HandleFunc("/api/graph/node/", NodeHandler(s.store))
	s.mux.HandleFunc("/api/status", StatusHandler(s.store))
	s.mux.HandleFunc("/api/crawl", TriggerCrawlHandler(triggerCrawl))
	s.mux.HandleFunc("/api/health", HealthHandler())
	s.mux.HandleFunc("/api/config", ConfigHandler(crawlerInterval, bfsProgressSec))
	s.mux.HandleFunc("/ws", s.hub.ServeWS)

	// Serve embedded Vue frontend with SPA fallback for history-mode routing.
	fileServer := http.FileServer(web.DistFileSystem())
	s.mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Try to open the requested path from the embedded FS.
		f, err := web.DistFileSystem().Open(r.URL.Path)
		if err != nil {
			// File not found — serve index.html so Vue Router can handle the path.
			r2 := *r
			r2.URL.Path = "/"
			fileServer.ServeHTTP(w, &r2)
			return
		}
		f.Close()
		fileServer.ServeHTTP(w, r)
	})
}

// Handler returns the http.Handler with the full middleware chain applied.
func (s *Server) Handler() http.Handler {
	return s.recoverPanic(
		s.logRequest(
			s.maxBytes(
				s.rateLimit(
					s.authMiddleware(
						s.securityHeaders(
							s.cors(s.mux),
						),
					),
				),
			),
		),
	)
}

// ServeHTTP implements http.Handler so Server itself can be used directly.
func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.Handler().ServeHTTP(w, r)
}

// Addr returns the listen address derived from the ServerConfig.
func (s *Server) Addr() string {
	return fmt.Sprintf("%s:%d", s.cfg.Bind, s.cfg.Port)
}

// ---------------------------------------------------------------------------
// Middleware
// ---------------------------------------------------------------------------

func (s *Server) recoverPanic(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.logger.Printf("api: panic recovered: %v\n%s", rec, debug.Stack())
				http.Error(w, "internal server error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) logRequest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		s.logger.Printf("[%s] %s → %d in %dms",
			r.Method,
			r.URL.Path,
			rw.status,
			time.Since(start).Milliseconds(),
		)
	})
}

// maxBytes limits request body size to 4 KiB to prevent body-based DoS.
func (s *Server) maxBytes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4<<10)
		next.ServeHTTP(w, r)
	})
}

// rateLimit enforces per-IP fixed-window rate limiting when configured.
// Skipped entirely when RateLimitPerMin is 0.
func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if s.rl != nil {
			ip := clientIP(r)
			if !s.rl.allow(ip) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// authMiddleware enforces Bearer token authentication when AuthToken is configured.
// Exempt paths: /api/health (liveness probes), OPTIONS preflight.
// WebSocket connections may pass the token as ?token= query parameter.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth when no token is configured.
		if s.sec == nil || s.sec.AuthToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Always allow health checks and OPTIONS preflight.
		if r.URL.Path == "/api/health" || r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		token := s.sec.AuthToken
		provided := ""

		// Extract from Authorization: Bearer <token> header.
		if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
			provided = strings.TrimPrefix(auth, "Bearer ")
		}

		// WebSocket clients cannot set custom headers in the browser; accept ?token= as fallback.
		if provided == "" {
			provided = r.URL.Query().Get("token")
		}

		if provided == "" || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="yggmap"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// securityHeaders applies a comprehensive set of security response headers.
// API routes additionally receive Cache-Control: no-store.
func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("Permissions-Policy", "geolocation=(), microphone=(), camera=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Content-Security-Policy",
			"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self' ws: wss:")

		// API responses must never be cached by proxies or browsers.
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
		}

		next.ServeHTTP(w, r)
	})
}

// cors handles cross-origin requests.
// Empty AllowedOrigins = public mode (Access-Control-Allow-Origin: *).
// Non-empty AllowedOrigins = restricted mode (explicit list only).
// The Vary: Origin header is always set so caches handle origin-specific responses correctly.
func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		w.Header().Add("Vary", "Origin")

		if origin != "" && s.sec != nil {
			if len(s.sec.AllowedOrigins) == 0 {
				// Public mode: no restriction, allow any origin.
				w.Header().Set("Access-Control-Allow-Origin", "*")
				w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			} else {
				// Restricted mode: only allow listed origins.
				for _, allowed := range s.sec.AllowedOrigins {
					if strings.EqualFold(origin, allowed) {
						w.Header().Set("Access-Control-Allow-Origin", origin)
						w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
						w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
						break
					}
				}
			}
		}

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}
