package node

import (
	"sync/atomic"
)

const (
	statusFree = iota
	statusRunning
	statusStopped
)

func (s *Server) isRunning() bool {
	return atomic.LoadUint32(&s.status) == statusRunning
}

func (s *Server) markRunning() {
	atomic.StoreUint32(&s.status, statusRunning)
}

func (s *Server) isStopped() bool {
	return atomic.LoadUint32(&s.status) == statusStopped
}

func (s *Server) markStopped() {
	atomic.StoreUint32(&s.status, statusStopped)
}
