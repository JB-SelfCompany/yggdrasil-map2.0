// Package admin provides a JSON-RPC client for the Yggdrasil admin API.
// It supports both Unix socket (unix:///path) and TCP (tcp://host:port) endpoints.
package admin

import (
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"time"
)

// Client is a JSON-RPC client for the Yggdrasil admin socket.
type Client struct {
	endpoint string
	timeout  time.Duration
}

// New constructs a Client for the given endpoint with the specified dial timeout.
// endpoint formats:
//   - "unix:///var/run/yggdrasil/yggdrasil.sock"
//   - "tcp://[::1]:9001"
func New(endpoint string, timeout time.Duration) *Client {
	return &Client{endpoint: endpoint, timeout: timeout}
}

// ---------------------------------------------------------------------------
// Wire types
// ---------------------------------------------------------------------------

// request is the JSON body sent for every admin API call.
type request struct {
	Request   string      `json:"request"`
	Arguments interface{} `json:"arguments"`
	KeepAlive bool        `json:"keepalive"`
}

// envelope is the top-level JSON response wrapper.
type envelope struct {
	Status   string          `json:"status"`
	Error    string          `json:"error"`
	Response json.RawMessage `json:"response"`
}

// ---------------------------------------------------------------------------
// Response types
// ---------------------------------------------------------------------------

// SelfResponse contains information about the local Yggdrasil node.
type SelfResponse struct {
	BuildName      string `json:"build_name"`
	BuildVersion   string `json:"build_version"`
	Key            string `json:"key"`     // hex-encoded Ed25519 public key
	Address        string `json:"address"` // 200::/7 IPv6 address
	Subnet         string `json:"subnet"`
	RoutingEntries int    `json:"routing_entries"`
}

// Peer represents a single peer connection.
type Peer struct {
	Remote     string  `json:"remote"`
	Up         bool    `json:"up"`
	Inbound    bool    `json:"inbound"`
	Address    string  `json:"address"`
	Key        string  `json:"key"`
	Port       int     `json:"port"`
	Priority   int     `json:"priority"`
	Cost       uint64  `json:"cost"`
	BytesRecvd uint64  `json:"bytes_recvd"`
	BytesSent  uint64  `json:"bytes_sent"`
	RateRecvd  uint64  `json:"rate_recvd"`
	RateSent   uint64  `json:"rate_sent"`
	Uptime     float64 `json:"uptime"`
	Latency    int64   `json:"latency"`    // nanoseconds (time.Duration marshalled as int64, v0.5.12+)
	LastError  string  `json:"last_error"`
}

// PeersResponse is the response from the getPeers admin method.
type PeersResponse struct {
	Peers []Peer `json:"peers"`
}

// TreeEntry is a single entry in the spanning-tree response.
type TreeEntry struct {
	Address  string `json:"address"`
	Key      string `json:"key"`
	Parent   string `json:"parent"`
	Sequence uint64 `json:"sequence"`
}

// TreeResponse is the response from the getTree admin method.
type TreeResponse struct {
	Tree []TreeEntry `json:"tree"`
}

// RemoteKeysResponse is used when unmarshalling debug_remoteGetPeers responses.
// The wire format is {"<address>": {"keys": [...]}, ...}.
type RemoteKeysResponse struct {
	Keys []string `json:"keys"`
}

// RemoteTreeEntry is a single entry returned by debug_remoteGetTree.
type RemoteTreeEntry struct {
	Key    string `json:"key"`
	Parent string `json:"parent"`
}

// RemoteTreeResponse holds the results of debug_remoteGetTree for a remote node.
type RemoteTreeResponse struct {
	// Key is the hex-encoded public key of the queried node.
	Key     string            `json:"key"`
	Entries []RemoteTreeEntry `json:"entries"`
}

// ---------------------------------------------------------------------------
// Low-level transport
// ---------------------------------------------------------------------------

// dial opens a connection to the admin socket using the client's endpoint.
func (c *Client) dial() (net.Conn, error) {
	network, address, err := parseEndpoint(c.endpoint)
	if err != nil {
		return nil, err
	}
	conn, err := net.DialTimeout(network, address, c.timeout)
	if err != nil {
		return nil, fmt.Errorf("admin: dial %s: %w", c.endpoint, err)
	}
	return conn, nil
}

// parseEndpoint splits an endpoint string into network and address components
// suitable for net.Dial.
func parseEndpoint(endpoint string) (network, address string, err error) {
	switch {
	case strings.HasPrefix(endpoint, "unix://"):
		// unix:///absolute/path  →  network="unix", address="/absolute/path"
		addr := strings.TrimPrefix(endpoint, "unix://")
		if !strings.HasPrefix(addr, "/") {
			return "", "", fmt.Errorf("admin: unix socket path must be absolute, got %q", endpoint)
		}
		return "unix", addr, nil
	case strings.HasPrefix(endpoint, "tcp://"):
		return "tcp", strings.TrimPrefix(endpoint, "tcp://"), nil
	default:
		return "", "", fmt.Errorf("admin: unsupported endpoint scheme %q (use unix:// or tcp://)", endpoint)
	}
}

// Call executes a single JSON-RPC request against the admin socket and returns
// the raw JSON value of the "response" field on success.
func (c *Client) Call(method string, args interface{}) (json.RawMessage, error) {
	conn, err := c.dial()
	if err != nil {
		return nil, err
	}
	defer conn.Close()

	// Set a deadline for the entire exchange.
	if err := conn.SetDeadline(time.Now().Add(c.timeout)); err != nil {
		return nil, fmt.Errorf("admin: set deadline: %w", err)
	}

	req := request{
		Request:   method,
		Arguments: args,
		KeepAlive: false,
	}

	enc := json.NewEncoder(conn)
	if err := enc.Encode(req); err != nil {
		return nil, fmt.Errorf("admin: encode request for %q: %w", method, err)
	}

	var env envelope
	dec := json.NewDecoder(conn)
	if err := dec.Decode(&env); err != nil {
		return nil, fmt.Errorf("admin: decode response for %q: %w", method, err)
	}

	if env.Status != "success" {
		msg := env.Error
		if msg == "" {
			msg = "unknown error"
		}
		return nil, fmt.Errorf("admin: %q returned status %q: %s", method, env.Status, msg)
	}

	return env.Response, nil
}

// ---------------------------------------------------------------------------
// Typed wrappers
// ---------------------------------------------------------------------------

// GetSelf returns information about the local Yggdrasil node.
func (c *Client) GetSelf() (*SelfResponse, error) {
	raw, err := c.Call("getSelf", struct{}{})
	if err != nil {
		return nil, err
	}
	var resp SelfResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("admin: unmarshal getSelf: %w", err)
	}
	return &resp, nil
}

// GetPeers returns the list of directly connected peers.
func (c *Client) GetPeers() (*PeersResponse, error) {
	raw, err := c.Call("getPeers", struct{}{})
	if err != nil {
		return nil, err
	}
	var resp PeersResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("admin: unmarshal getPeers: %w", err)
	}
	return &resp, nil
}

// GetTree returns the current spanning-tree state.
func (c *Client) GetTree() (*TreeResponse, error) {
	raw, err := c.Call("getTree", struct{}{})
	if err != nil {
		return nil, err
	}
	var resp TreeResponse
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("admin: unmarshal getTree: %w", err)
	}
	return &resp, nil
}

// GetNodeInfo fetches the nodeinfo metadata for the node identified by key
// (hex-encoded Ed25519 public key). The response is an arbitrary JSON object
// defined by each node operator. A timeout of 8 seconds is recommended because
// Yggdrasil's internal nodeinfo timeout is 6 seconds.
func (c *Client) GetNodeInfo(key string) (map[string]interface{}, error) {
	raw, err := c.Call("getNodeInfo", map[string]string{"key": key})
	if err != nil {
		return nil, err
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return nil, fmt.Errorf("admin: unmarshal getNodeInfo: %w", err)
	}
	return resp, nil
}

// DebugRemoteGetPeers queries a remote node's peer list via the debug API.
// It returns the slice of hex-encoded public keys reported by the remote node.
func (c *Client) DebugRemoteGetPeers(key string) ([]string, error) {
	raw, err := c.Call("debug_remoteGetPeers", map[string]string{"key": key})
	if err != nil {
		return nil, err
	}

	// Wire shape: {"200:xxxx::1": {"keys": ["hex1", "hex2", ...]}}
	var outer map[string]RemoteKeysResponse
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("admin: unmarshal debug_remoteGetPeers: %w", err)
	}

	if len(outer) == 0 {
		return nil, fmt.Errorf("admin: debug_remoteGetPeers: empty response for key %q", key)
	}
	if len(outer) > 1 {
		return nil, fmt.Errorf("admin: debug_remoteGetPeers: unexpected %d entries in response for key %q", len(outer), key)
	}

	// There is exactly one entry whose key is the node's IPv6 address.
	for _, v := range outer {
		return v.Keys, nil
	}

	return nil, fmt.Errorf("admin: debug_remoteGetPeers: empty response for key %q", key)
}

// DebugRemoteGetTree queries a remote node's spanning-tree entries via the debug API.
func (c *Client) DebugRemoteGetTree(key string) (*RemoteTreeResponse, error) {
	raw, err := c.Call("debug_remoteGetTree", map[string]string{"key": key})
	if err != nil {
		return nil, err
	}

	// Wire shape: {"200:xxxx::1": {"key": "...", "parent": "..."}}
	// The outer map has one entry; its value contains key/parent fields.
	var outer map[string]struct {
		Key    string `json:"key"`
		Parent string `json:"parent"`
	}
	if err := json.Unmarshal(raw, &outer); err != nil {
		return nil, fmt.Errorf("admin: unmarshal debug_remoteGetTree: %w", err)
	}

	if len(outer) == 0 {
		return nil, fmt.Errorf("admin: debug_remoteGetTree: empty response for key %q", key)
	}
	if len(outer) > 1 {
		return nil, fmt.Errorf("admin: debug_remoteGetTree: unexpected %d entries in response for key %q", len(outer), key)
	}

	resp := &RemoteTreeResponse{Key: key}
	for _, v := range outer {
		resp.Entries = append(resp.Entries, RemoteTreeEntry{
			Key:    v.Key,
			Parent: v.Parent,
		})
	}

	if len(resp.Entries) == 0 {
		return nil, fmt.Errorf("admin: debug_remoteGetTree: empty response for key %q", key)
	}

	return resp, nil
}
