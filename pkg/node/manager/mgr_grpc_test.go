package manager

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func newTestGrpcSrvAndStub() (connMgr *GRPCManager, stub connectivity.ConnectivityClient) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	connMgr = NewGRPCManager(grpc.NewServer(), l)
	go func() {
		if err := connMgr.Start(); err != nil {
			panic(err)
		}
	}()

	conn, err := grpc.DialContext(context.TODO(), l.Addr().String(),
		grpc.WithInsecure(),
		grpc.WithBlock())
	if err != nil {
		panic(err)
	}
	stub = connectivity.NewConnectivityClient(conn)

	return
}

func TestNewGrpcConnectivity(t *testing.T) {
	c := NewGRPCManager(grpc.NewServer(), nil)
	assert.NotEmpty(t, c)
	assert.NotEmpty(t, c.sessions)
	assert.Empty(t, c.syncSrv)
}

func TestGrpcSrv(t *testing.T) {
	const (
		orphanedMsgCount = 10
	)

	mgr, stub := newTestGrpcSrvAndStub()
	defer mgr.Stop()

	cmd := connectivity.NewPodListCmd("foo", "bar", true)

	msgCh, err := mgr.PostCmd(context.TODO(), cmd)
	assert.Error(t, err)
	assert.Empty(t, msgCh)

	syncClient, err := stub.Sync(context.TODO())
	assert.NoError(t, err, "start sync client failed")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		i := 0
		for msg := range mgr.GlobalMessages() {
			i++
			assert.NotEmpty(t, msg)
		}

		assert.Equal(t, orphanedMsgCount, i)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-mgr.DeviceConnected()

		msgCh, err := mgr.PostCmd(context.TODO(), cmd)
		assert.NoError(t, err)
		assert.NotEqual(t, nil, msgCh)

		msg, more := <-msgCh
		assert.True(t, more)
		assert.NotEmpty(t, msg)
		assert.Equal(t, cmd.GetSessionId(), msg.GetSessionId())

		_, more = <-msgCh
		assert.False(t, more)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		cmdRecv, err := syncClient.Recv()
		assert.NoError(t, err)
		assert.Equal(t, cmd.GetSessionId(), cmdRecv.GetSessionId())

		err = syncClient.Send(&connectivity.Msg{
			SessionId: cmdRecv.GetSessionId(),
			Completed: true,
			Msg: &connectivity.Msg_Pod{
				Pod: &connectivity.Pod{},
			},
		})
		assert.NoError(t, err)

		for i := 0; i < orphanedMsgCount; i++ {
			err = syncClient.Send(&connectivity.Msg{
				Msg: &connectivity.Msg_Node{
					Node: &connectivity.Node{},
				},
			})
			assert.NoError(t, err)
		}

		err = syncClient.CloseSend()
		assert.NoError(t, err)
	}()

	wg.Wait()
}
