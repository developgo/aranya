package pod

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
)

// GetContainerLogs
// custom implementation
func (m *Manager) GetContainerLogs(namespace, pod, container string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	reader, writer := io.Pipe()
	defer func() { _ = writer.Close() }()

	return reader, nil
}

// ExecInContainer
// implements k8s.io/kubernetes/pkg/kubelet/server/remotecommand.Executor
func (m *Manager) ExecInContainer(name string, uid types.UID, container string, cmd []string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	return nil
}

// AttachContainer
// implements k8s.io/kubernetes/pkg/kubelet/server/remotecommand.Attacher
func (m *Manager) AttachContainer(name string, uid types.UID, container string, in io.Reader, out, err io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	return nil
}

// PortForward
// implements k8s.io/kubernetes/pkg/kubelet/server/portforward.PortForwarder
func (m *Manager) PortForward(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	return nil
}
