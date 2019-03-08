package client

import (
	"context"
	"fmt"
	"sync"

	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type GrpcClient struct {
	baseClient
	client     connectivity.ConnectivityClient
	syncClient connectivity.Connectivity_SyncClient

	mu sync.RWMutex
}

func NewGrpcClient(conn *grpc.ClientConn, options ...Option) (*GrpcClient, error) {
	base := baseClient{}
	for _, setOption := range options {
		if err := setOption(&base); err != nil {
			return nil, err
		}
	}

	return &GrpcClient{
		baseClient: base,
		client:     connectivity.NewConnectivityClient(conn),
	}, nil
}

func (c *GrpcClient) Run(ctx context.Context) error {
	err := func() error {
		c.mu.Lock()
		defer c.mu.Unlock()

		if c.syncClient != nil {
			return fmt.Errorf("client already started")
		}

		syncClient, err := c.client.Sync(ctx)
		if err != nil {
			return err
		}

		c.syncClient = syncClient
		return nil
	}()

	if err != nil {
		return err
	}

	cmdCh := make(chan *connectivity.Cmd, 1)
	go func() {
		for {
			cmd, err := c.syncClient.Recv()
			if err != nil {
				return
			}

			cmdCh <- cmd
		}
	}()

	defer func() {
		c.mu.Lock()
		defer c.mu.Unlock()

		close(cmdCh)
		c.syncClient = nil
	}()

	for {
		select {
		case cmd, more := <-cmdCh:
			if !more {
				return nil
			}
			c.baseClient.handleCmd(cmd)
		}
	}
}
