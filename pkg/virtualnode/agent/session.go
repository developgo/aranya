package agent

import (
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

type streamSession struct {
	inputCh  map[uint64]chan []byte
	resizeCh map[uint64]chan remotecommand.TerminalSize
	mu       sync.RWMutex
}

func (s *streamSession) add(sid uint64, dataCh chan []byte, resizeCh chan remotecommand.TerminalSize) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldInputCh, ok := s.inputCh[sid]; ok {
		close(oldInputCh)
	}

	if oldResizeCh, ok := s.resizeCh[sid]; ok {
		close(oldResizeCh)
	}

	s.inputCh[sid] = dataCh
	s.resizeCh[sid] = resizeCh
}

func (s *streamSession) getInputChan(sid uint64) (chan []byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.inputCh[sid]
	return ch, ok
}

func (s *streamSession) getResizeChan(sid uint64) (chan remotecommand.TerminalSize, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.resizeCh[sid]
	return ch, ok
}

func (s *streamSession) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.inputCh[sid]; ok {
		if ch != nil {
			close(ch)
		}
		delete(s.inputCh, sid)
	}

	if ch, ok := s.resizeCh[sid]; ok {
		if ch != nil {
			close(ch)
		}
		delete(s.resizeCh, sid)
	}
}
