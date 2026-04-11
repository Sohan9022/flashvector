package cluster

import (
	"errors"
	"flashvector/cluster/rpc"
	"flashvector/storage"
	"flashvector/wal"
	"fmt"
	"sync"
	"time"
	"net"
	"google.golang.org/grpc"
)

type Node struct{
	Config *ClusterConfig
	Store *storage.Store
	WAL *wal.WAL
	Clients map[string]*rpc.ReplicationClient
	lastHeartbeat time.Time
	stopCh chan struct{}
	unhealthy map[string]bool
	wg sync.WaitGroup
}

// constructor to create a node

func NewNode(cfg *ClusterConfig,store *storage.Store,w *wal.WAL)(*Node,error){
	
	
	node := &Node{
		Config: cfg,
		Store : store,
		WAL: w,
		Clients: make(map[string]*rpc.ReplicationClient),
		unhealthy: make(map[string]bool),
	}

	if cfg.IsLeader(){
		for _,peer := range cfg.Peers {
			client,err := rpc.NewReplicationClient(peer.Address)
			if err != nil{
				return nil,err
			}
			node.Clients[peer.ID] = client
		}
	}

	return node,nil
}

func (n *Node) IsLeader() bool{
	return n.Config.IsLeader()
}

func (n *Node) Set(key string,value []byte,metadata map[string]string) error{
	if !n.IsLeader(){
		return errors.New("not leader")
	}
// write to local wal
	if err := n.WAL.LogSet(key,value,metadata);err != nil{
		return err
	}
// apply localy
	if err := n.Store.Set(key,value,metadata);err != nil{
		return err
	}

	// replicate to followers
	record := &rpc.WALRecord{
		Op : 1,
		Key : key,
		Value : value,
		Metadata : metadata,
	}

	for id,clients := range n.Clients{
		if n.unhealthy[id]{
			continue
		}
		// synchronous replication
		if err := clients.Replicate(record);err != nil{
		 n.unhealthy[id] = true

		 if n.Store.Metrics != nil{
			n.Store.Metrics.IncReplicationFailures()
		 }
		}
	}

	return nil
}

func (n *Node) Delete(key string)error{
	if !n.IsLeader(){
		return fmt.Errorf("not leader")
	}

	if err := n.WAL.LogDelete(key);err != nil{
		return err
	}

	if err := n.Store.Delete(key);err != nil{
		return err
	}

	record := &rpc.WALRecord{
		Op: 2,
		Key: key,
	}

	for _,client := range n.Clients{
		if err := client.Replicate(record);err != nil{
			return err
		}
	}

	return nil

}

// --- Implementation of rpc.ReplicaHandler Interface ---

// ApplySet delegates the apply operation to the underlying store
func (n *Node) ApplySet(key string, value []byte) {
	n.Store.ApplySet(key, value,nil)
}

// ApplyDelete delegates the apply operation to the underlying store
func (n *Node) ApplyDelete(key string) {
	n.Store.ApplyDelete(key)
}

func (n *Node) startHeartbeat(){
	if !n.IsLeader(){
		return
	}

	ticker := time.NewTicker(HeartBeatInterval)
// NO 'go func()' here! Just the loop.
	go func(){
		for{
			select{
			case <-ticker.C:
				n.broadcastHeartbeat()

			case <-n.stopCh:
				ticker.Stop()
				return

			}
		}
	}()

}

func (n *Node) broadcastHeartbeat(){
	for _,client := range n.Clients{
		_ = client.SendHeartbeat()
	}
}

func (n *Node) RecordHeartbeat(){
	n.lastHeartbeat = time.Now()
}

func (n *Node) Start(){
	n.stopCh = make(chan struct{})
	n.lastHeartbeat = time.Now()

	if n.IsLeader() {
        n.wg.Add(1) // <--- Add
		// Wrap in anonymous func to handle Done()
		go func() {
			defer n.wg.Done() // <--- Done
			n.startHeartbeat() // Ensure startHeartbeat is blocking or modify it to be the loop itself
		}()
	} else {
        n.wg.Add(1) // <--- Add
		go func() {
			defer n.wg.Done() // <--- Done
			n.startLeaderMonitor()
		}()
	}
}

func (n *Node) Stop() {
	// Defensively check if the channel was initialized
	if n.stopCh != nil {
		// A select with a default prevents panicking if Stop() is called twice
		select {
		case <-n.stopCh:
			// already closed
		default:
			close(n.stopCh)
		}
	}
	n.wg.Wait()
}

func (n *Node) Get(key string)([]byte,bool,error){
	if !n.IsLeader(){
		return nil,false,errors.New("Not leader")
	}

	val,_,ok := n.Store.Get(key)

	return val,ok,nil
}

// StartGRPCServer opens a port and listens for replication commands from the Leader
func (n *Node) StartGRPCServer() error {
	// Parse the port from the node's address (e.g., "localhost:8081" -> ":8081")
	lis, err := net.Listen("tcp", n.Config.Self.Address)
	if err != nil {
		return err
	}

	// Create a new gRPC server
	grpcServer := grpc.NewServer()

	// Register our replication service, passing the Node as the handler
	rpc.RegisterReplicationServiceServer(grpcServer, &rpc.ReplicationServer{Node: n})

	// Run the server in a background goroutine so it doesn't block
	go func() {
		fmt.Printf("🛡️ Node %s listening for cluster replication on %s\n", n.Config.Self.ID, n.Config.Self.Address)
		if err := grpcServer.Serve(lis); err != nil {
			fmt.Printf("gRPC server failed: %v\n", err)
		}
	}()

	return nil
}