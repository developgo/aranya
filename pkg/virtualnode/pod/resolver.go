package pod

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func newContainerCreatingStatus(pod *corev1.Pod) []corev1.ContainerStatus {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			Image: ctr.Image,
		}
	}
	return status
}

func newContainerRunningStatus(pod *corev1.Pod, devicePod *connectivity.Pod) []corev1.ContainerStatus {
	status := make([]corev1.ContainerStatus, len(pod.Spec.Containers))
	for i, ctr := range pod.Spec.Containers {
		status[i] = corev1.ContainerStatus{
			Name:  ctr.Name,
			State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.Now()}},
			Image: ctr.Image,
			Ready: true,
		}
	}

	podStatus, err := devicePod.GetResolvedKubePodStatus()
	if err != nil {
		return status
	}
	ctrStatusMap := make(map[string]*kubecontainer.ContainerStatus)
	for _, s := range podStatus.ContainerStatuses {
		ctrStatusMap[s.Name] = s
	}

	for i, s := range status {
		newStatus := s
		if ctrStatus, ok := ctrStatusMap[s.Name]; ok {
			newStatus.ImageID = ctrStatus.ImageID
			if !ctrStatus.StartedAt.IsZero() {
				newStatus.State = corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(ctrStatus.StartedAt)}}
			}
			newStatus.ContainerID = ctrStatus.ID.ID
			status[i] = newStatus
		}
	}

	return status
}
