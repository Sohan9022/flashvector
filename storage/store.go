package storage

import (
	"context"
	"encoding/binary"
	"flashvector/metrics"
	"flashvector/vector"
	"flashvector/wal"
	"fmt"
	"math"
	"sync"
)

// Metadata is a simple key-value map for storing tags (e.g., "category": "news")
type Metadata map[string]string

// Store holds the data, metadata, inverted keyword index, and the vector index
type Store struct {
	mu            sync.RWMutex
	data          map[string][]byte
	meta          map[string]Metadata
	inverted      map[string][]string // Lexical Inverted Index: token -> list of doc IDs
	wal           *wal.WAL
	index         vector.VectorIndex
	Metrics       *metrics.Metrics
	ctx           context.Context
	opCount       int
	snapshotEvery int
}

// NewStore creates and returns a pointer to a new store
func NewStore(ctx context.Context, w *wal.WAL) (*Store, error) {
	// 4 Voronoi clusters with deterministic centroids
	centroids := vector.DeterministicCentroids(4, 384, 42)
	index := vector.NewIVFIndex(centroids, 4)

	s := &Store{
		data:          make(map[string][]byte),
		meta:          make(map[string]Metadata),
		inverted:      make(map[string][]string),
		wal:           w,
		index:         index,
		opCount:       0,
		snapshotEvery: 1000,
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
func (s *Store) Set(key string, value []byte, metadata Metadata) error {
	// 1. Check for shutdown
	select {
	case <-s.ctx.Done():
		return fmt.Errorf("store shutting down")
	default:
	}

	// 2. Write to WAL first
	if s.wal != nil {
		if err := s.wal.LogSet(key, value, metadata); err != nil {
			return err
		}
	}

	// 3. LOCK HERE
	s.mu.Lock()

	// 4. Update Memory
	s.ApplySet(key, value, metadata)

	// 5. Snapshot Trigger
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
		s.Metrics.IncWrites()
	}

	return nil
}

// Get retrieves a value for a given key
func (s *Store) Get(key string) ([]byte, Metadata, bool) {
	select {
	case <-s.ctx.Done():
		return nil, nil, false
	default:
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]
	meta := s.meta[key]

	if ok && s.Metrics != nil {
		s.Metrics.IncReads()
	}

	return val, meta, ok
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
	numFloats := len(b) / 4
	vec := make([]float32, numFloats)

	for i := 0; i < numFloats; i++ {
		bits := binary.LittleEndian.Uint32(b[i*4 : (i+1)*4])
		vec[i] = math.Float32frombits(bits)
	}
	return vec
}

func (s *Store) VectorSearch(query []float32, k int, filterMap map[string]string) []vector.Result {
	s.mu.RLock()
	defer s.mu.RUnlock()

	predicate := func(id string) bool {
		if len(filterMap) == 0 {
			return true
		}

		meta, exists := s.meta[id]
		if !exists {
			return false
		}

		for key, requiredValue := range filterMap {
			if meta[key] != requiredValue {
				return false
			}
		}
		return true
	}

	return s.index.Search(query, k, predicate)
}

// --- INTERNAL FUNCTIONS (Caller MUST hold lock, or use WithLock variants) ---

func (s *Store) ApplySet(key string, value []byte, metadata map[string]string) {
	s.data[key] = value
	s.meta[key] = Metadata(metadata)
	s.index.Remove(key)
	vec := bytesToVector(value)
	if vec != nil {
		s.index.Add(key, vec)
	}

	// Update Lexical Inverted Index
	// Index text if provided in metadata, or fallback to value if it's plaintext
	var textToIndex string
	if metadata != nil && metadata["text"] != "" {
		textToIndex = metadata["text"]
	}

	if textToIndex != "" {
		tokens := tokenize(textToIndex)
		for _, token := range tokens {
			s.inverted[token] = append(s.inverted[token], key)
		}
	}
}

func (s *Store) ApplyDelete(key string) {
	delete(s.data, key)
	delete(s.meta, key)
	s.index.Remove(key)

	// Remove document key from inverted index postings
	for token, docList := range s.inverted {
		filtered := make([]string, 0, len(docList))
		for _, docID := range docList {
			if docID != key {
				filtered = append(filtered, docID)
			}
		}
		if len(filtered) > 0 {
			s.inverted[token] = filtered
		} else {
			delete(s.inverted, token)
		}
	}
}

// Thread-safe wrapper for replication handlers
func (s *Store) ApplySetWithLock(key string, value []byte, metadata map[string]string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ApplySet(key, value, metadata)
}

// Thread-safe wrapper for replication handlers
func (s *Store) ApplyDeleteWithLock(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ApplyDelete(key)
}

func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal != nil {
		return s.wal.Close()
	}
	return nil
}
