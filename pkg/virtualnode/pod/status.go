package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func newContainerErrorStatus(pod *corev1.Pod) (corev1.PodPhase, []corev1.ContainerStatus) {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerErrored"}},
			Image: ctr.Image,
		}
	}

	return corev1.PodFailed, status
}

func newContainerCreatingStatus(pod *corev1.Pod) (corev1.PodPhase, []corev1.ContainerStatus) {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			Image: ctr.Image,
		}
	}

	return corev1.PodPending, status
}

func resolveContainerStatus(pod *corev1.Pod, devicePodStatus *connectivity.PodStatus) (corev1.PodPhase, []corev1.ContainerStatus) {
	ctrStatusMap := devicePodStatus.GetContainerStatuses()
	if ctrStatusMap == nil {
		// generalize to avoid panic
		ctrStatusMap = make(map[string]*connectivity.PodStatus_ContainerStatus)
	}

	podPhase := corev1.PodRunning
	statuses := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		if s, ok := ctrStatusMap[ctr.Name]; ok {
			status := corev1.ContainerStatus{
				Name:        ctr.Name,
				ContainerID: s.ContainerId,
				Image:       ctr.Image,
				ImageID:     s.ImageId,
			}

			containerExited := false
			switch s.GetState() {
			case connectivity.StateUnknown:
			case connectivity.StatePending:
				podPhase = corev1.PodPending
				status.State.Waiting = &corev1.ContainerStateWaiting{
					Reason:  s.Reason,
					Message: s.Message,
				}
			case connectivity.StateRunning:
				status.Ready = true
				status.State.Running = &corev1.ContainerStateRunning{
					StartedAt: metav1.Unix(0, s.StartedAt),
				}
			case connectivity.StateSucceeded:
				containerExited = true
				podPhase = corev1.PodSucceeded
			case connectivity.StateFailed:
				containerExited = true
				podPhase = corev1.PodFailed
			}

			if containerExited {
				status.State.Terminated = &corev1.ContainerStateTerminated{
					ExitCode:    s.ExitCode,
					Reason:      s.Reason,
					Message:     s.Message,
					StartedAt:   metav1.Unix(0, s.StartedAt),
					FinishedAt:  metav1.Unix(0, s.FinishedAt),
					ContainerID: s.ContainerId,
				}
			}
		} else {
			statuses[i] = corev1.ContainerStatus{
				Name: ctr.Name,
			}
		}
	}

	return podPhase, statuses
}
