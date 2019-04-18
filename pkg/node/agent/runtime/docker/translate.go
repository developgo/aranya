// +build rt_docker

package docker

import (
	"time"

	dockerType "github.com/docker/docker/api/types"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
)

func (r *dockerRuntime) translateDockerContainerStatusToCRISandboxStatus(ctrInfo *dockerType.ContainerJSON) *criRuntime.PodSandboxStatus {
	podCreatedAt, _ := time.Parse(time.RFC3339Nano, ctrInfo.Created)
	return &criRuntime.PodSandboxStatus{
		Id:        r.Name() + "://" + ctrInfo.ID,
		State:     translateDockerContainerStateToCRISandboxState(ctrInfo),
		CreatedAt: podCreatedAt.UnixNano(),
	}
}

func (r *dockerRuntime) translateDockerContainerStatusToCRIContainerStatus(ctrInfo *dockerType.ContainerJSON) *criRuntime.ContainerStatus {
	ctrCreatedAt, _ := time.Parse(time.RFC3339Nano, ctrInfo.Created)
	ctrStartedAt := time.Time{}
	ctrFinishedAt := time.Time{}
	if ctrInfo.State != nil {
		ctrStartedAt, _ = time.Parse(time.RFC3339Nano, ctrInfo.State.StartedAt)
		ctrFinishedAt, _ = time.Parse(time.RFC3339Nano, ctrInfo.State.FinishedAt)
	}
	return &criRuntime.ContainerStatus{
		Id:         r.Name() + "://" + ctrInfo.ID,
		State:      translateDockerContainerStateToCRIContainerState(ctrInfo),
		CreatedAt:  ctrCreatedAt.UnixNano(),
		StartedAt:  ctrStartedAt.UnixNano(),
		FinishedAt: ctrFinishedAt.UnixNano(),
		ExitCode: func() int32 {
			if ctrInfo.State != nil {
				return int32(ctrInfo.State.ExitCode)
			}
			return 0
		}(),
		Image: &criRuntime.ImageSpec{
			Image: ctrInfo.Image,
		},
		ImageRef: ctrInfo.Image,
		Mounts: func() []*criRuntime.Mount {
			var mounts []*criRuntime.Mount
			if ctrInfo.HostConfig != nil {
				for _, m := range ctrInfo.HostConfig.Mounts {
					mounts = append(mounts, &criRuntime.Mount{
						ContainerPath: m.Target,
						HostPath:      m.Source,
						Readonly:      m.ReadOnly,
					})
				}
			}
			return mounts
		}(),
		LogPath: ctrInfo.LogPath,
	}
}

func translateDockerContainerStateToCRISandboxState(ctrInfo *dockerType.ContainerJSON) criRuntime.PodSandboxState {
	if ctrInfo.State == nil {
		return criRuntime.PodSandboxState_SANDBOX_NOTREADY
	}

	state := ctrInfo.State
	switch {
	case state.Running:
		return criRuntime.PodSandboxState_SANDBOX_READY
	default:
		return criRuntime.PodSandboxState_SANDBOX_NOTREADY
	}
}

func translateDockerContainerStateToCRIContainerState(ctrInfo *dockerType.ContainerJSON) criRuntime.ContainerState {
	if ctrInfo.State == nil {
		return criRuntime.ContainerState_CONTAINER_UNKNOWN
	}

	state := ctrInfo.State
	switch {
	case state.OOMKilled, state.Dead:
		return criRuntime.ContainerState_CONTAINER_EXITED
	case state.Paused:
		return criRuntime.ContainerState_CONTAINER_CREATED
	case state.Running:
		return criRuntime.ContainerState_CONTAINER_RUNNING
	default:
		return criRuntime.ContainerState_CONTAINER_UNKNOWN
	}
}
