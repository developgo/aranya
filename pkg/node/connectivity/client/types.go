package client

import (
	"arhat.dev/aranya/pkg/node/connectivity"
)

type PodCreateOrUpdateHandler func(identity *connectivity.PodIdentity, options *connectivity.PodCreateOptions)
type PodDeleteHandler func(identity *connectivity.PodIdentity, options *connectivity.PodDeleteOptions)
type PodListHandler func(identity *connectivity.PodIdentity, options *connectivity.PodListOptions)
type PodPortForwardHandler func(identity *connectivity.PodIdentity, options *connectivity.PodPortForwardOptions)

type ContainerLogHandler func(identity *connectivity.PodIdentity, options *connectivity.PodLogOptions)
type ContainerExecHandler func(identity *connectivity.PodIdentity, options *connectivity.PodExecOptions)
type ContainerAttachHandler func(identity *connectivity.PodIdentity, options *connectivity.PodExecOptions)
type ContainerInputHandler func(identity *connectivity.PodIdentity, options *connectivity.PodDataOptions)
type ContainerTtyResizeHandler func(identity *connectivity.PodIdentity, options *connectivity.TtyResizeOptions)

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
}

func (c *baseClient) handleCmd(cmd *connectivity.Cmd) {
	switch cmd.GetCmd().(type) {
	case *connectivity.Cmd_NodeCmd:
		nodeCmd := cmd.GetNodeCmd()
		_ = nodeCmd
	case *connectivity.Cmd_PodCmd:
		podCmd := cmd.GetPodCmd()

		switch podCmd.GetAction() {

		// pod scope commands
		case connectivity.PodCmd_CreateOrUpdate:
			c.podCreateOrUpdate(podCmd.GetId(), podCmd.GetCreateOptions())
		case connectivity.PodCmd_Delete:
			c.podDelete(podCmd.GetId(), podCmd.GetDeleteOptions())
		case connectivity.PodCmd_List:
			c.podList(podCmd.GetId(), podCmd.GetListOptions())
		case connectivity.PodCmd_PortForward:
			c.podPortForward(podCmd.GetId(), podCmd.GetPortForwardOptions())

		// container scope commands
		case connectivity.PodCmd_Exec:
			c.containerExec(podCmd.GetId(), podCmd.GetExecOptions())
		case connectivity.PodCmd_Attach:
			c.containerAttachHandler(podCmd.GetId(), podCmd.GetExecOptions())
		case connectivity.PodCmd_Log:
			c.containerLog(podCmd.GetId(), podCmd.GetLogOptions())
		case connectivity.PodCmd_Data:
			// data for user input, check sid
			c.containerInput(podCmd.GetId(), podCmd.GetDataOptions())
		case connectivity.PodCmd_ResizeTty:
			c.containerTtyResize(podCmd.GetId(), podCmd.GetResizeOptions())
		}
	}
}

func (c *baseClient) podCreateOrUpdate(identity *connectivity.PodIdentity, options *connectivity.PodCreateOptions) {
	if c.podCreateOrUpdateHandler != nil {
		c.podCreateOrUpdateHandler(identity, options)
	}
}

func (c *baseClient) podDelete(identity *connectivity.PodIdentity, options *connectivity.PodDeleteOptions) {
	if c.podDeleteHandler != nil {
		c.podDeleteHandler(identity, options)
	}
}

func (c *baseClient) podList(identity *connectivity.PodIdentity, options *connectivity.PodListOptions) {
	if c.podListHandler != nil {
		c.podListHandler(identity, options)
	}
}

func (c *baseClient) podPortForward(identity *connectivity.PodIdentity, options *connectivity.PodPortForwardOptions) {
	if c.podPortForwardHandler != nil {
		c.podPortForwardHandler(identity, options)
	}
}

func (c *baseClient) containerLog(identity *connectivity.PodIdentity, options *connectivity.PodLogOptions) {
	if c.containerLogHandler != nil {
		c.containerLogHandler(identity, options)
	}
}

func (c *baseClient) containerExec(identity *connectivity.PodIdentity, options *connectivity.PodExecOptions) {
	if c.containerExecHandler != nil {
		c.containerExecHandler(identity, options)
	}
}

func (c *baseClient) containerAttach(identity *connectivity.PodIdentity, options *connectivity.PodExecOptions) {
	if c.containerAttachHandler != nil {
		c.containerAttachHandler(identity, options)
	}
}

func (c *baseClient) containerInput(identity *connectivity.PodIdentity, options *connectivity.PodDataOptions) {
	if c.containerInputHandler != nil {
		c.containerInputHandler(identity, options)
	}
}

func (c *baseClient) containerTtyResize(identity *connectivity.PodIdentity, options *connectivity.TtyResizeOptions) {
	if c.containerTtyResizeHandler != nil {
		c.containerTtyResizeHandler(identity, options)
	}
}
