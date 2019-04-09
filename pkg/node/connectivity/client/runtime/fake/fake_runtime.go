package fake

import (
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
)

func NewFakeRuntime(faulty bool) (runtime.Interface, error) {
	return &fakeRuntime{faulty: faulty}, nil
}

type fakeRuntime struct {
	faulty bool
}

func (r *fakeRuntime) CreatePod(
	namespace, name string,
	containers map[string]*connectivity.ContainerSpec,
	authConfig map[string]*criRuntime.AuthConfig,
	volumeData map[string]*connectivity.NamedData,
	hostVolumes map[string]string,
) (*connectivity.Pod, error) {

	if r.faulty {
		return nil, fmt.Errorf("faulty: create pod")
	}

	return connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (r *fakeRuntime) DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: delete pod")
	}

	return connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (r *fakeRuntime) ListPod(namespace, name string) ([]*connectivity.Pod, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: list pod")
	}

	return []*connectivity.Pod{
		connectivity.NewPod(namespace, name, &criRuntime.PodSandboxStatus{
			Metadata: &criRuntime.PodSandboxMetadata{
				Namespace: "foo",
				Name:      "bar",
			},
		}, []*criRuntime.ContainerStatus{}),
	}, nil
}

func (r *fakeRuntime) ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	if r.faulty {
		return fmt.Errorf("faulty: exec in container")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (r *fakeRuntime) AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	if r.faulty {
		return fmt.Errorf("faulty: attach container")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (r *fakeRuntime) GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error {
	if r.faulty {
		return fmt.Errorf("faulty: get container logs")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	closeAllIfNotNil(stdout, stderr)
	return nil
}

func (r *fakeRuntime) PortForward(namespace, name string, ports []int32, in io.Reader, out io.WriteCloser) error {
	if r.faulty {
		return fmt.Errorf("faulty: port forward")
	}

	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("bar"))

	closeAllIfNotNil(out)
	return nil
}

func (r *fakeRuntime) Version() (name, ver string) {
	return "fake", "0.0.1"
}

func closeAllIfNotNil(c ...io.Closer) {
	for _, v := range c {
		if v != nil {
			_ = v.Close()
		}
	}
}
