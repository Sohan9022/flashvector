package storage

import (
	"context"
	"testing"
)

func TestMetadataFiltering(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)

	// 1. Create Dummy Vectors
	// We use the same vector for both so they are identical in similarity.
	// The ONLY difference will be the metadata.
	vecData := make([]byte, 1536) 
	vecData[0] = 1 // Just some data

	// 2. Insert Data with Metadata
	metaCat := map[string]string{"type": "cat", "name": "Whiskers"}
	metaDog := map[string]string{"type": "dog", "name": "Buddy"}

	if err := store.Set("cat1", vecData, metaCat); err != nil {
		t.Fatal(err)
	}
	if err := store.Set("dog1", vecData, metaDog); err != nil {
		t.Fatal(err)
	}

	// 3. Define the Query Vector (Same as data)
	query := make([]float32, 384)
	query[0] = float32(1)

	// --- TEST CASE 1: Search for CATS only ---
	filterCat := map[string]string{"type": "cat"}
	results := store.VectorSearch(query, 10, filterCat)

	if len(results) != 1 {
		t.Fatalf("Expected 1 result for 'cat' filter, got %d", len(results))
	}
	if results[0].ID != "cat1" {
		t.Errorf("Expected 'cat1', got '%s'", results[0].ID)
	}

	// --- TEST CASE 2: Search for DOGS only ---
	filterDog := map[string]string{"type": "dog"}
	results2 := store.VectorSearch(query, 10, filterDog)

	if len(results2) != 1 {
		t.Fatalf("Expected 1 result for 'dog' filter, got %d", len(results2))
	}
	if results2[0].ID != "dog1" {
		t.Errorf("Expected 'dog1', got '%s'", results2[0].ID)
	}

	// --- TEST CASE 3: Search for UNKNOWN type ---
	filterBird := map[string]string{"type": "bird"}
	results3 := store.VectorSearch(query, 10, filterBird)

	if len(results3) != 0 {
		t.Errorf("Expected 0 results for 'bird', got %d", len(results3))
	}
}