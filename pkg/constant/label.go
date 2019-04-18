package constant

const (
	LabelRole = "arhat.dev/role"
)

const (
	LabelRoleValueEdgeDevice = "EdgeDevice"
	LabelRoleValueService    = "Service"
	LabelRoleValueController = "Controller"
)

const (
	LabelNamespace = "arhat.dev/namespace"
	LabelName      = "arhat.dev/name"
)

// labels used by container runtime
const (
	ContainerLabelPodNamespace     = "container.arhat.dev/pod-namespace"
	ContainerLabelPodName          = "container.arhat.dev/pod-name"
	ContainerLabelPodUID           = "container.arhat.dev/pod-uid"
	ContainerLabelPodContainer     = "container.arhat.dev/pod-container"
	ContainerLabelPodContainerRole = "container.arhat.dev/pod-container-role"
)

const (
	ContainerRoleInfra = "infra"
	ContainerRoleWork  = "work"
)
