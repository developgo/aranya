package agent

import (
	"io"
	"sync"

	"k8s.io/client-go/tools/remotecommand"
)

type streamRW struct {
	r io.ReadCloser
	w io.WriteCloser
}

func newStreamRW() *streamRW {
	r, w := io.Pipe()
	return &streamRW{r: r, w: w}
}

func (s *streamRW) close() {
	_ = s.r.Close()
	_ = s.w.Close()
}

type streamSession struct {
	inputWriter map[uint64]*streamRW
	resizeCh    map[uint64]chan remotecommand.TerminalSize
	mu          sync.RWMutex
}

func (s *streamSession) add(sid uint64, rw *streamRW, resizeCh chan remotecommand.TerminalSize) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldRW, ok := s.inputWriter[sid]; ok {
		oldRW.close()
		delete(s.inputWriter, sid)
	}

	if oldResizeCh, ok := s.resizeCh[sid]; ok {
		close(oldResizeCh)
		delete(s.resizeCh, sid)
	}

	if rw != nil {
		s.inputWriter[sid] = rw
	}

	if resizeCh != nil {
		s.resizeCh[sid] = resizeCh
	}
}

func (s *streamSession) getInputWriter(sid uint64) (io.Writer, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rw, ok := s.inputWriter[sid]
	if !ok {
		return nil, false
	}

	return rw.w, true
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

	if w, ok := s.inputWriter[sid]; ok {
		w.close()
		delete(s.inputWriter, sid)
	}

	if ch, ok := s.resizeCh[sid]; ok {
		close(ch)
		delete(s.resizeCh, sid)
	}
}
