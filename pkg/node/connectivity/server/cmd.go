package server

import (
	"time"

	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func NewNodeCmd() *connectivity.Cmd {
	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_NodeCmd{
			NodeCmd: &connectivity.NodeCmd{},
		},
	}
}

func NewPodCreateCmd(pod corev1.Pod, pullSecrets []corev1.Secret) *connectivity.Cmd {
	podBytes, _ := pod.Marshal()

	secrets := make([][]byte, len(pullSecrets))
	for i, secret := range pullSecrets {
		secrets[i], _ = secret.Marshal()
	}

	return &connectivity.Cmd{
		Cmd: &connectivity.Cmd_PodCmd{
			PodCmd: &connectivity.PodCmd{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Action:    connectivity.Create,
				Options: &connectivity.PodCmd_CreateOptions{
					CreateOptions: &connectivity.CreateOptions{
						Pod: &connectivity.CreateOptions_PodV1{
							PodV1: &connectivity.PodV1{
								Pod:        podBytes,
								PullSecret: secrets,
							},
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
				Action:    connectivity.Delete,
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
				Action:    connectivity.List,
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
				Action:    connectivity.Exec,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.ExecOptions{
						Options: &connectivity.ExecOptions_OptionsV1{
							OptionsV1: optionBytes,
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
				Action:    connectivity.Attach,
				Options: &connectivity.PodCmd_ExecOptions{
					ExecOptions: &connectivity.ExecOptions{
						Options: &connectivity.ExecOptions_OptionsV1{
							OptionsV1: optionBytes,
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
				Action:    connectivity.Log,
				Options: &connectivity.PodCmd_LogOptions{
					LogOptions: &connectivity.LogOptions{
						Options: &connectivity.LogOptions_OptionsV1{
							OptionsV1: optionBytes,
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
				Action:    connectivity.PortForward,
				Options: &connectivity.PodCmd_PortForwardOptions{
					PortForwardOptions: &connectivity.PortForwardOptions{
						Options: &connectivity.PortForwardOptions_OptionsV1{
							OptionsV1: optionBytes,
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
				Action: connectivity.Input,
				Options: &connectivity.PodCmd_InputOptions{
					InputOptions: &connectivity.InputOptions{
						Data: data,
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
				Action: connectivity.ResizeTty,
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
