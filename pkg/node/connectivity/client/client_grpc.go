package client

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

type GrpcClient struct {
	baseClient
	client     connectivity.ConnectivityClient
	syncClient connectivity.Connectivity_SyncClient
}

func NewGrpcClient(conn *grpc.ClientConn, rt runtime.Interface) (*GrpcClient, error) {
	client := &GrpcClient{
		baseClient: newBaseClient(rt),
		client:     connectivity.NewConnectivityClient(conn),
	}

	(&client.baseClient).doPostMsg = client.PostMsg
	return client, nil
}

func (c *GrpcClient) Run(ctx context.Context) error {
	if err := c.baseClient.onConnect(func() error {
		if c.syncClient != nil {
			return ErrClientAlreadyConnected
		}

		syncClient, err := c.client.Sync(ctx)
		if err != nil {
			return err
		}
		c.syncClient = syncClient
		return nil
	}); err != nil {
		return err
	}

	cmdCh := make(chan *connectivity.Cmd, 1)
	go func() {
		for {
			cmd, err := c.syncClient.Recv()
			if err != nil {
				close(cmdCh)
				if err != io.EOF {
					// TODO: log error
				}
				return
			}

			cmdCh <- cmd
		}
	}()

	defer c.baseClient.onDisconnected(func() {
		c.syncClient = nil
	})

	for {
		select {
		case cmd, more := <-cmdCh:
			if !more {
				return nil
			}
			c.baseClient.onSrvCmd(cmd)
		}
	}
}

func (c *GrpcClient) PostMsg(msg *connectivity.Msg) error {
	return c.baseClient.onPostMsg(msg, func(msg *connectivity.Msg) error {
		if c.syncClient == nil {
			return ErrClientNotConnected
		}

		err := c.syncClient.Send(msg)
		if err != nil {
			return err
		}

		return nil
	})
}
