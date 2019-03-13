package server

import (
	"sync"
	"time"
	"unsafe"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type session struct {
	timeout time.Duration
	timer   *time.Timer
	msgCh   chan *connectivity.Msg
}

type sessionManager struct {
	m  map[uint64]*session
	mu sync.RWMutex
}

func (s *sessionManager) add(cmd *connectivity.Cmd, timeout time.Duration) (sid uint64, ch chan *connectivity.Msg) {
	s.mu.Lock()
	defer s.mu.Unlock()

	sid = cmd.SessionId
	if oldSession, ok := s.m[sid]; ok {
		ch = oldSession.msgCh
	} else {
		sid = *(*uint64)(unsafe.Pointer(&cmd))

		ch = make(chan *connectivity.Msg, 1)
		s.m[sid] = &session{
			msgCh:   ch,
			timeout: timeout,
			timer: func() *time.Timer {
				if timeout > 0 {
					t := time.NewTimer(timeout)
					go func() {
						<-t.C
						s.del(sid)
					}()
					return t
				}
				return nil
			}(),
		}
	}

	return sid, ch
}

func (s *sessionManager) get(sid uint64) (chan *connectivity.Msg, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.m[sid]
	if ok {
		if session.timeout > 0 {
			// reset timeout with best effort
			if session.timer.Stop() {
				session.timer.Reset(session.timeout)
			}
		}

		return session.msgCh, true
	} else {
		return nil, false
	}
}

func (s *sessionManager) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if session, ok := s.m[sid]; ok {
		if session.timer != nil {
			session.timer.Stop()
		}
		close(session.msgCh)
		delete(s.m, sid)
	}
}

func newSessionManager() *sessionManager {
	return &sessionManager{
		m: make(map[uint64]*session),
	}
}
