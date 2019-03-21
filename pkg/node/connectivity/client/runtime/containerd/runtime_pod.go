package containerd

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/armon/circbuf"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/klog"
	runtimeapi "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/types"
	"k8s.io/kubernetes/pkg/kubelet/util/format"
	"k8s.io/kubernetes/pkg/util/tail"
)

// Newest first.
type podSandboxByCreated []*runtimeapi.PodSandbox

func (p podSandboxByCreated) Len() int           { return len(p) }
func (p podSandboxByCreated) Swap(i, j int)      { p[i], p[j] = p[j], p[i] }
func (p podSandboxByCreated) Less(i, j int) bool { return p[i].CreatedAt > p[j].CreatedAt }

type containerStatusByCreated []*kubecontainer.ContainerStatus

func (c containerStatusByCreated) Len() int           { return len(c) }
func (c containerStatusByCreated) Swap(i, j int)      { c[i], c[j] = c[j], c[i] }
func (c containerStatusByCreated) Less(i, j int) bool { return c[i].CreatedAt.After(c[j].CreatedAt) }

// GetPodStatus retrieves the status of the pod, including the
// information of all containers in the pod that are visible in Runtime.
func (r *Runtime) GetPodStatus(uid kubetypes.UID, name, namespace string) (*kubecontainer.PodStatus, error) {
	// Now we retain restart count of container as a container label. Each time a container
	// restarts, pod will read the restart count from the registered dead container, increment
	// it to get the new restart count, and then add a label with the new restart count on
	// the newly started container.
	// However, there are some limitations of this method:
	// 	1. When all dead containers were garbage collected, the container status could
	// 	not get the historical value and would be *inaccurate*. Fortunately, the chance
	// 	is really slim.
	// 	2. When working with old version containers which have no restart count label,
	// 	we can only assume their restart count is 0.
	// Anyhow, we only promised "best-effort" restart count reporting, we can just ignore
	// these limitations now.
	// TODO: move this comment to SyncPod.
	podSandboxIDs, err := r.getSandboxIDByPodUID(uid, nil)
	if err != nil {
		return nil, err
	}

	podFullName := format.Pod(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			UID:       uid,
		},
	})

	sandboxStatuses := make([]*runtimeapi.PodSandboxStatus, len(podSandboxIDs))
	podIP := ""
	for idx, podSandboxID := range podSandboxIDs {
		podSandboxStatus, err := r.remotePodSandboxStatus(podSandboxID)
		if err != nil {
			return nil, err
		}
		sandboxStatuses[idx] = podSandboxStatus

		// Only get pod IP from latest sandbox
		if idx == 0 && podSandboxStatus.State == runtimeapi.PodSandboxState_SANDBOX_READY {
			podIP = determinePodSandboxIP(podSandboxStatus)
		}
	}

	// Get statuses of all containers visible in the pod.
	containerStatuses, err := r.getPodContainerStatuses(uid, name, namespace)
	if err != nil {
		klog.Errorf("getPodContainerStatuses for pod %q failed: %v", podFullName, err)
		return nil, err
	}

	return &kubecontainer.PodStatus{
		ID:                uid,
		Name:              name,
		Namespace:         namespace,
		IP:                podIP,
		SandboxStatuses:   sandboxStatuses,
		ContainerStatuses: containerStatuses,
	}, nil
}

// getPodSandboxID gets the sandbox id by podUID and returns ([]sandboxID, error).
// Param state could be nil in order to get all sandboxes belonging to same pod.
func (r *Runtime) getSandboxIDByPodUID(podUID kubetypes.UID, state *runtimeapi.PodSandboxState) ([]string, error) {
	filter := &runtimeapi.PodSandboxFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: string(podUID)},
	}
	if state != nil {
		filter.State = &runtimeapi.PodSandboxStateValue{
			State: *state,
		}
	}
	sandboxes, err := r.remoteListPodSandbox(filter)
	if err != nil {
		klog.Errorf("remoteListPodSandbox with pod UID %q failed: %v", podUID, err)
		return nil, err
	}

	if len(sandboxes) == 0 {
		return nil, nil
	}

	// Sort with newest first.
	sandboxIDs := make([]string, len(sandboxes))
	sort.Sort(podSandboxByCreated(sandboxes))
	for i, s := range sandboxes {
		sandboxIDs[i] = s.Id
	}

	return sandboxIDs, nil
}

// getPodContainerStatuses gets all containers' statuses for the pod.
func (r *Runtime) getPodContainerStatuses(uid kubetypes.UID, name, namespace string) ([]*kubecontainer.ContainerStatus, error) {
	// Select all containers of the given pod.
	containers, err := r.remoteListContainers(&runtimeapi.ContainerFilter{
		LabelSelector: map[string]string{types.KubernetesPodUIDLabel: string(uid)},
	})
	if err != nil {
		klog.Errorf("remoteListContainers error: %v", err)
		return nil, err
	}

	statuses := make([]*kubecontainer.ContainerStatus, len(containers))
	// TODO: optimization: set maximum number of containers per container name to examine.
	for i, c := range containers {
		status, err := r.remoteContainerStatus(c.Id)
		if err != nil {
			klog.Errorf("remoteContainerStatus for %s error: %v", c.Id, err)
			return nil, err
		}
		cStatus := toKubeContainerStatus(status, r.runtimeName)
		if status.State == runtimeapi.ContainerState_CONTAINER_EXITED {
			// Populate the termination message if needed.
			annotatedInfo := getContainerInfoFromAnnotations(status.Annotations)
			labeledInfo := getContainerInfoFromLabels(status.Labels)
			fallbackToLogs := annotatedInfo.TerminationMessagePolicy == corev1.TerminationMessageFallbackToLogsOnError && cStatus.ExitCode != 0
			tMessage, checkLogs := getTerminationMessage(status, annotatedInfo.TerminationMessagePath, fallbackToLogs)
			if checkLogs {
				path := buildFullContainerLogsPath(uid, labeledInfo.ContainerName, annotatedInfo.RestartCount)
				tMessage = r.readLastStringFromContainerLogs(path)
			}
			// Use the termination message written by the application is not empty
			if len(tMessage) != 0 {
				cStatus.Message = tMessage
			}
		}
		statuses[i] = cStatus
	}

	sort.Sort(containerStatusByCreated(statuses))
	return statuses, nil
}

// readLastStringFromContainerLogs attempts to read up to the max log length from the end of the CRI log represented
// by path. It reads up to max log lines.
func (r *Runtime) readLastStringFromContainerLogs(path string) string {
	value := int64(kubecontainer.MaxContainerTerminationMessageLogLines)
	buf, _ := circbuf.NewBuffer(kubecontainer.MaxContainerTerminationMessageLogLength)
	if err := r.ReadLogs(context.Background(), path, "", &corev1.PodLogOptions{TailLines: &value}, buf, buf); err != nil {
		return fmt.Sprintf("Error on reading termination message from logs: %v", err)
	}
	return buf.String()
}

// ReadLogs read the container log and redirect into stdout and stderr.
// Note that containerID is only needed when following the log, or else
// just pass in empty string "".
func (r *Runtime) ReadLogs(ctx context.Context, path, containerID string, apiOpts *corev1.PodLogOptions, stdout, stderr io.Writer) error {
	// Convert v1.PodLogOptions into internal log options.
	opts := NewLogOptions(apiOpts, time.Now())

	return ReadLogs(ctx, path, containerID, opts, r.remoteContainerStatus, stdout, stderr)
}

// buildFullContainerLogsPath builds absolute log path for container.
func buildFullContainerLogsPath(podUID kubetypes.UID, containerName string, restartCount int) string {
	return filepath.Join(buildPodLogsDirectory(podUID), buildContainerLogsPath(containerName, restartCount))
}

// buildContainerLogsPath builds log path for container relative to pod logs directory.
func buildContainerLogsPath(containerName string, restartCount int) string {
	return filepath.Join(containerName, fmt.Sprintf("%d.log", restartCount))
}

// getTerminationMessage looks on the filesystem for the provided termination message path, returning a limited
// amount of those bytes, or returns true if the logs should be checked.
func getTerminationMessage(status *runtimeapi.ContainerStatus, terminationMessagePath string, fallbackToLogs bool) (string, bool) {
	if len(terminationMessagePath) == 0 {
		return "", fallbackToLogs
	}
	for _, mount := range status.Mounts {
		if mount.ContainerPath != terminationMessagePath {
			continue
		}
		path := mount.HostPath
		data, _, err := tail.ReadAtMost(path, kubecontainer.MaxContainerTerminationMessageLength)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fallbackToLogs
			}
			return fmt.Sprintf("Error on reading termination log %s: %v", path, err), false
		}
		return string(data), fallbackToLogs && len(data) == 0
	}
	return "", fallbackToLogs
}

func toKubeContainerStatus(status *runtimeapi.ContainerStatus, runtimeName string) *kubecontainer.ContainerStatus {
	annotatedInfo := getContainerInfoFromAnnotations(status.Annotations)
	labeledInfo := getContainerInfoFromLabels(status.Labels)
	cStatus := &kubecontainer.ContainerStatus{
		ID: kubecontainer.ContainerID{
			Type: runtimeName,
			ID:   status.Id,
		},
		Name:         labeledInfo.ContainerName,
		Image:        status.Image.Image,
		ImageID:      status.ImageRef,
		Hash:         annotatedInfo.Hash,
		RestartCount: annotatedInfo.RestartCount,
		State:        toKubeContainerState(status.State),
		CreatedAt:    time.Unix(0, status.CreatedAt),
	}

	if status.State != runtimeapi.ContainerState_CONTAINER_CREATED {
		// If container is not in the created state, we have tried and
		// started the container. Set the StartedAt time.
		cStatus.StartedAt = time.Unix(0, status.StartedAt)
	}
	if status.State == runtimeapi.ContainerState_CONTAINER_EXITED {
		cStatus.Reason = status.Reason
		cStatus.Message = status.Message
		cStatus.ExitCode = int(status.ExitCode)
		cStatus.FinishedAt = time.Unix(0, status.FinishedAt)
	}
	return cStatus
}

// toKubeContainerState converts runtimeapi.ContainerState to kubecontainer.ContainerState.
func toKubeContainerState(state runtimeapi.ContainerState) kubecontainer.ContainerState {
	switch state {
	case runtimeapi.ContainerState_CONTAINER_CREATED:
		return kubecontainer.ContainerStateCreated
	case runtimeapi.ContainerState_CONTAINER_RUNNING:
		return kubecontainer.ContainerStateRunning
	case runtimeapi.ContainerState_CONTAINER_EXITED:
		return kubecontainer.ContainerStateExited
	case runtimeapi.ContainerState_CONTAINER_UNKNOWN:
		return kubecontainer.ContainerStateUnknown
	}

	return kubecontainer.ContainerStateUnknown
}
