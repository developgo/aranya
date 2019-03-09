package server

import (
	"context"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func newTestGrpcSrvAndStub() (srvStop func(), connectivitySrv *GrpcManager, stub connectivity.ConnectivityClient) {
	connectivitySrv = NewGrpcManager("test").(*GrpcManager)
	srv := grpc.NewServer()
	connectivity.RegisterConnectivityServer(srv, connectivitySrv)

	l, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}

	go func() {
		if err := srv.Serve(l); err != nil {
			panic(err)
		}
	}()

	srvStop = srv.Stop

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
	c := NewGrpcManager("test").(*GrpcManager)
	assert.NotEmpty(t, c)
	assert.NotEmpty(t, c.sessions)
	assert.Empty(t, c.syncSrv)
}

func TestGrpcSrv(t *testing.T) {
	const (
		orphanedMsgCount = 10
	)
	srvStop, srv, stub := newTestGrpcSrvAndStub()
	defer srvStop()

	cmd := NewPodListCmd("foo", "bar")

	msgCh, err := srv.PostCmd(cmd, 0)
	assert.Error(t, err)
	assert.Empty(t, msgCh)

	syncClient, err := stub.Sync(context.TODO())
	assert.NoError(t, err, "start sync client failed")

	wg := &sync.WaitGroup{}
	wg.Add(1)
	go func() {
		defer wg.Done()

		i := 0
		for msg := range srv.ConsumeGlobalMsg() {
			i++
			assert.NotEmpty(t, msg)
			assert.Equal(t, []byte("foo"), msg.GetNodeInfo().GetNodeV1())
		}

		assert.Equal(t, orphanedMsgCount, i)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-srv.WaitUntilDeviceConnected()

		msgCh, err := srv.PostCmd(cmd, 0)
		assert.NoError(t, err)
		assert.NotEqual(t, nil, msgCh)

		msg, more := <-msgCh
		assert.True(t, more)
		assert.NotEmpty(t, msg)
		assert.Equal(t, cmd.GetSessionId(), msg.GetSessionId())
		assert.Equal(t, "foo", string(msg.GetPodInfo().GetPodV1()))

		_, more = <-msgCh
		assert.False(t, more)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		cmdRecv, err := syncClient.Recv()
		assert.NoError(t, err)
		assert.Equal(t, cmd.GetSessionId(), cmdRecv.GetSessionId())
		assert.Equal(t, "foo", cmdRecv.GetPodCmd().GetNamespace())
		assert.Equal(t, "bar", cmdRecv.GetPodCmd().GetName())

		err = syncClient.Send(&connectivity.Msg{
			SessionId: cmdRecv.GetSessionId(),
			Completed: true,
			Msg: &connectivity.Msg_PodInfo{
				PodInfo: &connectivity.PodInfo{
					Pod: &connectivity.PodInfo_PodV1{
						PodV1: []byte("foo"),
					},
				},
			},
		})
		assert.NoError(t, err)

		for i := 0; i < orphanedMsgCount; i++ {
			err = syncClient.Send(&connectivity.Msg{
				Msg: &connectivity.Msg_NodeInfo{
					NodeInfo: &connectivity.NodeInfo{
						Node: &connectivity.NodeInfo_NodeV1{
							NodeV1: []byte("foo"),
						},
					},
				},
			})
			assert.NoError(t, err)
		}

		err = syncClient.CloseSend()
		assert.NoError(t, err)
	}()

	wg.Wait()
}
