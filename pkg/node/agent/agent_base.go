package agent

import (
	"context"
	"errors"
	"log"
	"sync"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/node/agent/runtime"
	"arhat.dev/aranya/pkg/node/connectivity"
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
	sid := cmd.GetSessionId()

	switch cm := cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		switch cm.NodeCmd.GetAction() {
		case connectivity.GetInfoAll:
			log.Printf("recv node cmd get all info, sid: %v", sid)
			c.doGetNodeInfoAll(sid)
		case connectivity.GetSystemInfo:
			log.Printf("recv node cmd get system info, sid: %v", sid)
			c.doGetNodeSystemInfo(sid)
		case connectivity.GetResources:
			log.Printf("recv node cmd get resources, sid: %v", sid)
			c.doGetNodeResources(sid)
		case connectivity.GetConditions:
			log.Printf("recv node cmd get resources, sid: %v", sid)
			c.doGetNodeConditions(sid)
		default:
			log.Printf("unknown node cmd: %v", cm.NodeCmd)
		}
	case *connectivity.Cmd_ImageCmd:
		switch cm.ImageCmd.GetAction() {
		case connectivity.ListImages:
			log.Printf("recv image cmd list, sid: %v", sid)
			go c.doImageList(sid)
		default:
			log.Printf("unknown image cmd: %v", cm.ImageCmd)
		}
	case *connectivity.Cmd_PodCmd:
		switch cm.PodCmd.GetAction() {
		// pod scope commands
		case connectivity.CreatePod:
			log.Printf("recv pod create cmd, sid: %v", sid)
			go c.doPodCreate(sid, cm.PodCmd.GetCreateOptions())
		case connectivity.DeletePod:
			log.Printf("recv pod delete cmd, sid: %v", sid)
			go c.doPodDelete(sid, cm.PodCmd.GetDeleteOptions())
		case connectivity.ListPods:
			log.Printf("recv pod list cmd, sid: %v", sid)
			go c.doPodList(sid, cm.PodCmd.GetListOptions())
		case connectivity.PortForward:
			log.Printf("recv pod port forward cmd, sid: %v", sid)
			inputCh := make(chan []byte, 1)
			c.openedStreams.add(sid, inputCh, nil)
			go c.doPortForward(sid, cm.PodCmd.GetPortForwardOptions(), inputCh)

		// container scope commands
		case connectivity.Exec:
			log.Printf("recv pod exec cmd, sid: %v", sid)
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			go c.doContainerExec(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Attach:
			log.Printf("recv pod attach cmd, sid: %v", sid)
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			c.openedStreams.add(sid, inputCh, resizeCh)

			go c.doContainerAttach(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
		case connectivity.Log:
			log.Printf("recv pod log cmd, sid: %v", sid)
			go c.doContainerLog(sid, cm.PodCmd.GetLogOptions())
		case connectivity.Input:
			log.Printf("recv input cmd, sid: %v", sid)
			inputCh, ok := c.openedStreams.getInputChan(sid)
			if !ok {
				c.handleError(sid, ErrStreamSessionClosed)
				return
			}

			go func() {
				inputCh <- cm.PodCmd.GetInputOptions().GetData()
			}()
		case connectivity.ResizeTty:
			log.Printf("recv resize cmd, sid: %v", sid)
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
