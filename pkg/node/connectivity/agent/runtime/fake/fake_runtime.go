package fake

import (
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/agent/runtime"
)

func NewFakeRuntime(faulty bool) (runtime.Interface, error) {
	return &fakeRuntime{faulty: faulty}, nil
}

type fakeRuntime struct {
	faulty bool
}

func (r *fakeRuntime) CreatePod(options *connectivity.CreateOptions) (*connectivity.Pod, error) {

	if r.faulty {
		return nil, fmt.Errorf("faulty: create pod")
	}

	return connectivity.NewPod(options.GetPodUid(), &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (r *fakeRuntime) DeletePod(options *connectivity.DeleteOptions) (*connectivity.Pod, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: delete pod")
	}

	return connectivity.NewPod(options.GetPodUid(), &criRuntime.PodSandboxStatus{
		Metadata: &criRuntime.PodSandboxMetadata{
			Namespace: "foo",
			Name:      "bar",
		},
	}, []*criRuntime.ContainerStatus{}), nil
}

func (r *fakeRuntime) ListPod(options *connectivity.ListOptions) ([]*connectivity.Pod, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: list pod")
	}

	return []*connectivity.Pod{
		connectivity.NewPod("", &criRuntime.PodSandboxStatus{
			Metadata: &criRuntime.PodSandboxMetadata{
				Namespace: "foo",
				Name:      "bar",
			},
		}, []*criRuntime.ContainerStatus{}),
	}, nil
}

func (r *fakeRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	defer closeAllIfNotNil(stdout, stderr)

	if r.faulty {
		return fmt.Errorf("faulty: exec in container")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))

	return nil
}

func (r *fakeRuntime) AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error {
	defer closeAllIfNotNil(stdout, stderr)

	if r.faulty {
		return fmt.Errorf("faulty: attach container")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))
	return nil
}

func (r *fakeRuntime) GetContainerLogs(podUID string, options *corev1.PodLogOptions, stdout, stderr io.WriteCloser) error {
	defer closeAllIfNotNil(stdout, stderr)

	if r.faulty {
		return fmt.Errorf("faulty: get container logs")
	}

	_, _ = stdout.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stderr.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = stdout.Write([]byte("bar"))
	return nil
}

func (r *fakeRuntime) PortForward(podUID string, ports []int32, in io.Reader, out io.WriteCloser) error {
	defer closeAllIfNotNil(out)

	if r.faulty {
		return fmt.Errorf("faulty: port forward")
	}

	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("foo"))
	time.Sleep(time.Second)
	_, _ = out.Write([]byte("bar"))
	return nil
}

func (r *fakeRuntime) Name() string {
	return "fake"
}

func (r *fakeRuntime) Version() string {
	return "0.0.0"
}

func closeAllIfNotNil(c ...io.Closer) {
	for _, v := range c {
		if v != nil {
			_ = v.Close()
		}
	}
}
