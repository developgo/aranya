package podman

import (
	"fmt"

	libpodRuntime "github.com/containers/libpod/libpod"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func translateLibpodStatusToCriStatus(runtime *libpodRuntime.Runtime, podUID string, pod *libpodRuntime.Pod, infraCtrID string) (*criRuntime.PodSandboxStatus, []*criRuntime.ContainerStatus, error) {
	infraCtr, err := runtime.GetContainer(infraCtrID)
	if err != nil {
		return nil, nil, err
	}

	ips, err := infraCtr.IPs()
	if err != nil {
		return nil, nil, err
	}

	if len(ips) == 0 {
		return nil, nil, fmt.Errorf("no ip address assigned for pod")
	}

	podStatus := &criRuntime.PodSandboxStatus{
		Id: pod.ID(),
		Metadata: &criRuntime.PodSandboxMetadata{
			Uid:     podUID,
			Attempt: 1,
		},
		State:     criRuntime.PodSandboxState_SANDBOX_READY,
		CreatedAt: pod.CreatedTime().UnixNano(),
		Network: &criRuntime.PodSandboxNetworkStatus{
			Ip: ips[0].String(),
		},
		Linux: &criRuntime.LinuxPodSandboxStatus{
			Namespaces: &criRuntime.Namespace{
				Options: &criRuntime.NamespaceOption{
					Network: criRuntime.NamespaceMode_POD,
					Pid:     criRuntime.NamespaceMode_POD,
					Ipc:     criRuntime.NamespaceMode_POD,
				},
			},
		},
		Labels: pod.Labels(),
	}

	var containerStatuses []*criRuntime.ContainerStatus
	containerStatusMap, err := pod.Status()
	if err != nil {
		return nil, nil, err
	}

	for ctrID, ctrStatus := range containerStatusMap {
		libpodCtr, err := runtime.GetContainer(ctrID)
		if err != nil {
			return nil, nil, err
		}

		startedTime, _ := libpodCtr.StartedTime()
		finishedTime, _ := libpodCtr.FinishedTime()
		exitCode, exited, _ := libpodCtr.ExitCode()
		imageID, _ := libpodCtr.Image()
		bindMounts, _ := libpodCtr.BindMounts()
		var mounts []*criRuntime.Mount
		for ctrPath, hostPath := range bindMounts {
			mounts = append(mounts, &criRuntime.Mount{
				ContainerPath: ctrPath,
				HostPath:      hostPath,
			})
		}

		containerStatuses = append(containerStatuses, &criRuntime.ContainerStatus{
			Id: ctrID,
			Metadata: &criRuntime.ContainerMetadata{
				Name:    libpodCtr.Name(),
				Attempt: 1,
			},
			State:      translateLibpodContainerStatusToCriContainerStatus(ctrStatus),
			CreatedAt:  libpodCtr.CreatedTime().UnixNano(),
			StartedAt:  startedTime.UnixNano(),
			FinishedAt: finishedTime.UnixNano(),
			ExitCode:   exitCode,
			Image:      &criRuntime.ImageSpec{Image: imageID},
			ImageRef:   imageID,
			Reason: func() string {
				switch ctrStatus {
				case libpodRuntime.ContainerStateUnknown:
					return "Unknown"
				case libpodRuntime.ContainerStateStopped, libpodRuntime.ContainerStateExited:
					oomKilled, _ := libpodCtr.OOMKilled()
					if oomKilled {
						return "OutOfMemoryKilled"
					}
					if exited {
						return "CommandExited"
					}
				case libpodRuntime.ContainerStatePaused:
					return "UserActionPause"
				case libpodRuntime.ContainerStateConfigured:
					return "UserActionConfigure"
				case libpodRuntime.ContainerStateRunning:
					return "NormalRunning"
				}
				return ""
			}(),
			Message:     "",
			Labels:      libpodCtr.Labels(),
			Annotations: libpodCtr.Spec().Annotations,
			Mounts:      mounts,
			LogPath:     libpodCtr.LogPath(),
		})
	}

	return podStatus, containerStatuses, nil
}

func translateLibpodContainerStatusToCriContainerStatus(status libpodRuntime.ContainerStatus) criRuntime.ContainerState {
	m := map[libpodRuntime.ContainerStatus]criRuntime.ContainerState{
		libpodRuntime.ContainerStateConfigured: criRuntime.ContainerState_CONTAINER_CREATED,
		libpodRuntime.ContainerStateCreated:    criRuntime.ContainerState_CONTAINER_CREATED,
		libpodRuntime.ContainerStatePaused:     criRuntime.ContainerState_CONTAINER_CREATED,

		libpodRuntime.ContainerStateRunning: criRuntime.ContainerState_CONTAINER_RUNNING,
		libpodRuntime.ContainerStateExited:  criRuntime.ContainerState_CONTAINER_EXITED,
		libpodRuntime.ContainerStateUnknown: criRuntime.ContainerState_CONTAINER_UNKNOWN,
	}

	return m[status]
}
