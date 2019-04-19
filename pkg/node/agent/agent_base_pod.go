package agent

import (
	"bufio"
	"io"
	"log"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/util"
)

func (c *baseAgent) doPodCreate(sid uint64, options *connectivity.CreateOptions) {
	podResp, err := c.runtime.CreatePod(options)

	if err != nil {
		c.handleError(sid, err)
		return
	}

	if err := c.doPostMsg(connectivity.NewPodMsg(sid, true, podResp)); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doPodDelete(sid uint64, options *connectivity.DeleteOptions) {
	podDeleted, err := c.runtime.DeletePod(options)
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if err := c.doPostMsg(connectivity.NewPodMsg(sid, true, podDeleted)); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doPodList(sid uint64, options *connectivity.ListOptions) {
	pods, err := c.runtime.ListPod(options)
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if len(pods) == 0 {
		if err := c.doPostMsg(connectivity.NewPodMsg(sid, true, nil)); err != nil {
			c.handleError(sid, err)
		}
		return
	}

	lastIndex := len(pods) - 1
	for i, p := range pods {
		if err := c.doPostMsg(connectivity.NewPodMsg(sid, i == lastIndex, p)); err != nil {
			c.handleError(sid, err)
			return
		}
	}
}

func (c *baseAgent) doContainerAttach(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer c.openedStreams.del(sid)

	opt, err := options.GetResolvedExecOptions()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	var (
		stdin  io.ReadCloser
		stdout io.WriteCloser
		stderr io.WriteCloser

		remoteStdin  io.WriteCloser
		remoteStdout io.ReadCloser
		remoteStderr io.ReadCloser
	)

	if opt.Stdin {
		stdin, remoteStdin = io.Pipe()
		defer func() { _, _ = stdin.Close(), remoteStdin.Close() }()

		go func() {
			for inputData := range inputCh {
				_, err := remoteStdin.Write(inputData)
				if err != nil {
					return
				}
			}
		}()
	}

	if opt.Stdout {
		remoteStdout, stdout = io.Pipe()
		defer func() { _, _ = remoteStdout.Close(), stdout.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStdout)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
					c.handleError(sid, err)
					return
				}
			}
		}()
	}

	if opt.Stderr {
		remoteStderr, stderr = io.Pipe()
		defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStderr)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
					c.handleError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = c.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := c.runtime.AttachContainer(options.GetPodUid(), opt.Container, stdin, stdout, stderr, resizeCh); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doContainerExec(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer func() {
		c.openedStreams.del(sid)
		log.Printf("finished contaienr exec")
	}()

	opt, err := options.GetResolvedExecOptions()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	var (
		stdin  io.ReadCloser
		stdout io.WriteCloser
		stderr io.WriteCloser

		remoteStdin  io.WriteCloser
		remoteStdout io.ReadCloser
		remoteStderr io.ReadCloser
	)

	if opt.Stdin {
		stdin, remoteStdin = io.Pipe()

		go func() {
			defer func() { _, _ = stdin.Close(), remoteStdin.Close() }()

			for inputData := range inputCh {
				_, err := remoteStdin.Write(inputData)
				if err != nil {
					return
				}
			}
		}()
	}

	if opt.Stdout {
		remoteStdout, stdout = io.Pipe()
		defer func() { _, _ = remoteStdout.Close(), stdout.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStdout)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				data := s.Bytes()
				if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, data)); err != nil {
					c.handleError(sid, err)
					return
				}
			}
		}()
	}

	if opt.Stderr {
		remoteStderr, stderr = io.Pipe()
		defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStderr)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				data := s.Bytes()
				if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, data)); err != nil {
					c.handleError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = c.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()
	if err := c.runtime.ExecInContainer(options.GetPodUid(), opt.Container, stdin, stdout, stderr, resizeCh, opt.Command, opt.TTY); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doContainerLog(sid uint64, options *connectivity.LogOptions) {
	opt, err := options.GetResolvedLogOptions()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	remoteStdout, stdout := io.Pipe()
	defer func() { _, _ = remoteStdout.Close(), stdout.Close() }()

	remoteStderr, stderr := io.Pipe()
	defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

	// read stdout
	go func() {
		s := bufio.NewScanner(remoteStdout)
		s.Split(util.ScanAnyAvail)

		for s.Scan() {
			if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// read stderr
	go func() {
		s := bufio.NewScanner(remoteStderr)
		s.Split(util.ScanAnyAvail)

		for s.Scan() {
			if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// best effort
	defer func() { _ = c.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := c.runtime.GetContainerLogs(options.GetPodUid(), opt, stdout, stderr); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doPortForward(sid uint64, options *connectivity.PortForwardOptions, inputCh <-chan []byte) {
	defer c.openedStreams.del(sid)

	opt, err := options.GetResolvedOptions()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	input, remoteInput := io.Pipe()
	remoteOutput, output := io.Pipe()
	defer func() { _, _ = remoteOutput.Close(), output.Close() }()

	// read input
	go func() {
		defer func() { _, _ = input.Close(), remoteInput.Close() }()

		for inputData := range inputCh {
			_, err := remoteInput.Write(inputData)
			if err != nil {
				return
			}
		}
	}()

	// read output
	go func() {
		s := bufio.NewScanner(remoteOutput)
		s.Split(util.ScanAnyAvail)

		for s.Scan() {
			if err := c.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// best effort
	defer func() { _ = c.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := c.runtime.PortForward(options.GetPodUid(), opt, input, output); err != nil {
		c.handleError(sid, err)
		return
	}
}
