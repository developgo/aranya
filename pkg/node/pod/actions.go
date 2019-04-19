package pod

import (
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	kubeletpf "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
)

type containerExecutor func(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error

func (doExec containerExecutor) ExecInContainer(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
	return doExec(name, uid, container, cmd, stdin, stdout, stderr, tty, resize, timeout)
}

type containerAttacher func(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error

func (doAttach containerAttacher) AttachContainer(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
	return doAttach(name, uid, container, stdin, stdout, stderr, tty, resize)
}

type portForwarder func(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error

func (doPortForward portForwarder) PortForward(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
	return doPortForward(name, uid, port, stream)
}

func (m *Manager) getContainerLogs(uid types.UID, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	msgCh, err := m.remoteManager.PostCmd(m.ctx, connectivity.NewContainerLogCmd(string(uid), *options))
	if err != nil {
		return nil, err
	}

	go func() {
		defer func() { _ = writer.Close() }()

		for msg := range msgCh {
			_, err := writer.Write(msg.GetData().GetData())
			if err != nil {
				return
			}
		}
	}()

	return reader, nil
}

func (m *Manager) handleExecInContainer(errCh chan<- error) kubeletrc.Executor {
	return containerExecutor(func(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
		defer close(errCh)

		options := corev1.PodExecOptions{
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    stderr != nil,
			TTY:       tty,
			Container: container,
			Command:   cmd,
		}

		execCmd := connectivity.NewContainerExecCmd(string(uid), options)
		err := m.handleBidirectionalStream(execCmd, stdin, stdout, stderr, resize)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}

func (m *Manager) handleAttachContainer(errCh chan<- error) kubeletrc.Attacher {
	return containerAttacher(func(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
		defer close(errCh)

		options := corev1.PodExecOptions{
			Stdin:     stdin != nil,
			Stdout:    stdout != nil,
			Stderr:    stderr != nil,
			TTY:       tty,
			Container: container,
		}

		attachCmd := connectivity.NewContainerAttachCmd(string(uid), options)
		err := m.handleBidirectionalStream(attachCmd, stdin, stdout, stderr, resize)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}

func (m *Manager) handlePortForward(errCh chan<- error) kubeletpf.PortForwarder {
	return portForwarder(func(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
		defer close(errCh)

		options := corev1.PodPortForwardOptions{
			Ports: []int32{port},
		}

		portForwardCmd := connectivity.NewPortForwardCmd(string(uid), options)
		err := m.handleBidirectionalStream(portForwardCmd, stream, stream, nil, nil)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}
