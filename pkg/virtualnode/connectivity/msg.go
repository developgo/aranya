package connectivity

import (
	corev1 "k8s.io/api/core/v1"
)

func newMsg(sid uint64, completed bool, m isMsg_Msg) *Msg {
	return &Msg{SessionId: sid, Completed: completed, Msg: m}
}

func NewNodeMsg(
	sid uint64,
	systemInfo *corev1.NodeSystemInfo,
	capacity, allocatable corev1.ResourceList,
	conditions []corev1.NodeCondition,
) *Msg {
	var (
		systemInfoBytes     []byte
		conditionBytes      [][]byte
		capacityBytesMap    = make(map[string][]byte)
		allocatableBytesMap = make(map[string][]byte)
	)

	if systemInfo != nil {
		systemInfoBytes, _ = systemInfo.Marshal()
	}

	for name, quantity := range capacity {
		capacityBytesMap[string(name)], _ = quantity.Marshal()
	}

	for name, quantity := range allocatable {
		allocatableBytesMap[string(name)], _ = quantity.Marshal()
	}

	for _, cond := range conditions {
		condBytes, _ := cond.Marshal()
		conditionBytes = append(conditionBytes, condBytes)
	}

	return newMsg(sid, true,
		&Msg_NodeStatus{
			NodeStatus: &NodeStatus{
				SystemInfo: systemInfoBytes,
				Resources: &NodeStatus_Resource{
					Capacity:    capacityBytesMap,
					Allocatable: allocatableBytesMap,
				},
				Conditions: &NodeStatus_Condition{
					Conditions: conditionBytes,
				},
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
