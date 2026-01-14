package cluster

import "flashvector/cluster/rpc"

func (n *Node) startElection() {
	lowest := n.Config.Self.ID

	for _, peer := range n.Config.Peers {
		if peer.ID < lowest {
			lowest = peer.ID
		}
	}

	n.Config.LeaderId = lowest

	if n.IsLeader() {
	
		n.becomeLeader()
	}
}

func (n *Node) becomeLeader() {
	n.initReplicationClients()
	n.startHeartbeat()
}

func (n *Node) initReplicationClients() {
	n.Clients = make(map[string]*rpc.ReplicationClient)

	for _,peer := range n.Config.Peers{
		client,err := rpc.NewReplicationClient(peer.Address)
		if err == nil{
			n.Clients[peer.ID] = client
		}
	}
}