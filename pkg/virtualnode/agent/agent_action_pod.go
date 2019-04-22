package agent

import (
	"bufio"
	"errors"
	"io"
	"log"
	"strings"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/util"
)

func (b *baseAgent) doPodCreate(sid uint64, options *connectivity.CreateOptions) {
	podResp, err := b.runtime.CreatePod(options)

	if err != nil {
		b.handleError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodMsg(sid, true, podResp)); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doPodDelete(sid uint64, options *connectivity.DeleteOptions) {
	podDeleted, err := b.runtime.DeletePod(options)
	if err != nil {
		b.handleError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodMsg(sid, true, podDeleted)); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doPodList(sid uint64, options *connectivity.ListOptions) {
	pods, err := b.runtime.ListPod(options)
	if err != nil {
		b.handleError(sid, err)
		return
	}

	if len(pods) == 0 {
		if err := b.doPostMsg(connectivity.NewPodMsg(sid, true, nil)); err != nil {
			b.handleError(sid, err)
		}
		return
	}

	lastIndex := len(pods) - 1
	for i, p := range pods {
		if err := b.doPostMsg(connectivity.NewPodMsg(sid, i == lastIndex, p)); err != nil {
			b.handleError(sid, err)
			return
		}
	}
}

func (b *baseAgent) doContainerAttach(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer b.openedStreams.del(sid)

	opt, err := options.GetResolvedExecOptions()
	if err != nil {
		b.handleError(sid, err)
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
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
					b.handleError(sid, err)
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
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
					b.handleError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := b.runtime.AttachContainer(options.GetPodUid(), opt.Container, stdin, stdout, stderr, resizeCh); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doContainerExec(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer func() {
		b.openedStreams.del(sid)
		log.Printf("finished contaienr exec")
	}()

	opt, err := options.GetResolvedExecOptions()
	if err != nil {
		b.handleError(sid, err)
		return
	}

	if len(opt.Command) == 0 {
		b.handleError(sid, errors.New("command not provided for exec"))
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
			// input closed, notify other
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
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
					b.handleError(sid, err)
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
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
					b.handleError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if strings.HasPrefix(opt.Command[0], "#") {
		if b.config.AllowHostExec {
			// host exec
			opt.Command[0] = opt.Command[0][1:]
			if err := execInHost(stdin, stdout, stderr, resizeCh, opt.Command, opt.TTY); err != nil {
				b.handleError(sid, err)
				return
			}
		} else {
			b.handleError(sid, errors.New("host exec not allowed"))
		}

		return
	} else {
		// container exec
		if err := b.runtime.ExecInContainer(options.GetPodUid(), opt.Container, stdin, stdout, stderr, resizeCh, opt.Command, opt.TTY); err != nil {
			b.handleError(sid, err)
			return
		}

		return
	}
}

func (b *baseAgent) doContainerLog(sid uint64, options *connectivity.LogOptions) {
	opt, err := options.GetResolvedLogOptions()
	if err != nil {
		b.handleError(sid, err)
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
			if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// read stderr
	go func() {
		s := bufio.NewScanner(remoteStderr)
		s.Split(util.ScanAnyAvail)

		for s.Scan() {
			if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := b.runtime.GetContainerLogs(options.GetPodUid(), opt, stdout, stderr); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doPortForward(sid uint64, options *connectivity.PortForwardOptions, inputCh <-chan []byte) {
	defer b.openedStreams.del(sid)

	opt, err := options.GetResolvedOptions()
	if err != nil {
		b.handleError(sid, err)
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
			if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
				return
			}
		}
	}()

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := b.runtime.PortForward(options.GetPodUid(), opt, input, output); err != nil {
		b.handleError(sid, err)
		return
	}
}
