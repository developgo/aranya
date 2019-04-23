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
	ErrClientAlreadyConnected = errors.New("client already connected")
	ErrClientNotConnected     = errors.New("client not connected")
	ErrStreamSessionClosed    = connectivity.NewCommonError("stream session closed")
)

type Interface interface {
	Start(ctx context.Context) error
	PostMsg(msg *connectivity.Msg) error
}

func newBaseAgent(ctx context.Context, config *Config, rt runtime.Interface) baseAgent {
	return baseAgent{
		Config:  *config,
		ctx:     ctx,
		runtime: rt,
		openedStreams: streamSession{
			inputCh:  make(map[uint64]chan []byte),
			resizeCh: make(map[uint64]chan remotecommand.TerminalSize),
		},
	}
}

type baseAgent struct {
	Config

	ctx       context.Context
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
	case *connectivity.Cmd_Node:
		switch cm.Node.GetAction() {
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
			log.Printf("[%d] unknown node cmd: %v", sid, cm.Node)
		}
	case *connectivity.Cmd_Pod:
		switch cm.Pod.GetAction() {
		// pod scope commands
		case connectivity.CreatePod:
			processInNewGoroutine(sid, "pod.create", func() {
				b.doPodCreate(sid, cm.Pod.GetCreateOptions())
			})
		case connectivity.DeletePod:
			processInNewGoroutine(sid, "pod.delete", func() {
				b.doPodDelete(sid, cm.Pod.GetDeleteOptions())
			})
		case connectivity.ListPods:
			processInNewGoroutine(sid, "pod.list", func() {
				b.doPodList(sid, cm.Pod.GetListOptions())
			})
		case connectivity.PortForward:
			inputCh := make(chan []byte, 1)
			b.openedStreams.add(sid, inputCh, nil)

			processInNewGoroutine(sid, "pod.portforward", func() {
				b.doPortForward(sid, cm.Pod.GetPortForwardOptions(), inputCh)
			})
		case connectivity.Exec:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			b.openedStreams.add(sid, inputCh, resizeCh)

			processInNewGoroutine(sid, "pod.exec", func() {
				b.doContainerExec(sid, cm.Pod.GetExecOptions(), inputCh, resizeCh)
			})
		case connectivity.Attach:
			inputCh := make(chan []byte, 1)
			resizeCh := make(chan remotecommand.TerminalSize, 1)
			b.openedStreams.add(sid, inputCh, resizeCh)

			processInNewGoroutine(sid, "pod.attach", func() {
				b.doContainerAttach(sid, cm.Pod.GetExecOptions(), inputCh, resizeCh)
			})
		case connectivity.Log:
			processInNewGoroutine(sid, "pod.log", func() {
				b.doContainerLog(sid, cm.Pod.GetLogOptions())
			})
		case connectivity.Input:
			inputCh, ok := b.openedStreams.getInputChan(sid)
			if !ok {
				b.handleRuntimeError(sid, ErrStreamSessionClosed)
				return
			}

			processInNewGoroutine(sid, "pod.input", func() {
				select {
				case inputCh <- cm.Pod.GetInputOptions().GetData():
				case <-b.ctx.Done():
				}
			})
		case connectivity.ResizeTty:
			resizeCh, ok := b.openedStreams.getResizeChan(sid)
			if !ok {
				b.handleRuntimeError(sid, ErrStreamSessionClosed)
				return
			}

			processInNewGoroutine(sid, "pod.resizeTty", func() {
				size := remotecommand.TerminalSize{
					Width:  uint16(cm.Pod.GetResizeOptions().GetCols()),
					Height: uint16(cm.Pod.GetResizeOptions().GetRows()),
				}
				select {
				case resizeCh <- size:
				case <-b.ctx.Done():
				}
			})
		default:
			log.Printf("[%d] unknown pod cmd: %v", sid, cm.Pod)
		}
	default:
		log.Printf("[%d] unknown cmd: %v", sid, cm)
	}
}

func processInNewGoroutine(sid uint64, cmdName string, process func()) {
	go func() {
		log.Printf("[%d] I DO %s", sid, cmdName)
		defer log.Printf("[%d] I FIN %s", sid, cmdName)

		process()
	}()
}

func (b *baseAgent) handleRuntimeError(sid uint64, err *connectivity.Error) {
	log.Printf("[%d] E runtime error: %v", sid, err)

	if err := b.doPostMsg(connectivity.NewErrorMsg(sid, err)); err != nil {
		b.handleConnectivityError(sid, err)
	}
}

func (b *baseAgent) handleConnectivityError(sid uint64, err error) {
	log.Printf("[%d] E connectivity error: %v", sid, err)
}
