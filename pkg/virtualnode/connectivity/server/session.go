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
	"sync"
	"unsafe"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type session struct {
	ctx     context.Context
	ctxExit context.CancelFunc
	msgCh   chan *connectivity.Msg

	closed bool
	mu     sync.RWMutex
}

func (s *session) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true
	s.ctxExit()
	close(s.msgCh)
}

func (s *session) deliver(msg *connectivity.Msg) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	select {
	case <-s.ctx.Done():
		return false
	default:
		break
	}

	if !s.closed {
		s.msgCh <- msg
		return true
	}

	return false
}

type sessionManager struct {
	m  map[uint64]*session
	mu sync.RWMutex
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		m: make(map[uint64]*session),
	}
}

func (s *sessionManager) add(ctx context.Context, cmd *connectivity.Cmd) (sid uint64, ch chan *connectivity.Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sid = cmd.SessionId
	if oldSession, ok := s.m[sid]; ok {
		ch = oldSession.msgCh
	} else {
		sid = *(*uint64)(unsafe.Pointer(&cmd))

		ch = make(chan *connectivity.Msg, 1)

		session := &session{msgCh: ch}
		session.ctx, session.ctxExit = context.WithCancel(ctx)
		go func() {
			select {
			case <-session.ctx.Done():
				s.del(sid)
			}
		}()

		s.m[sid] = session
	}

	return sid, ch
}

func (s *sessionManager) dispatch(msg *connectivity.Msg) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.m[msg.GetSessionId()]
	if ok {
		return session.deliver(msg)
	}

	return false
}

func (s *sessionManager) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.m[sid]; ok {
		session.close()
		delete(s.m, sid)
	}
}

func (s *sessionManager) cleanup() {
	s.mu.Lock()
	defer s.mu.Unlock()

	keys := make([]uint64, len(s.m))
	i := 0
	for key, session := range s.m {
		session.close()
		keys[i] = key
		i++
	}

	for _, k := range keys {
		delete(s.m, k)
	}
}
