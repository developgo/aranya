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

package server

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"arhat.dev/aranya/pkg/connectivity"
)

func TestSessionManager_Add(t *testing.T) {
	mgr := newSessionManager()
	ctx := context.Background()
	sidA, chA := mgr.add(ctx, connectivity.NewPodListCmd("", "", true))
	sidB, chB := mgr.add(ctx, connectivity.NewContainerTtyResizeCmd(sidA, 0, 0))
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()
	sidC, chC := mgr.add(timeoutCtx, connectivity.NewPodListCmd("", "", true))

	assert.NotNil(t, sidA)
	assert.Equal(t, sidA, sidB)
	assert.Equal(t, chA, chB)
	assert.NotEqual(t, chA, chC)
	assert.NotEqual(t, sidA, sidC)

	time.Sleep(time.Second)
	_, more := <-chC
	assert.Equal(t, false, more)
}

func TestSessionManager_Del(t *testing.T) {
	mgr := newSessionManager()
	ctx := context.Background()
	sid, ch := mgr.add(ctx, connectivity.NewPodListCmd("", "", true))
	mgr.del(sid)
	ok := mgr.dispatch(nil)
	assert.Equal(t, false, ok)

	_, more := <-ch
	assert.Equal(t, false, more)
}

func TestSessionManager_Get(t *testing.T) {
	// mgr := newSessionManager()
	// ctx := context.Background()
	// sidA, _ := mgr.add(ctx, connectivity.NewPodListCmd("", "", true))
	// timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
	// defer cancel()
	// sidB, _ := mgr.add(timeoutCtx, connectivity.NewPodListCmd("", "", true))

	// ok := mgr.dispatch(sidA)
	// assert.Equal(t, true, ok)
	//
	// _, ok = mgr.dispatch(sidB)
	// assert.Equal(t, true, ok)
	//
	// time.Sleep(time.Second)
	//
	// _, ok = mgr.dispatch(sidA)
	// assert.Equal(t, true, ok)
	//
	// _, ok = mgr.dispatch(sidB)
	// assert.Equal(t, false, ok)
}
