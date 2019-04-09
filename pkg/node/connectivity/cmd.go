package connectivity

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func NewNodeCmd() *Cmd {
	return &Cmd{
		Cmd: &Cmd_NodeCmd{
			NodeCmd: &NodeCmd{},
		},
	}
}

func NewPodCreateCmd(
	pod *corev1.Pod,
	imagePullSecrets map[string]*criRuntime.AuthConfig,
	containerEnvs map[string]map[string]string,
	volumeData map[string]map[string][]byte,
	hostVolume map[string]string,
) *Cmd {
	authConfigBytes := make(map[string][]byte, len(imagePullSecrets))
	for name, authConf := range imagePullSecrets {
		authConfigBytes[name], _ = authConf.Marshal()
	}

	actualVolumeData := make(map[string]*NamedData)
	for k, namedVolumeData := range volumeData {
		actualVolumeData[k] = &NamedData{Data: namedVolumeData}
	}

	containers := make(map[string]*ContainerSpec)
	for _, ctr := range pod.Spec.Containers {
		containers[ctr.Name] = &ContainerSpec{
			Image:           ctr.Image,
			ImagePullPolicy: string(ctr.ImagePullPolicy),
			Command:         ctr.Command,
			Args:            ctr.Args,
			WorkingDir:      ctr.WorkingDir,
			Stdin:           ctr.Stdin,
			Tty:             ctr.TTY,

			Ports: func() map[string]*ContainerPort {
				m := make(map[string]*ContainerPort)
				for _, p := range ctr.Ports {
					m[p.Name] = &ContainerPort{
						Protocol:      string(p.Protocol),
						HostPort:      p.HostPort,
						ContainerPort: p.ContainerPort,
						HostIp:        p.HostIP,
					}
				}
				return m
			}(),
			Envs: containerEnvs[ctr.Name],
		}
	}

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Action:    Create,
				Options: &PodCmd_CreateOptions{
					CreateOptions: &CreateOptions{
						Containers:          containers,
						ImagePullAuthConfig: authConfigBytes,
						VolumeData:          actualVolumeData,
						HostVolumes:         hostVolume,
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
				Namespace: namespace,
				Name:      name,
				Action:    Delete,
				Options: &PodCmd_DeleteOptions{
					DeleteOptions: &DeleteOptions{
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
				Namespace: namespace,
				Name:      name,
				Action:    List,
				Options: &PodCmd_ListOptions{
					ListOptions: &ListOptions{},
				},
			},
		},
	}
}

func NewContainerExecCmd(namespace, name string, options corev1.PodExecOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    Exec,
				Options: &PodCmd_ExecOptions{
					ExecOptions: &ExecOptions{
						Options: &ExecOptions_OptionsV1{
							OptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerAttachCmd(namespace, name string, options corev1.PodExecOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    Attach,
				Options: &PodCmd_ExecOptions{
					ExecOptions: &ExecOptions{
						Options: &ExecOptions_OptionsV1{
							OptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerLogCmd(namespace, name string, options corev1.PodLogOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    Log,
				Options: &PodCmd_LogOptions{
					LogOptions: &LogOptions{
						Options: &LogOptions_OptionsV1{
							OptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewPortForwardCmd(namespace, name string, options corev1.PodPortForwardOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return &Cmd{
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Namespace: namespace,
				Name:      name,
				Action:    PortForward,
				Options: &PodCmd_PortForwardOptions{
					PortForwardOptions: &PortForwardOptions{
						Options: &PortForwardOptions_OptionsV1{
							OptionsV1: optionBytes,
						},
					},
				},
			},
		},
	}
}

func NewContainerInputCmd(sid uint64, data []byte) *Cmd {
	return &Cmd{
		SessionId: sid,
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Action: Input,
				Options: &PodCmd_InputOptions{
					InputOptions: &InputOptions{
						Data: data,
					},
				},
			},
		},
	}
}

func NewContainerTtyResizeCmd(sid uint64, cols uint16, rows uint16) *Cmd {
	return &Cmd{
		SessionId: sid,
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Action: ResizeTty,
				Options: &PodCmd_ResizeOptions{
					ResizeOptions: &TtyResizeOptions{
						Cols: uint32(cols),
						Rows: uint32(rows),
					},
				},
			},
		},
	}
}
