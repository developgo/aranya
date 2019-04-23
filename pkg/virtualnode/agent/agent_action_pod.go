package agent

import (
	"bufio"
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
		b.handleRuntimeError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodStatusMsg(sid, podResp)); err != nil {
		b.handleConnectivityError(sid, err)
		return
	}
}

func (b *baseAgent) doPodDelete(sid uint64, options *connectivity.DeleteOptions) {
	podDeleted, err := b.runtime.DeletePod(options)
	if err != nil {
		b.handleRuntimeError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodStatusMsg(sid, podDeleted)); err != nil {
		b.handleConnectivityError(sid, err)
		return
	}
}

func (b *baseAgent) doPodList(sid uint64, options *connectivity.ListOptions) {
	pods, err := b.runtime.ListPods(options)
	if err != nil {
		b.handleRuntimeError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodStatusListMsg(sid, pods)); err != nil {
		b.handleConnectivityError(sid, err)
		return
	}
}

func (b *baseAgent) doContainerAttach(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer b.openedStreams.del(sid)

	var (
		stdin  io.ReadCloser
		stdout io.WriteCloser
		stderr io.WriteCloser

		remoteStdin  io.WriteCloser
		remoteStdout io.ReadCloser
		remoteStderr io.ReadCloser
	)

	if options.Stdin {
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

	if options.Stdout {
		remoteStdout, stdout = io.Pipe()
		defer func() { _, _ = remoteStdout.Close(), stdout.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStdout)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
					b.handleConnectivityError(sid, err)
					return
				}
			}
		}()
	}

	if options.Stderr {
		remoteStderr, stderr = io.Pipe()
		defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStderr)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
					b.handleConnectivityError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := b.runtime.AttachContainer(options.PodUid, options.Container, stdin, stdout, stderr, resizeCh); err != nil {
		b.handleRuntimeError(sid, err)
		return
	}
}

func (b *baseAgent) doContainerExec(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
	defer func() {
		b.openedStreams.del(sid)
		log.Printf("finished contaienr exec")
	}()

	if len(options.Command) == 0 {
		b.handleRuntimeError(sid, ErrCommandNotProvided)
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

	if options.Stdin {
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

	if options.Stdout {
		remoteStdout, stdout = io.Pipe()
		defer func() { _, _ = remoteStdout.Close(), stdout.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStdout)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDOUT, s.Bytes())); err != nil {
					b.handleConnectivityError(sid, err)
					return
				}
			}
		}()
	}

	if options.Stderr {
		remoteStderr, stderr = io.Pipe()
		defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

		go func() {
			s := bufio.NewScanner(remoteStderr)
			s.Split(util.ScanAnyAvail)

			for s.Scan() {
				if err := b.doPostMsg(connectivity.NewDataMsg(sid, false, connectivity.STDERR, s.Bytes())); err != nil {
					b.handleConnectivityError(sid, err)
					return
				}
			}
		}()
	}

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if strings.HasPrefix(options.Command[0], "#") {
		if b.AllowHostExec {
			// host exec
			options.Command[0] = options.Command[0][1:]
			if err := execInHost(stdin, stdout, stderr, resizeCh, options.Command, options.Tty); err != nil {
				b.handleRuntimeError(sid, err)
				return
			}
		} else {
			b.handleRuntimeError(sid, connectivity.NewCommonError("host exec not allowed"))
		}

		return
	} else {
		// container exec
		if err := b.runtime.ExecInContainer(options.PodUid, options.Container, stdin, stdout, stderr, resizeCh, options.Command, options.Tty); err != nil {
			b.handleRuntimeError(sid, err)
		}

		return
	}
}

func (b *baseAgent) doContainerLog(sid uint64, options *connectivity.LogOptions) {
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
				b.handleConnectivityError(sid, err)
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
				b.handleConnectivityError(sid, err)
				return
			}
		}
	}()

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if err := b.runtime.GetContainerLogs(options.PodUid, options, stdout, stderr); err != nil {
		b.handleRuntimeError(sid, err)
		return
	}
}

func (b *baseAgent) doPortForward(sid uint64, options *connectivity.PortForwardOptions, inputCh <-chan []byte) {
	defer b.openedStreams.del(sid)

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
				b.handleConnectivityError(sid, err)
				return
			}
		}
	}()

	// best effort
	defer func() { _ = b.doPostMsg(connectivity.NewDataMsg(sid, true, connectivity.OTHER, nil)) }()

	if options.Protocol == "" {
		options.Protocol = "tcp"
	}

	if err := b.runtime.PortForward(options.PodUid, options.Protocol, options.Port, input, output); err != nil {
		b.handleRuntimeError(sid, err)
		return
	}
}
