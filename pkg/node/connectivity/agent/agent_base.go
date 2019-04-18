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

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/agent/runtime"
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
	sid := cmd.GetSessionId()

	switch cm := cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		log.Printf("recv node cmd session: %v", sid)
		c.doNodeInfo(sid)
	case *connectivity.Cmd_PodCmd:
		switch cm.PodCmd.GetAction() {
		// pod scope commands
		case connectivity.Create:
			log.Printf("recv pod create cmd session: %v", sid)
			go c.doPodCreate(sid, cm.PodCmd.GetCreateOptions())
		case connectivity.Delete:
			log.Printf("recv pod delete cmd session: %v", sid)
			go c.doPodDelete(sid, cm.PodCmd.GetDeleteOptions())
		case connectivity.List:
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
				log.Printf("send resize cmd for session: %v", sid)
				resizeCh <- remotecommand.TerminalSize{
					Width:  uint16(cm.PodCmd.GetResizeOptions().GetCols()),
					Height: uint16(cm.PodCmd.GetResizeOptions().GetRows()),
				}
				log.Printf("send resize cmd for session: %v", sid)
			}()
		}
	}
}

// Internal processing

func (c *baseClient) handleError(sid uint64, e error) {
	log.Printf("exception happened in session %v: %v", sid, e)
	if err := c.doPostMsg(connectivity.NewErrorMsg(sid, e)); err != nil {
		log.Printf("handle error: %v", err)
	}
}

func (c baseClient) doNodeInfo(sid uint64) {
	now := metav1.Time{Time: time.Now()}

	nodeSystemInfo := systemInfo()
	nodeSystemInfo.MachineID, _ = machineid.ID()
	nodeSystemInfo.OperatingSystem = c.runtime.OS()
	nodeSystemInfo.Architecture = c.runtime.Arch()
	nodeSystemInfo.KernelVersion = c.runtime.KernelVersion()
	nodeSystemInfo.ContainerRuntimeVersion = c.runtime.Name() + "://" + c.runtime.Version()
	// set KubeletVersion and KubeProxyVersion at server side
	// nodeSystemInfo.KubeletVersion
	// nodeSystemInfo.KubeProxyVersion

	nodeMsg := connectivity.NewNodeMsg(sid, &corev1.Node{
		Status: corev1.NodeStatus{
			Capacity: corev1.ResourceList{
				corev1.ResourceCPU:              *resourcev1.NewQuantity(1, resourcev1.DecimalSI),
				corev1.ResourceMemory:           *resourcev1.NewQuantity(512*(2<<20), resourcev1.BinarySI),
				corev1.ResourcePods:             *resourcev1.NewQuantity(20, resourcev1.DecimalSI),
				corev1.ResourceEphemeralStorage: *resourcev1.NewQuantity(1*(2<<30), resourcev1.BinarySI),
			},
			Allocatable: corev1.ResourceList{
				corev1.ResourceCPU:              *resourcev1.NewQuantity(1, resourcev1.DecimalSI),
				corev1.ResourceMemory:           *resourcev1.NewQuantity(512*(2<<20), resourcev1.BinarySI),
				corev1.ResourcePods:             *resourcev1.NewQuantity(20, resourcev1.DecimalSI),
				corev1.ResourceEphemeralStorage: *resourcev1.NewQuantity(1*(2<<30), resourcev1.BinarySI),
			},
			Phase: corev1.NodeRunning,
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: now, LastTransitionTime: now},
				{Type: corev1.NodeOutOfDisk, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
			},
			NodeInfo:        *nodeSystemInfo,
			Images:          []corev1.ContainerImage{},
			VolumesInUse:    []corev1.UniqueVolumeName{},
			VolumesAttached: []corev1.AttachedVolume{},
		},
	})

	if err := c.doPostMsg(nodeMsg); err != nil {
		c.handleError(sid, err)
		return
	}
}

func (c *baseClient) doPodCreate(sid uint64, options *connectivity.CreateOptions) {
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

func (c *baseClient) doPodDelete(sid uint64, options *connectivity.DeleteOptions) {
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

func (c *baseClient) doPodList(sid uint64, options *connectivity.ListOptions) {
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

func (c *baseClient) doContainerAttach(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
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

func (c *baseClient) doContainerExec(sid uint64, options *connectivity.ExecOptions, inputCh <-chan []byte, resizeCh <-chan remotecommand.TerminalSize) {
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

func (c *baseClient) doContainerLog(sid uint64, options *connectivity.LogOptions) {
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

func (c *baseClient) doPortForward(sid uint64, options *connectivity.PortForwardOptions, inputCh <-chan []byte) {
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
