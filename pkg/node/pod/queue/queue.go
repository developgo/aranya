package queue

import (
	"sync"
	"sync/atomic"

	"k8s.io/apimachinery/pkg/types"
)

type workType uint8

const (
	actionInvalid workType = iota
	ActionCreate
	ActionUpdate
	ActionDelete
)

var actionNames = map[workType]string{
	actionInvalid: "Invalid",
	ActionCreate:  "Create",
	ActionUpdate:  "Update",
	ActionDelete:  "Delete",
}

type Work struct {
	Action workType
	UID    types.UID
}

func (w Work) String() string {
	return actionNames[w.Action] + "/" + string(w.UID)
}

// NewWorkQueue will create a stopped new work queue,
// you can offer work to it, but any acquire will fail until
// you have called its Start()
func NewWorkQueue() *WorkQueue {
	// prepare a closed channel for this work queue
	hasWork := make(chan struct{})
	close(hasWork)

	return &WorkQueue{
		queue: make([]Work, 0, 16),
		index: make(map[Work]int),

		// set work queue to closed
		hasWork:    hasWork,
		chanClosed: true,
		closed:     1,
	}
}

// WorkQueue
type WorkQueue struct {
	queue []Work
	index map[Work]int

	hasWork    chan struct{}
	chanClosed bool
	mu         sync.RWMutex

	// protected by atomic
	closed uint32
}

func (q *WorkQueue) has(action workType, uid types.UID) bool {
	_, ok := q.index[Work{Action: action, UID: uid}]
	return ok
}

func (q *WorkQueue) add(w Work) {
	q.index[w] = len(q.queue)
	q.queue = append(q.queue, w)
}

func (q *WorkQueue) delete(action workType, uid types.UID) {
	workToDelete := Work{Action: action, UID: uid}
	if idx, ok := q.index[workToDelete]; ok {
		delete(q.index, workToDelete)
		q.queue = append(q.queue[:idx], q.queue[idx+1:]...)

		// refresh index
		for i, w := range q.queue {
			q.index[w] = i
		}
	}
}

func (q *WorkQueue) Remains() []Work {
	q.mu.RLock()
	defer q.mu.RUnlock()

	works := make([]Work, len(q.queue))
	for i, w := range q.queue {
		works[i] = Work{Action: w.Action, UID: w.UID}
	}
	return works
}

// Acquire a work item from the work queue
// if shouldAcquireMore is false, w will be an empty work
func (q *WorkQueue) Acquire() (w Work, shouldAcquireMore bool) {
	// wait until we have got some work to do
	// or we have stopped stopped work queue
	<-q.hasWork

	if q.isClosed() {
		return Work{Action: actionInvalid}, false
	}

	q.mu.Lock()
	defer func() {
		if len(q.queue) == 0 {
			if !q.isClosed() {
				q.hasWork = make(chan struct{})
			}
		}

		q.mu.Unlock()
	}()

	w = q.queue[0]
	q.delete(w.Action, w.UID)

	return w, true
}

// Offer a work item to the work queue
// if offered work was not added, a false result will return, otherwise true
func (q *WorkQueue) Offer(action workType, uid types.UID) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	newWork := Work{Action: action, UID: uid}
	_, hasSameWorkForSamePod := q.index[newWork]
	if hasSameWorkForSamePod {
		return false
	}

	switch action {
	case ActionCreate:
		if q.has(ActionUpdate, uid) {
			// pod create should only happen when pod doesn't exists or has been deleted
			return false
		}

		q.add(newWork)
	case ActionUpdate:
		if q.has(ActionCreate, uid) || q.has(ActionDelete, uid) {
			// pod update should only happens when pod has been created and running
			return false
		}

		q.add(newWork)
	case ActionDelete:
		// pod need to be deleted
		if q.has(ActionCreate, uid) {
			// cancel according create work
			q.delete(ActionCreate, uid)
			return false
		}

		if q.has(ActionUpdate, uid) {
			// cancel according update work
			q.delete(ActionUpdate, uid)
		}

		q.add(newWork)
	}

	// we reach here means we have added some work to the queue
	// we should signal those consumers to go for it
	select {
	case <-q.hasWork:
		// we can reach here means q.hasWork has been closed
	default:
		// release the signal
		close(q.hasWork)
		// mark the channel closed to prevent a second close which would panic
		q.chanClosed = true
	}

	return true
}

// Start do nothing but mark you can perform acquire
// actions to the work queue
func (q *WorkQueue) Start() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.chanClosed && len(q.queue) == 0 {
		// reopen signal channel for wait
		q.hasWork = make(chan struct{})
		q.chanClosed = false
	}

	atomic.StoreUint32(&q.closed, 0)
}

// Stop do nothing but mark this work queue is closed,
// you should not perform acquire actions to the work queue
func (q *WorkQueue) Stop() {
	q.mu.Lock()
	defer q.mu.Unlock()

	if !q.chanClosed {
		// close wait channel to prevent wait
		close(q.hasWork)
		q.chanClosed = true
	}

	atomic.StoreUint32(&q.closed, 1)
}

func (q *WorkQueue) isClosed() bool {
	return atomic.LoadUint32(&q.closed) == 1
}
