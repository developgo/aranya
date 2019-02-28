package connectivity

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	messageChannelSize = 10
)

type ConnectivityService struct {
	log             logr.Logger
	sessions        *sessionManager
	deviceConnected chan struct{}
	syncSrv         Connectivity_SyncServer
	mu              sync.RWMutex
	sessionTimeout  time.Duration
	globalChan      chan *Message
}

func NewDeviceService(name string) *ConnectivityService {
	return &ConnectivityService{
		log:             logf.Log.WithName("service.pod").WithValues("name", name),
		sessions:        newSessionMap(),
		deviceConnected: make(chan struct{}),
		globalChan:      make(chan *Message, messageChannelSize),
	}
}

func (p *ConnectivityService) Sync(server Connectivity_SyncServer) error {
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
		p.globalChan = make(chan *Message, messageChannelSize)
	}()

	// signal device connected
	close(p.deviceConnected)

	ctx, exit := context.WithCancel(server.Context())
	msgCh := make(chan *Message, messageChannelSize)
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

func (p *ConnectivityService) ConsumeOrphanedMessage() <-chan *Message {
	return p.globalChan
}

func (p *ConnectivityService) WaitUntilDeviceConnected() {
	p.mu.RLock()
	defer p.mu.RUnlock()

	<-p.deviceConnected
}

// PostCmd sends a command to remote device
func (p *ConnectivityService) PostCmd(c *Cmd, timeout time.Duration) (ch <-chan *Message, err error) {
	p.mu.RLock()
	defer p.mu.RUnlock()

	// fail if device not connected,
	// you should call WaitUntilDeviceConnected first
	// to get notified when device connected
	if p.syncSrv == nil {
		return nil, ErrDeviceNotConnected
	}

	sid, ch := p.sessions.new(c, timeout)
	c.SessionId = sid

	defer func() {
		if err != nil {
			p.sessions.del(sid)
		}
	}()

	err = p.syncSrv.Send(c)
	if err != nil {
		return nil, err
	}

	return ch, nil
}
