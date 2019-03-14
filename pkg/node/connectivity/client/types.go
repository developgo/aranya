package client

import (
	"errors"
	"sync"

	"arhat.dev/aranya/pkg/node/connectivity"
)

var (
	ErrClientAlreadyConnected = errors.New("client already connected ")
	ErrClientNotConnected     = errors.New("client not connected ")
)

type (
	PodCreateHandler          func(sid uint64, namespace, name string, options *connectivity.CreateOptions) (pod *connectivity.Pod, err error)
	PodDeleteHandler          func(sid uint64, namespace, name string, options *connectivity.DeleteOptions) (pod *connectivity.Pod, err error)
	PodListHandler            func(sid uint64, namespace, name string, options *connectivity.ListOptions) (pods []*connectivity.Pod, err error)
	PortForwardHandler        func(sid uint64, namespace, name string, options *connectivity.PortForwardOptions)
	ContainerLogHandler       func(sid uint64, namespace, name string, options *connectivity.LogOptions)
	ContainerExecHandler      func(sid uint64, namespace, name string, options *connectivity.ExecOptions)
	ContainerAttachHandler    func(sid uint64, namespace, name string, options *connectivity.ExecOptions)
	ContainerInputHandler     func(sid uint64, options *connectivity.InputOptions)
	ContainerTtyResizeHandler func(sid uint64, options *connectivity.TtyResizeOptions)
)

type Option func(*baseClient) error

type baseClient struct {
	podCreateHandler          PodCreateHandler
	podDeleteHandler          PodDeleteHandler
	podListHandler            PodListHandler
	podPortForwardHandler     PortForwardHandler
	containerLogHandler       ContainerLogHandler
	containerExecHandler      ContainerExecHandler
	containerAttachHandler    ContainerAttachHandler
	containerInputHandler     ContainerInputHandler
	containerTtyResizeHandler ContainerTtyResizeHandler

	postMsgFunc func(msg *connectivity.Msg) error
	mu          sync.RWMutex
}

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

func (c *baseClient) onPostMsg(m *connectivity.Msg, sendMsg func(msg *connectivity.Msg) error) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return sendMsg(m)
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
		case connectivity.PodCmd_Create:
			c.podCreate(sid, ns, name, cm.PodCmd.GetCreateOptions())
		case connectivity.PodCmd_Delete:
			c.podDelete(sid, ns, name, cm.PodCmd.GetDeleteOptions())
		case connectivity.PodCmd_List:
			c.podList(sid, ns, name, cm.PodCmd.GetListOptions())
		case connectivity.PodCmd_PortForward:
			c.podPortForward(sid, ns, name, cm.PodCmd.GetPortForwardOptions())

		// container scope commands
		case connectivity.PodCmd_Exec:
			c.containerExec(sid, ns, name, cm.PodCmd.GetExecOptions())
		case connectivity.PodCmd_Attach:
			c.containerAttachHandler(sid, ns, name, cm.PodCmd.GetExecOptions())
		case connectivity.PodCmd_Log:
			c.containerLog(sid, ns, name, cm.PodCmd.GetLogOptions())
		case connectivity.PodCmd_Input:
			c.containerInput(sid, cm.PodCmd.GetInputOptions())
		case connectivity.PodCmd_ResizeTty:
			c.containerTtyResize(sid, cm.PodCmd.GetResizeOptions())
		}
	}
}

func (c *baseClient) sendError(sid uint64, e error) {
	if err := c.postMsgFunc(NewErrorMsg(sid, e)); err != nil {
		// TODO: log error
	}
}

func (c *baseClient) podCreate(sid uint64, namespace, name string, options *connectivity.CreateOptions) {
	if c.podCreateHandler != nil {
		podResp, err := c.podCreateHandler(sid, namespace, name, options)
		if err != nil {
			c.sendError(sid, err)
			return
		}

		if err := c.postMsgFunc(NewPodMsg(sid, true, podResp)); err != nil {
			// TODO: log error
		}
	}
}

func (c *baseClient) podDelete(sid uint64, namespace, name string, options *connectivity.DeleteOptions) {
	if c.podDeleteHandler != nil {
		podDeleted, err := c.podDeleteHandler(sid, namespace, name, options)
		if err != nil {
			c.sendError(sid, err)
			return
		}

		if err := c.postMsgFunc(NewPodMsg(sid, true, podDeleted)); err != nil {
			// TODO: log error
		}
	}
}

func (c *baseClient) podList(sid uint64, namespace, name string, options *connectivity.ListOptions) {
	if c.podListHandler != nil {
		pods, err := c.podListHandler(sid, namespace, name, options)
		if err != nil {
			c.sendError(sid, err)
			return
		}

		size := len(pods)
		for i, pod := range pods {
			if err := c.postMsgFunc(NewPodMsg(sid, i == size-1, pod)); err != nil {
				// TODO: log error
			}
		}
	}
}

func (c *baseClient) podPortForward(sid uint64, namespace, name string, options *connectivity.PortForwardOptions) {
	if c.podPortForwardHandler != nil {
		c.podPortForwardHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerLog(sid uint64, namespace, name string, options *connectivity.LogOptions) {
	if c.containerLogHandler != nil {
		c.containerLogHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerExec(sid uint64, namespace, name string, options *connectivity.ExecOptions) {
	if c.containerExecHandler != nil {
		c.containerExecHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerAttach(sid uint64, namespace, name string, options *connectivity.ExecOptions) {
	if c.containerAttachHandler != nil {
		c.containerAttachHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerInput(sid uint64, options *connectivity.InputOptions) {
	if c.containerInputHandler != nil {
		c.containerInputHandler(sid, options)
	}
}

func (c *baseClient) containerTtyResize(sid uint64, options *connectivity.TtyResizeOptions) {
	if c.containerTtyResizeHandler != nil {
		c.containerTtyResizeHandler(sid, options)
	}
}
