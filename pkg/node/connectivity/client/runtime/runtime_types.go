package runtime

import (
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type Interface interface {
	// CreatePod create a pod according to corev1.PodSpec
	// steps:
	//
	// 		- pull images
	// 		- create pod with `pause` container
	// 		- TODO: create and start init containers
	// 		- create and start containers
	CreatePod(namespace, name string, pod *corev1.PodSpec, authConfig map[string]*criRuntime.AuthConfig, volumeData map[string][]byte) (*connectivity.Pod, error)
	DeletePod(namespace, name string, options *connectivity.DeleteOptions) (*connectivity.Pod, error)
	ListPod(namespace, name string) ([]*connectivity.Pod, error)

	ExecInContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize, command []string, tty bool) error
	AttachContainer(namespace, name, container string, stdin io.Reader, stdout, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) error
	GetContainerLogs(namespace, name string, stdout, stderr io.WriteCloser, options *corev1.PodLogOptions) error
	PortForward(namespace, name string, ports []int32, in io.Reader, out io.WriteCloser) error
}
