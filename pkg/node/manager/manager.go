package manager

import (
	"context"
	"errors"
	"sync"
	"time"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
)

const (
	messageChannelSize = 10
)

var log = logf.Log.WithName("node.manager")

var (
	ErrDeviceAlreadyConnected = errors.New("device has already connected")
	ErrDeviceNotConnected     = errors.New("device is not connected")
	ErrSessionNotValid        = errors.New("session must present in this command")
	ErrManagerClosed          = errors.New("connectivity manager has been closed")
)

type Interface interface {
	// Start manager and block until stopped
	Start() error
	// Stop manager at once
	Stop()
	// Connected signal
	Connected() <-chan struct{}
	// Disconnected signal
	Disconnected() <-chan struct{}
	// GlobalMessages message with no session attached
	GlobalMessages() <-chan *connectivity.Msg
	// send a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error)
}

type baseManager struct {
	sessions       *sessionManager
	globalMsgChan  chan *connectivity.Msg
	sessionTimeout time.Duration
	closed         bool

	// signals
	connected    chan struct{}
	disconnected chan struct{}

	mu sync.RWMutex
}

func newBaseServer() baseManager {
	deviceDisconnected := make(chan struct{})
	close(deviceDisconnected)

	return baseManager{
		sessions:      newSessionManager(),
		connected:     make(chan struct{}),
		disconnected:  deviceDisconnected,
		globalMsgChan: make(chan *connectivity.Msg, messageChannelSize),
	}
}

func (s *baseManager) GlobalMessages() <-chan *connectivity.Msg {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.globalMsgChan
}

// Connected
func (s *baseManager) Connected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.connected
}

// Disconnected
func (s *baseManager) Disconnected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.disconnected
}

func (s *baseManager) onConnected(setConnected func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrManagerClosed
	}

	if setConnected() {
		// signal device connected
		close(s.connected)
		// refresh device disconnected signal
		s.disconnected = make(chan struct{})

		return nil
	}

	return ErrDeviceAlreadyConnected
}

func (s *baseManager) onRecvMsg(msg *connectivity.Msg) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return
	}

	if ch, ok := s.sessions.get(msg.GetSessionId()); ok {
		ch <- msg

		// close session when error happened on device or session complete
		if msg.GetCompleted() || msg.Error() != nil {
			s.sessions.del(msg.GetSessionId())
		}
	} else {
		s.globalMsgChan <- msg
	}
}

// onDisconnected delete device connection related jobs
func (s *baseManager) onDisconnected(setDisconnected func()) {
	// release device connection, refresh device connection semaphore
	// and orphaned message channel
	s.mu.Lock()
	defer s.mu.Unlock()

	setDisconnected()

	s.sessions.cleanup()

	// refresh connected signal
	s.connected = make(chan struct{})
	// signal device disconnected
	close(s.disconnected)
	// refresh global msg chan
	close(s.globalMsgChan)
	s.globalMsgChan = make(chan *connectivity.Msg, messageChannelSize)
}

func (s *baseManager) onPostCmd(ctx context.Context, cmd *connectivity.Cmd, sendCmd func(c *connectivity.Cmd) error) (ch <-chan *connectivity.Msg, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return nil, ErrManagerClosed
	}

	var (
		sid                uint64
		sessionMustPresent bool
	)

	// session id should not be empty if it's a input or resize command
	switch c := cmd.GetCmd().(type) {
	case *connectivity.Cmd_PodCmd:
		switch c.PodCmd.GetAction() {
		case connectivity.ResizeTty, connectivity.Input:
			sessionMustPresent = true
			if cmd.GetSessionId() == 0 {
				return nil, ErrSessionNotValid
			}
		}
	}

	sid, ch = s.sessions.add(ctx, cmd)
	defer func() {
		if err != nil {
			s.sessions.del(sid)
		}
	}()

	if sessionMustPresent && sid != cmd.GetSessionId() {
		return nil, ErrSessionNotValid
	}

	// TODO: check race condition
	cmd.SessionId = sid

	if err := sendCmd(cmd); err != nil {
		return nil, err
	}

	return ch, nil
}

func (s *baseManager) onStop(closeManager func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	closeManager()
	s.closed = true
}
