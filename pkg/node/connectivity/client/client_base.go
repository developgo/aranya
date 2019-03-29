package client

import (
	"bufio"
	"context"
	"errors"
	"io"
	"sync"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/client/runtime"
	"arhat.dev/aranya/pkg/node/util"
)

var (
	ErrClientAlreadyConnected = errors.New("client already connected ")
	ErrClientNotConnected     = errors.New("client not connected ")
	ErrStreamSessionClosed    = errors.New("stream session closed ")
)

type Interface interface {
	Run(ctx context.Context) error
	PostMsg(msg *connectivity.Msg) error
}

type streamSession struct {
	inputCh  map[uint64]chan []byte
	resizeCh map[uint64]chan remotecommand.TerminalSize
	mu       sync.RWMutex
}

func (s *streamSession) add(sid uint64, dataCh chan []byte, resizeCh chan remotecommand.TerminalSize) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if oldInputCh, ok := s.inputCh[sid]; ok {
		close(oldInputCh)
	}

	if oldResizeCh, ok := s.resizeCh[sid]; ok {
		close(oldResizeCh)
	}

	s.inputCh[sid] = dataCh
	s.resizeCh[sid] = resizeCh
}

func (s *streamSession) getInputChan(sid uint64) (chan []byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.inputCh[sid]
	return ch, ok
}

func (s *streamSession) getResizeChan(sid uint64) (chan remotecommand.TerminalSize, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ch, ok := s.resizeCh[sid]
	return ch, ok
}

func (s *streamSession) del(sid uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ch, ok := s.inputCh[sid]; ok {
		if ch != nil {
			close(ch)
		}
		delete(s.inputCh, sid)
	}

	if ch, ok := s.resizeCh[sid]; ok {
		if ch != nil {
			close(ch)
		}
		delete(s.resizeCh, sid)
	}
}

func newBaseClient(rt runtime.Interface) baseClient {
	return baseClient{
		runtime: rt,
		openedStreams: streamSession{
			inputCh:  make(map[uint64]chan []byte),
			resizeCh: make(map[uint64]chan remotecommand.TerminalSize),
		},
	}
}

type baseClient struct {
	doPostMsg func(msg *connectivity.Msg) error

	openedStreams streamSession
	mu            sync.RWMutex
	runtime       runtime.Interface
}

// Called by actual connectivity client

func (c *baseClient) onConnect(connect func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return connect()
}

func (c *baseClient) onDisconnected(setDisconnected func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	setDisconnected()
}

func (c *baseClient) onPostMsg(msg *connectivity.Msg, send func(*connectivity.Msg) error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return send(msg)
}

func (c *baseClient) onSrvCmd(cmd *connectivity.Cmd) {
	switch cm := cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		_ = cm.NodeCmd
	case *connectivity.Cmd_PodCmd:
		sid := cmd.GetSessionId()
		ns := cm.PodCmd.GetNamespace()
		name := cm.PodCmd.GetName()

		switch cm.PodCmd.GetAction() {

		// pod scope commands
		case connectivity.Create:
			c.doPodCreate(sid, ns, name, cm.PodCmd.GetCreateOptions())
		case connectivity.Delete:
			c.doPodDelete(sid, ns, name, cm.PodCmd.GetDeleteOptions())
		case connectivity.List:
			c.doPodList(sid, ns, name)
		case connectivity.PortForward:
			inputCh := make(chan []byte, 1)
			c.openedStreams.add(sid, inputCh, nil)
			c.doPortForward(sid, ns, name, cm.PodCmd.GetPortForwardOptions(), inputCh)

		// container scope commands
		case connectivity.Exec:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			c.doContainerExec(sid, ns, name, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Attach:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			c.doContainerAttach(sid, ns, name, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Log:
			c.doContainerLog(sid, ns, name, cm.PodCmd.GetLogOptions())
		case connectivity.Input:
			inputCh, ok := c.openedStreams.getInputChan(sid)
			if !ok {
				c.handleError(sid, ErrStreamSessionClosed)
				return
			}

			inputCh <- cm.PodCmd.GetInputOptions().GetData()
		case connectivity.ResizeTty:
			resizeCh, ok := c.openedStreams.getResizeChan(sid)
			if !ok {
				c.handleError(sid, ErrStreamSessionClosed)
				return
			}

			resizeCh <- remotecommand.TerminalSize{
				Width:  uint16(cm.PodCmd.GetResizeOptions().GetCols()),
				Height: uint16(cm.PodCmd.GetResizeOptions().GetRows()),
			}
		}
	}
}

// Internal processing

func (c *baseClient) handleError(sid uint64, e error) {
	if err := c.doPostMsg(connectivity.NewErrorMsg(sid, e)); err != nil {
		// TODO: log error
	}
}

func (c *baseClient) doPodCreate(sid uint64, namespace, name string, options *connectivity.CreateOptions) {
	podSpec, authConfig, volumeData, err := options.GetResolvedCreateOptions()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	podResp, err := c.runtime.CreatePod(namespace, name, podSpec, authConfig, volumeData)
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if err := c.doPostMsg(connectivity.NewPodMsg(sid, true, podResp)); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doPodDelete(sid uint64, namespace, name string, options *connectivity.DeleteOptions) {
	podDeleted, err := c.runtime.DeletePod(namespace, name, options)
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if err := c.doPostMsg(connectivity.NewPodMsg(sid, true, podDeleted)); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doPodList(sid uint64, namespace, name string) {
	pods, err := c.runtime.ListPod(namespace, name)
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

func (c *baseClient) doContainerAttach(sid uint64, namespace, name string, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
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

	if err := c.runtime.AttachContainer(namespace, name, opt.Container, stdin, stdout, stderr, resizeCh); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doContainerExec(sid uint64, namespace, name string, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
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
		defer func() { _, _ = remoteStderr.Close(), stderr.Close() }()

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

	if err := c.runtime.ExecInContainer(namespace, name, opt.Container, stdin, stdout, stderr, resizeCh, opt.Command, opt.TTY); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doContainerLog(sid uint64, namespace, name string, options *connectivity.LogOptions) {
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

	if err := c.runtime.GetContainerLogs(namespace, name, stdout, stderr, opt); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doPortForward(sid uint64, namespace, name string, options *connectivity.PortForwardOptions, inputCh <-chan []byte) {
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

	if err := c.runtime.PortForward(namespace, name, opt, input, output); err != nil {
		c.handleError(sid, err)
		return
	}
}
