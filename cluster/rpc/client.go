package rpc

import (
	"context"
	"time"
	"google.golang.org/grpc"
)

type ReplicationClient struct{
	client ReplicationServiceClient
}

func NewReplicationClient(addr string) (*ReplicationClient,error){
	conn,err := grpc.Dial(addr,grpc.WithInsecure())

	if err != nil{
		return nil,err
	}

	return &ReplicationClient{
		client: NewReplicationServiceClient(conn),
	},nil
}

func (c *ReplicationClient) Replicate(record *WALRecord) error{
	ctx,cancel := context.WithTimeout(context.Background(),time.Second)
	defer cancel()

	_,err := c.client.Replicate(ctx,&ReplicateRequest{
		Record : record,
	})

	return err
}

func (c *ReplicationClient) SendHeartbeat() error{
	ctx,cancel := context.WithTimeout(context.Background(),time.Second)

	defer cancel()

	_,err := c.client.Heartbeat(ctx,&HeartbeatRequest{})
	return err
}