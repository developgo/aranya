package runtimeutil

import (
	dockerFilter "github.com/docker/docker/api/types/filters"

	"arhat.dev/aranya/pkg/constant"
)

func DockerContainerFilter(podNamespace, podName, podUID string, includeWork bool) dockerFilter.Args {
	filter := dockerFilter.NewArgs()
	if podNamespace != "" {
		filter.Add("label", constant.ContainerLabelPodNamespace+"="+podNamespace)
	}
	if podName != "" {
		filter.Add("label", constant.ContainerLabelPodName+"="+podName)
	}
	if podUID != "" {
		filter.Add("label", constant.ContainerLabelPodUID+"="+podUID)
	}
	if includeWork {
		filter.Add("label", constant.ContainerLabelPodContainerRole+"="+constant.ContainerRoleWork)
	} else {
		filter.Add("label", constant.ContainerLabelPodContainerRole+"="+constant.ContainerRoleInfra)
	}
	return filter
}
