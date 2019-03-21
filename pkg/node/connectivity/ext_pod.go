package connectivity

import (
	"time"

	"k8s.io/apimachinery/pkg/types"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
)

func (m *Pod) GetResolvedKubePodStatus() (*kubeletContainer.PodStatus, error) {
	containerStatuses, err := m.getResolvedKubeContainerStatuses()
	if err != nil {
		return nil, err
	}

	sandboxStatus, err := m.getResolvedV1Alpha2SandboxStatuses()
	if err != nil {
		return nil, err
	}

	return &kubeletContainer.PodStatus{
		Namespace:         m.Namespace,
		Name:              m.Name,
		ID:                types.UID(m.Uid),
		IP:                m.Ip,
		ContainerStatuses: containerStatuses,
		SandboxStatuses:   []*criRuntime.PodSandboxStatus{sandboxStatus},
	}, nil
}

func (m *Pod) getResolvedV1Alpha2SandboxStatuses() (*criRuntime.PodSandboxStatus, error) {
	status := &criRuntime.PodSandboxStatus{}
	err := status.Unmarshal(m.GetSandboxV1Alpha2())
	if err != nil {
		return nil, err
	}

	return status, nil
}

func (m *Pod) getResolvedV1Alpha2ContainerStatuses() ([]*criRuntime.ContainerStatus, error) {
	allBytes := m.GetContainerV1Alpha2().GetV1Alpha2()
	containerStatuses := make([]*criRuntime.ContainerStatus, len(allBytes))
	for i, statusBytes := range allBytes {
		status := &criRuntime.ContainerStatus{}
		err := status.Unmarshal(statusBytes)
		if err != nil {
			return nil, err
		}

		containerStatuses[i] = status
	}

	return containerStatuses, nil
}

func (m *Pod) getResolvedKubeContainerStatuses() ([]*kubeletContainer.ContainerStatus, error) {
	var kubeContainerStatuses []*kubeletContainer.ContainerStatus

	switch m.GetContainerStatus().(type) {
	case *Pod_ContainerV1Alpha2:
		criContainerStatuses, err := m.getResolvedV1Alpha2ContainerStatuses()
		if err != nil {
			return nil, err
		}
		for _, status := range criContainerStatuses {
			kubeContainerStatuses = append(kubeContainerStatuses, &kubeletContainer.ContainerStatus{
				ID:         kubeletContainer.ParseContainerID(status.GetId()),
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
				RestartCount: int(status.GetMetadata().GetAttempt()) - 1,
				Reason:       status.GetReason(),
				Message:      status.GetMessage(),
			})
		}
	}

	return kubeContainerStatuses, nil
}