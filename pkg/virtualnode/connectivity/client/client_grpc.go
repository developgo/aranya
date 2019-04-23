// +build agent_grpc

package client

import (
	"context"
	"io"

	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/connectivity/client/runtime"
)

var _ Interface = &GRPCAgent{}

type GRPCAgent struct {
	baseAgent
	client     connectivity.ConnectivityClient
	syncClient connectivity.Connectivity_SyncClient
}

func NewGRPCAgent(ctx context.Context, config *Config, conn *grpc.ClientConn, rt runtime.Interface) (*GRPCAgent, error) {
	client := &GRPCAgent{
		baseAgent: newBaseAgent(ctx, config, rt),
		client:    connectivity.NewConnectivityClient(conn),
	}

	(&client.baseAgent).doPostMsg = client.PostMsg
	return client, nil
}

func (c *GRPCAgent) Start(ctx context.Context) error {
	if err := c.baseAgent.onConnect(func() error {
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

	defer c.baseAgent.onDisconnected(func() {
		c.syncClient = nil
	})

	for {
		select {
		case <-c.syncClient.Context().Done():
			// disconnected from cloud controller
			return nil
		case <-ctx.Done():
			// leaving
			return nil
		case cmd, more := <-cmdCh:
			if !more {
				return nil
			}
			c.baseAgent.onRecvCmd(cmd)
		}
	}
}

func (c *GRPCAgent) PostMsg(msg *connectivity.Msg) error {
	return c.baseAgent.onPostMsg(msg, func(msg *connectivity.Msg) error {
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
