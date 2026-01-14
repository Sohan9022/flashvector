package cluster

type NodeConfig struct{
	ID string
	Address string
}

type ClusterConfig struct{
	Self NodeConfig
	Peers []NodeConfig
	LeaderId string
}

func (c *ClusterConfig) IsLeader() bool{
	return c.LeaderId == c.Self.ID
}


