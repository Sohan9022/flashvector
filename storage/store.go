package storage

import (
	"context"
	"flashvector/metrics"
	"flashvector/vector"
	"flashvector/wal"
	"fmt"
	"sync"
	"encoding/binary"
	"math"
)

// Metadata is a simple key-value map for storing tags (e.g., "category": "news")
type Metadata map[string]string

// Store holds data and protects it with a lock
// Store holds the data, metadata, and the vector index
type Store struct {
	mu            sync.RWMutex
	data          map[string][]byte
	meta          map[string]Metadata
	wal           *wal.WAL
	index         vector.VectorIndex
	Metrics       *metrics.Metrics
	ctx           context.Context
	opCount       int
	snapshotEvery int
}

// NewStore creates and returns a pointer to a new store
func NewStore(ctx context.Context, w *wal.WAL) (*Store, error) {
	// Use 384 dimensions for Real World compatibility (e.g. all-MiniLM-L6-v2)
	centroids := vector.RandomCentroids(2, 384)
	index := vector.NewIVFIndex(centroids, 3)

	s := &Store{
		data:          make(map[string][]byte),
		meta:          make(map[string]Metadata), // <--- Initialize metadata map
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
func (s *Store) Set(key string, value []byte,metadata Metadata) error {
	// 1. Check for shutdown
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("store shutting down")
	default:
	}

	// 2. Write to WAL first
	if s.wal != nil {
		if err := s.wal.LogSet(key, value,metadata); err != nil {
			return err
		}
	}

	// 3. LOCK HERE (The only lock)
	s.mu.Lock()
	// NOTE: We DO NOT defer Unlock() here because we might unlock early for snapshots

	// 4. Update Memory (Calls internal function)
	s.ApplySet(key, value,metadata)

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
func (s *Store) Get(key string) ([]byte,Metadata,bool) {
	select {
	case <-s.ctx.Done():
		return nil,nil, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	meta := s.meta[key] // Retrieve metadata from the new map

	if ok && s.Metrics != nil {
		s.Metrics.IncReads()
	}

	return val,meta,ok
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

// changed from here down
func bytesToVector(b []byte) []float32 {
	// A float32 takes 4 bytes. So if we have 12 bytes, we have 3 floats.
	numFloats := len(b) / 4
	vec := make([]float32, numFloats)
	
	for i := 0; i < numFloats; i++ {
		// Read 4 bytes at a time
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		// Convert those bits back into a decimal number
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}

func (s *Store) VectorSearch(query []float32, k int,filterMap map[string]string) []vector.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Define the Bouncer Function
	predicate := func(id string) bool {
		// If no filter is requested, everyone is allowed
		if len(filterMap) == 0 {
			return true
		}

		// Get the metadata for this candidate ID
		meta, exists := s.meta[id]
		if !exists {
			return false // No metadata? Blocked.
		}

		// Check if it matches ALL criteria
		for key, requiredValue := range filterMap {
			if meta[key] != requiredValue {
				return false // Mismatch? Blocked.
			}
		}
		return true // Allowed!
	}

	return s.index.Search(query, k,predicate)
}



// --- INTERNAL FUNCTIONS (NO LOCKS) ---
// These are called by Set/Delete which ALREADY hold the lock.

func (s *Store) ApplySet(key string, value []byte,metadata map[string]string) {
	// REMOVED LOCK
	s.data[key] = value
	s.meta[key] = Metadata(metadata) // <--- Store the metadata in RAM
	s.index.Remove(key)
	vec := bytesToVector(value)
	if vec != nil {
		s.index.Add(key, vec)
	}
}

func (s *Store) ApplyDelete(key string) {
	// REMOVED LOCK
	delete(s.data, key)
	delete(s.meta, key) // <--- Remove metadata from RAM
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

