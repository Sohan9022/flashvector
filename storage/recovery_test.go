package storage

import (
	"bytes"
	"context"
	"flashvector/wal"
	"os"
	"testing"
)

// --- HELPER FUNCTION ---
// FIX: Use 8 bytes to match your IVFIndex dimension
func mockDataRecovery(content string) []byte {
	out := make([]byte, 384) // <--- CHANGED FROM 64 TO 8
	copy(out, []byte(content))
	return out
}

func TestCrashRecovery(t *testing.T) {
	walPath := "test_crash.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	ctx := context.Background()

	// 1. Write data
	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}

	store, err := NewStore(ctx, w)
	if err != nil {
		t.Fatal(err)
	}

	// USE HELPER for consistent 8-byte data
	valA := mockDataRecovery("valA")
	valB := mockDataRecovery("valB")

	if err := store.Set("a", valA); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("b", valB); err != nil {
		t.Fatal(err)
	}

	// 2. Simulate crash
	w.Close()

	// 3. Restart
	w1, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer w1.Close()

	store1, err := NewStore(ctx, w1)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Verify
	val, ok := store1.Get("a")
	if !ok {
		t.Fatalf("key a not recovered correctly")
	}
	if !bytes.Equal(val, valA) {
		t.Fatalf("key a corrupted")
	}

	// 5. Verify Vector Search
	queryVec := make([]float32, 384) // <--- CHANGED TO 8 DIMENSIONS
	copy(queryVec, bytesToVector(valA))

	results := store1.VectorSearch(queryVec, 1)
	if len(results) != 1 || results[0].ID != "a" {
		t.Fatalf("vector index not rebuilt correctly")
	}
}

func TestCorruptedWALRecovery(t *testing.T) {
	walPath := "corrupted.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	ctx := context.Background()

	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(ctx, w)

	val1 := mockDataRecovery("val1")
	val2 := mockDataRecovery("val2")

	store.Set("key1", val1)
	store.Set("key2", val2)
	w.Close()

	// 2. CORRUPT THE FILE
	f, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Write garbage
	f.Write([]byte{1, 5, 255, 255})
	f.Close()

	// 3. Attempt Recovery
	w2, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()

	store2, err := NewStore(ctx, w2)
	if err != nil {
		t.Fatal("Store failed to start with corrupted WAL")
	}

	if val, ok := store2.Get("key1"); !ok || !bytes.Equal(val, val1) {
		t.Error("key1 lost or corrupted")
	}

	// 4. Verify we can continue writing
	if err := store2.Set("key3", mockDataRecovery("val3")); err != nil {
		t.Fatalf("failed to write after recovery: %v", err)
	}
}

func TestInterleavedOperations(t *testing.T) {
	walPath := "interleaved.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	ctx := context.Background()

	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := NewStore(ctx, w)

	store.Set("a", mockDataRecovery("1"))
	store.Set("b", mockDataRecovery("2"))
	store.Delete("a")
	store.Set("c", mockDataRecovery("3"))
	store.Set("b", mockDataRecovery("4")) // Update
	store.Delete("c")
	store.Set("c", mockDataRecovery("5")) // Re-create
	w.Close()

	// 2. Recover
	w2, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	store2, _ := NewStore(ctx, w2)

	// 3. Verify State
	if _, ok := store2.Get("a"); ok {
		t.Error("key 'a' should be deleted, but was found")
	}

	valB, ok := store2.Get("b")
	if !ok || !bytes.Equal(valB, mockDataRecovery("4")) {
		t.Errorf("key 'b' should be '4'")
	}

	valC, ok := store2.Get("c")
	if !ok || !bytes.Equal(valC, mockDataRecovery("5")) {
		t.Errorf("key 'c' should be '5'")
	}
}