package pod

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"

	connectivitySrv "arhat.dev/aranya/pkg/node/connectivity/server"
)

// GetContainerLogs
// custom implementation
func (m *Manager) GetContainerLogs(namespace, pod, container string, options corev1.PodLogOptions) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	msgCh, err := m.remoteManager.PostCmd(connectivitySrv.NewContainerLogCmd(namespace, pod, options), 0)
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() { _ = writer.Close() }()

		for msg := range msgCh {
			_, err := writer.Write(msg.GetPodData().GetData())
			if err != nil {
				return
			}
		}
	}()

	return reader, nil
}

// ExecInContainer
// implements k8s.io/kubernetes/pkg/kubelet/server/remotecommand.Executor
func (m *Manager) ExecInContainer(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	options := corev1.PodExecOptions{
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
		TTY:       tty,
		Container: container,
		Command:   cmd,
	}

	execCmd := connectivitySrv.NewContainerExecCmd("", name, options)
	return m.handleBidirectionalStream(execCmd, timeout, stdin, stdout, stderr, resize)
}

// AttachContainer
// implements k8s.io/kubernetes/pkg/kubelet/server/remotecommand.Attacher
func (m *Manager) AttachContainer(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	options := corev1.PodExecOptions{
		Stdin:     stdin != nil,
		Stdout:    stdout != nil,
		Stderr:    stderr != nil,
		TTY:       tty,
		Container: container,
	}

	attachCmd := connectivitySrv.NewContainerAttachCmd("", name, options)
	return m.handleBidirectionalStream(attachCmd, 0, stdin, stdout, stderr, resize)
}

// PortForward
// implements k8s.io/kubernetes/pkg/kubelet/server/portforward.PortForwarder
func (m *Manager) PortForward(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	options := corev1.PodPortForwardOptions{
		Ports: []int32{port},
	}
	portForwardCmd := connectivitySrv.NewPortForwardCmd("", name, options)
	return m.handleBidirectionalStream(portForwardCmd, 0, stream, stream, nil, nil)
}

func (m *Manager) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	cmd := connectivitySrv.NewPodCreateOrUpdateCmd(pod.Namespace, pod.Name, *pod, nil)
	msgCh, err := m.remoteManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		createdPod := msg.GetPodInfo()
		_ = createdPod
	}

	return nil
}

func (m *Manager) DeletePodInDevice(namespace, name string) error {
	cmd := connectivitySrv.NewPodDeleteCmd(namespace, name, 0)
	msgCh, err := m.remoteManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_ = msg.GetPodInfo()
	}
	return nil
}
