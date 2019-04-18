package manager

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"arhat.dev/aranya/pkg/node/connectivity"
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
	_, ok := mgr.get(sid)
	assert.Equal(t, false, ok)

	_, more := <-ch
	assert.Equal(t, false, more)
}

func TestSessionManager_Get(t *testing.T) {
	mgr := newSessionManager()
	ctx := context.Background()
	sidA, _ := mgr.add(ctx, connectivity.NewPodListCmd("", "", true))
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Millisecond)
	defer cancel()
	sidB, _ := mgr.add(timeoutCtx, connectivity.NewPodListCmd("", "", true))

	_, ok := mgr.get(sidA)
	assert.Equal(t, true, ok)

	_, ok = mgr.get(sidB)
	assert.Equal(t, true, ok)

	time.Sleep(time.Second)

	_, ok = mgr.get(sidA)
	assert.Equal(t, true, ok)

	_, ok = mgr.get(sidB)
	assert.Equal(t, false, ok)
}
