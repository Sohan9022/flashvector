package main

import (
	"context"
	shutdown "flashvector/internal"
	"flashvector/storage"
	"flashvector/wal"
	"fmt"
	"log"
)

func main() {
	// open or create wal file

	rootCtx := context.Background()
	ctx := shutdown.WithSignals(rootCtx)

	w,err := wal.Open("data.wal")
	if err != nil{
		log.Fatal(err)
	}
	defer w.Close()
	
	// create a new store
	store,err := storage.NewStore(ctx,w)
	if err != nil {
    log.Fatal(err) // Crash if we can't load data
}

	// store a value
	if err := store.Set("greeting",[]byte("Hello,World!"));err!=nil{
		 log.Fatal(err)
	}

	// retrieve the value
	value,ok := store.Get("greeting")
	if ok{
		fmt.Println(string(value))
	}
	

	if err := store.Delete("greeting");err!=nil{
		log.Fatal(err)
	}
	

	_,ok = store.Get("greeting")
	fmt.Println("Key exists after deletion:", ok)	

	// --- STEP 15: ORCHESTRATION ---
	// Wait here until we get a shutdown signal (Ctrl+C)
	fmt.Println("Server started. Press Ctrl+C to stop.")
	<-ctx.Done()
	fmt.Println("\nShutdown signal received. Stopping...")

	// Cleanup logic
	// If you have a node, close it here: node.Stop() / node.Close()
	if err := store.Close(); err != nil {
		fmt.Printf("Error closing store: %v\n", err)
	}
	
	fmt.Println("Shutdown complete.")

}

