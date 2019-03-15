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

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/server"
)

var (
	expectedDataMsgList = func() []*connectivity.Msg {
		return []*connectivity.Msg{
			NewDataMsg(0, false, connectivity.Data_STDOUT, []byte("foo")),
			NewDataMsg(0, false, connectivity.Data_STDERR, []byte("foo")),
			NewDataMsg(0, true, connectivity.Data_STDOUT, []byte("bar")),
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

		podReq = corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: corev1.PodSpec{NodeName: "foo"},
		}
	)

	sendPodDataMsgAll := func(sid uint64) {
		for _, m := range expectedDataMsgList() {
			msg := *m
			msg.SessionId = sid
			err := client.PostMsg(&msg)
			assert.NoError(t, err)
		}
	}

	opts := []Option{
		WithPodCreateHandler(func(sid uint64, namespace, name string, options *connectivity.CreateOptions) (*connectivity.Pod, error) {
			pod := &corev1.Pod{}
			err := pod.Unmarshal(options.GetPodV1().GetPod())
			assert.NoError(t, err)

			return NewPod(*pod, "", nil, nil), nil
		}),
		WithPodDeleteHandler(func(sid uint64, namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
			return &connectivity.Pod{
				Namespace: namespace,
				Name:      name,
			}, nil
		}),
		WithPodListHandler(func(sid uint64, namespace, name string, options *connectivity.ListOptions) ([]*connectivity.Pod, error) {
			return []*connectivity.Pod{{
				Namespace: namespace,
				Name:      name,
			}}, nil
		}),
		WithPortForwardHandler(func(sid uint64, namespace, name string, options *connectivity.PortForwardOptions) error {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			sendPodDataMsgAll(sid)
			return nil
		}),

		// stream cmd
		WithContainerAttachHandler(func(sid uint64, namespace, name string, options *connectivity.ExecOptions) error {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			sendPodDataMsgAll(sid)
			return nil
		}),
		// stream cmd
		WithContainerExecHandler(func(sid uint64, namespace, name string, options *connectivity.ExecOptions) error {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			sendPodDataMsgAll(sid)
			return nil
		}),
		// stream/onetime cmd
		WithContainerLogHandler(func(sid uint64, namespace, name string, options *connectivity.LogOptions) error {
			assert.Equal(t, "foo", namespace)
			assert.Equal(t, "bar", name)

			sendPodDataMsgAll(sid)
			return nil
		}),
		// onetime cmd (no reply, best effort)
		WithContainerInputHandler(func(sid uint64, options *connectivity.InputOptions) error {
			assert.Equal(t, "foo", string(options.GetData()))

			return nil
		}),
		// onetime cmd (on reply, best effort)
		WithContainerTtyResizeHandler(func(sid uint64, options *connectivity.TtyResizeOptions) error {
			assert.Equal(t, 10, options.GetCols())
			assert.Equal(t, 10, options.GetRows())
			return nil
		}),
	}

	mgr, srvStop, client = newGrpcTestServerAndClient(opts)
	defer srvStop()

	err := client.PostMsg(NewNodeMsg(0, true, corev1.Node{Spec: corev1.NodeSpec{Unschedulable: true}}))
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
			msg.GetNode()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.WaitUntilDeviceConnected()

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodCreateCmd(podReq, nil),
			*NewPodMsg(0, true, NewPod(podReq, "", nil, nil)))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodListCmd(podReq.Namespace, podReq.Name),
			*NewPodMsg(0, true, NewPod(podReq, "", nil, nil)))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			server.NewPodDeleteCmd(podReq.Namespace, podReq.Name, time.Second),
			*NewPodMsg(0, true, NewPod(podReq, "", nil, nil)))

		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewPortForwardCmd(podReq.Namespace, podReq.Name,
				corev1.PodPortForwardOptions{Ports: []int32{2048}}),
			expectedDataMsgList())

		execOptions := corev1.PodExecOptions{Stdin: true, Stdout: true, Stderr: true, TTY: true, Container: "", Command: []string{}}
		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerExecCmd(podReq.Namespace, podReq.Name, execOptions),
			expectedDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerAttachCmd(podReq.Namespace, podReq.Name, execOptions),
			expectedDataMsgList())

		logOptions := corev1.PodLogOptions{Container: "", Follow: true, Previous: true, Timestamps: true}
		testStreamCmdWithExpectedMsgList(t, mgr,
			server.NewContainerLogCmd(podReq.Namespace, podReq.Name, logOptions),
			expectedDataMsgList())

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

	if expectedMsg.GetPod() == nil {
		assert.Nil(t, msg.GetPod())
	} else {
		assert.NotNil(t, msg.GetPod())
		assert.Equal(t, expectedMsg.GetPod().GetNamespace(), msg.GetPod().GetNamespace())
		assert.Equal(t, expectedMsg.GetPod().GetName(), msg.GetPod().GetName())
		assert.Equal(t, expectedMsg.GetPod().GetUid(), msg.GetPod().GetUid())
		assert.Equal(t, expectedMsg.GetPod().GetIp(), msg.GetPod().GetIp())
		// assert.EqualValues(t, expectedMsg.GetPod().GetContainerStatusV1Alpha2().GetV1Alpha2(), msg.GetPod().GetContainerStatusV1Alpha2().GetV1Alpha2())
		// assert.EqualValues(t, expectedMsg.GetPod().GetSandboxStatusV1Alpha2().GetV1Alpha2(), msg.GetPod().GetSandboxStatusV1Alpha2().GetV1Alpha2())
	}

	if expectedMsg.GetData() == nil {
		assert.Nil(t, msg.GetData())
	} else {
		assert.NotNil(t, msg.GetData())
		assert.Equal(t, expectedMsg.GetData().GetData(), msg.GetData().GetData())
	}

	if expectedMsg.GetNode() == nil {
		assert.Nil(t, msg.GetNode())
	} else {
		assert.NotNil(t, msg.GetNode())
		assert.Equal(t, expectedMsg.GetNode().GetNodeV1(), msg.GetNode().GetNodeV1())
	}

	if expectedMsg.GetAck() == nil {
		assert.Nil(t, msg.GetAck())
	} else {
		assert.NotNil(t, msg.GetAck())
		assert.Equal(t, expectedMsg.GetAck().GetHash().GetSha256(), msg.GetAck().GetHash().GetSha256())
		assert.Equal(t, expectedMsg.GetAck().GetError(), msg.GetAck().GetError())
	}
}
