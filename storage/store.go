package storage

import (
	"context"
	"flashvector/metrics"
	"flashvector/vector"
	"flashvector/wal"
	"fmt"
	"sync"
)

// Store holds data and protects it with a lock
type Store struct {
	mu            sync.RWMutex
	data          map[string][]byte
	wal           *wal.WAL
	index         vector.VectorIndex
	Metrics       *metrics.Metrics
	ctx           context.Context
	opCount       int
	snapshotEvery int
}

// NewStore creates and returns a pointer to a new store
func NewStore(ctx context.Context, w *wal.WAL) (*Store, error) {
	centroids := vector.RandomCentroids(64, 384)
	index := vector.NewIVFIndex(centroids, 3)

	s := &Store{
		data:          make(map[string][]byte),
		wal:           w,
		index:         index,
		opCount:       0,
		snapshotEvery: 1000, // Set to 1000 for real use (10 was for testing)
		ctx:           ctx,
	}

	// 1. Try to load Snapshot first
	if err := s.LoadSnapshot("data.snap"); err != nil {
		// It's okay if snapshot doesn't exist yet
	}

	// 2. Replay WAL (only events AFTER the snapshot)
	if w != nil {
		if err := w.Replay(s); err != nil {
			return nil, err
		}
	}

	return s, nil
}

// Set stores a value for a given key
func (s *Store) Set(key string, value []byte) error {
	// 1. Check for shutdown
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("store shutting down")
	default:
	}

	// 2. Write to WAL first
	if s.wal != nil {
		if err := s.wal.LogSet(key, value); err != nil {
			return err
		}
	}

	// 3. LOCK HERE (The only lock)
	s.mu.Lock()
	// NOTE: We DO NOT defer Unlock() here because we might unlock early for snapshots

	// 4. Update Memory (Calls internal function)
	s.ApplySet(key, value)

	// 5. Snapshot Trigger
	s.opCount++
	if s.wal != nil && s.opCount%s.snapshotEvery == 0 {
		// UNLOCK BEFORE SNAPSHOT to avoid deadlock
		s.mu.Unlock()

		if err := s.SaveSnapShot("data.snap"); err != nil {
			fmt.Printf("Error saving snapshot: %v\n", err)
		} else {
			if err := s.wal.Reset(); err != nil {
				fmt.Printf("Error resetting WAL: %v\n", err)
			}
		}
	} else {
		// Normal unlock
		s.mu.Unlock()
	}

	if s.Metrics != nil {
		s.Metrics.IncWrites()
	}

	return nil
}

// Get retrieves a value for a given key
func (s *Store) Get(key string) ([]byte, bool) {
	select {
	case <-s.ctx.Done():
		return nil, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]

	if ok && s.Metrics != nil {
		s.Metrics.IncReads()
	}

	return val, ok
}

// Delete removes a value for a given key
func (s *Store) Delete(key string) error {
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("store shutting down")
	default:
	}

	if s.wal != nil {
		if err := s.wal.LogDelete(key); err != nil {
			return err
		}
	}

	s.mu.Lock()
	// No defer here either

	s.ApplyDelete(key)

	s.opCount++
	if s.wal != nil && s.opCount%s.snapshotEvery == 0 {
		s.mu.Unlock()
		if err := s.SaveSnapShot("data.snap"); err != nil {
			fmt.Printf("Error saving snapshot: %v\n", err)
		} else {
			if err := s.wal.Reset(); err != nil {
				fmt.Printf("Error resetting WAL: %v\n", err)
			}
		}
	} else {
		s.mu.Unlock()
	}

	if s.Metrics != nil {
		s.Metrics.IncDeletes()
	}

	return nil
}

func bytesToVector(b []byte) []float32 {
	vec := make([]float32, len(b))
	for i := range b {
		vec[i] = float32(b[i])
	}
	return vec
}

func (s *Store) VectorSearch(query []float32, k int) []vector.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.index.Search(query, k)
}

// --- INTERNAL FUNCTIONS (NO LOCKS) ---
// These are called by Set/Delete which ALREADY hold the lock.

func (s *Store) ApplySet(key string, value []byte) {
	// REMOVED LOCK
	s.data[key] = value
	s.index.Remove(key)
	s.index.Add(key, bytesToVector(value))
	// REMOVED UNLOCK
}

func (s *Store) ApplyDelete(key string) {
	// REMOVED LOCK
	delete(s.data, key)
	s.index.Remove(key)
	// REMOVED UNLOCK
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		return s.wal.Close()
	}
	return nil
}