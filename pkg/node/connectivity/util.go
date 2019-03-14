package connectivity

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
)

func TranslatePodToPodStatus(pod *Pod) *kubeletContainer.PodStatus {
	var (
		sandboxStatuses   []*criRuntime.PodSandboxStatus
		containerStatuses []*kubeletContainer.ContainerStatus
	)

	switch statuses := pod.GetCriPodStatus().(type) {
	case *Pod_PodStatusV1Alpha2:
		allBytes := statuses.PodStatusV1Alpha2.GetV1Alpha2()
		for _, statusBytes := range allBytes {
			status := &criRuntime.PodSandboxStatus{}
			_ = status.Unmarshal(statusBytes)
			sandboxStatuses = append(sandboxStatuses, status)
		}
	}

	switch statuses := pod.GetCriContainerStatus().(type) {
	case *Pod_ContainerStatusV1Alpha2:
		allBytes := statuses.ContainerStatusV1Alpha2.GetV1Alpha2()
		for _, statusBytes := range allBytes {
			status := &criRuntime.ContainerStatus{}
			_ = status.Unmarshal(statusBytes)

			containerStatuses = append(containerStatuses, &kubeletContainer.ContainerStatus{
				ID:         kubeletContainer.ContainerID{Type: "", ID: status.GetId()},
				Name:       status.GetMetadata().GetName(),
				State:      kubeletContainer.ContainerState(status.GetState().String()),
				CreatedAt:  time.Unix(status.GetCreatedAt(), 0),
				StartedAt:  time.Unix(status.GetStartedAt(), 0),
				FinishedAt: time.Unix(status.GetFinishedAt(), 0),
				ExitCode:   int(status.GetExitCode()),
				Image:      status.GetImage().GetImage(),
				ImageID:    status.GetImageRef(),
				// TODO: calculate hash for this pod
				Hash:         0,
				RestartCount: int(status.GetMetadata().GetAttempt()),
				Reason:       status.GetReason(),
				Message:      status.GetMessage(),
			})
		}
	}

	return &kubeletContainer.PodStatus{
		Namespace:         pod.Namespace,
		Name:              pod.Name,
		ID:                types.UID(pod.Uid),
		IP:                pod.Ip,
		ContainerStatuses: containerStatuses,
		SandboxStatuses:   sandboxStatuses,
	}
}
