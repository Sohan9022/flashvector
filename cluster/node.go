package cluster

import (
	"errors"
	"flashvector/cluster/rpc"
	"flashvector/storage"
	"flashvector/wal"
	"fmt"
	"sync"
	"time"
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

func (n *Node) Set(key string,value []byte) error{
	if !n.IsLeader(){
		return errors.New("Not leader")
	}
// write to local wal
	if err := n.WAL.LogSet(key,value);err != nil{
		return err
	}
// apply localy
	if err := n.Store.Set(key,value);err != nil{
		return err
	}

	// replicate to followers
	record := &rpc.WALRecord{
		Op : 1,
		Key : key,
		Value : value,
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
		return fmt.Errorf("Not leader")
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
	n.Store.ApplySet(key, value)
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

func (n *Node) Stop(){
	close(n.stopCh)
	n.wg.Wait()
}

func (n *Node) Get(key string)([]byte,bool,error){
	if !n.IsLeader(){
		return nil,false,errors.New("not leader")
	}

	val,ok := n.Store.Get(key)

	return val,ok,nil
}
