package agent

import (
	"context"
	"errors"
	"log"
	"sync"

	"k8s.io/client-go/tools/remotecommand"

	"arhat.dev/aranya/pkg/virtualnode/agent/runtime"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

var (
	ErrClientAlreadyConnected = errors.New("client already connected ")
	ErrClientNotConnected     = errors.New("client not connected ")
	ErrStreamSessionClosed    = errors.New("stream session closed ")
)

type Interface interface {
	Start(ctx context.Context) error
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

func newBaseClient(ctx context.Context, config *Config, rt runtime.Interface) baseAgent {
	return baseAgent{
		ctx:     ctx,
		config:  config,
		runtime: rt,
		openedStreams: streamSession{
			inputCh:  make(map[uint64]chan []byte),
			resizeCh: make(map[uint64]chan remotecommand.TerminalSize),
		},
	}
}

type baseAgent struct {
	ctx       context.Context
	config    *Config
	doPostMsg func(msg *connectivity.Msg) error

	openedStreams streamSession
	mu            sync.RWMutex
	runtime       runtime.Interface
}

// Called by actual connectivity client

func (b *baseAgent) onConnect(connect func() error) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	return connect()
}

func (b *baseAgent) onDisconnected(setDisconnected func()) {
	b.mu.Lock()
	defer b.mu.Unlock()

	setDisconnected()
}

func (b *baseAgent) onPostMsg(msg *connectivity.Msg, send func(*connectivity.Msg) error) error {
	b.mu.RLock()
	defer b.mu.RUnlock()

	return send(msg)
}

func (b *baseAgent) onRecvCmd(cmd *connectivity.Cmd) {
	sid := cmd.GetSessionId()

	switch cm := cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		switch cm.NodeCmd.GetAction() {
		case connectivity.GetInfoAll:
			processInNewGoroutine(sid, "node.get.all", func() {
				b.doGetNodeInfoAll(sid)
			})
		case connectivity.GetSystemInfo:
			processInNewGoroutine(sid, "node.get.sys", func() {
				b.doGetNodeSystemInfo(sid)
			})
		case connectivity.GetResources:
			processInNewGoroutine(sid, "node.get.res", func() {
				b.doGetNodeResources(sid)
			})
		case connectivity.GetConditions:
			processInNewGoroutine(sid, "node.get.cond", func() {
				b.doGetNodeConditions(sid)
			})
		default:
			log.Printf("[%d] unknown node cmd: %v", sid, cm.NodeCmd)
		}
	case *connectivity.Cmd_ImageCmd:
		switch cm.ImageCmd.GetAction() {
		case connectivity.ListImages:
			processInNewGoroutine(sid, "image.list", func() {
				b.doImageList(sid)
			})
		default:
			log.Printf("[%d] unknown image cmd: %v", sid, cm.ImageCmd)
		}
	case *connectivity.Cmd_PodCmd:
		switch cm.PodCmd.GetAction() {
		// pod scope commands
		case connectivity.CreatePod:
			processInNewGoroutine(sid, "pod.create", func() {
				b.doPodCreate(sid, cm.PodCmd.GetCreateOptions())
			})
		case connectivity.DeletePod:
			processInNewGoroutine(sid, "pod.delete", func() {
				b.doPodDelete(sid, cm.PodCmd.GetDeleteOptions())
			})
		case connectivity.ListPods:
			processInNewGoroutine(sid, "pod.list", func() {
				b.doPodList(sid, cm.PodCmd.GetListOptions())
			})
		case connectivity.PortForward:
			inputCh := make(chan []byte, 1)
			b.openedStreams.add(sid, inputCh, nil)

			processInNewGoroutine(sid, "pod.portforward", func() {
				b.doPortForward(sid, cm.PodCmd.GetPortForwardOptions(), inputCh)
			})
		// container scope commands
		case connectivity.Exec:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			b.openedStreams.add(sid, inputCh, resizeCh)

			processInNewGoroutine(sid, "pod.exec", func() {
				b.doContainerExec(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
			})
		case connectivity.Attach:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			b.openedStreams.add(sid, inputCh, resizeCh)

			processInNewGoroutine(sid, "pod.attach", func() {
				b.doContainerAttach(sid, cm.PodCmd.GetExecOptions(), inputCh, resizeCh)
			})
		case connectivity.Log:
			processInNewGoroutine(sid, "pod.log", func() {
				b.doContainerLog(sid, cm.PodCmd.GetLogOptions())
			})
		case connectivity.Input:
			inputCh, ok := b.openedStreams.getInputChan(sid)
			if !ok {
				b.handleError(sid, ErrStreamSessionClosed)
				return
			}

			processInNewGoroutine(sid, "pod.input", func() {
				select {
				case inputCh <- cm.PodCmd.GetInputOptions().GetData():
				case <-b.ctx.Done():
				}
			})
		case connectivity.ResizeTty:
			resizeCh, ok := b.openedStreams.getResizeChan(sid)
			if !ok {
				b.handleError(sid, ErrStreamSessionClosed)
				return
			}

			processInNewGoroutine(sid, "pod.resizeTty", func() {
				size := remotecommand.TerminalSize{
					Width:  uint16(cm.PodCmd.GetResizeOptions().GetCols()),
					Height: uint16(cm.PodCmd.GetResizeOptions().GetRows()),
				}
				select {
				case resizeCh <- size:
				case <-b.ctx.Done():
				}
			})
		default:
			log.Printf("[%d] unknown pod cmd: %v", sid, cm.PodCmd)
		}
	default:
		log.Printf("[%d] unknown cmd: %v", sid, cm)
	}
}

func processInNewGoroutine(sid uint64, cmdName string, process func()) {
	go func() {
		log.Printf("[%d] trying to handle %s", sid, cmdName)
		defer log.Printf("[%d] finished handling %s", sid, cmdName)

		process()
	}()
}

// Internal processing

func (b *baseAgent) handleError(sid uint64, e error) {
	log.Printf("[%d] error: %v", sid, e)
	if err := b.doPostMsg(connectivity.NewErrorMsg(sid, e)); err != nil {
		log.Printf("failed to post error msg: %v", err)
	}
}
