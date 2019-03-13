package client

import (
	"crypto/sha256"
	"encoding/hex"
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
				Node: &connectivity.NodeInfo_NodeV1{
					NodeV1: nodeBytes,
				},
			},
		},
	}
}

func NewPodDataMsg(sid uint64, completed bool, kind connectivity.Data_Kind, data []byte) *connectivity.Msg {
	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_PodData{
			PodData: &connectivity.Data{
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
				Pod: &connectivity.PodInfo_PodV1{
					PodV1: podBytes,
				},
			},
		},
	}
}

func NewAckSha256Msg(sid uint64, receivedData []byte) *connectivity.Msg {
	h := sha256.New()
	h.Write(receivedData)
	hash := hex.EncodeToString(h.Sum(nil))

	return &connectivity.Msg{
		SessionId: sid,
		Completed: false,
		Msg: &connectivity.Msg_Ack{
			Ack: &connectivity.Ack{
				Hash: &connectivity.Ack_Sha256{
					Sha256: hash,
				},
			},
		},
	}
}
