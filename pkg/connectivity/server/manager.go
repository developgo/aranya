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
	"errors"
	"sync"

	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/constant"
)

var log = logf.Log.WithName("manager")

var (
	ErrDeviceAlreadyConnected = errors.New("device has already connected")
	ErrDeviceNotConnected     = errors.New("device is not connected")
	ErrSessionNotValid        = errors.New("session must present in this command")
	ErrManagerClosed          = errors.New("connectivity manager has been closed")
)

// Manager is the connectivity manager interface, and is designed for message queue based
// managers such as MQTT
type Manager interface {
	// Start manager and block until stopped
	Start() error
	// Stop manager at once
	Stop()
	// Reject current device connection if any
	Reject(reason connectivity.RejectReason, message string)
	// Connected signal
	Connected() <-chan struct{}
	// Disconnected signal
	Disconnected() <-chan struct{}
	// GlobalMessages message with no session attached
	GlobalMessages() <-chan *connectivity.Msg
	// dispatch a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(ctx context.Context, c *connectivity.Cmd) (ch <-chan *connectivity.Msg, err error)
}

type baseManager struct {
	Config

	sessionManager *sessionManager
	globalMsgChan  chan *connectivity.Msg

	// signals
	connected       chan struct{}
	disconnected    chan struct{}
	rejected        chan struct{}
	alreadyRejected bool

	// status
	stopped bool
	mu      sync.RWMutex
}

func newBaseManager() baseManager {
	disconnected := make(chan struct{})
	close(disconnected)

	return baseManager{
		sessionManager: newSessionManager(),
		connected:      make(chan struct{}),
		rejected:       make(chan struct{}),
		disconnected:   disconnected,
		globalMsgChan:  make(chan *connectivity.Msg, constant.DefaultConnectivityMsgChannelSize),
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

	if s.alreadyRejected {
		s.rejected = make(chan struct{})
	}
	s.alreadyRejected = false

	return s.connected
}

// Disconnected
func (s *baseManager) Disconnected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.disconnected
}

func (s *baseManager) Rejected() <-chan struct{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rejected
}

func (s *baseManager) onReject(reject func()) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.alreadyRejected {
		return
	}
	s.alreadyRejected = true

	reject()

	close(s.rejected)
}

func (s *baseManager) onConnected(setConnected func() bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
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

	if s.stopped {
		return
	}

	if ok := s.sessionManager.dispatch(msg); ok {
		// close session when session is marked complete
		if msg.GetCompleted() {
			s.sessionManager.del(msg.GetSessionId())
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

	s.sessionManager.cleanup()

	// refresh connected signal
	s.connected = make(chan struct{})
	// signal device disconnected
	close(s.disconnected)

	// refresh global msg chan
	close(s.globalMsgChan)
	s.globalMsgChan = make(chan *connectivity.Msg, constant.DefaultConnectivityMsgChannelSize)
}

func (s *baseManager) onPostCmd(ctx context.Context, cmd *connectivity.Cmd, sendCmd func(c *connectivity.Cmd) error) (ch <-chan *connectivity.Msg, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.stopped {
		return nil, ErrManagerClosed
	}

	var (
		sid                uint64
		sessionMustPresent bool
		recordSession      = true
		timeout            = s.Timers.UnarySessionTimeout
	)

	// session id should not be empty if it's a input or resize command
	switch c := cmd.GetCmd().(type) {
	case *connectivity.Cmd_CloseSession:
		recordSession = false
	case *connectivity.Cmd_Pod:
		switch c.Pod.GetAction() {
		// we don't control stream session timeout,
		// it is controlled by pod manager
		case connectivity.PortForward, connectivity.Exec, connectivity.Attach, connectivity.Log:
			timeout = 0
		case connectivity.ResizeTty, connectivity.Input:
			timeout = 0
			sessionMustPresent = true
			if cmd.GetSessionId() == 0 {
				return nil, ErrSessionNotValid
			}
		}
	}

	if recordSession {
		sid, ch = s.sessionManager.add(ctx, cmd, timeout)
		defer func() {
			if err != nil {
				s.sessionManager.del(sid)
			}
		}()

		if sessionMustPresent && sid != cmd.GetSessionId() {
			return nil, ErrSessionNotValid
		}

		// TODO: check race condition
		cmd.SessionId = sid
	}

	if err := sendCmd(cmd); err != nil {
		return nil, err
	}

	return ch, nil
}

func (s *baseManager) onStop(closeManager func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.stopped {
		return
	}

	closeManager()
	s.stopped = true
}
