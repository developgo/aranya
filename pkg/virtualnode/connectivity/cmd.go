package connectivity

import (
	"time"

	corev1 "k8s.io/api/core/v1"
)

func NewNodeCmd(action NodeCmd_Action) *Cmd {
	return &Cmd{
		Cmd: &Cmd_Node{
			Node: &NodeCmd{
				Action: action,
			},
		},
	}
}

func newPodCmd(sid uint64, action PodCmd_Action, options isPodCmd_Options) *Cmd {
	return &Cmd{
		SessionId: sid,
		Cmd: &Cmd_Pod{
			Pod: &PodCmd{
				Action:  action,
				Options: options,
			},
		},
	}
}

func NewPodCreateCmd(
	pod *corev1.Pod,
	imagePullSecrets map[string]*AuthConfig,
	containerEnvs map[string]map[string]string,
	volumeData map[string]*NamedData,
	hostVolume map[string]string,
) *Cmd {
	containers := make(map[string]*ContainerSpec)
	for _, ctr := range pod.Spec.Containers {
		containers[ctr.Name] = &ContainerSpec{
			Image: ctr.Image,
			ImagePullPolicy: func() ImagePullPolicy {
				switch ctr.ImagePullPolicy {
				case corev1.PullNever:
					return ImagePullNever
				case corev1.PullIfNotPresent:
					return ImagePullIfNotPresent
				case corev1.PullAlways:
					return ImagePullAlways
				default:
					return ImagePullNever
				}
			}(),
			Command:    ctr.Command,
			Args:       ctr.Args,
			WorkingDir: ctr.WorkingDir,
			Stdin:      ctr.Stdin,
			Tty:        ctr.TTY,

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
			ImagePullAuthConfig: imagePullSecrets,
			VolumeData:          volumeData,
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

func NewContainerExecCmd(podUID, container string, command []string, stdin, stdout, stderr, tty bool) *Cmd {
	return newPodCmd(0, Exec, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Command:   command,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
		},
	})
}

func NewContainerAttachCmd(podUID, container string, stdin, stdout, stderr, tty bool) *Cmd {
	return newPodCmd(0, Attach, &PodCmd_ExecOptions{
		ExecOptions: &ExecOptions{
			PodUid:    podUID,
			Container: container,
			Stdin:     stdin,
			Stderr:    stderr,
			Stdout:    stdout,
			Tty:       tty,
		},
	})
}

func NewContainerLogCmd(podUID, container string, follow, timestamp bool, since time.Time, tailLines int64) *Cmd {
	return newPodCmd(0, Log, &PodCmd_LogOptions{
		LogOptions: &LogOptions{
			PodUid:    podUID,
			Container: container,
			Follow:    follow,
			Timestamp: timestamp,
			Since:     since.UnixNano(),
			TailLines: tailLines,
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

func NewSessionCloseCmd(sessionToClose uint64) *Cmd {
	return &Cmd{
		Cmd: &Cmd_CloseSession{
			CloseSession: sessionToClose,
		},
	}
}
