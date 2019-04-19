package agent

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log"
	"sync"
	"time"

	"github.com/denisbrodbeck/machineid"
	corev1 "k8s.io/api/core/v1"
	resourcev1 "k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/connectivity"
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

func newBaseClient(rt runtime.Interface) baseAgent {
	return baseAgent{
		runtime: rt,
		openedStreams: streamSession{
			inputCh:  make(map[uint64]chan []byte),
			resizeCh: make(map[uint64]chan remotecommand.TerminalSize),
		},
	}
}

type baseAgent struct {
	doPostMsg func(msg *connectivity.Msg) error

	openedStreams streamSession
	mu            sync.RWMutex
	runtime       runtime.Interface
}

// Called by actual connectivity client

func (c *baseAgent) onConnect(connect func() error) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	return connect()
}

func (c *baseAgent) onDisconnected(setDisconnected func()) {
	c.mu.Lock()
	defer c.mu.Unlock()

	setDisconnected()
}

func (c *baseAgent) onPostMsg(msg *connectivity.Msg, send func(*connectivity.Msg) error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return send(msg)
}

func (c *baseAgent) onRecvCmd(cmd *connectivity.Cmd) {
	// TODO: should we add work queue to get failed work rescheduled locally or controlled by aranya completely?
	sid := cmd.GetSessionId()

	switch cm := cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		switch cm.NodeCmd.GetAction() {
		case connectivity.GetSystemInfo:
			log.Printf("recv node cmd get system info session: %v", sid)
			c.doGetNodeSystemInfo(sid)
		case connectivity.GetResources:
			log.Printf("recv node cmd get resources, sid: %v", sid)

		default:
			log.Printf("unknown node cmd: %v", cm.NodeCmd)
		}
	case *connectivity.Cmd_ImageCmd:
		switch cm.ImageCmd.GetAction() {
		case connectivity.ListImages:
			log.Printf("recv image cmd list, session: %v", sid)
			go c.doImageList(sid)
		default:
			log.Printf("unknown image cmd: %v", cm.ImageCmd)
		}
	case *connectivity.Cmd_PodCmd:
		switch cm.PodCmd.GetAction() {
		// pod scope commands
		case connectivity.CreatePod:
			log.Printf("recv pod create cmd session: %v", sid)
			go c.doPodCreate(sid, cm.PodCmd.GetCreateOptions())
		case connectivity.DeletePod:
			log.Printf("recv pod delete cmd session: %v", sid)
			go c.doPodDelete(sid, cm.PodCmd.GetDeleteOptions())
		case connectivity.ListPods:
			log.Printf("recv pod list cmd session: %v", sid)
			go c.doPodList(sid, cm.PodCmd.GetListOptions())
		case connectivity.PortForward:
			log.Printf("recv pod port forward cmd session: %v", sid)
			inputCh := make(chan []byte, 1)
			c.openedStreams.add(sid, inputCh, nil)
			go c.doPortForward(sid, cm.PodCmd.GetPortForwardOptions(), inputCh)

		// container scope commands
		case connectivity.Exec:
			log.Printf("recv pod exec cmd session: %v", sid)
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			go c.doContainerExec(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Attach:
			log.Printf("recv pod attach cmd session: %v", sid)
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			go c.doContainerAttach(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Log:
			log.Printf("recv pod log cmd session: %v", sid)
			go c.doContainerLog(sid, cm.PodCmd.GetLogOptions())
		case connectivity.Input:
			log.Printf("recv input cmd for session: %v", sid)
			inputCh, ok := c.openedStreams.getInputChan(sid)
			if !ok {
				c.handleError(sid, ErrStreamSessionClosed)
				return
			}

			go func() {
				inputCh <- cm.PodCmd.GetInputOptions().GetData()
			}()
		case connectivity.ResizeTty:
			log.Printf("recv resize cmd for session: %v", sid)
			resizeCh, ok := c.openedStreams.getResizeChan(sid)
			if !ok {
				c.handleError(sid, ErrStreamSessionClosed)
				return
			}

			go func() {
				resizeCh <- remotecommand.TerminalSize{
					Width:  uint16(cm.PodCmd.GetResizeOptions().GetCols()),
					Height: uint16(cm.PodCmd.GetResizeOptions().GetRows()),
				}
			}()
		default:
			log.Printf("unknown pod cmd: %v", cm.PodCmd)
		}
	default:
		log.Printf("unknown cmd: %v", cm)
	}
}

// Internal processing

func (c *baseAgent) handleError(sid uint64, e error) {
	log.Printf("exception happened in session %v: %v", sid, e)
	if err := c.doPostMsg(connectivity.NewErrorMsg(sid, e)); err != nil {
		log.Printf("handle error: %v", err)
	}
}

func (c *baseAgent) doGetNodeSystemInfo(sid uint64) {
	nodeSystemInfo := systemInfo()
	nodeSystemInfo.MachineID, _ = machineid.ID()
	nodeSystemInfo.OperatingSystem = c.runtime.OS()
	nodeSystemInfo.Architecture = c.runtime.Arch()
	nodeSystemInfo.KernelVersion = c.runtime.KernelVersion()
	nodeSystemInfo.ContainerRuntimeVersion = c.runtime.Name() + "://" + c.runtime.Version()
	// set KubeletVersion and KubeProxyVersion at server side
	// nodeSystemInfo.KubeletVersion
	// nodeSystemInfo.KubeProxyVersion

	nodeMsg := connectivity.NewNodeMsg(sid, nodeSystemInfo, nil, nil, nil)
	if err := c.doPostMsg(nodeMsg); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doGetNodeResources(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, nil, corev1.ResourceList{
		corev1.ResourceCPU:              *resourcev1.NewQuantity(1, resourcev1.DecimalSI),
		corev1.ResourceMemory:           *resourcev1.NewQuantity(512*(2<<20), resourcev1.BinarySI),
		corev1.ResourcePods:             *resourcev1.NewQuantity(20, resourcev1.DecimalSI),
		corev1.ResourceEphemeralStorage: *resourcev1.NewQuantity(1*(2<<30), resourcev1.BinarySI),
	}, corev1.ResourceList{
		corev1.ResourceCPU:              *resourcev1.NewQuantity(1, resourcev1.DecimalSI),
		corev1.ResourceMemory:           *resourcev1.NewQuantity(512*(2<<20), resourcev1.BinarySI),
		corev1.ResourcePods:             *resourcev1.NewQuantity(20, resourcev1.DecimalSI),
		corev1.ResourceEphemeralStorage: *resourcev1.NewQuantity(1*(2<<30), resourcev1.BinarySI),
	}, nil)
	if err := c.doPostMsg(nodeMsg); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doGetNodeConditions(sid uint64) {
	now := metav1.NewTime(time.Now())
	nodeMsg := connectivity.NewNodeMsg(sid, nil, nil, nil, []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeOutOfDisk, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
	})
	if err := c.doPostMsg(nodeMsg); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseAgent) doImageList(sid uint64) {
	images, err := c.runtime.ListImages()
	if err != nil {
		c.handleError(sid, err)
		return
	}

	if len(images) == 0 {
		if err := c.doPostMsg(connectivity.NewImageMsg(sid, true, nil)); err != nil {
			c.handleError(sid, err)
		}
		return
	}

	lastIndex := len(images) - 1
	for i, p := range images {
		if err := c.doPostMsg(connectivity.NewImageMsg(sid, i == lastIndex, p)); err != nil {
			c.handleError(sid, err)
			return
		}
	}
}

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
