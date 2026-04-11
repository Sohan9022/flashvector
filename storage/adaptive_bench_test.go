package storage

import (
	"context"
	"strconv"
	"testing"
)

// BenchmarkAdaptiveSearch measures the speed of the hybrid execution and RRF blending
func BenchmarkAdaptiveSearch(b *testing.B) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil) //
	vec := make([]float32, 384)

	// Pre-populate with 1,000 items to have a realistic search space
	for i := 0; i < 1000; i++ {
		id := strconv.Itoa(i)
		// We use 1536 bytes because 384 floats * 4 bytes each = 1536
		data := make([]byte, 1536) 
		store.Set(id, data, nil)
	}

	b.ResetTimer()

	b.Run("Adaptive-Semantic-Weight", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Tests RRF with a 'semantic' weight (k=20)
			store.AdaptiveSearch("long query text here", vec, 5, 20) 
		}
	})

	b.Run("Adaptive-Keyword-Weight", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Tests RRF with a 'keyword' weight (k=100)
			store.AdaptiveSearch("ID_001", vec, 5, 100)
		}
	})
}