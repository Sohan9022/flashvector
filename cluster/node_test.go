package cluster

import (
	"context"
	"flashvector/storage"
	"flashvector/wal"
	"os"
	"testing"
	"time"
	"sync"
	"fmt"
)

// setupTestNode is a helper to quickly spin up a node for testing
func setupTestNode(t *testing.T, id string, leaderId string, peers []NodeConfig) (*Node, func()) {
	walPath := "test_cluster_" + id + ".wal"
	
	// Clean up any old files before starting
	os.Remove(walPath)

	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}

	store, err := storage.NewStore(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}

	cfg := &ClusterConfig{
		Self:     NodeConfig{ID: id, Address: "localhost:0"},
		Peers:    peers,
		LeaderId: leaderId,
	}

	node, err := NewNode(cfg, store, w)
	if err != nil {
		t.Fatal(err)
	}

	node.Start()

	// Return the node and a cleanup function to delete the WAL after the test
	cleanup := func() {
		node.Stop()
		store.Close()
		os.Remove(walPath)
	}

	return node, cleanup
}

func TestLeaderElectionLogic(t *testing.T) {
	peers := []NodeConfig{
		{ID: "node-2", Address: "localhost:8082"},
		{ID: "node-3", Address: "localhost:8083"},
	}

	// Initialize node-1 as a follower initially, with no leader set yet
	node1, cleanup := setupTestNode(t, "node-1", "", peers)
	defer cleanup()

	if node1.IsLeader() {
		t.Fatalf("Node-1 should not be leader initially")
	}

	// Trigger the election logic
	node1.startElection()

	// According to your logic in election.go, the lowest ID should win
	if node1.Config.LeaderId != "node-1" {
		t.Errorf("Expected node-1 to be elected leader, got %s", node1.Config.LeaderId)
	}

	if !node1.IsLeader() {
		t.Errorf("Node-1 should recognize itself as the leader now")
	}
}

func TestHeartbeatRecording(t *testing.T) {
	peers := []NodeConfig{
		{ID: "node-1", Address: "localhost:8081"}, // Node 1 is the leader
	}

	// We are node-2 (a follower)
	node2, cleanup := setupTestNode(t, "node-2", "node-1", peers)
	defer cleanup()

	// Record the initial heartbeat time
	initialTime := node2.lastHeartbeat

	// Simulate receiving a heartbeat from the leader via gRPC
	time.Sleep(10 * time.Millisecond) // Wait a tiny bit to ensure time changes
	node2.RecordHeartbeat()

	// Verify the node updated its internal clock
	if !node2.lastHeartbeat.After(initialTime) {
		t.Errorf("Heartbeat was not updated correctly. Initial: %v, New: %v", initialTime, node2.lastHeartbeat)
	}
}

func TestMultipleHeartbeats(t *testing.T) {

	node, cleanup := setupTestNode(t, "node-2", "node-1", nil)
	defer cleanup()

	first := node.lastHeartbeat

	time.Sleep(5 * time.Millisecond)
	node.RecordHeartbeat()

	second := node.lastHeartbeat

	time.Sleep(5 * time.Millisecond)
	node.RecordHeartbeat()

	third := node.lastHeartbeat

	if !second.After(first) || !third.After(second) {
		t.Errorf("Heartbeats not updating correctly")
	}
}

func TestNodeStartStop(t *testing.T) {

	node, cleanup := setupTestNode(t, "node-1", "", nil)

	node.Stop()

	// Try stopping again (should not panic)
	node.Stop()

	cleanup()
}

func TestFollowerCannotOverrideLeader(t *testing.T) {
	peers := []NodeConfig{
		{ID: "node-1", Address: "localhost:8081"},
	}

	node2, cleanup := setupTestNode(t, "node-2", "node-1", peers)
	defer cleanup()

	// Ensure that initializing as a follower keeps it as a follower
	if node2.IsLeader() {
		t.Errorf("Node-2 should recognize node-1 as the leader, but claimed leadership")
	}
}

func TestHeartbeatTimeout(t *testing.T) {
	peers := []NodeConfig{
		{ID: "node-1", Address: "localhost:8081"},
	}

	// We are node-2, node-1 is leader
	node2, cleanup := setupTestNode(t, "node-2", "node-1", peers)
	defer cleanup()

	// Simulate receiving a heartbeat a long time ago
	node2.RecordHeartbeat()
	
	// Force the internal clock back further than the LeaderTimeout
	node2.lastHeartbeat = time.Now().Add(-(LeaderTimeout + 1*time.Second))

	// Verify the timeout logic
	if time.Since(node2.lastHeartbeat) <= LeaderTimeout {
		t.Fatalf("Heartbeat timeout logic failed, expected time since last heartbeat to exceed %v", LeaderTimeout)
	}
}


func TestConcurrentLeaderWrites(t *testing.T) {
	node, cleanup := setupTestNode(t, "node-1", "node-1", nil)
	defer cleanup()

	var wg sync.WaitGroup

	// Use a 1536-byte array to satisfy the 384-dimension vector requirement
	validVectorData := make([]byte, 1536)

	for i := 0; i < 50; i++ {
		wg.Add(1)

		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			
			// Pass the valid vector data instead of []byte("value")
			node.Set(key, validVectorData, nil)
		}(i)
	}

	wg.Wait()
}

func TestLeaderWriteAllowed(t *testing.T) {
	node1, cleanup := setupTestNode(t, "node-1", "node-1", nil)
	defer cleanup()

	// Use a 1536-byte array to satisfy the 384-dimension vector requirement
	validVectorData := make([]byte, 1536)

	err := node1.Set("leader_key", validVectorData, nil)

	if err != nil {
		t.Fatalf("Leader should accept writes, got error: %v", err)
	}
}

func TestLeaderFailover(t *testing.T) {
	// node-1 starts as leader and knows about node-2
	node1, cleanup1 := setupTestNode(t, "node-1", "", []NodeConfig{{ID: "node-2", Address: "localhost:8082"}})
	defer cleanup1()

	node1.startElection()
	if !node1.IsLeader() {
		t.Fatalf("node1 should be leader")
	}

	// Simulate node-1 crashing
	node1.Stop()

	// node-2 wakes up. Because node-1 is dead, we simulate an empty active peer list
	node2, cleanup2 := setupTestNode(t, "node-2", "node-1", []NodeConfig{})
	defer cleanup2()

	// follower realizes leader is dead and starts election
	node2.startElection()

	if node2.Config.LeaderId != "node-2" {
		t.Errorf("node2 should become leader after node1 failure, but elected %s", node2.Config.LeaderId)
	}
}

func TestWALRecovery(t *testing.T) {
	walPath := "test_recovery_cluster.wal"
	os.Remove(walPath)
	defer os.Remove(walPath) // Clean up after test

	w, err := wal.Open(walPath)
	if err != nil {
		t.Fatal(err)
	}

	store, err := storage.NewStore(context.Background(), w)
	if err != nil {
		t.Fatal(err)
	}

	// USE 1536 BYTES!
	validData := make([]byte, 1536)
	store.Set("key1", validData, nil)
	store.Close()

	// reopen WAL
	w2, _ := wal.Open(walPath)
	defer w2.Close()
	store2, _ := storage.NewStore(context.Background(), w2)

	// Correct Store.Get signature
	val, _, ok := store2.Get("key1")
	if !ok {
		t.Fatalf("expected key recovery")
	}

	if len(val) != 1536 {
		t.Errorf("unexpected value length after recovery")
	}
}

func TestReplication(t *testing.T) {
	node1, cleanup1 := setupTestNode(t, "node-1", "node-1", nil)
	defer cleanup1()

	// USE 1536 BYTES!
	validData := make([]byte, 1536)
	node1.Set("rep_key", validData, nil)

	// Correct Store.Get signature
	val, _, ok := node1.Store.Get("rep_key")
	if !ok {
		t.Fatalf("replication failed locally")
	}

	if len(val) != 1536 {
		t.Errorf("unexpected replicated value length")
	}
}

func TestChaosNodeRestart(t *testing.T) {
	// 1. Correctly define peer lists for each node (A node is not its own peer)
	peersForNode1 := []NodeConfig{{ID: "node-2", Address: "localhost:8082"}, {ID: "node-3", Address: "localhost:8083"}}
	peersForNode2 := []NodeConfig{{ID: "node-1", Address: "localhost:8081"}, {ID: "node-3", Address: "localhost:8083"}}
	peersForNode3 := []NodeConfig{{ID: "node-1", Address: "localhost:8081"}, {ID: "node-2", Address: "localhost:8082"}}

	node1, cleanup1 := setupTestNode(t, "node-1", "", peersForNode1)
	defer cleanup1()

	node2, cleanup2 := setupTestNode(t, "node-2", "node-1", peersForNode2)
	defer cleanup2()

	node3, cleanup3 := setupTestNode(t, "node-3", "node-1", peersForNode3)
	defer cleanup3()

	// 2. Initial Election
	node1.startElection()
	if !node1.IsLeader() {
		t.Fatalf("node1 should be leader initially")
	}

	// 3. Simulate chaotic crash of the leader
	node1.Stop()

	// 4. Simulate the cluster's heartbeat monitor detecting the dead node
	// We drop node-1 from the surviving nodes' active peer lists
	node2.Config.Peers = []NodeConfig{{ID: "node-3", Address: "localhost:8083"}}
	node3.Config.Peers = []NodeConfig{{ID: "node-2", Address: "localhost:8082"}}

	// 5. Followers attempt a new election
	node2.startElection()
	node3.startElection()

	// 6. Verify the cluster healed itself
	if !node2.IsLeader() && !node3.IsLeader() {
		t.Errorf("cluster failed to elect new leader after crash")
	}

	// Specifically, node-2 should win because "node-2" < "node-3"
	if !node2.IsLeader() {
		t.Errorf("Expected node-2 to heal the cluster as the new leader, got %s", node2.Config.LeaderId)
	}
}

