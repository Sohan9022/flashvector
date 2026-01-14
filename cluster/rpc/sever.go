package rpc

import (
	"context"
	// "flashvector/cluster"
)

// Define an interface for the operations the server needs to perform on the Node.
type ReplicaHandler interface {
	ApplySet(key string, value []byte)
	ApplyDelete(key string)
	RecordHeartbeat()
}

type ReplicationServer struct{
	UnimplementedReplicationServiceServer
	Node ReplicaHandler
}

func (s *ReplicationServer) Replicate(ctx context.Context,req *ReplicateRequest)(*ReplicateResponse,error){
	rec := req.Record

	switch rec.Op{
	case 1:
		s.Node.ApplySet(rec.Key,rec.Value)
		
	case 2:
	    s.Node.ApplyDelete(rec.Key)
		
	}

	return &ReplicateResponse{Success : true},nil

}

func (s *ReplicationServer) Heartbeat(ctx context.Context,req *HeartbeatRequest)(*HeartbeatResponse,error){
	s.Node.RecordHeartbeat()
	return &HeartbeatResponse{},nil
}

