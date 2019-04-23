package runtime

import (
	"io"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

type Interface interface {
	// Name the runtime name
	Name() string
	// Version the runtime version
	Version() string
	// OS the kernel name of the container runtime
	OS() string
	// Arch the cpu arch of the container runtime
	Arch() string
	// KernelVersion of the container runtime
	KernelVersion() string
	// CreatePod create a pod according to corev1.PodSpec
	// steps:
	//
	// 		- pull images
	// 		- create pod using `pause` container
	// 		- TODO: create and start init containers
	// 		- create and start containers
	CreatePod(options *connectivity.CreateOptions) (*connectivity.PodStatus, *connectivity.Error)
	DeletePod(options *connectivity.DeleteOptions) (*connectivity.PodStatus, *connectivity.Error)
	ListPods(options *connectivity.ListOptions) ([]*connectivity.PodStatus, *connectivity.Error)
	ExecInContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) *connectivity.Error
	AttachContainer(podUID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) *connectivity.Error
	GetContainerLogs(podUID string, options *connectivity.LogOptions, stdout, stderr io.WriteCloser) *connectivity.Error
	PortForward(podUID string, protocol string, port int32, in io.Reader, out io.WriteCloser) *connectivity.Error
}
