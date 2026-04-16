// Package api provides the HTTP API and WebSocket hub for yggmap.
package api

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// Keepalive timing constants.
// Mobile browsers (especially Android Chrome) aggressively kill idle TCP
// connections. A 20-second ping interval ensures the connection is refreshed
// well within any OS-level idle timeout, and the 45-second read deadline gives
// the client 2+ ping intervals to respond before the server drops the link.
const (
	pongWait   = 45 * time.Second // read deadline; reset on every pong received
	pingPeriod = 20 * time.Second // must be less than pongWait
	writeWait  = 10 * time.Second // write deadline for each individual frame
)

// WSMessage is the envelope sent to every WebSocket client.
type WSMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// CrawlProgress describes the status of the most recent crawl cycle.
type CrawlProgress struct {
	Running  bool  `json:"running"`
	Visited  int   `json:"visited"`      // nodes in current snapshot
	Duration int64 `json:"duration_ms"`
}

// wsClient is a single connected WebSocket peer.
type wsClient struct {
	hub  *Hub
	conn *websocket.Conn
	send chan []byte
}

// Hub manages all active WebSocket connections and serialises broadcasts.
type Hub struct {
	clients        map[*wsClient]bool
	broadcast      chan []byte
	register       chan *wsClient
	unregister     chan *wsClient
	mu             sync.Mutex
	latestSnapshot []byte // cached for immediate delivery to new clients
	logger         *log.Logger
	done           chan struct{}
	connCount      int32
	maxConn        int
	allowedOrigins []string
}

// NewHub allocates a Hub ready to be started with Run.
// allowedOrigins lists permitted WebSocket origins (empty = same-origin only).
// maxConn caps concurrent connections (0 = default 256).
func NewHub(logger *log.Logger, allowedOrigins []string, maxConn int) *Hub {
	if logger == nil {
		logger = log.Default()
	}
	if maxConn <= 0 {
		maxConn = 256
	}
	return &Hub{
		clients:        make(map[*wsClient]bool),
		broadcast:      make(chan []byte, 64),
		register:       make(chan *wsClient, 16),
		unregister:     make(chan *wsClient, 16),
		logger:         logger,
		done:           make(chan struct{}),
		maxConn:        maxConn,
		allowedOrigins: allowedOrigins,
	}
}

// Run starts the hub event loop. Stop it by calling Shutdown.
func (h *Hub) Run() {
	for {
		select {
		case <-h.done:
			h.mu.Lock()
			for c := range h.clients {
				close(c.send)
				delete(h.clients, c)
			}
			h.mu.Unlock()
			return

		case c := <-h.register:
			h.mu.Lock()
			h.clients[c] = true
			h.mu.Unlock()

		case c := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[c]; ok {
				delete(h.clients, c)
				close(c.send)
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			h.mu.Lock()
			for c := range h.clients {
				select {
				case c.send <- msg:
				default:
					// Slow client: drop and disconnect.
					delete(h.clients, c)
					close(c.send)
				}
			}
			h.mu.Unlock()
		}
	}
}

// Shutdown signals the hub to close all client connections and exit Run.
func (h *Hub) Shutdown() {
	close(h.done)
}

// Broadcast encodes msg as JSON and enqueues for all clients.
// Non-blocking: drops oldest message if channel is full.
func (h *Hub) Broadcast(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	select {
	case h.broadcast <- data:
	default:
		// Channel full: evict oldest and retry once.
		select {
		case <-h.broadcast:
		default:
		}
		select {
		case h.broadcast <- data:
		default:
		}
	}
	return nil
}

// BroadcastSnapshot is like Broadcast but also caches the payload for new clients.
func (h *Hub) BroadcastSnapshot(msg interface{}) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	h.mu.Lock()
	h.latestSnapshot = data
	h.mu.Unlock()
	select {
	case h.broadcast <- data:
	default:
		// Channel full: evict oldest and retry once.
		select {
		case <-h.broadcast:
		default:
		}
		select {
		case h.broadcast <- data:
		default:
		}
	}
	return nil
}

// checkOrigin validates the WebSocket Origin header.
// Public mode (empty allowedOrigins): any origin accepted — service is open.
// Restricted mode (non-empty allowedOrigins): same-origin + explicit list only.
// No Origin header (non-browser clients) is always allowed.
func (h *Hub) checkOrigin(r *http.Request) bool {
	// Public mode: empty list means no restriction.
	if len(h.allowedOrigins) == 0 {
		return true
	}
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	for _, a := range h.allowedOrigins {
		au, err := url.Parse(a)
		if err != nil {
			continue
		}
		if strings.EqualFold(u.Host, au.Host) && strings.EqualFold(u.Scheme, au.Scheme) {
			return true
		}
	}
	return false
}

// ServeWS upgrades the HTTP connection to WebSocket and manages the client lifecycle.
func (h *Hub) ServeWS(w http.ResponseWriter, r *http.Request) {
	if atomic.LoadInt32(&h.connCount) >= int32(h.maxConn) {
		http.Error(w, "too many WebSocket connections", http.StatusServiceUnavailable)
		return
	}

	upgrader := websocket.Upgrader{
		ReadBufferSize:  1024,
		WriteBufferSize: 4096,
		CheckOrigin:     h.checkOrigin,
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Printf("ws: upgrade error: %v", err)
		return
	}

	atomic.AddInt32(&h.connCount, 1)

	// Grab the cached snapshot before registering the client. Registering
	// first would allow a concurrent BroadcastSnapshot to enqueue a message
	// while we are also about to enqueue the same cached payload — resulting
	// in duplicate delivery. Reading the cache before registration guarantees
	// we enqueue exactly one copy of the latest data.
	h.mu.Lock()
	snap := h.latestSnapshot
	h.mu.Unlock()

	c := &wsClient{
		hub:  h,
		conn: conn,
		send: make(chan []byte, 256),
	}

	// Pre-fill the send buffer with the cached snapshot so writePump delivers
	// it as the very first frame — before any ping and before the next crawl.
	// This must happen before h.register so that a concurrent Broadcast cannot
	// interleave a newer snapshot ahead of the cached one in the channel.
	if snap != nil {
		c.send <- snap
	}

	h.register <- c

	go c.writePump()
	go c.readPump()
}

// readPump drains inbound frames (we don't process client messages) and
// unregisters the client on close or error.
func (c *wsClient) readPump() {
	defer func() {
		atomic.AddInt32(&c.hub.connCount, -1)
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, _, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err,
				websocket.CloseNormalClosure,
				websocket.CloseGoingAway,
				websocket.CloseAbnormalClosure,
			) {
				c.hub.logger.Printf("ws: read error: %v", err)
			}
			break
		}
	}
}

// writePump delivers queued messages to the client and sends periodic pings.
// The initial cached snapshot (if any) is sent first so the client sees current
// graph state immediately on connect without waiting for the next crawl cycle.
func (c *wsClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case msg, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub closed the channel.
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
