package server

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
)

const (
	messageChannelSize = 10
)

var (
	ErrDeviceNotConnected = errors.New("error device not connected ")
)

type Interface interface {
	WaitUntilDeviceConnected() <-chan struct{}
	ConsumeGlobalMsg() <-chan *connectivity.Msg
	// send a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(c *connectivity.Cmd, timeout time.Duration) (ch <-chan *connectivity.Msg, err error)
}

type baseServer struct {
	log             logr.Logger
	sessions        *sessionManager
	deviceConnected chan struct{}
	globalMsgChan   chan *connectivity.Msg
	sessionTimeout  time.Duration

	mu sync.RWMutex
}

func newBaseServer(name string) baseServer {
	return baseServer{
		log:             logf.Log.WithName("connectivity.server").WithValues("name", name),
		sessions:        newSessionManager(),
		deviceConnected: make(chan struct{}),
		globalMsgChan:   make(chan *connectivity.Msg, messageChannelSize),
	}
}

func (s *baseServer) ConsumeGlobalMsg() <-chan *connectivity.Msg {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.globalMsgChan
}

// WaitUntilDeviceConnected
func (s *baseServer) WaitUntilDeviceConnected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.deviceConnected
}

func (s *baseServer) onDeviceConnected(setConnected func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if setConnected() {
		// signal device connected
		close(s.deviceConnected)

		return nil
	}

	return fmt.Errorf("device already connected")
}

func (s *baseServer) onDeviceMsg(msg *connectivity.Msg) {
	if ch, ok := s.sessions.get(msg.GetSessionId()); ok {
		ch <- msg

		if msg.GetCompleted() {
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
	close(s.globalMsgChan)
	s.globalMsgChan = make(chan *connectivity.Msg, messageChannelSize)
}

func (s baseServer) onPostCmd(cmd *connectivity.Cmd, timeout time.Duration, sendCmd func(c *connectivity.Cmd) error) (ch <-chan *connectivity.Msg, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cmd.SessionId, ch = s.sessions.add(cmd, timeout)

	if err := sendCmd(cmd); err != nil {
		s.sessions.del(cmd.SessionId)
		return nil, err
	}

	return ch, nil
}
