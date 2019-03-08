package server

import (
	"context"
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func newTestGrpcSrvAndStub() (srvStop func(), stub connectivity.ConnectivityClient) {
	srv := grpc.NewServer()
	connectivity.RegisterConnectivityServer(srv, NewGrpcConnectivity("test"))

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

	conn, err := grpc.DialContext(context.TODO(), l.Addr().String())
	if err != nil {
		panic(err)
	}
	stub = connectivity.NewConnectivityClient(conn)

	return
}

func TestNewGrpcConnectivity(t *testing.T) {
	c := NewGrpcConnectivity("test")
	assert.NotEmpty(t, c)
	assert.NotEmpty(t, c.sessions)
	assert.Empty(t, c.syncSrv)
}

func TestGrpcConnectivity_Sync(t *testing.T) {
	srvStop, stub := newTestGrpcSrvAndStub()
	defer srvStop()

	syncClient, err := stub.Sync(context.TODO())
	assert.NoError(t, err, "start sync client failed")
}
