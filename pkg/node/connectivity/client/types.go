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

type PodCreateOrUpdateHandler func(sid uint64, namespace, name string, options *connectivity.PodCreateOptions)
type PodDeleteHandler func(sid uint64, namespace, name string, options *connectivity.PodDeleteOptions)
type PodListHandler func(sid uint64, namespace, name string, options *connectivity.PodListOptions)
type PodPortForwardHandler func(sid uint64, namespace, name string, options *connectivity.PodPortForwardOptions)

type ContainerLogHandler func(sid uint64, namespace, name string, options *connectivity.PodLogOptions)
type ContainerExecHandler func(sid uint64, namespace, name string, options *connectivity.PodExecOptions)
type ContainerAttachHandler func(sid uint64, namespace, name string, options *connectivity.PodExecOptions)
type ContainerInputHandler func(sid uint64, namespace, name string, options *connectivity.PodDataOptions)
type ContainerTtyResizeHandler func(sid uint64, namespace, name string, options *connectivity.TtyResizeOptions)

type Option func(*baseClient) error

type baseClient struct {
	podCreateOrUpdateHandler PodCreateOrUpdateHandler
	podDeleteHandler         PodDeleteHandler
	podListHandler           PodListHandler
	podPortForwardHandler    PodPortForwardHandler

	containerLogHandler       ContainerLogHandler
	containerExecHandler      ContainerExecHandler
	containerAttachHandler    ContainerAttachHandler
	containerInputHandler     ContainerInputHandler
	containerTtyResizeHandler ContainerTtyResizeHandler

	globalMsgCh chan *connectivity.Msg
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
	switch cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		nodeCmd := cmd.GetNodeCmd()
		_ = nodeCmd
	case *connectivity.Cmd_PodCmd:
		podCmd := cmd.GetPodCmd()

		sid := cmd.GetSessionId()
		ns := podCmd.GetNamespace()
		name := podCmd.GetName()

		switch podCmd.GetAction() {

		// pod scope commands
		case connectivity.PodCmd_CreateOrUpdate:
			c.podCreateOrUpdate(sid, ns, name, podCmd.GetCreateOptions())
		case connectivity.PodCmd_Delete:
			c.podDelete(sid, ns, name, podCmd.GetDeleteOptions())
		case connectivity.PodCmd_List:
			c.podList(sid, ns, name, podCmd.GetListOptions())
		case connectivity.PodCmd_PortForward:
			c.podPortForward(sid, ns, name, podCmd.GetPortForwardOptions())

		// container scope commands
		case connectivity.PodCmd_Exec:
			c.containerExec(sid, ns, name, podCmd.GetExecOptions())
		case connectivity.PodCmd_Attach:
			c.containerAttachHandler(sid, ns, name, podCmd.GetExecOptions())
		case connectivity.PodCmd_Log:
			c.containerLog(sid, ns, name, podCmd.GetLogOptions())
		case connectivity.PodCmd_Data:
			// data for user input, check sid
			c.containerInput(sid, ns, name, podCmd.GetDataOptions())
		case connectivity.PodCmd_ResizeTty:
			c.containerTtyResize(sid, ns, name, podCmd.GetResizeOptions())
		}
	}
}

func (c *baseClient) podCreateOrUpdate(sid uint64, namespace, name string, options *connectivity.PodCreateOptions) {
	if c.podCreateOrUpdateHandler != nil {
		c.podCreateOrUpdateHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) podDelete(sid uint64, namespace, name string, options *connectivity.PodDeleteOptions) {
	if c.podDeleteHandler != nil {
		c.podDeleteHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) podList(sid uint64, namespace, name string, options *connectivity.PodListOptions) {
	if c.podListHandler != nil {
		c.podListHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) podPortForward(sid uint64, namespace, name string, options *connectivity.PodPortForwardOptions) {
	if c.podPortForwardHandler != nil {
		c.podPortForwardHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerLog(sid uint64, namespace, name string, options *connectivity.PodLogOptions) {
	if c.containerLogHandler != nil {
		c.containerLogHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerExec(sid uint64, namespace, name string, options *connectivity.PodExecOptions) {
	if c.containerExecHandler != nil {
		c.containerExecHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerAttach(sid uint64, namespace, name string, options *connectivity.PodExecOptions) {
	if c.containerAttachHandler != nil {
		c.containerAttachHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerInput(sid uint64, namespace, name string, options *connectivity.PodDataOptions) {
	if c.containerInputHandler != nil {
		c.containerInputHandler(sid, namespace, name, options)
	}
}

func (c *baseClient) containerTtyResize(sid uint64, namespace, name string, options *connectivity.TtyResizeOptions) {
	if c.containerTtyResizeHandler != nil {
		c.containerTtyResizeHandler(sid, namespace, name, options)
	}
}
