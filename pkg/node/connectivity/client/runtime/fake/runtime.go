package fake

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

var _ runtime.Interface = &fakeRuntime{}

func NewFakeRuntime() (runtime.Interface, error) {
	return &fakeRuntime{}, nil
}

type fakeRuntime struct {
}

func (*fakeRuntime) CreatePod(namespace, name string, pod *corev1.PodSpec, authConfig map[string]*criRuntime.AuthConfig, volumeData map[string][]byte) (*connectivity.Pod, error) {
	return connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (*fakeRuntime) DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	return connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (*fakeRuntime) ListPod(namespace, name string) ([]*connectivity.Pod, error) {
	return []*connectivity.Pod{
		connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
			Metadata: &criRuntime.PodSandboxMetadata{
				Namespace: "foo",
				Name:      "bar",
			},
		}, []*criRuntime.ContainerStatus{}),
	}, nil
}

func (*fakeRuntime) ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (*fakeRuntime) AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (*fakeRuntime) GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error {
	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (*fakeRuntime) PodPortForward(namespace, name string, ports []int32, in io.Reader, out io.WriteCloser) error {
	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("bar"))

	closeAllIfNotNil(out)
	return nil
}

func closeAllIfNotNil(c ...io.Closer) {
	for _, v := range c {
		if v != nil {
			_ = v.Close()
		}
	}
}
