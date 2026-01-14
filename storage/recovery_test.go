package storage

import (
	"flashvector/wal"	
	"os"
	"context"
	"testing"
)

func TestCrashRecovery(t *testing.T){
	walPath := "test.wal"

	// write data 
	w,err := wal.Open(walPath)

	if err != nil{
		t.Fatal(err)
	}

	store,err := NewStore(context.Background(), w)
	if err != nil {
    t.Fatal(err)
}
	if err := store.Set("a",[]byte{1,0}); err != nil{
		t.Fatal(err)
	}

	if err := store.Set("b",[]byte{0,1}); err != nil{
		t.Fatal(err)
	}

	// simulate crash

	w.Close()

	// restart
	w1,err := wal.Open(walPath)

	if err != nil{
		t.Fatal(err)
	}

	defer w1.Close()

	store1,err := NewStore(context.Background(), w1)

	// verify

	val,ok :=  store1.Get("a")

	if !ok || val[0] != 1{
		t.Fatalf("key a not recovered correctly")
	}

	results := store1.VectorSearch([]float32{1,0},1)
	if len(results) != 1 || results[0].ID != "a"{
			t.Fatalf("vector index not rebuilt correctly")
		}
	

	os.Remove(walPath)

}

func TestCorruptedWALRecovery(t *testing.T) {
	walPath := "corrupted.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	// 1. Create initial valid data
	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	store,err := NewStore(context.Background(), w)
	if err := store.Set("key1", []byte("val1")); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("key2", []byte("val2")); err != nil {
		t.Fatal(err)
	}
	w.Close()

	// 2. CORRUPT THE FILE
	// Simulate a crash by appending partial bytes (e.g., OpCode + half a length)
	f, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	// Write OpSet (1 byte) + 2 bytes of garbage (incomplete length)
	if _, err := f.Write([]byte{1, 5, 0}); err != nil {
		t.Fatal(err)
	}
	f.Close()

	// 3. Attempt Recovery
	w2, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	// If your Replay logic is correct, this should truncate the garbage and load key1, key2
	store2,err := NewStore(context.Background(), w2)

	// Verify old data is safe
	if val, ok := store2.Get("key1"); !ok || string(val) != "val1" {
		t.Error("key1 lost or corrupted after recovery")
	}
	if val, ok := store2.Get("key2"); !ok || string(val) != "val2" {
		t.Error("key2 lost or corrupted after recovery")
	}

	// 4. Verify we can continue writing NEW data
	// If the file wasn't truncated correctly, this write might be lost or corrupt the file further
	if err := store2.Set("key3", []byte("val3")); err != nil {
		t.Fatalf("failed to write after recovery: %v", err)
	}
	w2.Close()

	// 5. Final Restart to prove persistence
	w3, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer w3.Close()
	store3,err := NewStore(context.Background(), nil)

	if val, ok := store3.Get("key3"); !ok || string(val) != "val3" {
		t.Errorf("key3 failed to persist. WAL might not have been truncated correctly.")
	}
}

func TestInterleavedOperations(t *testing.T) {
	walPath := "interleaved.wal"
	os.Remove(walPath)
	defer os.Remove(walPath)

	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	store,err := NewStore(context.Background(), w)

	// 1. Sequence of operations
	store.Set("a", []byte("1"))    // Create a
	store.Set("b", []byte("2"))    // Create b
	store.Delete("a")              // Delete a
	store.Set("c", []byte("3"))    // Create c
	store.Set("b", []byte("4"))    // Update b
	store.Delete("c")              // Delete c
	store.Set("c", []byte("5"))    // Re-create c
	w.Close()

	// 2. Recover
	w2, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}
	defer w2.Close()
	store2,err := NewStore(context.Background(), w2)

	// 3. Verify State
	// "a" should be gone
	if _, ok := store2.Get("a"); ok {
		t.Error("key 'a' should be deleted, but was found")
	}

	// "b" should be "4" (update worked)
	if val, ok := store2.Get("b"); !ok || string(val) != "4" {
		t.Errorf("key 'b' should be '4', got %s", string(val))
	}

	// "c" should be "5" (re-create worked)
	if val, ok := store2.Get("c"); !ok || string(val) != "5" {
		t.Errorf("key 'c' should be '5', got %s", string(val))
	}
}