package client

func WithPodCreateOrUpdateHandler(f PodCreateOrUpdateHandler) Option {
	return func(c *baseClient) error {
		c.podCreateOrUpdateHandler = f
		return nil
	}
}

func WithPodDeleteHandler(f PodDeleteHandler) Option {
	return func(c *baseClient) error {
		c.podDeleteHandler = f
		return nil
	}
}

func WithPodListHandler(f PodListHandler) Option {
	return func(c *baseClient) error {
		c.podListHandler = f
		return nil
	}
}

func WithPortForwardHandler(f PortForwardHandler) Option {
	return func(c *baseClient) error {
		c.podPortForwardHandler = f
		return nil
	}
}

func WithContainerLogHandler(f ContainerLogHandler) Option {
	return func(c *baseClient) error {
		c.containerLogHandler = f
		return nil
	}
}

func WithContainerExecHandler(f ContainerExecHandler) Option {
	return func(c *baseClient) error {
		c.containerExecHandler = f
		return nil
	}
}

func WithContainerAttachHandler(f ContainerAttachHandler) Option {
	return func(c *baseClient) error {
		c.containerAttachHandler = f
		return nil
	}
}

func WithContainerInputHandler(f ContainerInputHandler) Option {
	return func(c *baseClient) error {
		c.containerInputHandler = f
		return nil
	}
}

func WithContainerTtyResizeHandler(f ContainerTtyResizeHandler) Option {
	return func(c *baseClient) error {
		c.containerTtyResizeHandler = f
		return nil
	}
}
