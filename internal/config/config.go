// Package config provides YAML-based configuration loading with auto-generated defaults.
// If no config file exists, all defaults are used without error.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config is the top-level application configuration.
type Config struct {
	Admin    AdminConfig    `yaml:"admin"`
	Crawler  CrawlerConfig  `yaml:"crawler"`
	Server   ServerConfig   `yaml:"server"`
	Storage  StorageConfig  `yaml:"storage"`
	Security SecurityConfig `yaml:"security"`
}

// AdminConfig holds settings for the Yggdrasil admin API connection.
type AdminConfig struct {
	// Socket is the admin socket address.
	// Supports unix:///path/to/socket and tcp://host:port.
	Socket string `yaml:"socket"`
}

// CrawlerConfig controls the network topology crawler behaviour.
type CrawlerConfig struct {
	// Interval is a duration string (e.g. "10m") between full crawl cycles.
	Interval string `yaml:"interval"`
	// EnableNodeInfo controls whether nodeinfo is fetched for each node.
	EnableNodeInfo bool `yaml:"enable_nodeinfo"`
	// NodeInfoConcurrency is the maximum number of concurrent nodeinfo requests.
	NodeInfoConcurrency int `yaml:"nodeinfo_concurrency"`
	// RemoteTimeoutSec is the per-call timeout in seconds for debug_remoteGetPeers.
	// Shorter than the nodeinfo timeout because remote peer queries are fast
	// when they work; failures should be detected quickly to keep BFS snappy.
	RemoteTimeoutSec int `yaml:"remote_timeout_sec"`
	// BFSProgressSec is the interval in seconds between intermediate map updates
	// sent to the UI during BFS. Lower values show faster progressive loading
	// but increase CPU and bandwidth. Set to 0 to disable mid-crawl updates.
	BFSProgressSec int `yaml:"bfs_progress_sec"`
}

// ServerConfig holds HTTP server settings.
type ServerConfig struct {
	// Bind is the IP address to listen on.
	Bind string `yaml:"bind"`
	// Port is the TCP port to listen on.
	Port int `yaml:"port"`
}

// StorageConfig controls optional persistence.
type StorageConfig struct {
	// DBPath is the path to the SQLite database file.
	// Empty string disables persistence.
	DBPath string `yaml:"db_path"`
}

// SecurityConfig holds settings for API authentication and network security.
type SecurityConfig struct {
	// AuthToken is the bearer token required for all API requests.
	// Leave empty to disable authentication (NOT recommended for public deployments).
	// Generate a secure token: openssl rand -hex 32
	AuthToken string `yaml:"auth_token"`
	// AllowedOrigins lists origins allowed for CORS and WebSocket access.
	// Example: ["https://yggmap.example.com"]
	// Empty list blocks all cross-origin requests.
	AllowedOrigins []string `yaml:"allowed_origins"`
	// RateLimitPerMin is the max HTTP requests per IP per minute (0 = disabled).
	RateLimitPerMin int `yaml:"rate_limit_per_min"`
	// MaxWSConnections caps concurrent WebSocket clients (0 = default 256).
	MaxWSConnections int `yaml:"max_ws_connections"`
}

// Validate checks that the configuration is valid.
// It verifies that Crawler.Interval is a valid duration string.
func (c *Config) Validate() error {
	if _, err := time.ParseDuration(c.Crawler.Interval); err != nil {
		return fmt.Errorf("config: invalid crawler interval %q: %w", c.Crawler.Interval, err)
	}
	return nil
}

// Default returns a Config populated with sensible defaults.
// All fields are valid for immediate use without a config file.
func Default() *Config {
	return &Config{
		Admin: AdminConfig{
			Socket: "unix:///var/run/yggdrasil/yggdrasil.sock",
		},
		Crawler: CrawlerConfig{
			Interval:            "10m",
			EnableNodeInfo:      true,
			NodeInfoConcurrency: 4,
			RemoteTimeoutSec:    5,
			BFSProgressSec:      5,
		},
		Server: ServerConfig{
			Bind: "127.0.0.1",
			Port: 8080,
		},
		Storage: StorageConfig{
			DBPath: "~/.yggmap/graph.db",
		},
		Security: SecurityConfig{
			AllowedOrigins:   []string{},
			RateLimitPerMin:  60,
			MaxWSConnections: 256,
		},
	}
}

// Load reads a YAML config file at path and merges it over the defaults.
// If the file does not exist, the pure defaults are returned without error.
// Home-directory expansion (~/) is performed on path before opening.
func Load(path string) (*Config, error) {
	var err error
	path, err = expandPath(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Default(), nil
		}
		return nil, fmt.Errorf("config: read %q: %w", path, err)
	}

	// Start from defaults so missing YAML keys keep their default values.
	cfg := Default()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("config: parse %q: %w", path, err)
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// Save writes cfg to path as YAML, creating parent directories as needed.
// The file is written with 0600 permissions.
func Save(cfg *Config, path string) error {
	var err error
	path, err = expandPath(path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("config: create directory %q: %w", dir, err)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("config: marshal: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("config: write %q: %w", path, err)
	}

	return nil
}

// expandPath replaces a leading ~ with the current user's home directory.
func expandPath(p string) (string, error) {
	if len(p) == 0 || p[0] != '~' {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("config: resolve home directory: %w", err)
	}
	return filepath.Join(home, p[1:]), nil
}
