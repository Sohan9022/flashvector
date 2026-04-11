package main

import (
	"context"
	"fmt"
	"log"

	shutdown "flashvector/internal"
	"flashvector/server" // <-- ADDED: Import the new server package
	"flashvector/storage"
	"flashvector/wal"
)

func main() {
	fmt.Println("⚡ Booting up FlashVector Engine...")

	// 1. Setup Graceful Shutdown Context
	rootCtx := context.Background()
	ctx := shutdown.WithSignals(rootCtx)

	// 2. Open or Create WAL file
	w, err := wal.Open("data.wal")
	if err != nil {
		log.Fatalf("Failed to open WAL: %v", err)
	}
	// Note: We don't defer w.Close() here anymore because store.Close() will handle it!

	// 3. Create a new Store (This automatically replays the WAL!)
	store, err := storage.NewStore(ctx, w)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// --- WE DELETED THE "greeting" TEST CODE HERE ---

	// 4. Initialize the API Server
	api := server.NewAPI(store)

	// 5. Start the Web Server in a background Goroutine (so it doesn't block Ctrl+C)
	port := "8080"
	go func() {
		fmt.Printf("🚀 FlashVector REST API is live on http://localhost:%s\n", port)
		if err := api.Start(port); err != nil {
			log.Fatalf("Server crashed: %v", err)
		}
	}()

	// 6. Wait here until we get a shutdown signal (Ctrl+C)
	fmt.Println("Press Ctrl+C to safely shut down the database.")
	<-ctx.Done()

	fmt.Println("\nShutdown signal received. Saving data and stopping...")

	// 7. Cleanup logic
	if err := store.Close(); err != nil {
		fmt.Printf("Error closing store: %v\n", err)
	}

	fmt.Println("Shutdown complete. All data secured in WAL.")
}