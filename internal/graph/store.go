package graph

import "sync"

// Store is a thread-safe container for the most recent GraphSnapshot.
// It is safe for concurrent use by multiple goroutines.
type Store struct {
	mu       sync.RWMutex
	snapshot *GraphSnapshot
}

// NewStore allocates a ready-to-use Store with no snapshot.
func NewStore() *Store {
	return &Store{}
}

// Update atomically replaces the stored snapshot with snap.
func (s *Store) Update(snap *GraphSnapshot) {
	s.mu.Lock()
	s.snapshot = snap
	s.mu.Unlock()
}

// Get returns the current snapshot, or nil if no snapshot has been stored yet.
// The returned pointer is safe to read without additional locking because
// snapshots are immutable once published.
func (s *Store) Get() *GraphSnapshot {
	s.mu.RLock()
	snap := s.snapshot
	s.mu.RUnlock()
	return snap
}
