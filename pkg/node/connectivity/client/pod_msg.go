package client

import (
	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewNodeInfoMsg(sid uint64, completed bool, node corev1.Node) *connectivity.Msg {
	nodeBytes, _ := node.Marshal()

	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_NodeInfo{
			NodeInfo: &connectivity.NodeInfo{
				Node: &connectivity.NodeInfo_Nodev1{
					Nodev1: nodeBytes,
				},
			},
		},
	}
}

func NewPodDataMsg(sid uint64, completed bool, kind connectivity.PodData_Kind, data []byte) *connectivity.Msg {
	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_PodData{
			PodData: &connectivity.PodData{
				Kind: kind,
				Data: data,
			},
		},
	}
}

func NewPodInfoMsg(sid uint64, completed bool, pod corev1.Pod) *connectivity.Msg {
	podBytes, _ := pod.Marshal()

	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_PodInfo{
			PodInfo: &connectivity.PodInfo{
				Pod: &connectivity.PodInfo_Podv1{
					Podv1: podBytes,
				},
			},
		},
	}
}
