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

package server

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/constant"
)

var _ Manager = &GRPCManager{}

type GRPCManager struct {
	baseManager

	syncSrv   connectivity.Connectivity_SyncServer
	closeConn context.CancelFunc

	server   *grpc.Server
	listener net.Listener
}

func NewGRPCManager(server *grpc.Server, listener net.Listener, mgrConfig *Config) *GRPCManager {
	mgr := &GRPCManager{
		baseManager: newBaseManager(mgrConfig),
		listener:    listener,
		server:      server,
	}
	connectivity.RegisterConnectivityServer(server, mgr)

	return mgr
}

func (m *GRPCManager) Start() error {
	return m.server.Serve(m.listener)
}

func (m *GRPCManager) Stop() {
	m.onStop(func() {
		m.server.Stop()
	})
}

func (m *GRPCManager) Reject(reason connectivity.RejectReason, message string) {
	m.onReject(func() {
		_ = m.syncSrv.Send(connectivity.NewRejectCmd(reason, message))
	})
}

func (m *GRPCManager) Sync(server connectivity.Connectivity_SyncServer) error {
	connCtx, closeConn := context.WithCancel(server.Context())

	if err := m.onConnected(func() (accept bool) {
		if m.syncSrv == nil {
			m.syncSrv = server
			m.closeConn = closeConn
			return true
		}
		return false
	}); err != nil {
		log.Error(err, "")
		return err
	}

	defer m.onDisconnected(func() {
		log.Info("device disconnected")
		m.syncSrv = nil
	})

	msgCh := make(chan *connectivity.Msg, constant.DefaultConnectivityMsgChannelSize)
	go func() {
		for {
			msg, err := server.Recv()

			if err != nil {
				close(msgCh)

				s, _ := status.FromError(err)
				switch s.Code() {
				case codes.Canceled, codes.OK:
				default:
					log.Error(s.Err(), "stream recv failed")
				}
				return
			}

			msgCh <- msg
		}
	}()

	for {
		select {
		case <-m.rejected:
			// device rejected, return to close this stream
			return nil
		case <-connCtx.Done():
			return nil
		case msg, more := <-msgCh:
			if !more {
				return nil
			}

			m.onRecvMsg(msg)
		}
	}
}

// PostCmd sends a command to remote device
func (m *GRPCManager) PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error) {
	return m.onPostCmd(ctx, c, func(c *connectivity.Cmd) error {
		// fail if device not connected,
		// you should call Connected first
		// to get notified when device connected
		if m.syncSrv == nil {
			return ErrDeviceNotConnected
		}

		return m.syncSrv.Send(c)
	})
}
