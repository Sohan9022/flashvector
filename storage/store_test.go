package storage

import (
	"bytes"
	"context"
	"testing"
)

// Helper to create 8-byte vectors (Matches your NewStore configuration)
func mockDataTest(content string) []byte {
	out := make([]byte, 384)
	copy(out, []byte(content))
	return out
}

func TestSetAndGet(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)

	// FIX: Use mockDataTest instead of raw strings
	val := mockDataTest("val1")
	
	if err := store.Set("key1", val); err != nil {
		t.Fatal(err)
	}

	retrieved, ok := store.Get("key1")

	if !ok {
		t.Fatalf("expected key to exist")
	}
	if !bytes.Equal(retrieved, val) {
		t.Fatalf("expected value matches")
	}
}

func TestDelete(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)

	// FIX: Use mockDataTest
	if err := store.Set("key2", mockDataTest("val2")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("key2"); err != nil {
		t.Fatal(err)
	}
	_, ok := store.Get("key2")

	if ok {
		t.Fatalf("expected key to be deleted")
	}
}

func TestConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	store, _ := NewStore(ctx, nil)

	done := make(chan bool)

	for i := 0; i < 100; i++ {
		go func(i int) {
			key := "key"
			// FIX: Use mockDataTest
			if err := store.Set(key, mockDataTest("val")); err != nil {
				t.Error(err)
			}

			store.Get(key)

			if err := store.Delete(key); err != nil {
				t.Error(err)
			}
			done <- true
		}(i)
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}