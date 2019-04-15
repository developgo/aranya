package runtime

import (
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type Interface interface {
	// Name the runtime name
	Name() string
	// Version the runtime version
	Version() string

	// CreatePod create a pod according to corev1.PodSpec
	// steps:
	//
	// 		- pull images
	// 		- create pod with `pause` container
	// 		- TODO: create and start init containers
	// 		- create and start containers
	CreatePod(options *connectivity.CreateOptions) (*connectivity.Pod, error)
	DeletePod(options *connectivity.DeleteOptions) (*connectivity.Pod, error)
	ListPod(options *connectivity.ListOptions) ([]*connectivity.Pod, error)

	ExecInContainer(
		podUID, container string,
		stdin io.Reader, stdout, stderr io.WriteCloser,
		resizeCh <-chan remotecommand.TerminalSize,
		command []string, tty bool,
	) error

	AttachContainer(
		podUID, container string,
		stdin io.Reader, stdout, stderr io.WriteCloser,
		resizeCh <-chan remotecommand.TerminalSize,
	) error

	GetContainerLogs(
		podUID string, options *corev1.PodLogOptions,
		stdout, stderr io.WriteCloser,
	) error

	PortForward(
		podUID string,
		ports []int32,
		in io.Reader, out io.WriteCloser,
	) error
}
