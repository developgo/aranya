package connectivity

func newMsg(sid uint64, completed bool, m isMsg_Msg) *Msg {
	return &Msg{SessionId: sid, Completed: completed, Msg: m}
}

func NewNodeMsg(
	sid uint64,
	systemInfo *NodeSystemInfo,
	capacity, allocatable *NodeResources,
	conditions *NodeConditions,
) *Msg {
	return newMsg(sid, true,
		&Msg_NodeStatus{
			NodeStatus: &NodeStatus{
				SystemInfo:  systemInfo,
				Capacity:    capacity,
				Allocatable: allocatable,
				Conditions:  conditions,
			},
		},
	)
}

func NewDataMsg(sid uint64, completed bool, kind Data_Kind, data []byte) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &Msg_Data{
			Data: &Data{
				Kind: kind,
				Data: data,
			},
		},
	}
}

func NewPodStatus(podUID string, containerStatus map[string]*PodStatus_ContainerStatus) *PodStatus {
	return &PodStatus{
		Uid:               podUID,
		ContainerStatuses: containerStatus,
	}
}

func NewPodStatusMsg(sid uint64, pod *PodStatus) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: true,
		Msg:       &Msg_PodStatus{PodStatus: pod},
	}
}

func NewPodStatusListMsg(sid uint64, pods []*PodStatus) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: true,
		Msg: &Msg_PodStatusList{
			PodStatusList: &PodStatusList{
				Pods: pods,
			},
		},
	}
}

func newError(kind Error_Kind, description string) *Error {
	return &Error{
		Kind:        kind,
		Description: description,
	}
}

func NewCommonError(description string) *Error {
	return newError(ErrCommon, description)
}

func NewNotFoundError(description string) *Error {
	return newError(ErrNotFound, description)
}

func NewAlreadyExistsError(description string) *Error {
	return newError(ErrAlreadyExists, description)
}

func NewNotSupportedError(description string) *Error {
	return newError(ErrNotSupported, description)
}

func NewErrorMsg(sid uint64, err *Error) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: true,
		Msg: &Msg_Error{
			Error: err,
		},
	}
}
