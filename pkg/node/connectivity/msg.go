package connectivity

import (
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func NewNodeMsg(sid uint64, completed bool, node corev1.Node) *Msg {
	nodeBytes, _ := node.Marshal()

	return &Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &Msg_Node{
			Node: &Node{
				Node: &Node_NodeV1{
					NodeV1: nodeBytes,
				},
			},
		},
	}
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

func NewPod(namespace, name string, podStatus *criRuntime.PodSandboxStatus, containerStatuses []*criRuntime.ContainerStatus) *Pod {
	podStatusBytes, _ := podStatus.Marshal()

	containerStatusBytes := make([][]byte, len(containerStatuses))
	for i, containerStatus := range containerStatuses {
		containerStatusBytes[i], _ = containerStatus.Marshal()
	}

	return &Pod{
		Namespace: namespace,
		Name:      name,
		Ip:        podStatus.GetNetwork().GetIp(),

		ContainerStatus: &Pod_ContainerV1Alpha2{
			ContainerV1Alpha2: &Pod_ContainerStatusV1Alpha2{
				V1Alpha2: containerStatusBytes,
			},
		},
		SandboxStatus: &Pod_SandboxV1Alpha2{
			SandboxV1Alpha2: podStatusBytes,
		},
	}
}

func NewPodMsg(sid uint64, completed bool, pod *Pod) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &Msg_Pod{
			Pod: pod,
		},
	}
}

func NewAckSha256Msg(sid uint64, receivedData []byte) *Msg {
	h := sha256.New()
	h.Write(receivedData)
	hash := hex.EncodeToString(h.Sum(nil))

	return &Msg{
		SessionId: sid,
		Completed: true,
		Msg: &Msg_Ack{
			Ack: &Ack{
				Value: &Ack_Hash_{
					Hash: &Ack_Hash{
						Hash: &Ack_Hash_Sha256{
							Sha256: hash,
						},
					},
				},
			},
		},
	}
}

func NewErrorMsg(sid uint64, err error) *Msg {
	return &Msg{
		SessionId: sid,
		Completed: true,
		Msg: &Msg_Ack{
			Ack: &Ack{
				Value: &Ack_Error{
					Error: err.Error(),
				},
			},
		},
	}
}
