package pod

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
)

// GetContainerLogs
// custom implementation
func (m *Manager) GetContainerLogs(namespace, pod, container string, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	msgCh, err := m.remoteManager.PostCmd(connectivity.NewContainerLogCmd(namespace, pod, options), 0)
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
	execCmd := connectivity.NewPodExecCmd("", name, container, cmd)
	msgCh, err := m.remoteManager.PostCmd(execCmd, timeout)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_, err = stdout.Write(msg.GetPodData().GetData())
		if err != nil {
			return err
		}
	}

	return nil
}

// AttachContainer
// implements k8s.io/kubernetes/pkg/kubelet/server/remotecommand.Attacher
func (m *Manager) AttachContainer(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	attachCmd := connectivity.NewPodAttachCmd("", name, container)
	msgCh, err := m.remoteManager.PostCmd(attachCmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_, err = stdout.Write(msg.GetPodData().GetData())
		if err != nil {
			return err
		}
	}
	return nil
}

// PortForward
// implements k8s.io/kubernetes/pkg/kubelet/server/portforward.PortForwarder
func (m *Manager) PortForward(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	portForwardCmd := connectivity.NewPodPortForwardCmd("", name, port)
	msgCh, err := m.remoteManager.PostCmd(portForwardCmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_, err = stream.Write(msg.GetPodData().GetData())
		if err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) CreateOrUpdatePodInDevice(pod *corev1.Pod) error {
	cmd := connectivity.NewPodCreateOrUpdateCmd(pod)
	msgCh, err := m.remoteManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		createdPod := msg.GetPodInfo()

		createdPod.GetPhase()
		createdPod.GetSpec()
		createdPod.GetStatus()
	}

	return nil
}

func (m *Manager) DeletePodInDevice(namespace, name string) error {
	cmd := connectivity.NewPodDeleteCmd(namespace, name, 0)
	msgCh, err := m.remoteManager.PostCmd(cmd, 0)
	if err != nil {
		return err
	}

	for msg := range msgCh {
		_ = msg.GetPodInfo()
	}
	return nil
}