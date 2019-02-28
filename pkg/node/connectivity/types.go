package connectivity

import (
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrDeviceNotConnected = errors.New("device not connected ")
)

func NewPodCreateOrUpdateCmd(pod *corev1.Pod) *Cmd {
	podSpecBytes, _ := pod.Status.Marshal()

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
				Action: PodCmd_CreateOrUpdate,
				Options: &PodCmd_CreateOptions{
					CreateOptions: &PodCreateOptions{
						Spec: podSpecBytes,
					},
				},
			},
		},
	}
}

func NewPodDeleteCmd(namespace, name string, graceTime time.Duration) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_Delete,
				Options: &PodCmd_DeleteOptions{
					DeleteOptions: &PodDeleteOptions{
						GraceTime: int64(graceTime),
					},
				},
			},
		},
	}
}

func NewPodListCmd(namespace, name string) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_List,
				Options: &PodCmd_ListOptions{
					ListOptions: &PodListOptions{
					},
				},
			},
		},
	}
}
