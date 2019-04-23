package agent

import (
	"context"
	"io"
	"sync"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type streamHandler struct {
	_ctx context.Context

	r    io.ReadCloser
	_w   io.WriteCloser
	_wCh chan []byte
	sCh  chan *connectivity.TtyResizeOptions

	_closed bool
	_mu     sync.RWMutex
}

func newStreamRW(parentCtx context.Context) *streamHandler {
	r, w := io.Pipe()
	h := &streamHandler{
		_ctx: parentCtx,
		r:    r,
		_w:   w,
		_wCh: make(chan []byte, 1),
		sCh:  make(chan *connectivity.TtyResizeOptions, 1),
	}

	go func() {
		for {
			select {
			case <-parentCtx.Done():
				return
			case data, more := <-h._wCh:
				if !more {
					return
				}
				_, _ = h._w.Write(data)
			}
		}
	}()

	return h
}

func (s *streamHandler) resize(size *connectivity.TtyResizeOptions) {
	s._mu.RLock()
	defer s._mu.RUnlock()
	if s._closed {
		return
	}

	select {
	case <-s._ctx.Done():
		return
	case s.sCh <- size:
	}
}

func (s *streamHandler) write(data []byte) {
	s._mu.RLock()
	defer s._mu.RUnlock()
	if s._closed {
		return
	}

	select {
	case <-s._ctx.Done():
		return
	case s._wCh <- data:
	}
}

func (s *streamHandler) close() {
	s._mu.Lock()
	defer s._mu.Unlock()

	s._closed = true

	_ = s.r.Close()
	_ = s._w.Close()

	close(s.sCh)
	close(s._wCh)
}

type streamSession struct {
	streamHandlers map[uint64]*streamHandler
	mu             sync.RWMutex
}

func (s *streamSession) add(sid uint64, rw *streamHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldH, ok := s.streamHandlers[sid]; ok {
		oldH.close()
		delete(s.streamHandlers, sid)
	}

	if rw != nil {
		s.streamHandlers[sid] = rw
	}
}

func (s *streamSession) getStreamHandler(sid uint64) (*streamHandler, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rw, ok := s.streamHandlers[sid]
	if !ok {
		return nil, false
	}

	return rw, true
}

func (s *streamSession) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, ok := s.streamHandlers[sid]; ok {
		w.close()
		delete(s.streamHandlers, sid)
	}
}
