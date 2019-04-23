package agent

import (
	"context"
	"errors"
	"log"
	"sync"

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
			streamHandlers: make(map[uint64]*streamHandler),
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
	case *connectivity.Cmd_CloseSession:
		b.openedStreams.del(cm.CloseSession)
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
			h := newStreamRW(b.ctx)
			b.openedStreams.add(sid, h)
			processInNewGoroutine(sid, "pod.portforward", func() {
				b.doPortForward(sid, cm.Pod.GetPortForwardOptions(), h.r)
			})
		case connectivity.Exec:
			h := newStreamRW(b.ctx)
			b.openedStreams.add(sid, h)
			processInNewGoroutine(sid, "pod.exec", func() {
				b.doContainerExec(sid, cm.Pod.GetExecOptions(), h.r, h.sCh)
			})
		case connectivity.Attach:
			h := newStreamRW(b.ctx)
			b.openedStreams.add(sid, h)
			processInNewGoroutine(sid, "pod.attach", func() {
				b.doContainerAttach(sid, cm.Pod.GetExecOptions(), h.r, h.sCh)
			})
		case connectivity.Log:
			processInNewGoroutine(sid, "pod.log", func() {
				b.doContainerLog(sid, cm.Pod.GetLogOptions())
			})
		case connectivity.Input:
			h, ok := b.openedStreams.getStreamHandler(sid)
			if !ok {
				b.handleRuntimeError(sid, ErrStreamSessionClosed)
				return
			}
			h.write(cm.Pod.GetInputOptions().GetData())
		case connectivity.ResizeTty:
			h, ok := b.openedStreams.getStreamHandler(sid)
			if !ok {
				b.handleRuntimeError(sid, ErrStreamSessionClosed)
				return
			}
			h.resize(cm.Pod.GetResizeOptions())
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
	if err == nil {
		return
	}

	log.Printf("[%d] E runtime error: %v", sid, err)

	if err := b.doPostMsg(connectivity.NewErrorMsg(sid, err)); err != nil {
		b.handleConnectivityError(sid, err)
	}
}

func (b *baseAgent) handleConnectivityError(sid uint64, err error) {
	log.Printf("[%d] E connectivity error: %v", sid, err)
}
