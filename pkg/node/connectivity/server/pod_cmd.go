package server

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewPodCreateOrUpdateCmd(pod *corev1.Pod) *connectivity.Cmd {
	podSpecBytes, _ := pod.Status.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: pod.Namespace,
					Name:      pod.Name,
				},
				Action: connectivity.PodCmd_CreateOrUpdate,
				Options: &connectivity.PodCmd_CreateOptions{
					CreateOptions: &connectivity.PodCreateOptions{
						PodSpec: &connectivity.PodCreateOptions_PodSpecV1{
							PodSpecV1: podSpecBytes,
						},
					},
				},
			},
		},
	}
}

func NewPodDeleteCmd(namespace, name string, graceTime time.Duration) *connectivity.Cmd {
	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_Delete,
				Options: &connectivity.PodCmd_DeleteOptions{
					DeleteOptions: &connectivity.PodDeleteOptions{
						GraceTime: int64(graceTime),
					},
				},
			},
		},
	}
}

func NewPodListCmd(namespace, name string) *connectivity.Cmd {
	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_List,
				Options: &connectivity.PodCmd_ListOptions{
					ListOptions: &connectivity.PodListOptions{},
				},
			},
		},
	}
}

func NewPodExecCmd(namespace, name string, options corev1.PodExecOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_Exec,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.PodExecOptions{
						ExecOptions: &connectivity.PodExecOptions_ExecOptionsV1{
							ExecOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewPodAttachCmd(namespace, name string, options corev1.PodExecOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_Attach,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.PodExecOptions{
						ExecOptions: &connectivity.PodExecOptions_ExecOptionsV1{
							ExecOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerLogCmd(namespace, name string, options corev1.PodLogOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_Log,
				Options: &connectivity.PodCmd_LogOptions{
					LogOptions: &connectivity.PodLogOptions{
						LogOptions: &connectivity.PodLogOptions_LogOptionsV1{
							LogOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewPodPortForwardCmd(namespace, name string, options corev1.PodPortForwardOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Id: &connectivity.PodIdentity{
					Namespace: namespace,
					Name:      name,
				},
				Action: connectivity.PodCmd_PortForward,
				Options: &connectivity.PodCmd_PortForwardOptions{
					PortForwardOptions: &connectivity.PodPortForwardOptions{
						PortforwardOptions: &connectivity.PodPortForwardOptions_PortforwardOptionsV1{
							PortforwardOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewPodDataCmd(sid uint64, completed bool, data []byte) *connectivity.Cmd {
	return &connectivity.Cmd{
		SessionId: sid,
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Action: connectivity.PodCmd_Data,
				Options: &connectivity.PodCmd_DataOptions{
					DataOptions: &connectivity.PodDataOptions{
						Completed: completed,
						Data:      data,
					},
				},
			},
		},
	}
}

func NewPodResizeCmd(sid uint64, cols uint16, rows uint16) *connectivity.Cmd {
	return &connectivity.Cmd{
		SessionId: sid,
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Action: connectivity.PodCmd_ResizeTty,
				Options: &connectivity.PodCmd_ResizeOptions{
					ResizeOptions: &connectivity.TtyResizeOptions{
						Cols: uint32(cols),
						Rows: uint32(rows),
					},
				},
			},
		},
	}
}
