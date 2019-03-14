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

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/server"
)

var (
	expectedPodDataMsgList = func() []*connectivity.Msg {
		return []*connectivity.Msg{
			NewPodDataMsg(0, false, connectivity.Data_STDOUT, []byte("foo")),
			NewPodDataMsg(0, false, connectivity.Data_STDERR, []byte("foo")),
			NewPodDataMsg(0, true, connectivity.Data_STDOUT, []byte("bar")),
		}
	}
)

func newGrpcTestServerAndClient(opts []Option) (mgr *server.GrpcManager, srvStop func(), client *GrpcClient) {
	mgr = server.NewGrpcManager("client.test").(*server.GrpcManager)
	srv := grpc.NewServer()
	connectivity.RegisterConnectivityServer(srv, mgr)

	l, err := net.Listen("tcp", "localhost:0")
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
	client, err = NewGrpcClient(conn, opts...)
	if err != nil {
		panic(err)
	}

	return
}

func TestNewGrpcClient(t *testing.T) {
	var (
		mgr     *server.GrpcManager
		srvStop func()
		client  *GrpcClient

		podSpecReq = corev1.PodSpec{NodeName: "foo"}
		podReq     = corev1.Pod{Spec: podSpecReq}
	)

	sendPodDataMsgAll := func(sid uint64) {
		for _, m := range expectedPodDataMsgList() {
			msg := *m
			msg.SessionId = sid
			err := client.PostMsg(&msg)
			assert.NoError(t, err)
		}
	}

	opts := []Option{
		WithPodCreateOrUpdateHandler(func(sid uint64, namespace, name string, options *connectivity.CreateOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			podSpec := &corev1.PodSpec{}
			err := podSpec.Unmarshal(options.GetPodSpecV1())
			assert.NoError(t, err)

			err = client.PostMsg(NewPodInfoMsg(sid, true, corev1.Pod{Spec: *podSpec}))
			assert.NoError(t, err)
		}),
		WithPodDeleteHandler(func(sid uint64, namespace, name string, options *connectivity.DeleteOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)
			err := client.PostMsg(NewPodInfoMsg(sid, true, podReq))
			assert.NoError(t, err)
		}),
		WithPodListHandler(func(sid uint64, namespace, name string, options *connectivity.ListOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			err := client.PostMsg(NewPodInfoMsg(sid, true, podReq))
			assert.NoError(t, err)
		}),
		WithPortForwardHandler(func(sid uint64, namespace, name string, options *connectivity.PortForwardOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)
			sendPodDataMsgAll(sid)
		}),

		// stream cmd
		WithContainerAttachHandler(func(sid uint64, namespace, name string, options *connectivity.ExecOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)
			sendPodDataMsgAll(sid)
		}),
		// stream cmd
		WithContainerExecHandler(func(sid uint64, namespace, name string, options *connectivity.ExecOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)
			sendPodDataMsgAll(sid)
		}),
		// stream/onetime cmd
		WithContainerLogHandler(func(sid uint64, namespace, name string, options *connectivity.LogOptions) {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)
			sendPodDataMsgAll(sid)
		}),
		// onetime cmd (no reply, best effort)
		WithContainerInputHandler(func(sid uint64, options *connectivity.InputOptions) {
			assert.Equal(t, "foo", string(options.GetData()))
		}),
		// onetime cmd (on reply, best effort)
		WithContainerTtyResizeHandler(func(sid uint64, options *connectivity.TtyResizeOptions) {
			assert.Equal(t, 10, options.GetCols())
			assert.Equal(t, 10, options.GetRows())
		}),
	}

	mgr, srvStop, client = newGrpcTestServerAndClient(opts)
	defer srvStop()

	err := client.PostMsg(NewNodeInfoMsg(0, true, corev1.Node{Spec: corev1.NodeSpec{Unschedulable: true}}))
	assert.Error(t, err)

	wg := &sync.WaitGroup{}

	wg.Add(1)
	go func() {
		defer wg.Done()

		err = client.Run(context.TODO())
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.WaitUntilDeviceConnected()

		for msg := range mgr.ConsumeGlobalMsg() {
			msg.GetNodeInfo()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.WaitUntilDeviceConnected()

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodCreateOrUpdateCmd("foo", "bar", podReq, nil),
			*NewPodInfoMsg(0, true, podReq))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodListCmd("foo", "bar"),
			*NewPodInfoMsg(0, true, podReq))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodDeleteCmd("foo", "bar", time.Second),
			*NewPodInfoMsg(0, true, podReq))

		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewPortForwardCmd("foo", "bar",
				corev1.PodPortForwardOptions{Ports: []int32{2048}}),
			expectedPodDataMsgList())

		execOptions := corev1.PodExecOptions{Stdin: true, Stdout: true, Stderr: true, TTY: true, Container: "", Command: []string{}}
		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerExecCmd("foo", "bar", execOptions),
			expectedPodDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerAttachCmd("foo", "bar", execOptions),
			expectedPodDataMsgList())

		logOptions := corev1.PodLogOptions{Container: "", Follow: true, Previous: true, Timestamps: true}
		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerLogCmd("foo", "bar", logOptions),
			expectedPodDataMsgList())

		testOnetimeCmdWithNoExpectedMsg(t, mgr, server.NewContainerInputCmd(0, []byte("foo")))
		testOnetimeCmdWithNoExpectedMsg(t, mgr, server.NewContainerInputCmd(1, []byte("foo")))

		testOnetimeCmdWithNoExpectedMsg(t, mgr, server.NewContainerTtyResizeCmd(0, 10, 10))
		testOnetimeCmdWithNoExpectedMsg(t, mgr, server.NewContainerTtyResizeCmd(1, 10, 10))

		srvStop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

	}()

	wg.Wait()
}

func testOnetimeCmdWithNoExpectedMsg(t *testing.T, mgr server.Interface, cmd *connectivity.Cmd) {
	_, err := mgr.PostCmd(cmd, 0)
	assert.Equal(t, server.ErrSessionNotValid, err)
}

func testOnetimeCmdWithExpectedMsg(t *testing.T, mgr server.Interface, cmd *connectivity.Cmd, expectedMsg connectivity.Msg) {
	msgCh, err := mgr.PostCmd(cmd, 0)
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

func testStreamCmdWithExpectedMsgList(t *testing.T, mgr server.Interface, cmd *connectivity.Cmd, expectedMsgList []*connectivity.Msg) {
	msgCh, err := mgr.PostCmd(cmd, 0)
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

	if expectedMsg.GetPodInfo() == nil {
		assert.Nil(t, msg.GetPodInfo())
	} else {
		assert.Equal(t, *expectedMsg.GetPodInfo(), *msg.GetPodInfo())
	}

	if expectedMsg.GetPodData() == nil {
		assert.Nil(t, msg.GetPodData())
	} else {
		assert.Equal(t, *expectedMsg.GetPodData(), *msg.GetPodData())
	}

	if expectedMsg.GetNodeInfo() == nil {
		assert.Nil(t, msg.GetNodeInfo())
	} else {
		assert.Equal(t, *expectedMsg.GetNodeInfo(), *msg.GetNodeInfo())
	}
}
