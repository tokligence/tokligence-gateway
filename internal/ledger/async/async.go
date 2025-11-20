package async

import (
	"context"
	"log"
	"sync"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/ledger"
)

// Store wraps a ledger.Store with asynchronous batch writes.
// Entries are queued in memory and written in batches to reduce database load.
// WARNING: Entries may be lost if the process crashes before flushing.
type Store struct {
	underlying    ledger.Store
	entryChan     chan ledger.Entry
	batchSize     int
	flushInterval time.Duration
	wg            sync.WaitGroup
	stopChan      chan struct{}
	logger        *log.Logger
}

// Config configures the async ledger behavior.
type Config struct {
	BatchSize     int           // Maximum entries per batch (default: 100, recommended: 1000-10000 for high QPS)
	FlushInterval time.Duration // Maximum time between flushes (default: 1s, recommended: 100ms-500ms for high QPS)
	ChannelBuffer int           // Channel buffer size (default: 10000, recommended: 100000-1000000 for 1M QPS)
	NumWorkers    int           // Number of parallel batch writers (default: 1, recommended: 10-50 for high QPS)
	Logger        *log.Logger   // Optional logger for diagnostics
}

// New wraps an existing ledger store with async batch writing.
func New(underlying ledger.Store, cfg Config) *Store {
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.FlushInterval <= 0 {
		cfg.FlushInterval = 1 * time.Second
	}
	if cfg.ChannelBuffer <= 0 {
		cfg.ChannelBuffer = 10000
	}
	if cfg.NumWorkers <= 0 {
		cfg.NumWorkers = 1
	}

	s := &Store{
		underlying:    underlying,
		entryChan:     make(chan ledger.Entry, cfg.ChannelBuffer),
		batchSize:     cfg.BatchSize,
		flushInterval: cfg.FlushInterval,
		stopChan:      make(chan struct{}),
		logger:        cfg.Logger,
	}

	// Start multiple worker goroutines for parallel processing
	for i := 0; i < cfg.NumWorkers; i++ {
		s.wg.Add(1)
		go s.batchWriter(i)
	}

	if s.logger != nil {
		s.logger.Printf("[async-ledger] started %d worker(s), batch_size=%d, flush_interval=%v, buffer=%d",
			cfg.NumWorkers, cfg.BatchSize, cfg.FlushInterval, cfg.ChannelBuffer)
	}

	return s
}

// batchWriter runs in a background goroutine, batching entries and writing them periodically.
func (s *Store) batchWriter(workerID int) {
	defer s.wg.Done()

	batch := make([]ledger.Entry, 0, s.batchSize)
	ticker := time.NewTicker(s.flushInterval)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}

		start := time.Now()

		// Write batch to underlying store
		ctx := context.Background()
		successCount := 0
		for _, entry := range batch {
			if err := s.underlying.Record(ctx, entry); err != nil {
				if s.logger != nil {
					s.logger.Printf("[async-ledger] worker-%d ERROR writing entry: %v", workerID, err)
				}
				// Continue processing other entries even if one fails
			} else {
				successCount++
			}
		}

		if s.logger != nil {
			elapsed := time.Since(start)
			s.logger.Printf("[async-ledger] worker-%d flushed %d/%d entries in %v (%.0f entries/sec)",
				workerID, successCount, len(batch), elapsed, float64(successCount)/elapsed.Seconds())
		}

		// Clear batch
		batch = batch[:0]
	}

	for {
		select {
		case entry := <-s.entryChan:
			batch = append(batch, entry)
			if len(batch) >= s.batchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case <-s.stopChan:
			// Drain remaining entries
			close(s.entryChan)
			for entry := range s.entryChan {
				batch = append(batch, entry)
				if len(batch) >= s.batchSize {
					flush()
				}
			}
			flush()
			return
		}
	}
}

// Record queues an entry for asynchronous writing (non-blocking).
func (s *Store) Record(ctx context.Context, entry ledger.Entry) error {
	select {
	case s.entryChan <- entry:
		return nil
	default:
		// Channel full - this is a warning condition
		if s.logger != nil {
			s.logger.Printf("[async-ledger] WARNING: channel full, dropping entry")
		}
		return nil // Don't block, drop the entry
	}
}

// Summary delegates to the underlying store (blocking operation).
func (s *Store) Summary(ctx context.Context, userID int64) (ledger.Summary, error) {
	return s.underlying.Summary(ctx, userID)
}

// ListRecent delegates to the underlying store (blocking operation).
func (s *Store) ListRecent(ctx context.Context, userID int64, limit int) ([]ledger.Entry, error) {
	return s.underlying.ListRecent(ctx, userID, limit)
}

// Close flushes remaining entries and closes the underlying store.
func (s *Store) Close() error {
	close(s.stopChan)
	s.wg.Wait()
	return s.underlying.Close()
}
