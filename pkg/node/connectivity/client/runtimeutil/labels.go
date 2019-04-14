package runtimeutil

import (
	"arhat.dev/aranya/pkg/constant"
)

func ContainerLabels(podUID, container string) map[string]string {
	return map[string]string{
		constant.ContainerLabelPodUID:       podUID,
		constant.ContainerLabelPodContainer: container,
		constant.ContainerLabelPodContainerRole: func() string {
			switch container {
			case constant.ContainerNamePause:
				return constant.ContainerRoleInfra
			default:
				return constant.ContainerRoleWork
			}
		}(),
	}
}
