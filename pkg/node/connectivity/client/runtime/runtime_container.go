package runtime

import (
	"fmt"
	"path"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/klog"
	"k8s.io/kubernetes/pkg/features"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
)

const (
	// legacyContainerLogsDir is the legacy location of container logs. It is the same with
	// kubelet.containerLogsDir.
	legacyContainerLogsDir = "/var/log/containers"
	// legacyLogSuffix is the legacy log suffix.
	legacyLogSuffix = "log"

	ext4MaxFileNameLen          = 255
	minimumGracePeriodInSeconds = 2
)

// legacyLogSymlink composes the legacy container log path. It is only used for legacy cluster
// logging support.
func legacyLogSymlink(containerID string, containerName, podName, podNamespace string) string {
	return logSymlink(legacyContainerLogsDir, kubecontainer.BuildPodFullName(podName, podNamespace),
		containerName, containerID)
}

func logSymlink(containerLogsDir, podFullName, containerName, dockerID string) string {
	suffix := fmt.Sprintf(".%s", legacyLogSuffix)
	logPath := fmt.Sprintf("%s_%s-%s", podFullName, containerName, dockerID)
	// Length of a filename cannot exceed 255 characters in ext4 on Linux.
	if len(logPath) > ext4MaxFileNameLen-len(suffix) {
		logPath = logPath[:ext4MaxFileNameLen-len(suffix)]
	}
	return path.Join(containerLogsDir, logPath+suffix)
}

func (r *Runtime) Run(containerID kubecontainer.ContainerID, pod *corev1.Pod, container *corev1.Container, handler *corev1.Handler) (string, error) {
	switch {
	case handler.Exec != nil:
		var msg string
		output, err := r.RunInContainer(containerID, handler.Exec.Command, 0)
		if err != nil {
			msg = fmt.Sprintf("remoteExec lifecycle hook (%v) for Container %q in Pod %q failed - error: %v, message: %q", handler.Exec.Command, container.Name, format.Pod(pod), err, string(output))
			klog.V(1).Infof(msg)
		}
		return msg, err
	default:
		err := fmt.Errorf("Invalid handler: %v ", handler)
		msg := fmt.Sprintf("Cannot run handler: %v", err)
		klog.Errorf(msg)
		return msg, err
	}
}

// RunInContainer synchronously executes the command in the container, and returns the output.
func (r *Runtime) RunInContainer(id kubecontainer.ContainerID, cmd []string, timeout time.Duration) ([]byte, error) {
	stdout, stderr, err := r.remoteExecSync(id.ID, cmd, timeout)
	// NOTE(tallclair): This does not correctly interleave stdout & stderr, but should be sufficient
	// for logging purposes. A combined output option will need to be added to the ExecSyncRequest
	// if more precise output ordering is ever required.
	return append(stdout, stderr...), err
}

// killContainer kills a container through the following steps:
// * Run the pre-stop lifecycle hooks (if applicable).
// * Stop the container.
func (r *Runtime) killContainer(pod *corev1.Pod, containerID kubecontainer.ContainerID, containerName string, reason string, gracePeriodOverride *int64) error {
	var containerSpec *corev1.Container
	if pod != nil {
		if containerSpec = kubecontainer.GetContainerSpec(pod, containerName); containerSpec == nil {
			return fmt.Errorf("failed to get containerSpec %q(id=%q) in pod %q when killing container for reason %q",
				containerName, containerID.String(), format.Pod(pod), reason)
		}
	} else {
		// Restore necessary information if one of the specs is nil.
		restoredPod, restoredContainer, err := r.restoreSpecsFromContainerLabels(containerID)
		if err != nil {
			return err
		}
		pod, containerSpec = restoredPod, restoredContainer
	}

	// From this point , pod and container must be non-nil.
	gracePeriod := int64(minimumGracePeriodInSeconds)
	switch {
	case pod.DeletionGracePeriodSeconds != nil:
		gracePeriod = *pod.DeletionGracePeriodSeconds
	case pod.Spec.TerminationGracePeriodSeconds != nil:
		gracePeriod = *pod.Spec.TerminationGracePeriodSeconds
	}

	// always give containers a minimal shutdown window to avoid unnecessary SIGKILLs
	if gracePeriod < minimumGracePeriodInSeconds {
		gracePeriod = minimumGracePeriodInSeconds
	}
	if gracePeriodOverride != nil {
		gracePeriod = *gracePeriodOverride
		// klog.V(3).Infof("Killing container %q, but using %d second grace period override", containerID, gracePeriod)
	}

	err := r.remoteStopContainer(containerID.ID, gracePeriod)
	if err != nil {
		klog.Errorf("Container %q termination failed with gracePeriod %d: %v", containerID.String(), gracePeriod, err)
	} else {
		klog.V(3).Infof("Container %q exited normally", containerID.String())
	}

	message := fmt.Sprintf("Killing container with id %s", containerID.String())
	if reason != "" {
		message = fmt.Sprint(message, ":", reason)
	}
	// m.recordContainerEvent(pod, containerSpec, containerID.ID, v1.EventTypeNormal, events.KillingContainer, message)

	r.containerRefManager.ClearRef(containerID)

	return err
}

// restoreSpecsFromContainerLabels restores all information needed for killing a container. In some
// case we may not have pod and container spec when killing a container, e.g. pod is deleted during
// kubelet restart.
// To solve this problem, we've already written necessary information into container labels. Here we
// just need to retrieve them from container labels and restore the specs.
// TODO(random-liu): Add a node e2e test to test this behaviour.
// TODO(random-liu): Change the lifecycle handler to just accept information needed, so that we can
// just pass the needed function not create the fake object.
func (r *Runtime) restoreSpecsFromContainerLabels(containerID kubecontainer.ContainerID) (*corev1.Pod, *corev1.Container, error) {
	var pod *corev1.Pod
	var container *corev1.Container
	s, err := r.remoteContainerStatus(containerID.ID)
	if err != nil {
		return nil, nil, err
	}

	l := getContainerInfoFromLabels(s.Labels)
	a := getContainerInfoFromAnnotations(s.Annotations)
	// Notice that the followings are not full spec. The container killing code should not use
	// un-restored fields.
	pod = &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			UID:                        l.PodUID,
			Name:                       l.PodName,
			Namespace:                  l.PodNamespace,
			DeletionGracePeriodSeconds: a.PodDeletionGracePeriod,
		},
		Spec: corev1.PodSpec{
			TerminationGracePeriodSeconds: a.PodTerminationGracePeriod,
		},
	}
	container = &corev1.Container{
		Name:                   l.ContainerName,
		Ports:                  a.ContainerPorts,
		TerminationMessagePath: a.TerminationMessagePath,
	}
	if a.PreStopHandler != nil {
		container.Lifecycle = &corev1.Lifecycle{
			PreStop: a.PreStopHandler,
		}
	}
	return pod, container, nil
}

type labeledContainerInfo struct {
	ContainerName string
	ContainerType kubecontainer.ContainerType
	PodName       string
	PodNamespace  string
	PodUID        kubetypes.UID
}

// getContainerInfoFromLabels gets labeledContainerInfo from labels.
func getContainerInfoFromLabels(labels map[string]string) *labeledContainerInfo {
	var containerType kubecontainer.ContainerType
	if utilfeature.DefaultFeatureGate.Enabled(features.DebugContainers) {
		containerType = kubecontainer.ContainerType(getStringValueFromLabel(labels, types.KubernetesContainerTypeLabel))
	}
	return &labeledContainerInfo{
		PodName:       getStringValueFromLabel(labels, types.KubernetesPodNameLabel),
		PodNamespace:  getStringValueFromLabel(labels, types.KubernetesPodNamespaceLabel),
		PodUID:        kubetypes.UID(getStringValueFromLabel(labels, types.KubernetesPodUIDLabel)),
		ContainerName: getStringValueFromLabel(labels, types.KubernetesContainerNameLabel),
		ContainerType: containerType,
	}
}
