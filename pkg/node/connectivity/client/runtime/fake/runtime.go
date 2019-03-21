package fake

import (
	"io"

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

func (*fakeRuntime) CreatePod(namespace, name, uid string, pod *corev1.PodSpec, authConfig map[string]*criRuntime.AuthConfig, volumeData map[string][]byte) (*connectivity.Pod, error) {
	return connectivity.NewPod(namespace, name, "", &criRuntime.PodSandboxStatus{}, []*criRuntime.ContainerStatus{}), nil
}

func (*fakeRuntime) DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	return connectivity.NewPod(namespace, name, "", &criRuntime.PodSandboxStatus{}, []*criRuntime.ContainerStatus{}), nil
}

func (*fakeRuntime) ListPod(namespace string) ([]*connectivity.Pod, error) {
	return nil, nil
}

func (*fakeRuntime) ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	if stdout != nil {
		_ = stdout.Close()
	}

	if stderr != nil {
		_ = stderr.Close()
	}

	return nil
}

func (*fakeRuntime) AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	if stdout != nil {
		_ = stdout.Close()
	}

	if stderr != nil {
		_ = stderr.Close()
	}

	return nil
}

func (*fakeRuntime) GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error {
	defer func() { _, _ = stdout.Close(), stderr.Close() }()

	return nil
}
