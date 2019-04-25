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

package client

import (
	"bufio"
	"io"
	"log"
	"strings"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
	"arhat.dev/aranya/pkg/virtualnode/util"
)

func (b *baseAgent) doPodCreate(sid uint64, options *connectivity.CreateOptions) {
	podStatus, err := b.runtime.CreatePod(options)
	if err != nil {
		b.handleRuntimeError(sid, err)
		return
	}

	if err := b.doPostMsg(connectivity.NewPodStatusMsg(sid, podStatus)); err != nil {
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

func (b *baseAgent) doContainerAttach(sid uint64, options *connectivity.ExecOptions, stdin io.Reader, resizeCh <-chan *connectivity.TtyResizeOptions) {
	defer b.openedStreams.del(sid)

	var (
		stdout io.WriteCloser
		stderr io.WriteCloser

		remoteStdout io.ReadCloser
		remoteStderr io.ReadCloser
	)

	if !options.Stdin {
		stdin = nil
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

func (b *baseAgent) doContainerExec(sid uint64, options *connectivity.ExecOptions, stdin io.Reader, resizeCh <-chan *connectivity.TtyResizeOptions) {
	defer func() {
		b.openedStreams.del(sid)
		log.Printf("finished contaienr exec")
	}()

	if len(options.Command) == 0 {
		b.handleRuntimeError(sid, ErrCommandNotProvided)
		return
	}

	var (
		stdout io.WriteCloser
		stderr io.WriteCloser

		remoteStdout io.ReadCloser
		remoteStderr io.ReadCloser
	)

	if !options.Stdin {
		stdin = nil
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
		if b.Features.AllowHostExec {
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

func (b *baseAgent) doPortForward(sid uint64, options *connectivity.PortForwardOptions, input io.ReadCloser) {
	remoteOutput, output := io.Pipe()

	defer func() {
		b.openedStreams.del(sid)
		_, _ = remoteOutput.Close(), output.Close()
		_ = input.Close()
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
