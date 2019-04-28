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
	config *Config

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

func newBaseManager(config *Config) baseManager {
	disconnected := make(chan struct{})
	close(disconnected)

	return baseManager{
		config:         config,
		sessionManager: newSessionManager(),
		connected:      make(chan struct{}),
		rejected:       make(chan struct{}),
		disconnected:   disconnected,
		globalMsgChan:  make(chan *connectivity.Msg, constant.DefaultConnectivityMsgChannelSize),
	}
}

func (m *baseManager) GlobalMessages() <-chan *connectivity.Msg {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.globalMsgChan
}

// Connected
func (m *baseManager) Connected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.connected
}

// Disconnected
func (m *baseManager) Disconnected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.disconnected
}

// Rejected
func (m *baseManager) Rejected() <-chan struct{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.rejected
}

func (m *baseManager) onReject(reject func()) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.alreadyRejected {
		return
	}
	m.alreadyRejected = true

	reject()

	close(m.rejected)
}

func (m *baseManager) onConnected(setConnected func() bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return ErrManagerClosed
	}

	if setConnected() {
		// signal device connected
		close(m.connected)
		// refresh device disconnected signal
		m.disconnected = make(chan struct{})
		m.rejected = make(chan struct{})
		m.alreadyRejected = false

		return nil
	}

	return ErrDeviceAlreadyConnected
}

func (m *baseManager) onRecvMsg(msg *connectivity.Msg) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return
	}

	if ok := m.sessionManager.dispatch(msg); ok {
		// close session when session is marked complete
		if msg.Completed {
			m.sessionManager.del(msg.SessionId)
		}
	} else {
		m.globalMsgChan <- msg
	}
}

// onDisconnected delete device connection related jobs
func (m *baseManager) onDisconnected(setDisconnected func()) {
	// release device connection, refresh device connection semaphore
	// and orphaned message channel
	m.mu.Lock()
	defer m.mu.Unlock()

	setDisconnected()

	m.sessionManager.cleanup()

	// refresh connected signal
	m.connected = make(chan struct{})
	// signal device disconnected
	close(m.disconnected)

	// refresh global msg chan
	close(m.globalMsgChan)
	m.globalMsgChan = make(chan *connectivity.Msg, constant.DefaultConnectivityMsgChannelSize)
}

func (m *baseManager) onPostCmd(ctx context.Context, cmd *connectivity.Cmd, sendCmd func(c *connectivity.Cmd) error) (ch <-chan *connectivity.Msg, err error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.stopped {
		return nil, ErrManagerClosed
	}

	var (
		sid                uint64
		sessionMustPresent bool
		recordSession      = true
		timeout            = m.config.Timers.UnarySessionTimeout
	)

	// session id should not be empty if it's a input or resize command
	switch c := cmd.GetCmd().(type) {
	case *connectivity.Cmd_CloseSession:
		recordSession = false
		m.sessionManager.del(c.CloseSession)
	case *connectivity.Cmd_Pod:
		switch c.Pod.GetAction() {
		case connectivity.PortForward, connectivity.Exec, connectivity.Attach, connectivity.Log:
			// we don't control stream session timeout,
			// it is controlled by pod manager
			timeout = 0
		case connectivity.ResizeTty, connectivity.Input:
			timeout = 0
			sessionMustPresent = true

			if cmd.GetSessionId() == 0 {
				// session must present, but empty
				return nil, ErrSessionNotValid
			}
		}
	}

	if recordSession {
		sid, ch = m.sessionManager.add(ctx, cmd, timeout)
		defer func() {
			if err != nil {
				m.sessionManager.del(sid)
			}
		}()

		if sessionMustPresent && sid != cmd.GetSessionId() {
			return nil, ErrSessionNotValid
		}

		// TODO: check race condition
		cmd.SessionId = sid
	}

	if err = sendCmd(cmd); err != nil {
		return nil, err
	}

	return ch, nil
}

func (m *baseManager) onStop(closeManager func()) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return
	}

	closeManager()
	m.stopped = true
}
