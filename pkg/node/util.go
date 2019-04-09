package node

import (
	"sort"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	podUtil "k8s.io/kubernetes/pkg/api/v1/pod"
	v1qos "k8s.io/kubernetes/pkg/apis/core/v1/helper/qos"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/status"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
)

func newPodCache() *PodCache {
	return &PodCache{
		m: make(map[types.NamespacedName]*PodPair),
	}
}

type PodPair struct {
	pod    *corev1.Pod
	status *kubeletContainer.PodStatus
}

type PodCache struct {
	// pod full name to kubeletContainer.PodPair
	m  map[types.NamespacedName]*PodPair
	mu sync.RWMutex
}

func (c *PodCache) Update(pod *corev1.Pod, status *kubeletContainer.PodStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
	c.m[name] = &PodPair{
		pod:    pod,
		status: status,
	}
}

func (c *PodCache) Get(namespace, name string) (*corev1.Pod, *kubeletContainer.PodStatus) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	podPair, ok := c.m[types.NamespacedName{Namespace: namespace, Name: name}]
	if !ok {
		return nil, nil
	}

	return podPair.pod, podPair.status
}

func newNodeCache(node *corev1.Node) *NodeCache {
	return &NodeCache{node: node}
}

type NodeCache struct {
	node *corev1.Node
	mu   sync.RWMutex
}

func (c *NodeCache) Update(pod corev1.Node) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.node = &pod
}

func (c *NodeCache) Get() corev1.Node {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.node == nil {
		return corev1.Node{}
	}
	return *c.node
}

// ConvertStatusToAPIStatus creates an api PodStatus for the given pod from
// the given internal pod status.  It is purely transformative and does not
// alter the kubelet state at all.
func (n *Node) ConvertStatusToAPIStatus(pod *corev1.Pod, podStatus *kubeletContainer.PodStatus) *corev1.PodStatus {
	var apiPodStatus corev1.PodStatus
	apiPodStatus.PodIP = podStatus.IP
	// set status for Pods created on versions of kube older than 1.6
	apiPodStatus.QOSClass = v1qos.GetPodQOS(pod)

	oldPodStatus, found := n.podStatusManager.GetPodStatus(pod.UID)
	if !found {
		oldPodStatus = pod.Status
	}

	apiPodStatus.ContainerStatuses = n.ConvertToAPIContainerStatuses(
		pod, podStatus,
		oldPodStatus.ContainerStatuses,
		pod.Spec.Containers,
		len(pod.Spec.InitContainers) > 0,
		false,
	)

	apiPodStatus.InitContainerStatuses = n.ConvertToAPIContainerStatuses(
		pod, podStatus,
		oldPodStatus.InitContainerStatuses,
		pod.Spec.InitContainers,
		len(pod.Spec.InitContainers) > 0,
		true,
	)

	// Preserves conditions not controlled by kubelet
	for _, c := range pod.Status.Conditions {
		if !kubeletTypes.PodConditionByKubelet(c.Type) {
			apiPodStatus.Conditions = append(apiPodStatus.Conditions, c)
		}
	}
	return &apiPodStatus
}

// generateAPIPodStatus creates the final API pod status for a pod, given the
// internal pod status.
func (n *Node) GenerateAPIPodStatus(pod *corev1.Pod, podStatus *kubeletContainer.PodStatus) corev1.PodStatus {
	klog.V(3).Infof("Generating status for %q", format.Pod(pod))

	s := n.ConvertStatusToAPIStatus(pod, podStatus)

	// // check if an internal module has requested the pod is evicted.
	// for _, podSyncHandler := range kl.PodSyncHandlers {
	// 	if result := podSyncHandler.ShouldEvict(pod); result.Evict {
	// 		s.Phase = corev1.PodFailed
	// 		s.Reason = result.Reason
	// 		s.Message = result.Message
	// 		return *s
	// 	}
	// }

	// Assume info is ready to process
	spec := &pod.Spec
	allStatus := append(append([]corev1.ContainerStatus{}, s.ContainerStatuses...), s.InitContainerStatuses...)
	s.Phase = getPhase(spec, allStatus)
	// Check for illegal phase transition
	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		// API server shows terminal phase; transitions are not allowed
		if s.Phase != pod.Status.Phase {
			klog.Errorf("Pod attempted illegal phase transition from %s to %s: %v", pod.Status.Phase, s.Phase, s)
			// Force back to phase from the API server
			s.Phase = pod.Status.Phase
		}
	}

	// kl.probeManager.UpdatePodStatus(pod.UID, s)
	s.Conditions = append(s.Conditions, status.GeneratePodInitializedCondition(spec, s.InitContainerStatuses, s.Phase))
	s.Conditions = append(s.Conditions, status.GeneratePodReadyCondition(spec, s.Conditions, s.ContainerStatuses, s.Phase))
	s.Conditions = append(s.Conditions, status.GenerateContainersReadyCondition(spec, s.ContainerStatuses, s.Phase))
	// Status manager will take care of the LastTransitionTimestamp, either preserve
	// the timestamp from apiserver, or set a new one. When kubelet sees the pod,
	// `PodScheduled` condition must be true.
	s.Conditions = append(s.Conditions, corev1.PodCondition{
		Type:   corev1.PodScheduled,
		Status: corev1.ConditionTrue,
	})

	// if n.kubeClient != nil {
	// 	hostIP, err := kl.getHostIPAnyWay()
	// 	if err != nil {
	// 		klog.V(4).Infof("Cannot get host IP: %v", err)
	// 	} else {
	// 		s.HostIP = hostIP.String()
	// 		if kubecontainer.IsHostNetworkPod(pod) && s.PodIP == "" {
	// 			s.PodIP = hostIP.String()
	// 		}
	// 	}
	// }

	return *s
}

// convertToAPIContainerStatuses converts the given internal container
// statuses into API container statuses.
func (n *Node) ConvertToAPIContainerStatuses(pod *corev1.Pod, podStatus *kubeletContainer.PodStatus, previousStatus []corev1.ContainerStatus, containers []corev1.Container, hasInitContainers, isInitContainer bool) []corev1.ContainerStatus {
	convertContainerStatus := func(cs *kubeletContainer.ContainerStatus) *corev1.ContainerStatus {
		cid := cs.ID.String()
		status := &corev1.ContainerStatus{
			Name:         cs.Name,
			RestartCount: int32(cs.RestartCount),
			Image:        cs.Image,
			ImageID:      cs.ImageID,
			ContainerID:  cid,
		}
		switch cs.State {
		case kubeletContainer.ContainerStateRunning:
			status.State.Running = &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(cs.StartedAt)}
		case kubeletContainer.ContainerStateCreated:
			// Treat containers in the "created" state as if they are exited.
			// The pod workers are supposed start all containers it creates in
			// one sync (syncPod) iteration. There should not be any normal
			// "created" containers when the pod worker generates the status at
			// the beginning of a sync iteration.
			fallthrough
		case kubeletContainer.ContainerStateExited:
			status.State.Terminated = &corev1.ContainerStateTerminated{
				ExitCode:    int32(cs.ExitCode),
				Reason:      cs.Reason,
				Message:     cs.Message,
				StartedAt:   metav1.NewTime(cs.StartedAt),
				FinishedAt:  metav1.NewTime(cs.FinishedAt),
				ContainerID: cid,
			}
		default:
			status.State.Waiting = &corev1.ContainerStateWaiting{}
		}
		return status
	}

	// Fetch old containers statuses from old pod status.
	oldStatuses := make(map[string]corev1.ContainerStatus, len(containers))
	for _, status := range previousStatus {
		oldStatuses[status.Name] = status
	}

	// Set all container statuses to default waiting state
	statuses := make(map[string]*corev1.ContainerStatus, len(containers))
	defaultWaitingState := corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}
	if hasInitContainers {
		defaultWaitingState = corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "PodInitializing"}}
	}

	for _, container := range containers {
		status := &corev1.ContainerStatus{
			Name:  container.Name,
			Image: container.Image,
			State: defaultWaitingState,
		}
		oldStatus, found := oldStatuses[container.Name]
		if found {
			if oldStatus.State.Terminated != nil {
				// Do not update status on terminated init containers as
				// they be removed at any time.
				status = &oldStatus
			} else {
				// Apply some values from the old statuses as the default values.
				status.RestartCount = oldStatus.RestartCount
				status.LastTerminationState = oldStatus.LastTerminationState
			}
		}
		statuses[container.Name] = status
	}

	// Make the latest container status comes first.
	sort.Sort(sort.Reverse(kubeletContainer.SortContainerStatusesByCreationTime(podStatus.ContainerStatuses)))
	// Set container statuses according to the statuses seen in pod status
	containerSeen := map[string]int{}
	for _, cStatus := range podStatus.ContainerStatuses {
		cName := cStatus.Name
		if _, ok := statuses[cName]; !ok {
			// This would also ignore the infra container.
			continue
		}
		if containerSeen[cName] >= 2 {
			continue
		}
		status := convertContainerStatus(cStatus)
		if containerSeen[cName] == 0 {
			statuses[cName] = status
		} else {
			statuses[cName].LastTerminationState = status.State
		}
		containerSeen[cName] = containerSeen[cName] + 1
	}

	// Handle the containers failed to be started, which should be in Waiting state.
	for _, container := range containers {
		if isInitContainer {
			// If the init container is terminated with exit code 0, it won't be restarted.
			// TODO(random-liu): Handle this in a cleaner way.
			s := podStatus.FindContainerStatusByName(container.Name)
			if s != nil && s.State == kubeletContainer.ContainerStateExited && s.ExitCode == 0 {
				continue
			}
		}
		// If a container should be restarted in next syncpod, it is *Waiting*.
		if !kubeletContainer.ShouldContainerBeRestarted(&container, pod, podStatus) {
			continue
		}
		status := statuses[container.Name]
		// reason, ok := kl.reasonCache.Get(pod.UID, container.Name)
		// if !ok {
		// 	// In fact, we could also apply Waiting state here, but it is less informative,
		// 	// and the container will be restarted soon, so we prefer the original state here.
		// 	// Note that with the current implementation of ShouldContainerBeRestarted the original state here
		// 	// could be:
		// 	//   * Waiting: There is no associated historical container and start failure reason record.
		// 	//   * Terminated: The container is terminated.
		// 	continue
		// }
		if status.State.Terminated != nil {
			status.LastTerminationState = status.State
		}
		status.State = corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{
				// Reason:  reason.Err.Error(),
				// Message: reason.Message,
			},
		}
		statuses[container.Name] = status
	}

	var containerStatuses []corev1.ContainerStatus
	for _, status := range statuses {
		containerStatuses = append(containerStatuses, *status)
	}

	// Sort the container statuses since clients of this interface expect the list
	// of containers in a pod has a deterministic order.
	if isInitContainer {
		kubeletTypes.SortInitContainerStatuses(pod, containerStatuses)
	} else {
		sort.Sort(kubeletTypes.SortedContainerStatuses(containerStatuses))
	}
	return containerStatuses
}

// getPhase returns the phase of a pod given its container info.
func getPhase(spec *corev1.PodSpec, info []corev1.ContainerStatus) corev1.PodPhase {
	initialized := 0
	pendingInitialization := 0
	failedInitialization := 0
	for _, container := range spec.InitContainers {
		containerStatus, ok := podUtil.GetContainerStatus(info, container.Name)
		if !ok {
			pendingInitialization++
			continue
		}

		switch {
		case containerStatus.State.Running != nil:
			pendingInitialization++
		case containerStatus.State.Terminated != nil:
			if containerStatus.State.Terminated.ExitCode == 0 {
				initialized++
			} else {
				failedInitialization++
			}
		case containerStatus.State.Waiting != nil:
			if containerStatus.LastTerminationState.Terminated != nil {
				if containerStatus.LastTerminationState.Terminated.ExitCode == 0 {
					initialized++
				} else {
					failedInitialization++
				}
			} else {
				pendingInitialization++
			}
		default:
			pendingInitialization++
		}
	}

	unknown := 0
	running := 0
	waiting := 0
	stopped := 0
	failed := 0
	succeeded := 0
	for _, container := range spec.Containers {
		containerStatus, ok := podUtil.GetContainerStatus(info, container.Name)
		if !ok {
			unknown++
			continue
		}

		switch {
		case containerStatus.State.Running != nil:
			running++
		case containerStatus.State.Terminated != nil:
			stopped++
			if containerStatus.State.Terminated.ExitCode == 0 {
				succeeded++
			} else {
				failed++
			}
		case containerStatus.State.Waiting != nil:
			if containerStatus.LastTerminationState.Terminated != nil {
				stopped++
			} else {
				waiting++
			}
		default:
			unknown++
		}
	}

	if failedInitialization > 0 && spec.RestartPolicy == corev1.RestartPolicyNever {
		return corev1.PodFailed
	}

	switch {
	case pendingInitialization > 0:
		fallthrough
	case waiting > 0:
		// klog.V(5).Infof("pod waiting > 0, pending")
		// One or more containers has not been started
		return corev1.PodPending
	case running > 0 && unknown == 0:
		// All containers have been started, and at least
		// one container is running
		return corev1.PodRunning
	case running == 0 && stopped > 0 && unknown == 0:
		// All containers are terminated
		if spec.RestartPolicy == corev1.RestartPolicyAlways {
			// All containers are in the process of restarting
			return corev1.PodRunning
		}
		if stopped == succeeded {
			// RestartPolicy is not Always, and all
			// containers are terminated in success
			return corev1.PodSucceeded
		}
		if spec.RestartPolicy == corev1.RestartPolicyNever {
			// RestartPolicy is Never, and all containers are
			// terminated with at least one in failure
			return corev1.PodFailed
		}
		// RestartPolicy is OnFailure, and at least one in failure
		// and in the process of restarting
		return corev1.PodRunning
	default:
		// klog.V(5).Infof("pod default case, pending")
		return corev1.PodPending
	}
}
