package agent

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
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/agent/runtime/fake"
	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/manager"
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

func newGRPCTestManagerAndAgent(rt runtime.Interface) (mgr *manager.GRPCManager, client *GRPCAgent) {
	l, err := net.Listen("tcp", "localhost:0")
	if err != nil {
		panic(err)
	}

	mgr = manager.NewGRPCManager(grpc.NewServer(), l)

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

	client, err = NewGRPCAgent(conn, rt)
	if err != nil {
		panic(err)
	}

	return
}

func TestGRPCAgent(t *testing.T) {
	var (
		mgr    *manager.GRPCManager
		client *GRPCAgent

		podReq = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "foo",
				Name:      "bar",
			},
			Spec: corev1.PodSpec{NodeName: "foo"},
		}
		podStatus = &criRuntime.PodSandboxStatus{
			Metadata: &criRuntime.PodSandboxMetadata{
				Namespace: "foo",
				Name:      "bar",
			},
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

		err = client.Run(context.TODO())
		if err != nil {
			panic(err)
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.Connected()

		for msg := range mgr.GlobalMessages() {
			msg.GetNode()
		}
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-mgr.Connected()

		createCmd := connectivity.NewPodCreateCmd(podReq, nil, nil, nil, nil)
		testOnetimeCmdWithExpectedMsg(t, mgr,
			createCmd,
			*connectivity.NewPodMsg(0, true, connectivity.NewPod(string(podReq.UID), podStatus, nil)))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			connectivity.NewPodListCmd(podReq.Namespace, podReq.Name, false),
			*connectivity.NewPodMsg(0, true, connectivity.NewPod(string(podReq.UID), podStatus, nil)))

		testOnetimeCmdWithExpectedMsg(t, mgr,
			connectivity.NewPodDeleteCmd(string(podReq.UID), time.Second),
			*connectivity.NewPodMsg(0, true, connectivity.NewPod(string(podReq.UID), podStatus, nil)))

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewPortForwardCmd(string(podReq.UID), corev1.PodPortForwardOptions{Ports: []int32{2048}}),
			expectedDataMsgList())

		execOptions := corev1.PodExecOptions{Stdin: true, Stdout: true, Stderr: true, TTY: true, Container: "", Command: []string{}}
		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerExecCmd(string(podReq.UID), execOptions),
			expectedDataMsgList())

		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerAttachCmd(string(podReq.UID), execOptions),
			expectedDataMsgList())

		logOptions := corev1.PodLogOptions{Container: "", Follow: true, Previous: true, Timestamps: true}
		testStreamCmdWithExpectedMsgList(t, mgr,
			connectivity.NewContainerLogCmd(string(podReq.UID), logOptions),
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

func testOnetimeCmdWithNoExpectedMsg(t *testing.T, mgr manager.Manager, cmd *connectivity.Cmd) {
	_, err := mgr.PostCmd(context.TODO(), cmd)
	assert.Equal(t, manager.ErrSessionNotValid, err)
}

func testOnetimeCmdWithExpectedMsg(t *testing.T, mgr manager.Manager, cmd *connectivity.Cmd, expectedMsg connectivity.Msg) {
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

func testStreamCmdWithExpectedMsgList(t *testing.T, mgr manager.Manager, cmd *connectivity.Cmd, expectedMsgList []*connectivity.Msg) {
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

	if expectedMsg.GetPod() == nil {
		assert.Nil(t, msg.GetPod())
	} else {
		assert.NotNil(t, msg.GetPod())
		// assert.True(t, expectedMsg.GetPod().Equal(msg.GetPod()))
		assert.Equal(t, expectedMsg.GetPod().GetUid(), msg.GetPod().GetUid())
		assert.Equal(t, expectedMsg.GetPod().GetIp(), msg.GetPod().GetIp())
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
	}

	if expectedMsg.GetAck() == nil {
		assert.Nil(t, msg.GetAck())
	} else {
		assert.NotNil(t, msg.GetAck())
		assert.Equal(t, expectedMsg.GetAck().GetHash().GetSha256(), msg.GetAck().GetHash().GetSha256())
		assert.Equal(t, expectedMsg.GetAck().GetError(), msg.GetAck().GetError())
	}
}
