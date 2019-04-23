package server

import (
	"context"
	"net"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

var _ Manager = &GRPCManager{}

type GRPCManager struct {
	baseManager

	syncSrv   connectivity.Connectivity_SyncServer
	closeConn context.CancelFunc

	server   *grpc.Server
	listener net.Listener
}

func NewGRPCManager(server *grpc.Server, listener net.Listener) *GRPCManager {
	mgr := &GRPCManager{
		baseManager: newBaseServer(),
		listener:    listener,
		server:      server,
	}
	connectivity.RegisterConnectivityServer(server, mgr)

	return mgr
}

func (m *GRPCManager) Start() error {
	return m.server.Serve(m.listener)
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

func (m *GRPCManager) Stop() {
	m.onStop(func() {
		m.server.Stop()
	})
}
