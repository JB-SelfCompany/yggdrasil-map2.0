package crawler

import (
	"context"
	"log"
	"time"

	"github.com/JB-SelfCompany/yggmap/internal/graph"
)

// OnSnapshotFunc is called each time a new snapshot is produced by the
// scheduler. Implementations must not block for long; heavy work should be
// dispatched to a goroutine.
type OnSnapshotFunc func(snap *graph.GraphSnapshot)

// Scheduler drives periodic crawl cycles using a time.Ticker.
type Scheduler struct {
	crawler    *Crawler
	interval   time.Duration
	onSnapshot OnSnapshotFunc
	logger     *log.Logger
	triggerCh  chan struct{}
}

// NewScheduler creates a Scheduler that invokes c.Crawl every interval and
// calls onSnapshot with each resulting snapshot.
func NewScheduler(c *Crawler, interval time.Duration, onSnapshot OnSnapshotFunc, logger *log.Logger) *Scheduler {
	return &Scheduler{
		crawler:    c,
		interval:   interval,
		onSnapshot: onSnapshot,
		logger:     logger,
		triggerCh:  make(chan struct{}, 1),
	}
}

// TriggerNow requests an immediate crawl outside the normal schedule.
// Returns true if the trigger was enqueued, false if one is already pending.
func (s *Scheduler) TriggerNow() bool {
	select {
	case s.triggerCh <- struct{}{}:
		return true
	default:
		return false // crawl already pending
	}
}

// Run starts the crawl loop. It performs an immediate crawl on entry, then
// repeats every s.interval measured from the start of each crawl (not its end).
// If a crawl takes longer than the interval, accumulated ticks are drained so
// a second crawl does not start immediately after the first one finishes.
// Run blocks until ctx is cancelled.
func (s *Scheduler) Run(ctx context.Context) {
	// Start the ticker before the first crawl so that the interval is measured
	// from crawl-start rather than crawl-end.
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Crawl immediately on entry.
	s.runOnce(ctx)
	drainTicker(ticker)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
			drainTicker(ticker)
		case <-s.triggerCh:
			s.runOnce(ctx)
			drainTicker(ticker)
		}
	}
}

// drainTicker discards any ticks that accumulated in t.C while a crawl was
// running. This prevents a crawl that overran its interval from immediately
// scheduling another one.
func drainTicker(t *time.Ticker) {
	for {
		select {
		case <-t.C:
		default:
			return
		}
	}
}

// runOnce executes a single crawl and dispatches the snapshot to onSnapshot.
func (s *Scheduler) runOnce(ctx context.Context) {
	snap, err := s.crawler.Crawl(ctx)
	if err != nil {
		s.logger.Printf("scheduler: crawl error: %v", err)
		return
	}
	s.logger.Printf("scheduler: crawl complete — %d nodes, %d edges, %dms",
		snap.NodeCount(), snap.EdgeCount(), snap.CrawlDurationMS)
	if s.onSnapshot != nil {
		s.onSnapshot(snap)
	}
}
