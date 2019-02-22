package node

import (
	"sync/atomic"
)

const (
	statusReady   = 0
	statusRunning = 1
	statusStopped = 2
)

func (s *Node) isRunning() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.status == statusRunning
}

func (s *Node) markRunning() {
	atomic.StoreUint32(&s.status, statusRunning)
}

func (s *Node) isStopped() bool {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	return s.status == statusStopped
}

func (s *Node) markStopped() {
	atomic.StoreUint32(&s.status, statusStopped)
}
