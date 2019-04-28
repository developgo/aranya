// +build agent_grpc

/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package client

import (
	"context"
	"log"
	"math"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
)

var _ Interface = &GRPCAgent{}

var (
	defaultCallOptions = []grpc.CallOption{
		grpc.WaitForReady(true),
		grpc.MaxCallRecvMsgSize(math.MaxInt32),
		grpc.MaxCallSendMsgSize(math.MaxInt32),
	}
)

type GRPCAgent struct {
	baseAgent
	client     connectivity.ConnectivityClient
	syncClient connectivity.Connectivity_SyncClient
}

func NewGRPCAgent(ctx context.Context, config *AgentConfig, conn *grpc.ClientConn, rt runtime.Interface) (*GRPCAgent, error) {
	client := &GRPCAgent{
		baseAgent: newBaseAgent(ctx, config, rt),
		client:    connectivity.NewConnectivityClient(conn),
	}

	(&client.baseAgent).doPostMsg = client.PostMsg
	return client, nil
}

func (c *GRPCAgent) Start(ctx context.Context) error {
	if err := c.onConnect(func() error {
		if c.syncClient != nil {
			return ErrClientAlreadyConnected
		}

		syncClient, err := c.client.Sync(ctx, defaultCallOptions...)
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

				s, _ := status.FromError(err)
				switch s.Code() {
				case codes.Canceled, codes.OK:
				default:
					log.Printf("exception happened when client recv: %v", err)
				}
				return
			}

			cmdCh <- cmd
		}
	}()

	defer c.onDisconnect(func() {
		c.syncClient = nil
	})

	for {
		select {
		case <-c.syncClient.Context().Done():
			// disconnected from cloud controller
			return c.syncClient.Context().Err()
		case <-ctx.Done():
			// leaving
			return nil
		case cmd, more := <-cmdCh:
			if !more {
				return ErrCmdRecvClosed
			}
			c.baseAgent.onRecvCmd(cmd)
		}
	}
}

func (c *GRPCAgent) Stop() {
	c.onStop(func() {
		if c.syncClient != nil {
			_ = c.syncClient.CloseSend()
		}
	})
}

func (c *GRPCAgent) PostMsg(msg *connectivity.Msg) error {
	return c.onPostMsg(msg, func(msg *connectivity.Msg) error {
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
