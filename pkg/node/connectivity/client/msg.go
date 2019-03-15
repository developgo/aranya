package client

import (
	"crypto/sha256"
	"encoding/hex"

	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewNodeMsg(sid uint64, completed bool, node corev1.Node) *connectivity.Msg {
	nodeBytes, _ := node.Marshal()

	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_Node{
			Node: &connectivity.Node{
				Node: &connectivity.Node_NodeV1{
					NodeV1: nodeBytes,
				},
			},
		},
	}
}

func NewDataMsg(sid uint64, completed bool, kind connectivity.Data_Kind, data []byte) *connectivity.Msg {
	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_Data{
			Data: &connectivity.Data{
				Kind: kind,
				Data: data,
			},
		},
	}
}

func NewPod(pod corev1.Pod, ip string, podStatuses []*criRuntime.PodSandboxStatus, containerStatuses []*criRuntime.ContainerStatus) *connectivity.Pod {
	podStatusBytes := make([][]byte, len(podStatuses))
	for i, podStatus := range podStatuses {
		podStatusBytes[i], _ = podStatus.Marshal()
	}

	containerStatusBytes := make([][]byte, len(containerStatuses))
	for i, containerStatus := range containerStatuses {
		containerStatusBytes[i], _ = containerStatus.Marshal()
	}

	return &connectivity.Pod{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Uid:       string(pod.UID),
		Ip:        ip,

		ContainerStatus: &connectivity.Pod_ContainerStatusV1Alpha2_{
			ContainerStatusV1Alpha2: &connectivity.Pod_ContainerStatusV1Alpha2{
				V1Alpha2: containerStatusBytes,
			},
		},
		SandboxStatus: &connectivity.Pod_SandboxStatusV1Alpha2_{
			SandboxStatusV1Alpha2: &connectivity.Pod_SandboxStatusV1Alpha2{
				V1Alpha2: podStatusBytes,
			},
		},
	}
}

func NewPodMsg(sid uint64, completed bool, pod *connectivity.Pod) *connectivity.Msg {
	return &connectivity.Msg{
		SessionId: sid,
		Completed: completed,
		Msg: &connectivity.Msg_Pod{
			Pod: pod,
		},
	}
}

func NewAckSha256Msg(sid uint64, receivedData []byte) *connectivity.Msg {
	h := sha256.New()
	h.Write(receivedData)
	hash := hex.EncodeToString(h.Sum(nil))

	return &connectivity.Msg{
		SessionId: sid,
		Completed: true,
		Msg: &connectivity.Msg_Ack{
			Ack: &connectivity.Ack{
				Value: &connectivity.Ack_Hash_{
					Hash: &connectivity.Ack_Hash{
						Hash: &connectivity.Ack_Hash_Sha256{
							Sha256: hash,
						},
					},
				},
			},
		},
	}
}

func NewErrorMsg(sid uint64, err error) *connectivity.Msg {
	return &connectivity.Msg{
		SessionId: sid,
		Completed: true,
		Msg: &connectivity.Msg_Ack{
			Ack: &connectivity.Ack{
				Value: &connectivity.Ack_Error{
					Error: err.Error(),
				},
			},
		},
	}
}
