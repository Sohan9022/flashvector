package wal

import (
	"bytes"
	"os"
	"testing"
)

type mockReplayer struct {
	data map[string][]byte
	meta map[string]map[string]string
}

func (m *mockReplayer) ApplySet(key string, value []byte, metadata map[string]string) {
	m.data[key] = value
	m.meta[key] = metadata
}

func (m *mockReplayer) ApplyDelete(key string) {
	delete(m.data, key)
	delete(m.meta, key)
}

func TestWALCRC32Validation(t *testing.T) {
	path := "test_wal_crc.wal"
	os.Remove(path)
	defer os.Remove(path)

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open WAL: %v", err)
	}

	meta := map[string]string{"category": "test"}
	if err := w.LogSet("key1", []byte("valid-data-1"), meta); err != nil {
		t.Fatalf("Failed to LogSet: %v", err)
	}
	if err := w.LogSet("key2", []byte("valid-data-2"), meta); err != nil {
		t.Fatalf("Failed to LogSet: %v", err)
	}
	w.Close()

	// 1. Verify clean replay
	w2, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open WAL for replay: %v", err)
	}
	replayer := &mockReplayer{data: make(map[string][]byte), meta: make(map[string]map[string]string)}
	if err := w2.Replay(replayer); err != nil {
		t.Fatalf("Replay failed: %v", err)
	}
	if !bytes.Equal(replayer.data["key1"], []byte("valid-data-1")) || !bytes.Equal(replayer.data["key2"], []byte("valid-data-2")) {
		t.Fatalf("Replay data mismatch")
	}
	w2.Close()

	// 2. Corrupt a single byte in key2's payload without altering file length
	fileBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read WAL file: %v", err)
	}
	// Flip the last byte before the 4-byte checksum
	corruptIdx := len(fileBytes) - 6
	fileBytes[corruptIdx] ^= 0xFF
	if err := os.WriteFile(path, fileBytes, 0644); err != nil {
		t.Fatalf("Failed to write corrupted WAL file: %v", err)
	}

	// 3. Replay corrupted WAL: CRC32 mismatch should trigger truncation of key2, keeping key1
	w3, err := Open(path)
	if err != nil {
		t.Fatalf("Failed to open corrupted WAL: %v", err)
	}
	replayerCorrupt := &mockReplayer{data: make(map[string][]byte), meta: make(map[string]map[string]string)}
	if err := w3.Replay(replayerCorrupt); err != nil {
		t.Fatalf("Replay should handle corruption gracefully, got error: %v", err)
	}

	if _, ok := replayerCorrupt.data["key1"]; !ok {
		t.Fatalf("Expected key1 to be recovered before corruption point")
	}
	if _, ok := replayerCorrupt.data["key2"]; ok {
		t.Fatalf("Expected corrupted key2 to be discarded by CRC32 check")
	}
	w3.Close()
}
