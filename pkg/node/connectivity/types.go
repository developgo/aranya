package connectivity

import (
	"errors"
	"time"

	corev1 "k8s.io/api/core/v1"
)

var (
	ErrDeviceNotConnected = errors.New("device not connected ")
)

type Interface interface {
	WaitUntilDeviceConnected()
	ConsumeOrphanedMessage() <-chan *Msg
	// send a command to remote device with timeout
	// return a channel of message for this session
	PostCmd(c *Cmd, timeout time.Duration) (ch <-chan *Msg, err error)
}

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
					ListOptions: &PodListOptions{},
				},
			},
		},
	}
}

func NewPodExecCmd(namespace, name string, container string, command []string) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_Exec,
				Options: &PodCmd_ExecOptions{
					ExecOptions: &PodExecOptions{
						ContainerName: container,
						Command:       command,
					},
				},
			},
		},
	}
}

func NewPodAttachCmd(namespace, name string, container string) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_Attach,
				Options: &PodCmd_ExecOptions{
					ExecOptions: &PodExecOptions{
						ContainerName: container,
					},
				},
			},
		},
	}
}

func NewContainerLogCmd(namespace, name string, options *corev1.PodLogOptions) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_Log,
				Options: &PodCmd_LogOptions{
					LogOptions: &PodLogOptions{
						ContainerName: options.Container,
					},
				},
			},
		},
	}
}

func NewPodPortForwardCmd(namespace, name string, port int32) *Cmd {
	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Id: &PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: PodCmd_PortForward,
				Options: &PodCmd_PortForwardOptions{
					PortForwardOptions: &PodPortForwardOptions{
						Port: port,
					},
				},
			},
		},
	}
}
