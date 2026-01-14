package storage

import (
	"context"
	"flashvector/metrics"
	"flashvector/vector"
	"flashvector/wal"
	"fmt"
	"sync"
)

// store is a structure that hold data and protects it with a lock

type Store struct {
	mu    sync.RWMutex
	data  map[string][]byte
	wal   *wal.WAL
	index vector.VectorIndex
	Metrics *metrics.Metrics
	ctx context.Context

	opCount int
	snapshotEvery int
} 

// new store create a poiner and returns a pointer to new store

func NewStore(ctx context.Context,w *wal.WAL) (*Store,error) {
	centroids := vector.RandomCentroids(64,8)
	index := vector.NewIVFIndex(centroids,3)


	s := &Store{
		data:  make(map[string][]byte),
		wal:   w,
		index: index,
		opCount:       0,    // Initialize counter
        snapshotEvery: 10,   // Set low (e.g., 10) for testing, 10000 for prod
		ctx : ctx,
	}

	// 1. Try to load Snapshot first
    // We ignore error here if file doesn't exist, but you could log it
    // Note: Ensure your snapshot filename matches what you use in Set/Delete ("data.snap")
    if err := s.LoadSnapshot("data.snap"); err != nil {
        // It's okay if snapshot doesn't exist yet
        // fmt.Println("No snapshot found, starting fresh")
    }

	if w != nil {
		if err := w.Replay(s);err != nil{
			return nil,err
		}
	}

	return s,nil
}

// set -> stores a value for a given key

func (s *Store) Set(key string, value []byte) error {
	// 1. Check for shutdown
    select {
    case <-s.ctx.Done():
        return fmt.Errorf("store shutting down")
    default:
    }
	if s.wal != nil {
		if err := s.wal.LogSet(key, value); err != nil {
			return err
		}
	}

	s.mu.Lock()

	s.ApplySet(key, value)

	// snapshot trigger part
	s.opCount++

	if s.wal != nil && s.opCount % s.snapshotEvery == 0{
		// We must release the lock before saving snapshot because
        // SaveSnapshot acquires a Read Lock (RLock).
        // If we hold Lock, RLock will deadlock.
		s.mu.Unlock()

		// Save snapshot to disk
		if err := s.SaveSnapShot("data.snap"); err != nil{
			fmt.Printf("error : %v",err)
		}else{
			// If snapshot successful, clear the WAL
            if err := s.wal.Reset(); err != nil {
                fmt.Printf("Error resetting WAL: %v\n", err)
            }
		}
	}else{
		// Normal unlock if no snapshot
		s.mu.Unlock() 
	}

	if s.Metrics != nil{
		s.Metrics.IncWrites()
	}

	return nil

}

// get -> retrieves a value for a given key

func (s *Store) Get(key string) ([]byte, bool) {
	select {
    case <-s.ctx.Done():
        return nil, false
    default:
    }
	s.mu.RLock()
	defer s.mu.RUnlock()

	val, ok := s.data[key]

	if ok && s.Metrics!=nil{
		s.Metrics.IncReads()
	}

	return val, ok
}

// delete -> removes a value for a given key

func (s *Store) Delete(key string) error {
	// 1. Check for shutdown
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
    // defer s.mu.Unlock() // <--- MOVE DOWN OR REMOVE

	s.ApplyDelete(key)

    // --- PART 7: SNAPSHOT TRIGGER START ---
    s.opCount++
    if s.wal != nil && s.opCount%s.snapshotEvery == 0 {
        s.mu.Unlock() // Release lock to avoid deadlock with SaveSnapshot
        
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
    // --- PART 7: SNAPSHOT TRIGGER END ---

	if s.Metrics != nil{
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

func (s *Store) ApplySet(key string, value []byte) {
    s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = value
	// Remove old vector version first (if it exists) so we don't have duplicates
    s.index.Remove(key)
	s.index.Add(key, bytesToVector(value))
}

func (s *Store) ApplyDelete(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, key)
	s.index.Remove(key)
}

func (s *Store) Close() error{
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.wal!=nil{
		return s.wal.Close()
	}
	return nil
}