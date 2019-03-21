package containerd

import (
	"fmt"
	"os"
	"path/filepath"

	"google.golang.org/grpc"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/klog"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"
)

// startContainer starts a container and returns a message indicates why it is failed on error.
// It starts the container through the following steps:
// * pull the image
// * create the container
// * start the container
// * run the post start lifecycle hooks (if applicable)
func (r *Runtime) startContainer(podSandboxID string, podSandboxConfig *criRuntime.PodSandboxConfig, container *corev1.Container, pod *corev1.Pod, podStatus *kubecontainer.PodStatus, pullSecrets []corev1.Secret, podIP string, containerType kubecontainer.ContainerType) (string, error) {
	// Step 1: pull the image.
	imageRef, err := r.ensureImageExists(pod, container, pullSecrets)
	if err != nil {
		// m.recordContainerEvent(pod, container, "", corev1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", grpc.ErrorDesc(err))
		return "", err
	}

	// Step 2: create the container.
	ref, err := kubecontainer.GenerateContainerRef(pod, container)
	if err != nil {
		// klog.Errorf("Can't make a ref to pod %q, container %v: %v", format.Pod(pod), container.Name, err)
	}
	// klog.V(4).Infof("Generating ref for container %s: %#v", container.Name, ref)

	// For a new container, the RestartCount should be 0
	restartCount := 0
	containerStatus := podStatus.FindContainerStatusByName(container.Name)
	if containerStatus != nil {
		restartCount = containerStatus.RestartCount + 1
	}

	containerConfig, cleanupAction, err := translateContainerV1ToCRIContainerConfig(container, pod, restartCount, podIP, imageRef, containerType)
	if cleanupAction != nil {
		defer cleanupAction()
	}

	if err != nil {
		// m.recordContainerEvent(pod, container, "", v1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", grpc.ErrorDesc(err))
		return grpc.ErrorDesc(err), kuberuntime.ErrCreateContainerConfig
	}

	containerID, err := r.remoteCreateContainer(podSandboxID, containerConfig, podSandboxConfig)
	if err != nil {
		// m.recordContainerEvent(pod, container, containerID, v1.EventTypeWarning, events.FailedToCreateContainer, "Error: %v", grpc.ErrorDesc(err))
		return grpc.ErrorDesc(err), kuberuntime.ErrCreateContainer
	}

	// err = m.internalLifecycle.PreStartContainer(pod, container, containerID)
	// if err != nil {
	// 	// m.recordContainerEvent(pod, container, containerID, v1.EventTypeWarning, events.FailedToStartContainer, "Internal PreStartContainer hook failed: %v", grpc.ErrorDesc(err))
	// 	return grpc.ErrorDesc(err), kuberuntime.ErrPreStartHook
	// }
	// m.recordContainerEvent(pod, container, containerID, corev1.EventTypeNormal, events.CreatedContainer, "Created container")

	if ref != nil {
		r.containerRefManager.SetRef(kubecontainer.ContainerID{
			Type: r.runtimeName,
			ID:   containerID,
		}, ref)
	}

	// Step 3: start the container.
	err = r.remoteStartContainer(containerID)
	if err != nil {
		// m.recordContainerEvent(pod, container, containerID, v1.EventTypeWarning, events.FailedToStartContainer, "Error: %v", grpc.ErrorDesc(err))
		return grpc.ErrorDesc(err), kubecontainer.ErrRunContainer
	}
	// m.recordContainerEvent(pod, container, containerID, v1.EventTypeNormal, events.StartedContainer, "Started container")

	// Symlink container logs to the legacy container log location for cluster logging
	// support.
	// TODO(random-liu): Remove this after cluster logging supports CRI container log path.
	containerMeta := containerConfig.GetMetadata()
	sandboxMeta := podSandboxConfig.GetMetadata()
	legacySymlink := legacyLogSymlink(containerID, containerMeta.Name, sandboxMeta.Name,
		sandboxMeta.Namespace)
	containerLog := filepath.Join(podSandboxConfig.LogDirectory, containerConfig.LogPath)
	// only create legacy symlink if containerLog path exists (or the error is not IsNotExist).
	// Because if containerLog path does not exist, only dandling legacySymlink is created.
	// This dangling legacySymlink is later removed by container gc, so it does not make sense
	// to create it in the first place. it happens when journald logging driver is used with docker.
	if _, err := os.Stat(containerLog); !os.IsNotExist(err) {
		if err := os.Symlink(containerLog, legacySymlink); err != nil {
			klog.Errorf("Failed to create legacy symbolic link %q to container %q log %q: %v",
				legacySymlink, containerID, containerLog, err)
		}
	}

	// Step 4: execute the post start hook.
	if container.Lifecycle != nil && container.Lifecycle.PostStart != nil {
		kubeContainerID := kubecontainer.ContainerID{
			Type: r.runtimeName,
			ID:   containerID,
		}

		msg, handlerErr := r.Run(kubeContainerID, pod, container, container.Lifecycle.PostStart)
		if handlerErr != nil {
			// m.recordContainerEvent(pod, container, kubeContainerID.ID, corev1.EventTypeWarning, events.FailedPostStartHook, msg)
			if err := r.killContainer(pod, kubeContainerID, container.Name, "FailedPostStartHook", nil); err != nil {
				// klog.Errorf("Failed to kill container %q(id=%q) in pod %q: %v, %v",
				// 	container.Name, kubeContainerID.String(), format.Pod(pod), kuberuntime.ErrPostStartHook, err)
			}
			return msg, fmt.Errorf("%s: %v", kuberuntime.ErrPostStartHook, handlerErr)
		}
	}

	return "", nil
}

// createPodSandbox creates a pod sandbox and returns (podSandBoxID, error).
func (r *Runtime) createPodSandbox(pod *corev1.Pod, attempt uint32) (string, error) {
	podSandboxConfig, err := translatePodV1ToCRIPodConfig(pod, attempt)
	if err != nil {
		// message := fmt.Sprintf("GeneratePodSandboxConfig for pod %q failed: %v", format.Pod(pod), err)
		// klog.Error(message)
		return "", err
	}

	// Create pod logs directory
	err = os.MkdirAll(podSandboxConfig.LogDirectory, 0755)
	if err != nil {
		// message := fmt.Sprintf("Create pod log directory for pod %q failed: %v", format.Pod(pod), err)
		// klog.Errorf(message)
		return "", err
	}

	runtimeHandler := ""
	podSandBoxID, err := r.remoteRunPodSandbox(podSandboxConfig, runtimeHandler)
	if err != nil {
		// message := fmt.Sprintf("CreatePodSandbox for pod %q failed: %v", format.Pod(pod), err)
		// klog.Error(message)
		return "", err
	}

	return podSandBoxID, nil
}
