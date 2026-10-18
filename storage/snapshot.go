package storage

import (
	"encoding/gob"
	"os"
)

// SnapshotData encapsulates both vector raw data and associated metadata tags for durability
type SnapshotData struct {
	Data map[string][]byte
	Meta map[string]Metadata
}

func (s *Store) SaveSnapShot(path string) error {
	s.mu.RLock()
	defer s.mu.RUnlock()

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	snap := SnapshotData{
		Data: s.data,
		Meta: s.meta,
	}

	encoder := gob.NewEncoder(file)
	return encoder.Encode(snap)
}

func (s *Store) LoadSnapshot(path string) error {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	decoder := gob.NewDecoder(file)
	var snap SnapshotData

	if err := decoder.Decode(&snap); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if snap.Data != nil {
		s.data = snap.Data
	} else {
		s.data = make(map[string][]byte)
	}

	if snap.Meta != nil {
		s.meta = snap.Meta
	} else {
		s.meta = make(map[string]Metadata)
	}

	s.index.RebuildFromData(s.data)
	return nil
}