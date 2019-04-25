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
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"

	"arhat.dev/aranya/pkg/connectivity"
)

func newTestGrpcSrvAndStub() (mgr *GRPCManager, stub connectivity.ConnectivityClient) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	mgr = NewGRPCManager(grpc.NewServer(), l)
	go func() {
		if err := mgr.Start(); err != nil {
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
	assert.NotEmpty(t, c.sessionManager)
	assert.Empty(t, c.syncSrv)
}

func TestGrpcSrv(t *testing.T) {
	const (
		GlobalMsgCount = 10
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

		assert.Equal(t, GlobalMsgCount, i)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-mgr.Connected()

		signalCorrect := true
		select {
		case <-mgr.Disconnected():
			signalCorrect = false
		default:
			signalCorrect = true
		}
		assert.True(t, signalCorrect)

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
		})
		assert.NoError(t, err)

		for i := 0; i < GlobalMsgCount; i++ {
			err = syncClient.Send(&connectivity.Msg{})
			assert.NoError(t, err)
		}

		err = syncClient.CloseSend()
		assert.NoError(t, err)
	}()

	wg.Wait()
}
