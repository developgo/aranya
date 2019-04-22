package connectivity

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func newNodeCmd(action NodeCmd_NodeAction) *Cmd {
	return &Cmd{
		Cmd: &Cmd_NodeCmd{
			NodeCmd: &NodeCmd{
				Action: action,
			},
		},
	}
}

func NewNodeGetInfoAllCmd() *Cmd {
	return newNodeCmd(GetInfoAll)
}

func NewNodeGetSystemInfoCmd() *Cmd {
	return newNodeCmd(GetSystemInfo)
}

func NewNodeGetConditionsCmd() *Cmd {
	return newNodeCmd(GetConditions)
}

func NewNodeGetResourcesCmd() *Cmd {
	return newNodeCmd(GetResources)
}

func newImageCmd(action ImageCmd_ImageAction) *Cmd {
	return &Cmd{
		Cmd: &Cmd_ImageCmd{
			ImageCmd: &ImageCmd{
				Action: action,
			},
		},
	}
}

func NewImageListCmd() *Cmd {
	return newImageCmd(ListImages)
}

func newPodCmd(sid uint64, action PodCmd_PodAction, options isPodCmd_Options) *Cmd {
	return &Cmd{
		SessionId: sid,
		Cmd: &Cmd_PodCmd{
			PodCmd: &PodCmd{
				Action:  action,
				Options: options,
			},
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

	return newPodCmd(0, CreatePod, &PodCmd_CreateOptions{
		CreateOptions: &CreateOptions{
			PodUid:              string(pod.UID),
			Namespace:           pod.Namespace,
			Name:                pod.Name,
			Containers:          containers,
			ImagePullAuthConfig: authConfigBytes,
			VolumeData:          actualVolumeData,
			HostVolumes:         hostVolume,

			HostNetwork: pod.Spec.HostNetwork,
			HostIpc:     pod.Spec.HostIPC,
			HostPid:     pod.Spec.HostPID,
			Hostname:    pod.Spec.Hostname,
		},
	})
}

func NewPodDeleteCmd(podUID string, graceTime time.Duration) *Cmd {
	return newPodCmd(0, DeletePod, &PodCmd_DeleteOptions{
		DeleteOptions: &DeleteOptions{
			PodUid:    podUID,
			GraceTime: int64(graceTime),
		},
	})
}

func NewPodListCmd(namespace, name string, all bool) *Cmd {
	return newPodCmd(0, ListPods, &PodCmd_ListOptions{
		ListOptions: &ListOptions{
			Namespace: namespace,
			Name:      name,
			All:       all,
		},
	})
}

func NewContainerExecCmd(podUID string, options corev1.PodExecOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return newPodCmd(0, Exec, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid: podUID,
			Options: &ExecOptions_OptionsV1{
				OptionsV1: optionBytes,
			},
		},
	})
}

func NewContainerAttachCmd(podUID string, options corev1.PodExecOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return newPodCmd(0, Attach, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid: podUID,
			Options: &ExecOptions_OptionsV1{
				OptionsV1: optionBytes,
			},
		},
	})
}

func NewContainerLogCmd(podUID string, options corev1.PodLogOptions) *Cmd {
	optionBytes, _ := options.Marshal()

	return newPodCmd(0, Log, &PodCmd_LogOptions{
		LogOptions: &LogOptions{
			PodUid: podUID,
			Options: &LogOptions_OptionsV1{
				OptionsV1: optionBytes,
			},
		},
	})
}

func NewPortForwardCmd(podUID string, port int32, protocol string) *Cmd {
	return newPodCmd(0, PortForward, &PodCmd_PortForwardOptions{
		PortForwardOptions: &PortForwardOptions{
			PodUid:   podUID,
			Port:     port,
			Protocol: protocol,
		},
	})
}

func NewContainerInputCmd(sid uint64, data []byte) *Cmd {
	return newPodCmd(sid, Input, &PodCmd_InputOptions{
		InputOptions: &InputOptions{
			Data: data,
		},
	})
}

func NewContainerTtyResizeCmd(sid uint64, cols uint16, rows uint16) *Cmd {
	return newPodCmd(sid, ResizeTty, &PodCmd_ResizeOptions{
		ResizeOptions: &TtyResizeOptions{
			Cols: uint32(cols),
			Rows: uint32(rows),
		},
	})
}
