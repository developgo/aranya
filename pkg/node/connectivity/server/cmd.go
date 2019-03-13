package server

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewPodCreateOrUpdateCmd(namespace, name string, podSpec corev1.PodSpec) *connectivity.Cmd {
	podSpecBytes, _ := podSpec.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_CreateOrUpdate,
				Options: &connectivity.PodCmd_CreateOptions{
					CreateOptions: &connectivity.CreateOptions{
						PodSpec: &connectivity.CreateOptions_PodSpecV1{
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
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_Delete,
				Options: &connectivity.PodCmd_DeleteOptions{
					DeleteOptions: &connectivity.DeleteOptions{
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
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_List,
				Options: &connectivity.PodCmd_ListOptions{
					ListOptions: &connectivity.ListOptions{},
				},
			},
		},
	}
}

func NewContainerExecCmd(namespace, name string, options corev1.PodExecOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_Exec,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.ExecOptions{
						ExecOptions: &connectivity.ExecOptions_ExecOptionsV1{
							ExecOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerAttachCmd(namespace, name string, options corev1.PodExecOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_Attach,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.ExecOptions{
						ExecOptions: &connectivity.ExecOptions_ExecOptionsV1{
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
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_Log,
				Options: &connectivity.PodCmd_LogOptions{
					LogOptions: &connectivity.LogOptions{
						LogOptions: &connectivity.LogOptions_LogOptionsV1{
							LogOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewPortForwardCmd(namespace, name string, options corev1.PodPortForwardOptions) *connectivity.Cmd {
	optionBytes, _ := options.Marshal()

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    connectivity.PodCmd_PortForward,
				Options: &connectivity.PodCmd_PortForwardOptions{
					PortForwardOptions: &connectivity.PortForwardOptions{
						PortForwardOptions: &connectivity.PortForwardOptions_PortforwardOptionsV1{
							PortforwardOptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerInputCmd(sid uint64, data []byte) *connectivity.Cmd {
	return &connectivity.Cmd{
		SessionId: sid,
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Action: connectivity.PodCmd_Input,
				Options: &connectivity.PodCmd_InputOptions{
					InputOptions: &connectivity.InputOptions{
						Data:      data,
					},
				},
			},
		},
	}
}

func NewContainerTtyResizeCmd(sid uint64, cols uint16, rows uint16) *connectivity.Cmd {
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
