package manager

import (
	"context"
	"sync"
	"unsafe"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type session struct {
	ctx     context.Context
	ctxExit context.CancelFunc
	msgCh   chan *connectivity.Msg
}

func (s *session) close() {
	s.ctxExit()
	close(s.msgCh)
}

func (s *session) isClosed() bool {
	select {
	case <-s.ctx.Done():
		return true
	default:
		return false
	}
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

func (s *sessionManager) get(sid uint64) (chan *connectivity.Msg, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.m[sid]
	if ok {
		if session.isClosed() {
			s.del(sid)
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
