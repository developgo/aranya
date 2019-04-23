package fake

import (
	"fmt"
	"io"
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/agent/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func NewFakeRuntime(faulty bool) (runtime.Interface, error) {
	return &fakeRuntime{faulty: faulty}, nil
}

type fakeRuntime struct {
	faulty bool
}

func (r *fakeRuntime) CreatePod(options *connectivity.CreateOptions) (*connectivity.PodStatus, error) {

	if r.faulty {
		return nil, fmt.Errorf("faulty: create pod")
	}

	return connectivity.NewPodStatus(options.GetPodUid(), nil), nil
}

func (r *fakeRuntime) DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: delete pod")
	}

	return connectivity.NewPodStatus(options.GetPodUid(), nil), nil
}

func (r *fakeRuntime) ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, error) {
	if r.faulty {
		return nil, fmt.Errorf("faulty: list pod")
	}

	return []*connectivity.PodStatus{connectivity.NewPodStatus("", nil)}, nil
}

func (r *fakeRuntime) ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error {
	defer closeIfNotNil(stdout, stderr)

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
	defer closeIfNotNil(stdout, stderr)

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

func (r *fakeRuntime) GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) error {
	defer closeIfNotNil(stdout, stderr)

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

func (r *fakeRuntime) PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) error {
	defer closeIfNotNil(out)

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

func (r *fakeRuntime) OS() string {
	return "fake"
}

func (r *fakeRuntime) Arch() string {
	return "fake"
}

func (r *fakeRuntime) KernelVersion() string {
	return "0.0.0-fake"
}

func closeIfNotNil(c ...io.Closer) {
	for _, v := range c {
		if v != nil {
			_ = v.Close()
		}
	}
}
