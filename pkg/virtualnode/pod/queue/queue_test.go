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

package queue

import (
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/types"
)

func TestWorkQueue_delete(t *testing.T) {
	const (
		workCount = 100
	)

	q := NewWorkQueue()
	for i := 0; i < workCount; i++ {
		assert.NoError(t, q.Offer(ActionCreate, types.UID(strconv.Itoa(i))))
	}

	for i := 0; i < workCount/2; i++ {
		// delete nothing
		q.delete(ActionDelete, types.UID(strconv.Itoa(i)))
		assert.Equal(t, workCount, len(q.queue))
		assert.Equal(t, workCount, len(q.index))
	}

	j := 0
	for i := 0; i < workCount; i += 2 {
		podUID := types.UID(strconv.Itoa(i))
		nextPodUID := types.UID(strconv.Itoa(i + 1))

		q.delete(ActionCreate, podUID)

		assert.Equal(t, workCount-i/2-1, len(q.queue))
		assert.False(t, q.has(ActionCreate, podUID))

		idxInWorkQueue, ok := q.index[Work{Action: ActionCreate, UID: nextPodUID}]
		assert.True(t, ok)
		assert.Equal(t, j, idxInWorkQueue)
		nextWork := q.queue[idxInWorkQueue]
		assert.Equal(t, ActionCreate, nextWork.Action)
		assert.Equal(t, nextPodUID, nextWork.UID)
		j++
	}

}

func TestWorkQueueLogic(t *testing.T) {
	var (
		foo = types.UID("foo")
	)
	q := NewWorkQueue()
	assert.True(t, q.isClosed())
	for i := 0; i < 10000; i++ {
		// work should be invalid since work queue has been closed
		work, more := q.Acquire()
		assert.False(t, more)
		assert.Equal(t, actionInvalid, work.Action)
	}

	q.Start()
	assert.False(t, q.isClosed())

	assert.NoError(t, q.Offer(ActionUpdate, foo))
	assert.Equal(t, ErrWorkDuplicate, q.Offer(ActionUpdate, foo))

	assert.Equal(t, 1, len(q.queue))
	assert.Equal(t, 1, len(q.index))

	work, more := q.Acquire()
	assert.True(t, more)
	assert.Equal(t, ActionUpdate, work.Action)
	assert.Equal(t, foo, work.UID)
	assert.Equal(t, 0, len(q.queue))
	assert.Equal(t, 0, len(q.index))

	assert.NoError(t, q.Offer(ActionCreate, foo))
	assert.Equal(t, ErrWorkDuplicate, q.Offer(ActionCreate, foo))
	assert.Equal(t, 1, len(q.queue))
	assert.Equal(t, 1, len(q.index))

	assert.Equal(t, ErrWorkCounteract, q.Offer(ActionDelete, foo))
	assert.Equal(t, 0, len(q.queue))
	assert.Equal(t, 0, len(q.index))

	assert.NoError(t, q.Offer(ActionUpdate, foo))
	assert.NoError(t, q.Offer(ActionDelete, foo))
	assert.Equal(t, 1, len(q.queue))
	assert.Equal(t, 1, len(q.index))

	work, more = q.Acquire()
	assert.True(t, more)
	assert.Equal(t, ActionDelete, work.Action)
	assert.Equal(t, foo, work.UID)
	assert.Equal(t, 0, len(q.queue))
	assert.Equal(t, 0, len(q.index))
}

func TestWorkQueueAction(t *testing.T) {
	const (
		WorkCount    = 100
		TargetAction = ActionCreate
		WaitTime     = 10 * time.Millisecond
	)

	q := NewWorkQueue()

	sigCh := make(chan struct{})
	finished := func() bool {
		select {
		case <-sigCh:
			return true
		default:
			return false
		}
	}

	startTime := time.Now()
	go func() {
		defer close(sigCh)

		for i := 0; i < WorkCount; i++ {
			if i == WorkCount/4 {
				q.Start()
			}

			if i == WorkCount/2 {
				q.Stop()
			}

			if i == WorkCount*3/4 {
				q.Start()
			}

			time.Sleep(WaitTime)
			q.Offer(ActionCreate, types.UID(strconv.Itoa(i)))
		}
	}()

	invalidCount := 0
	validCount := 0
	for !finished() {
		work, more := q.Acquire()

		if q.isClosed() {
			invalidCount++
			assert.False(t, more)
			assert.Equal(t, actionInvalid, work.Action)
		} else {
			validCount++
			assert.True(t, more)
			assert.Equal(t, TargetAction, work.Action)
		}
	}

	if time.Since(startTime) < WorkCount*WaitTime {
		t.Error("work time less than expected")
	}
	if invalidCount == 0 {
		t.Error("invalid count should not be zero")
	}

	assert.Equal(t, WorkCount, validCount)
}
