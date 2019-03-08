package server

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
)

const (
	messageChannelSize = 10
)

type GrpcSrv struct {
	log             logr.Logger
	sessions        *sessionManager
	deviceConnected chan struct{}
	syncSrv         connectivity.Connectivity_SyncServer
	mu              sync.RWMutex
	sessionTimeout  time.Duration
	globalChan      chan *connectivity.Msg
}

func NewGrpcConnectivity(name string) *GrpcSrv {
	return &GrpcSrv{
		log:             logf.Log.WithName("service.pod").WithValues("name", name),
		sessions:        newSessionMap(),
		deviceConnected: make(chan struct{}),
		globalChan:      make(chan *connectivity.Msg, messageChannelSize),
	}
}

func (p *GrpcSrv) Sync(server connectivity.Connectivity_SyncServer) error {
	if err := func() error {
		// check if device has already connected
		p.mu.Lock()
		defer p.mu.Unlock()

		if p.syncSrv == nil {
			p.syncSrv = server
			return nil
		}
		return fmt.Errorf("device already connected")
	}(); err != nil {
		return err
	}

	defer func() {
		// release device connection, refresh device connection semaphore
		// and orphaned message channel
		p.mu.Lock()
		defer p.mu.Unlock()

		p.syncSrv = nil
		p.deviceConnected = make(chan struct{})
		close(p.globalChan)
		p.globalChan = make(chan *connectivity.Msg, messageChannelSize)
	}()

	// signal device connected
	close(p.deviceConnected)

	ctx, exit := context.WithCancel(server.Context())
	msgCh := make(chan *connectivity.Msg, messageChannelSize)
	go func() {
		for {
			msg, err := server.Recv()
			if err != nil {
				close(msgCh)

				if err != io.EOF {
					exit()
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

			if ch, ok := p.sessions.get(msg.GetSessionId()); ok {
				ch <- msg

				if msg.GetCompleted() {
					p.sessions.del(msg.GetSessionId())
				}
			} else {
				p.globalChan <- msg
			}
		case <-ctx.Done():
			return nil
		}
	}
}

func (p *GrpcSrv) ConsumeOrphanedMessage() <-chan *connectivity.Msg {
	p.mu.RLock()
	defer p.mu.RUnlock()

	return p.globalChan
}

func (p *GrpcSrv) WaitUntilDeviceConnected() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	<-p.deviceConnected
}

// PostCmd sends a command to remote device
func (p *GrpcSrv) PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// fail if device not connected,
	// you should call WaitUntilDeviceConnected first
	// to get notified when device connected
	if p.syncSrv == nil {
		return nil, ErrDeviceNotConnected
	}

	c.SessionId, ch = p.sessions.add(c, timeout)

	err = p.syncSrv.Send(c)
	if err != nil {
		p.sessions.del(c.SessionId)
		return nil, err
	}

	return ch, nil
}
