// +build rt_docker

package docker

import (
	"time"

	dockerType "github.com/docker/docker/api/types"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (r *dockerRuntime) translatePodStatus(pauseContainer *dockerType.ContainerJSON, containers []*dockerType.ContainerJSON) *connectivity.PodStatus {
	podUID := pauseContainer.Config.Labels[constant.ContainerLabelPodUID]
	ctrStatus := make(map[string]*connectivity.PodStatus_ContainerStatus)
	for _, ctr := range containers {
		ctrPodUID := ctr.Config.Labels[constant.ContainerLabelPodUID]
		name := ctr.Config.Labels[constant.ContainerLabelPodContainer]
		if name != "" || ctrPodUID != podUID {
			// invalid container, skip
			continue
		}

		status := r.translateContainerStatus(ctr)
		ctrStatus[name] = status
	}

	return connectivity.NewPodStatus(podUID, ctrStatus)
}

func (r *dockerRuntime) translateContainerStatus(ctrInfo *dockerType.ContainerJSON) *connectivity.PodStatus_ContainerStatus {
	ctrCreatedAt, _ := time.Parse(time.RFC3339Nano, ctrInfo.Created)
	ctrStartedAt, _ := time.Parse(time.RFC3339Nano, ctrInfo.State.StartedAt)
	ctrFinishedAt, _ := time.Parse(time.RFC3339Nano, ctrInfo.State.FinishedAt)

	return &connectivity.PodStatus_ContainerStatus{
		ContainerId: r.Name() + "://" + ctrInfo.ID,
		ImageId:     ctrInfo.Image,
		CreatedAt:   ctrCreatedAt.UnixNano(),
		StartedAt:   ctrStartedAt.UnixNano(),
		FinishedAt:  ctrFinishedAt.UnixNano(),
		ExitCode: func() int32 {
			if ctrInfo.State != nil {
				return int32(ctrInfo.State.ExitCode)
			}
			return 0
		}(),
	}
}
