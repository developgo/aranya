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

func newGrpcTestServerAndClient(opts []Option) (mgr *server.GrpcManager, srvStop func(), client *GrpcClient) {
	mgr = server.NewGrpcManager("client.test").(*server.GrpcManager)
	srv := grpc.NewServer()
	connectivity.RegisterConnectivityServer(srv, mgr)

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

	opts := []Option{
		WithPodCreateOrUpdateHandler(func(sid uint64, namespace, name string, options *connectivity.PodCreateOptions) {
			podSpec := &corev1.PodSpec{}
			err := podSpec.Unmarshal(options.GetPodSpecV1())
			assert.NoError(t, err)

			err = client.PostMsg(NewPodInfoMsg(sid, true, corev1.Pod{Spec: *podSpec}))
			assert.NoError(t, err)
		}),
		WithPodDeleteHandler(func(sid uint64, namespace, name string, options *connectivity.PodDeleteOptions) {
			err := client.PostMsg(NewPodInfoMsg(sid, true, podReq))
			assert.NoError(t, err)
		}),
		WithPodListHandler(func(sid uint64, namespace, name string, options *connectivity.PodListOptions) {
			err := client.PostMsg(NewPodInfoMsg(sid, true, podReq))
			assert.NoError(t, err)
		}),
		WithPodPortForwardHandler(func(sid uint64, namespace, name string, options *connectivity.PodPortForwardOptions) {

		}),

		WithContainerAttachHandler(func(sid uint64, namespace, name string, options *connectivity.PodExecOptions) {

		}),
		WithContainerExecHandler(func(sid uint64, namespace, name string, options *connectivity.PodExecOptions) {

		}),
		WithContainerInputHandler(func(sid uint64, namespace, name string, options *connectivity.PodDataOptions) {

		}),
		WithContainerLogHandler(func(sid uint64, namespace, name string, options *connectivity.PodLogOptions) {

		}),
		WithContainerTtyResizeHandler(func(sid uint64, namespace, name string, options *connectivity.TtyResizeOptions) {

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

	go func() {
		defer wg.Done()
		<-mgr.WaitUntilDeviceConnected()

		testOnetimeCmd(t, mgr,
			server.NewPodCreateOrUpdateCmd("", "", podSpecReq),
			*NewPodInfoMsg(0, true, podReq))

		testOnetimeCmd(t, mgr,
			server.NewPodListCmd("", ""),
			*NewPodInfoMsg(0, true, podReq))

		testOnetimeCmd(t, mgr,
			server.NewPodDeleteCmd("", "", time.Second),
			*NewPodInfoMsg(0, true, podReq))

		srvStop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()

	}()

	wg.Wait()
}

func testOnetimeCmd(t *testing.T, mgr server.Interface, cmd *connectivity.Cmd, expectedMsg connectivity.Msg) {
	msgCh, err := mgr.PostCmd(cmd, 0)
	assert.NoError(t, err)
	assert.NotEqual(t, nil, msgCh)
	expectedMsg.SessionId = cmd.GetSessionId()

	msg, more := <-msgCh
	assert.True(t, more)
	assert.Equal(t, *msg, expectedMsg)

	_, more = <-msgCh
	assert.False(t, more)
}
