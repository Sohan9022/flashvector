package storage

import (
	"context"
	"strconv"
	"testing"
)

// --- HELPER FUNCTION ---
// Creates 8-byte data to match NewStore(64, 8)
func mockDataBench(content string) []byte {
	out := make([]byte, 384) // <--- MUST BE 8
	copy(out, []byte(content))
	return out
}

func BenchmarkStoreSet(b *testing.B) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil) 

	// Create 8-byte data
	data := mockDataBench("val") 

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := "key-" + strconv.Itoa(i)
		store.Set(key, data) // <--- Now sends 8 bytes, not 5
	}
}

func BenchmarkStoreGet(b *testing.B) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)
	data := mockDataBench("val")

	// Pre-populate 1000 items
	for i := 0; i < 1000; i++ {
		store.Set("key-"+strconv.Itoa(i), data)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// key-500 guarantees we find the item
		store.Get("key-500")
	}
}

func BenchmarkStoreDelete(b *testing.B) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)
	data := mockDataBench("val")

	// Stop timer so setup doesn't ruin the benchmark result
	b.StopTimer()
	for i := 0; i < b.N; i++ {
		store.Set("key-"+strconv.Itoa(i), data)
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		store.Delete("key-" + strconv.Itoa(i))
	}
}