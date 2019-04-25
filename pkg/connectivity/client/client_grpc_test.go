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

package client

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/connectivity/runtime/fake"
	"arhat.dev/aranya/pkg/connectivity/servertivity/server"
)

var (
	expectedDataMsgList = func() []*connectivity.Msg {
		return []*connectivity.Msg{
			connectivity.NewDataMsg(0, false, connectivity.STDOUT, []byte("foo")),
			connectivity.NewDataMsg(0, false, connectivity.STDERR, []byte("foo")),
			connectivity.NewDataMsg(0, false, connectivity.STDOUT, []byte("bar")),
			connectivity.NewDataMsg(0, true, connectivity.OTHER, nil),
		}
	}
)

func newGRPCTestManagerAndAgent(rt runtime.Interface) (mgr *server.GRPCManager, client *GRPCAgent) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	mgr = server.NewGRPCManager(grpc.NewServer(), l)

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

	client, err = NewGRPCAgent(context.TODO(), &Config{}, conn, rt)
	if err != nil {
		panic(err)
	}

	return
}

func TestGRPCAgent(t *testing.T) {
	var (
		mgr    *server.GRPCManager
		client *GRPCAgent

		podReq = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: corev1.PodSpec{NodeName: "foo"},
		}
	)

	okRt, err := fake.NewFakeRuntime(false)
	assert.NoError(t, err)

	mgr, client = newGRPCTestManagerAndAgent(okRt)
	defer mgr.Stop()

	err = client.PostMsg(connectivity.NewNodeMsg(0, nil, nil, nil, nil))
	assert.Error(t, err)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err = client.Start(context.TODO())
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.Connected()

		for msg := range mgr.GlobalMessages() {
			msg.GetPodStatus()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.Connected()

		createCmd := connectivity.NewPodCreateCmd(podReq, nil, nil, nil, nil)
		testOnetimeCmdWithExpectedMsg(t, mgr,
			createCmd,
			*connectivity.NewPodStatusMsg(0, nil))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			connectivity.NewPodListCmd(podReq.Namespace, podReq.Name, false),
			*connectivity.NewPodStatusMsg(0, nil))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			connectivity.NewPodDeleteCmd(string(podReq.UID), time.Second),
			*connectivity.NewPodStatusMsg(0, nil))

		// TODO: stream test is buggy due to buffered io, need to redesign

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewPortForwardCmd(string(podReq.UID), 2048, "tcp"),
			expectedDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerExecCmd(string(podReq.UID), "", []string{}, true, true, true, true),
			expectedDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerExecCmd(string(podReq.UID), "", []string{}, true, true, true, true),
			expectedDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerLogCmd(string(podReq.UID), "", true, true, time.Time{}, -1),
			expectedDataMsgList())

		testOnetimeCmdWithNoExpectedMsg(t, mgr, connectivity.NewContainerInputCmd(0, []byte("foo")))
		testOnetimeCmdWithNoExpectedMsg(t, mgr, connectivity.NewContainerInputCmd(1, []byte("foo")))

		testOnetimeCmdWithNoExpectedMsg(t, mgr, connectivity.NewContainerTtyResizeCmd(0, 10, 10))
		testOnetimeCmdWithNoExpectedMsg(t, mgr, connectivity.NewContainerTtyResizeCmd(1, 10, 10))

		mgr.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

	}()

	wg.Wait()
}

func testOnetimeCmdWithNoExpectedMsg(t *testing.T, mgr server.Manager, cmd *connectivity.Cmd) {
	_, err := mgr.PostCmd(context.TODO(), cmd)
	assert.Equal(t, server.ErrSessionNotValid, err)
}

func testOnetimeCmdWithExpectedMsg(t *testing.T, mgr server.Manager, cmd *connectivity.Cmd, expectedMsg connectivity.Msg) {
	msgCh, err := mgr.PostCmd(context.TODO(), cmd)
	assert.NoError(t, err)
	assert.NotNil(t, msgCh)
	expectedMsg.SessionId = cmd.GetSessionId()

	msg, more := <-msgCh
	assert.True(t, more)
	assert.NotNil(t, msg)
	assertMsgEqual(t, expectedMsg, *msg)

	_, more = <-msgCh
	assert.False(t, more)
}

func testStreamCmdWithExpectedMsgList(t *testing.T, mgr server.Manager, cmd *connectivity.Cmd, expectedMsgList []*connectivity.Msg) {
	msgCh, err := mgr.PostCmd(context.TODO(), cmd)
	assert.NoError(t, err)
	assert.NotNil(t, msgCh)

	// copy as value
	var msgList []connectivity.Msg
	for _, m := range expectedMsgList {
		msg := *m
		msg.SessionId = cmd.GetSessionId()
		msgList = append(msgList, msg)
	}

	i := 0
	for msg := range msgCh {
		assert.NotNil(t, msg)

		assertMsgEqual(t, msgList[i], *msg)
		i++
	}
}

func assertMsgEqual(t *testing.T, expectedMsg, msg connectivity.Msg) {
	assert.Equal(t, expectedMsg.GetCompleted(), msg.GetCompleted())
	assert.Equal(t, expectedMsg.GetSessionId(), msg.GetSessionId())

	if expectedMsg.GetPodStatus() == nil {
		assert.Nil(t, msg.GetPodStatus())
	} else {
		assert.NotNil(t, msg.GetPodStatus())
		// assert.True(t, expectedMsg.GetPod().Equal(msg.GetPod()))
		assert.Equal(t, expectedMsg.GetPodStatus().GetUid(), msg.GetPodStatus().GetUid())
	}

	if expectedMsg.GetData() == nil {
		assert.Nil(t, msg.GetData())
	} else {
		assert.NotNil(t, msg.GetData())
		assert.Equal(t, expectedMsg.GetData().GetData(), msg.GetData().GetData())
	}

	if expectedMsg.GetNodeStatus() == nil {
		assert.Nil(t, msg.GetNodeStatus())
	} else {
		assert.NotNil(t, msg.GetNodeStatus())
	}

	if expectedMsg.GetError() == nil {
		assert.Nil(t, msg.GetError())
	} else {
		assert.NotNil(t, msg.GetError())
	}
}
