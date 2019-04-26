// +build rt_docker

/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package docker

import (
	"time"

	dockertype "github.com/docker/docker/api/types"

	"arhat.dev/aranya/pkg/connectivity"
	"arhat.dev/aranya/pkg/constant"
)

func (r *dockerRuntime) translatePodStatus(pauseContainer *dockertype.ContainerJSON, containers []*dockertype.ContainerJSON) *connectivity.PodStatus {
	podUID := pauseContainer.Config.Labels[constant.ContainerLabelPodUID]
	ctrStatus := make(map[string]*connectivity.PodStatus_ContainerStatus)

	for _, ctr := range containers {
		ctrPodUID := ctr.Config.Labels[constant.ContainerLabelPodUID]
		name := ctr.Config.Labels[constant.ContainerLabelPodContainer]
		if name == "" || ctrPodUID != podUID {
			// invalid container, skip
			continue
		}

		status := r.translateContainerStatus(ctr)
		ctrStatus[name] = status
	}

	return connectivity.NewPodStatus(podUID, ctrStatus)
}

func (r *dockerRuntime) translateContainerStatus(ctrInfo *dockertype.ContainerJSON) *connectivity.PodStatus_ContainerStatus {
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
