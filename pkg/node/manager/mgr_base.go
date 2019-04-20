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
	// DeviceConnected signal
	DeviceConnected() <-chan struct{}
	// DeviceDisconnected signal
	DeviceDisconnected() <-chan struct{}
	// GlobalMessages message with no session attached
	GlobalMessages() <-chan *connectivity.Msg
	// send a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error)
}

type baseServer struct {
	sessions       *sessionManager
	globalMsgChan  chan *connectivity.Msg
	sessionTimeout time.Duration
	closed         bool

	deviceConnected    chan struct{}
	deviceDisconnected chan struct{}

	mu sync.RWMutex
}

func newBaseServer() baseServer {
	deviceDisconnected := make(chan struct{})
	close(deviceDisconnected)

	return baseServer{
		sessions:           newSessionManager(),
		deviceConnected:    make(chan struct{}),
		deviceDisconnected: deviceDisconnected,
		globalMsgChan:      make(chan *connectivity.Msg, messageChannelSize),
	}
}

func (s *baseServer) GlobalMessages() <-chan *connectivity.Msg {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.globalMsgChan
}

// DeviceConnected
func (s *baseServer) DeviceConnected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.deviceConnected
}

// DeviceDisconnected
func (s *baseServer) DeviceDisconnected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RLock()

	return s.deviceDisconnected
}

func (s *baseServer) onDeviceConnected(setConnected func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return ErrManagerClosed
	}

	if setConnected() {
		// signal device connected
		close(s.deviceConnected)
		// refresh device disconnected signal
		s.deviceDisconnected = make(chan struct{})

		return nil
	}

	return ErrDeviceAlreadyConnected
}

func (s *baseServer) onDeviceMsg(msg *connectivity.Msg) {
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

// onDeviceDisconnected delete device connection related jobs
func (s *baseServer) onDeviceDisconnected(setDisconnected func()) {
	// release device connection, refresh device connection semaphore
	// and orphaned message channel
	s.mu.Lock()
	defer s.mu.Unlock()

	setDisconnected()

	s.deviceConnected = make(chan struct{})
	s.sessions.cleanup()

	// signal device disconnected
	close(s.deviceDisconnected)

	close(s.globalMsgChan)
	s.globalMsgChan = make(chan *connectivity.Msg, messageChannelSize)
}

func (s *baseServer) onPostCmd(ctx context.Context, cmd *connectivity.Cmd, sendCmd func(c *connectivity.Cmd) error) (ch <-chan *connectivity.Msg, err error) {
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

func (s *baseServer) onStop(closeManager func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return
	}

	closeManager()
	s.closed = true
}
