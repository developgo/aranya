package server

import (
	"io"
	"time"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type GrpcManager struct {
	baseServer
	syncSrv connectivity.Connectivity_SyncServer
}

func NewGrpcManager(name string) Interface {
	return &GrpcManager{
		baseServer: newBaseServer(name),
	}
}

func (p *GrpcManager) Sync(server connectivity.Connectivity_SyncServer) error {
	if err := p.baseServer.onDeviceConnected(func() bool {
		if p.syncSrv == nil {
			p.syncSrv = server
			return true
		}
		return false
	}); err != nil {
		return err
	}

	defer p.baseServer.onDeviceDisconnected(func() {
		p.syncSrv = nil
	})

	msgCh := make(chan *connectivity.Msg, messageChannelSize)
	go func() {
		for {
			msg, err := server.Recv()
			if err != nil {
				close(msgCh)

				if err != io.EOF {
					p.log.Error(err, "stream recv failed")
				}
				return
			}

			msgCh <- msg
		}
	}()

	for {
		select {
		case msg, more := <-msgCh:
			if !more {
				return nil
			}

			p.baseServer.onDeviceMsg(msg)
		}
	}
}

// PostCmd sends a command to remote device
func (p *GrpcManager) PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error) {
	return p.baseServer.onPostCmd(c, timeout, func(c *connectivity.Cmd) error {
		// fail if device not connected,
		// you should call WaitUntilDeviceConnected first
		// to get notified when device connected
		if p.syncSrv == nil {
			return ErrDeviceNotConnected
		}

		return p.syncSrv.Send(c)
	})
}
