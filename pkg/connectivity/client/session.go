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

package client

import (
	"context"
	"io"
	"sync"

	"arhat.dev/aranya/pkg/connectivity"
)

type streamHandler struct {
	ctx context.Context

	r   io.ReadCloser
	w   io.WriteCloser
	wCh chan []byte
	sCh chan *connectivity.TtyResizeOptions

	closed bool
	mu     sync.RWMutex
}

func newStreamRW(parentCtx context.Context) *streamHandler {
	r, w := io.Pipe()
	h := &streamHandler{
		ctx: parentCtx,
		r:   r,
		w:   w,
		wCh: make(chan []byte, 1),
		sCh: make(chan *connectivity.TtyResizeOptions, 1),
	}

	go func() {
		for {
			select {
			case <-h.ctx.Done():
				return
			case data, more := <-h.wCh:
				if !more {
					return
				}
				// pipe writer will block until all data
				// has been read or reader has been closed
				_, _ = h.w.Write(data)
			}
		}
	}()

	return h
}

func (s *streamHandler) resize(size *connectivity.TtyResizeOptions) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return
	}

	select {
	case <-s.ctx.Done():
		return
	case s.sCh <- size:
	}
}

func (s *streamHandler) write(data []byte) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.closed {
		return
	}

	select {
	case <-s.ctx.Done():
		return
	case s.wCh <- data:
	}
}

func (s *streamHandler) close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.closed = true

	_ = s.r.Close()
	_ = s.w.Close()

	close(s.sCh)
	close(s.wCh)
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
