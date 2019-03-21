package server

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func TestSessionManager_Add(t *testing.T) {
	mgr := newSessionManager()
	sidA, chA := mgr.add(connectivity.NewPodListCmd("", ""), 0)
	sidB, chB := mgr.add(connectivity.NewContainerTtyResizeCmd(sidA, 0, 0), 0)
	sidC, chC := mgr.add(connectivity.NewPodListCmd("", ""), time.Millisecond)

	assert.NotEqual(t, nil, sidA)
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
	sid, ch := mgr.add(connectivity.NewPodListCmd("", ""), 0)
	mgr.del(sid)
	_, ok := mgr.get(sid)
	assert.Equal(t, false, ok)

	_, more := <-ch
	assert.Equal(t, false, more)
}

func TestSessionManager_Get(t *testing.T) {
	mgr := newSessionManager()
	sidA, _ := mgr.add(connectivity.NewPodListCmd("", ""), 0)
	sidB, _ := mgr.add(connectivity.NewPodListCmd("", ""), time.Millisecond)

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
