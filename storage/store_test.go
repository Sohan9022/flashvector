package storage

import (
	"context"
	"testing"
)

func TestSetAndGet(t *testing.T){
	store,_ := NewStore(context.Background(), nil)

	if err := store.Set("key1", []byte("val1")); err != nil {
		t.Fatal(err)
	}

	val,ok := store.Get("key1")

	if !ok{
		t.Fatalf("expected key to exist")
	}
	if string(val) != "val1"{
		t.Fatalf("expected value 'val1', got '%s'",string(val))
	}
	
}


func TestDelete(t *testing.T){
	store,_ := NewStore(context.Background(), nil)
	
	if err := store.Set("key2", []byte("val2")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete("key2"); err != nil {
		t.Fatal(err)
	}
	_,ok := store.Get("key2")

	if ok{
		t.Fatalf("expected key to be deleted")
	}
}

func TestConcurrentAccess(t *testing.T){
	store,_ := NewStore(context.Background(), nil)

	done := make(chan bool)

	for i := 0;i<100;i++{
		go func(i int){
			key := "key"
			if err := store.Set(key, []byte("val")); err != nil {
				t.Error(err)
			}

			store.Get(key)

			if err := store.Delete(key); err != nil {
				t.Error(err)
			}
			done <- true
		}(i)
	}

	for i:=0 ;i<100;i++{
		<- done
	}

}