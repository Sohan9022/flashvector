package main

import (
	"context"
	"flag"
	"fmt"
	"log"

	"flashvector/cluster"
	shutdown "flashvector/internal"
	"flashvector/server"
	"flashvector/storage"
	"flashvector/wal"
)

func main() {
	// Parse CLI flags
	port := flag.String("port", "8080", "HTTP REST API listen port")
	role := flag.String("role", "standalone", "Node role: standalone | leader | follower")
	nodeID := flag.String("node-id", "node-1", "Unique node ID")
	clusterAddr := flag.String("cluster-addr", "localhost:8081", "gRPC cluster replication listen address")
	leaderID := flag.String("leader-id", "node-1", "Cluster leader ID")
	walPath := flag.String("wal", "data.wal", "Path to WAL file")
	flag.Parse()

	fmt.Printf("⚡ Booting up FlashVector Engine (Role: %s, Node: %s)...\n", *role, *nodeID)

	// 1. Setup Graceful Shutdown Context
	rootCtx := context.Background()
	ctx := shutdown.WithSignals(rootCtx)

	// 2. Open or Create WAL file
	w, err := wal.Open(*walPath)
	if err != nil {
		log.Fatalf("Failed to open WAL: %v", err)
	}

	// 3. Create a new Store (automatically replays the WAL & snapshot)
	store, err := storage.NewStore(ctx, w)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}

	// 4. Initialize Cluster Node if role is leader or follower
	var node *cluster.Node
	if *role == "leader" || *role == "follower" {
		cfg := &cluster.ClusterConfig{
			Self:     cluster.NodeConfig{ID: *nodeID, Address: *clusterAddr},
			LeaderId: *leaderID,
		}
		node, err = cluster.NewNode(cfg, store, w)
		if err != nil {
			log.Fatalf("Failed to initialize cluster node: %v", err)
		}
		if err := node.StartGRPCServer(); err != nil {
			log.Printf("⚠️ Warning: Failed to start gRPC cluster server: %v", err)
		}
		node.Start()
	}

	// 5. Initialize the REST API Server
	api := server.NewAPI(store)

	go func() {
		fmt.Printf("🚀 FlashVector REST API is live on http://localhost:%s\n", *port)
		if err := api.Start(*port); err != nil {
			log.Fatalf("Server crashed: %v", err)
		}
	}()

	fmt.Println("Press Ctrl+C to safely shut down the database.")
	<-ctx.Done()

	fmt.Println("\nShutdown signal received. Saving data and stopping...")

	if node != nil {
		node.Stop()
	}

	if err := store.Close(); err != nil {
		fmt.Printf("Error closing store: %v\n", err)
	}

	fmt.Println("Shutdown complete. All data secured in WAL.")
}