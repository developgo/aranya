package connectivity

import (
	"sync"
	"time"
	"unsafe"
)

type sessionManager struct {
	timeoutMap map[uint64]time.Duration
	timerMap   map[uint64]*time.Timer
	msgMap     map[uint64]chan *Message
	mu         sync.RWMutex
}

func (s *sessionManager) new(cmd *Cmd, timeout time.Duration) (uint64, chan *Message) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ch := make(chan *Message, 1)
	sid := *(*uint64)(unsafe.Pointer(&cmd))

	s.msgMap[sid] = ch

	if timeout > 0 {
		t := time.NewTimer(timeout)

		go func() {
			<-t.C
			s.del(sid)
		}()

		s.timerMap[sid] = t
		s.timeoutMap[sid] = timeout
	}

	return sid, ch
}

func (s *sessionManager) get(sid uint64) (chan *Message, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.msgMap[sid]
	if ok {
		s.timerMap[sid].Reset(s.timeoutMap[sid])
	}

	return ch, ok
}

func (s *sessionManager) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.msgMap[sid]; ok {
		close(ch)
		delete(s.msgMap, sid)

		s.timerMap[sid].Stop()
		delete(s.timerMap, sid)
		delete(s.timeoutMap, sid)
	}
}

func newSessionMap() *sessionManager {
	return &sessionManager{
		msgMap: make(map[uint64]chan *Message),
	}
}
