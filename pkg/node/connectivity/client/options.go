package client

func WithPodCreateHandler(f PodCreateHandler) Option {
	return func(c *baseClient) error {
		c.doPodCreate = f
		return nil
	}
}

func WithPodDeleteHandler(f PodDeleteHandler) Option {
	return func(c *baseClient) error {
		c.doPodDelete = f
		return nil
	}
}

func WithPodListHandler(f PodListHandler) Option {
	return func(c *baseClient) error {
		c.doPodList = f
		return nil
	}
}

func WithPortForwardHandler(f PortForwardHandler) Option {
	return func(c *baseClient) error {
		c.doPodPortForward = f
		return nil
	}
}

func WithContainerLogHandler(f ContainerLogHandler) Option {
	return func(c *baseClient) error {
		c.doContainerLog = f
		return nil
	}
}

func WithContainerExecHandler(f ContainerExecHandler) Option {
	return func(c *baseClient) error {
		c.doContainerExec = f
		return nil
	}
}

func WithContainerAttachHandler(f ContainerAttachHandler) Option {
	return func(c *baseClient) error {
		c.doContainerAttach = f
		return nil
	}
}

func WithContainerInputHandler(f ContainerInputHandler) Option {
	return func(c *baseClient) error {
		c.doContainerInput = f
		return nil
	}
}

func WithContainerTtyResizeHandler(f ContainerTtyResizeHandler) Option {
	return func(c *baseClient) error {
		c.doContainerTtyResize = f
		return nil
	}
}
