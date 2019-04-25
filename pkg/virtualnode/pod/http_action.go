/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	"arhat.dev/aranya/pkg/connectivity"
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
	var (
		since     time.Time
		tailLines int64
	)

	if options.SinceTime != nil {
		since = options.SinceTime.Time
	} else if options.SinceSeconds != nil {
		since = time.Now().Add(-time.Duration(*options.SinceSeconds) * time.Second)
	}

	if options.TailLines != nil {
		tailLines = *options.TailLines
	} else {
		tailLines = -1
	}

	msgCh, err := m.connectivityManager.PostCmd(m.ctx, connectivity.NewContainerLogCmd(string(uid), options.Container, options.Follow, options.Timestamps, since, tailLines))
	if err != nil {
		return nil, err
	}

	reader, writer := io.Pipe()
	go func() {
		defer func() { _, _ = reader.Close(), writer.Close() }()

		for msg := range msgCh {
			_, err := writer.Write(msg.GetData().GetData())
			if err != nil {
				return
			}
		}
	}()

	return reader, nil
}

func (m *Manager) doHandleExecInContainer() kubeletrc.Executor {
	return containerExecutor(func(name string, uid types.UID, container string, cmd []string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize, timeout time.Duration) error {
		execCmd := connectivity.NewContainerExecCmd(string(uid), container, cmd, stdin != nil, stdout != nil, stderr != nil, tty)
		err := m.doServeStream(execCmd, stdin, stdout, stderr, resize)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doHandleAttachContainer() kubeletrc.Attacher {
	return containerAttacher(func(name string, uid types.UID, container string, stdin io.Reader, stdout, stderr io.WriteCloser, tty bool, resize <-chan remotecommand.TerminalSize) error {
		attachCmd := connectivity.NewContainerAttachCmd(string(uid), container, stdin != nil, stdout != nil, stderr != nil, tty)
		err := m.doServeStream(attachCmd, stdin, stdout, stderr, resize)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doHandlePortForward(portProto map[int32]string) kubeletpf.PortForwarder {
	return portForwarder(func(name string, uid types.UID, port int32, stream io.ReadWriteCloser) error {
		portForwardCmd := connectivity.NewPortForwardCmd(string(uid), port, portProto[port])
		err := m.doServeStream(portForwardCmd, stream, stream, nil, nil)
		if err != nil {
			return err
		}

		return nil
	})
}

func (m *Manager) doServeStream(initialCmd *connectivity.Cmd, in io.Reader, out, stderr io.WriteCloser, resizeCh <-chan remotecommand.TerminalSize) (err error) {
	log := m.log.WithValues("type", "stream")
	streamCtx, exitStream := context.WithCancel(m.ctx)

	defer func() {
		exitStream()

		// close out and stderr with best effort
		_ = out.Close()
		if stderr != nil {
			_ = stderr.Close()
		}

		log.Info("finished stream")
	}()

	if out == nil {
		return fmt.Errorf("output should not be nil")
	}

	var msgCh <-chan *connectivity.Msg
	if msgCh, err = m.connectivityManager.PostCmd(streamCtx, initialCmd); err != nil {
		log.Error(err, "failed to post initial cmd")
		return err
	}

	sid := initialCmd.GetSessionId()
	defer func() {
		_, err := m.connectivityManager.PostCmd(m.ctx, connectivity.NewSessionCloseCmd(sid))
		if err != nil {
			log.Error(err, "failed to post session close cmd")
		}
	}()

	// generalize resizeCh (or we may need to use reflect, which is inefficient)
	if resizeCh == nil {
		resizeCh = make(chan remotecommand.TerminalSize)
	}

	// read user input if needed
	inputCh := make(chan *connectivity.Cmd, 1)
	if in != nil {
		go func() {
			defer func() {
				close(inputCh)

				log.Info("finished stream input")
			}()

			s := bufio.NewScanner(in)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				select {
				case inputCh <- connectivity.NewContainerInputCmd(sid, s.Bytes()):
				case <-streamCtx.Done():
					return
				}
			}
		}()
	}

	for {
		select {
		case <-streamCtx.Done():
			return
		case userInput, more := <-inputCh:
			if !more {
				log.Info("input channel closed")
				return nil
			}

			if _, err = m.connectivityManager.PostCmd(streamCtx, userInput); err != nil {
				log.Error(err, "failed to post user input")
				return err
			}
		case msg, more := <-msgCh:
			if !more {
				log.Info("msg channel closed")
				return nil
			}

			// only PodData should be received in this session
			if dataMsg := msg.GetData(); dataMsg != nil {
				targetOutput := out
				switch dataMsg.GetKind() {
				case connectivity.OTHER, connectivity.STDOUT:
					targetOutput = out
				case connectivity.STDERR:
					if stderr != nil {
						targetOutput = stderr
					}
				default:
					return fmt.Errorf("data kind unknown")
				}

				if _, err = targetOutput.Write(dataMsg.Data); err != nil && err != io.EOF {
					log.Error(err, "failed to write output")
					return err
				}
			}
		case size, more := <-resizeCh:
			if !more {
				return err
			}

			resizeCmd := connectivity.NewContainerTtyResizeCmd(sid, size.Width, size.Height)
			if _, err = m.connectivityManager.PostCmd(streamCtx, resizeCmd); err != nil {
				log.Error(err, "failed to post resize cmd")
				return err
			}
		}
	}
}
