package pod

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/remotecommand"
	kubeletpf "k8s.io/kubernetes/pkg/kubelet/server/portforward"
	kubeletrc "k8s.io/kubernetes/pkg/kubelet/server/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/util"
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

func (m *Manager) doGetContainerLogs(uid types.UID, options *corev1.PodLogOptions) (io.ReadCloser, error) {
	reader, writer := io.Pipe()

	msgCh, err := m.manager.PostCmd(m.ctx, connectivity.NewContainerLogCmd(string(uid), *options))
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

func (m *Manager) doHandleExecInContainer(errCh chan<- error) kubeletrc.Executor {
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
		err := m.doServeStream(execCmd, stdin, stdout, stderr, resize)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}

func (m *Manager) doHandleAttachContainer(errCh chan<- error) kubeletrc.Attacher {
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
		err := m.doServeStream(attachCmd, stdin, stdout, stderr, resize)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}

func (m *Manager) doHandlePortForward(errCh chan<- error) kubeletpf.PortForwarder {
	return portForwarder(func(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
		defer close(errCh)

		options := corev1.PodPortForwardOptions{
			Ports: []int32{port},
		}

		portForwardCmd := connectivity.NewPortForwardCmd(string(uid), options)
		err := m.doServeStream(portForwardCmd, stream, stream, nil, nil)
		if err != nil {
			errCh <- err
			return err
		}

		return nil
	})
}

func (m *Manager) doServeStream(initialCmd *connectivity.Cmd, in io.Reader, out, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) (err error) {
	if out == nil {
		return fmt.Errorf("output should not be nil")
	}
	defer log.Info("finished stream handle")

	ctx, cancel := context.WithCancel(m.ctx)
	defer cancel()

	msgCh, err := m.manager.PostCmd(ctx, initialCmd)
	if err != nil {
		log.Error(err, "failed to post initial command")
		return err
	}

	sid := initialCmd.GetSessionId()

	// generalize resizeCh (or we may need to use reflect, which is inefficient)
	if resizeCh == nil {
		resizeCh = make(chan remotecommand.TerminalSize)
	}

	// read user input if needed
	inputCh := make(chan *connectivity.Cmd, 1)
	if in != nil {
		s := bufio.NewScanner(in)
		s.Split(util.ScanAnyAvail)

		go func() {
			// defer close(inputCh)

			for s.Scan() {
				inputCh <- connectivity.NewContainerInputCmd(sid, s.Bytes())
			}
			log.Error(s.Err(), "finished stream input", "remains", s.Text())
		}()
	}

	defer func() {
		// close out and stderr with best effort
		log.Info("close out writer")
		_ = out.Close()

		if stderr != nil {
			log.Info("close err writer")
			_ = stderr.Close()
		}
	}()

	for {
		select {
		case <-m.ctx.Done():
			return
		case userInput, more := <-inputCh:
			if !more {
				log.Info("input ch closed")
				return nil
			}

			_, err = m.manager.PostCmd(ctx, userInput)
			if err != nil {
				log.Error(err, "failed to post user input")
				return err
			}
		case msg, more := <-msgCh:
			if !more {
				log.Info("msg ch closed")
				return nil
			}

			// only PodData will be received in this session
			switch m := msg.GetMsg().(type) {
			case *connectivity.Msg_Data:
				targetOutput := out
				switch m.Data.GetKind() {
				case connectivity.OTHER, connectivity.STDOUT:
					targetOutput = out
				case connectivity.STDERR:
					if stderr != nil {
						targetOutput = stderr
					}
				default:
					return fmt.Errorf("data kind unknown")
				}

				_, err = targetOutput.Write(m.Data.GetData())
				if err != nil {
					log.Error(err, "failed to write output")
					return err
				}
			}
		case size, more := <-resizeCh:
			if !more {
				log.Info("resize ch closed")
				return nil
			}

			resizeCmd := connectivity.NewContainerTtyResizeCmd(sid, size.Width, size.Height)
			_, err = m.manager.PostCmd(ctx, resizeCmd)
			if err != nil {
				log.Error(err, "failed to post resize cmd")
				return err
			}
		}
	}
}
