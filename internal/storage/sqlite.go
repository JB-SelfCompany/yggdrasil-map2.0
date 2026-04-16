// Package storage provides SQLite-backed persistence for graph snapshots.
package storage

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/JB-SelfCompany/yggmap/internal/graph"
	_ "modernc.org/sqlite"
)

// DB wraps a SQLite database for snapshot persistence.
type DB struct {
	db     *sql.DB
	logger *log.Logger
	wg     sync.WaitGroup // tracks in-flight async saves
}

// Open opens (or creates) the SQLite database at path, runs schema migrations,
// and returns a ready-to-use DB. The caller must call Close() when done.
func Open(path string, logger *log.Logger) (*DB, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("storage: open %q: %w", path, err)
	}

	// Use WAL mode for better concurrent read performance.
	if _, err := db.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: set WAL mode: %w", err)
	}
	if _, err := db.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		db.Close()
		return nil, fmt.Errorf("storage: enable foreign keys: %w", err)
	}

	// Checkpoint any WAL data left by a previous process that exited without
	// calling Close (e.g. killed by SIGKILL). RESTART mode works even when
	// there are concurrent readers and merges all committed WAL frames into
	// the main database file so LoadLatest sees the full dataset.
	if _, err := db.Exec(`PRAGMA wal_checkpoint(RESTART)`); err != nil {
		if logger != nil {
			logger.Printf("storage: wal_checkpoint on open: %v (non-fatal)", err)
		}
	}

	d := &DB{db: db, logger: logger}
	if err := d.migrate(); err != nil {
		db.Close()
		return nil, err
	}
	return d, nil
}

// migrate creates the schema if it does not exist.
func (d *DB) migrate() error {
	const schema = `
CREATE TABLE IF NOT EXISTS snapshots (
  id               INTEGER PRIMARY KEY AUTOINCREMENT,
  timestamp        TEXT    NOT NULL,
  crawl_duration_ms INTEGER NOT NULL,
  created_at       DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS snapshot_nodes (
  snapshot_id  INTEGER NOT NULL,
  key          TEXT    NOT NULL,
  address      TEXT    NOT NULL,
  name         TEXT,
  os           TEXT,
  build_ver    TEXT,
  in_tree      BOOLEAN NOT NULL DEFAULT 0,
  first_seen   TEXT,
  last_seen    TEXT,
  latency_ns   INTEGER NOT NULL DEFAULT 0,
  PRIMARY KEY (snapshot_id, key),
  FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS snapshot_edges (
  snapshot_id  INTEGER NOT NULL,
  src_key      TEXT    NOT NULL,
  dst_key      TEXT    NOT NULL,
  is_tree_edge BOOLEAN NOT NULL DEFAULT 0,
  up           BOOLEAN NOT NULL DEFAULT 0,
  cost         INTEGER NOT NULL DEFAULT 0,
  inbound      BOOLEAN NOT NULL DEFAULT 0,
  FOREIGN KEY (snapshot_id) REFERENCES snapshots(id) ON DELETE CASCADE
);
`
	if _, err := d.db.Exec(schema); err != nil {
		return fmt.Errorf("storage: migrate: %w", err)
	}
	return nil
}

// Save persists a snapshot to the database inside a single transaction.
func (d *DB) Save(snap *graph.GraphSnapshot) error {
	tx, err := d.db.Begin()
	if err != nil {
		return fmt.Errorf("storage: begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	res, err := tx.Exec(
		`INSERT INTO snapshots (timestamp, crawl_duration_ms) VALUES (?, ?)`,
		snap.Timestamp.UTC().Format(time.RFC3339Nano),
		snap.CrawlDurationMS,
	)
	if err != nil {
		return fmt.Errorf("storage: insert snapshot: %w", err)
	}
	snapID, err := res.LastInsertId()
	if err != nil {
		return fmt.Errorf("storage: last insert id: %w", err)
	}

	nodeStmt, err := tx.Prepare(`
		INSERT INTO snapshot_nodes
		  (snapshot_id, key, address, name, os, build_ver, in_tree, first_seen, last_seen, latency_ns)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("storage: prepare node stmt: %w", err)
	}
	defer nodeStmt.Close()

	for _, n := range snap.Nodes {
		firstSeen := n.FirstSeen.UTC().Format(time.RFC3339Nano)
		lastSeen := n.LastSeen.UTC().Format(time.RFC3339Nano)
		if _, err := nodeStmt.Exec(
			snapID, n.Key, n.Address, n.Name, n.OS, n.BuildVer,
			n.InTree, firstSeen, lastSeen, int64(n.Latency),
		); err != nil {
			return fmt.Errorf("storage: insert node %q: %w", n.Key, err)
		}
	}

	edgeStmt, err := tx.Prepare(`
		INSERT INTO snapshot_edges
		  (snapshot_id, src_key, dst_key, is_tree_edge, up, cost, inbound)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return fmt.Errorf("storage: prepare edge stmt: %w", err)
	}
	defer edgeStmt.Close()

	for _, e := range snap.Edges {
		if _, err := edgeStmt.Exec(
			snapID, e.SrcKey, e.DstKey, e.IsTreeEdge, e.Up, e.Cost, e.Inbound,
		); err != nil {
			return fmt.Errorf("storage: insert edge: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("storage: commit: %w", err)
	}

	if d.logger != nil {
		d.logger.Printf("storage: saved snapshot id=%d nodes=%d edges=%d",
			snapID, len(snap.Nodes), len(snap.Edges))
	}
	return nil
}

// LoadLatest returns the most recent snapshot from the database.
// Returns (nil, nil) if no snapshots are stored yet.
func (d *DB) LoadLatest() (*graph.GraphSnapshot, error) {
	row := d.db.QueryRow(`SELECT id, timestamp, crawl_duration_ms FROM snapshots ORDER BY id DESC LIMIT 1`)

	var (
		snapID          int64
		timestampStr    string
		crawlDurationMS int64
	)
	if err := row.Scan(&snapID, &timestampStr, &crawlDurationMS); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("storage: load latest snapshot header: %w", err)
	}

	ts, err := time.Parse(time.RFC3339Nano, timestampStr)
	if err != nil {
		return nil, fmt.Errorf("storage: parse snapshot timestamp %q: %w", timestampStr, err)
	}

	snap := &graph.GraphSnapshot{
		Timestamp:       ts,
		CrawlDurationMS: crawlDurationMS,
		Nodes:           make(map[string]*graph.GraphNode),
	}

	// Load nodes.
	nodeRows, err := d.db.Query(`
		SELECT key, address, name, os, build_ver, in_tree, first_seen, last_seen, latency_ns
		FROM snapshot_nodes WHERE snapshot_id = ?`, snapID)
	if err != nil {
		return nil, fmt.Errorf("storage: query nodes: %w", err)
	}
	defer nodeRows.Close()

	for nodeRows.Next() {
		var (
			n                     graph.GraphNode
			firstSeenStr, lastSeenStr string
			latencyNS             int64
		)
		if err := nodeRows.Scan(
			&n.Key, &n.Address, &n.Name, &n.OS, &n.BuildVer,
			&n.InTree, &firstSeenStr, &lastSeenStr, &latencyNS,
		); err != nil {
			return nil, fmt.Errorf("storage: scan node: %w", err)
		}
		n.FirstSeen, _ = time.Parse(time.RFC3339Nano, firstSeenStr)
		n.LastSeen, _ = time.Parse(time.RFC3339Nano, lastSeenStr)
		n.Latency = time.Duration(latencyNS)
		snap.Nodes[n.Key] = &n
	}
	if err := nodeRows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate nodes: %w", err)
	}

	// Load edges.
	edgeRows, err := d.db.Query(`
		SELECT src_key, dst_key, is_tree_edge, up, cost, inbound
		FROM snapshot_edges WHERE snapshot_id = ?`, snapID)
	if err != nil {
		return nil, fmt.Errorf("storage: query edges: %w", err)
	}
	defer edgeRows.Close()

	for edgeRows.Next() {
		var e graph.GraphEdge
		if err := edgeRows.Scan(&e.SrcKey, &e.DstKey, &e.IsTreeEdge, &e.Up, &e.Cost, &e.Inbound); err != nil {
			return nil, fmt.Errorf("storage: scan edge: %w", err)
		}
		snap.Edges = append(snap.Edges, e)
	}
	if err := edgeRows.Err(); err != nil {
		return nil, fmt.Errorf("storage: iterate edges: %w", err)
	}

	if d.logger != nil {
		d.logger.Printf("storage: loaded snapshot id=%d nodes=%d edges=%d",
			snapID, len(snap.Nodes), len(snap.Edges))
	}
	return snap, nil
}

// Prune deletes all but the most recent `keep` snapshots.
// Cascading deletes handle nodes and edges automatically.
func (d *DB) Prune(keep int) error {
	if keep <= 0 {
		keep = 1
	}
	_, err := d.db.Exec(`
		DELETE FROM snapshots
		WHERE id NOT IN (
			SELECT id FROM snapshots ORDER BY id DESC LIMIT ?
		)`, keep)
	if err != nil {
		return fmt.Errorf("storage: prune: %w", err)
	}
	return nil
}

// SaveAsync persists snap in a background goroutine, then prunes old snapshots.
// Call Wait before Close to ensure all in-flight saves complete.
func (d *DB) SaveAsync(snap *graph.GraphSnapshot) {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()
		if err := d.Save(snap); err != nil && d.logger != nil {
			d.logger.Printf("storage: async save: %v", err)
		}
		if err := d.Prune(5); err != nil && d.logger != nil {
			d.logger.Printf("storage: async prune: %v", err)
		}
	}()
}

// Wait blocks until all in-flight SaveAsync goroutines have finished.
func (d *DB) Wait() {
	d.wg.Wait()
}

// Close waits for in-flight saves, checkpoints the WAL into the main database
// file, then closes the underlying connection.
func (d *DB) Close() error {
	d.wg.Wait()
	// Merge all WAL frames into the main file and truncate the WAL so the next
	// open starts clean.
	if _, err := d.db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`); err != nil && d.logger != nil {
		d.logger.Printf("storage: wal_checkpoint on close: %v (non-fatal)", err)
	}
	return d.db.Close()
}
