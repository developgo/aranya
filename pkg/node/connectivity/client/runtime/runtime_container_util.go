package runtime

import (
	"fmt"
	"os"
	"strconv"

	"k8s.io/kubernetes/pkg/kubelet/kuberuntime"

	corev1 "k8s.io/api/core/v1"
	kubetypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
)

type annotatedContainerInfo struct {
	Hash                      uint64
	RestartCount              int
	PodDeletionGracePeriod    *int64
	PodTerminationGracePeriod *int64
	TerminationMessagePath    string
	TerminationMessagePolicy  corev1.TerminationMessagePolicy
	PreStopHandler            *corev1.Handler
	ContainerPorts            []corev1.ContainerPort
}

const (
	// TODO: change those label names to follow kubernetes's format
	podDeletionGracePeriodLabel    = "io.kubernetes.pod.deletionGracePeriod"
	podTerminationGracePeriodLabel = "io.kubernetes.pod.terminationGracePeriod"

	containerHashLabel                     = "io.kubernetes.container.hash"
	containerRestartCountLabel             = "io.kubernetes.container.restartCount"
	containerTerminationMessagePathLabel   = "io.kubernetes.container.terminationMessagePath"
	containerTerminationMessagePolicyLabel = "io.kubernetes.container.terminationMessagePolicy"
	containerPreStopHandlerLabel           = "io.kubernetes.container.preStopHandler"
	containerPortsLabel                    = "io.kubernetes.container.ports"
)

// translateContainerV1ToCRIContainerConfig generates container config for kubelet runtime corev1.Container
func translateContainerV1ToCRIContainerConfig(container *corev1.Container, pod *corev1.Pod, restartCount int, podIP, imageRef string, containerType kubecontainer.ContainerType) (*criRuntime.ContainerConfig, func(), error) {
	opts, cleanupAction, err := m.runtimeHelper.GenerateRunContainerOptions(pod, container, podIP)
	if err != nil {
		return nil, nil, err
	}

	uid, username, err := m.getImageUser(container.Image)
	if err != nil {
		return nil, cleanupAction, err
	}

	// Verify RunAsNonRoot. Non-root verification only supports numeric user.
	if err := verifyRunAsNonRoot(pod, container, uid, username); err != nil {
		return nil, cleanupAction, err
	}

	command, args := kubecontainer.ExpandContainerCommandAndArgs(container, opts.Envs)
	logDir := kuberuntime.BuildContainerLogsDirectory(kubetypes.UID(pod.UID), container.Name)
	err = os.MkdirAll(logDir, 0755)
	if err != nil {
		return nil, cleanupAction, fmt.Errorf("create container log directory for container %s failed: %v", container.Name, err)
	}
	containerLogsPath := buildContainerLogsPath(container.Name, restartCount)
	restartCountUint32 := uint32(restartCount)
	config := &criRuntime.ContainerConfig{
		Metadata: &criRuntime.ContainerMetadata{
			Name:    container.Name,
			Attempt: restartCountUint32,
		},
		Image:       &criRuntime.ImageSpec{Image: imageRef},
		Command:     command,
		Args:        args,
		WorkingDir:  container.WorkingDir,
		Labels:      newContainerLabels(container, pod, containerType),
		Annotations: newContainerAnnotations(container, pod, restartCount, opts),
		Devices:     makeDevices(opts),
		Mounts:      m.makeMounts(opts, container),
		LogPath:     containerLogsPath,
		Stdin:       container.Stdin,
		StdinOnce:   container.StdinOnce,
		Tty:         container.TTY,
	}

	// set platform specific configurations.
	if err := m.applyPlatformSpecificContainerConfig(config, container, pod, uid, username); err != nil {
		return nil, cleanupAction, err
	}

	// set environment variables
	envs := make([]*criRuntime.KeyValue, len(opts.Envs))
	for idx := range opts.Envs {
		e := opts.Envs[idx]
		envs[idx] = &criRuntime.KeyValue{
			Key:   e.Name,
			Value: e.Value,
		}
	}
	config.Envs = envs

	return config, cleanupAction, nil
}

// getContainerInfoFromAnnotations gets annotatedContainerInfo from annotations.
func getContainerInfoFromAnnotations(annotations map[string]string) *annotatedContainerInfo {
	var err error
	containerInfo := &annotatedContainerInfo{
		TerminationMessagePath:   getStringValueFromLabel(annotations, containerTerminationMessagePathLabel),
		TerminationMessagePolicy: corev1.TerminationMessagePolicy(getStringValueFromLabel(annotations, containerTerminationMessagePolicyLabel)),
	}

	if containerInfo.Hash, err = getUint64ValueFromLabel(annotations, containerHashLabel); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", containerHashLabel, annotations, err)
	}
	if containerInfo.RestartCount, err = getIntValueFromLabel(annotations, containerRestartCountLabel); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", containerRestartCountLabel, annotations, err)
	}
	if containerInfo.PodDeletionGracePeriod, err = getInt64PointerFromLabel(annotations, podDeletionGracePeriodLabel); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", podDeletionGracePeriodLabel, annotations, err)
	}
	if containerInfo.PodTerminationGracePeriod, err = getInt64PointerFromLabel(annotations, podTerminationGracePeriodLabel); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", podTerminationGracePeriodLabel, annotations, err)
	}

	preStopHandler := &corev1.Handler{}
	if found, err := getJSONObjectFromLabel(annotations, containerPreStopHandlerLabel, preStopHandler); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", containerPreStopHandlerLabel, annotations, err)
	} else if found {
		containerInfo.PreStopHandler = preStopHandler
	}

	var containerPorts []corev1.ContainerPort
	if found, err := getJSONObjectFromLabel(annotations, containerPortsLabel, &containerPorts); err != nil {
		// klog.Errorf("Unable to get %q from annotations %q: %v", containerPortsLabel, annotations, err)
	} else if found {
		containerInfo.ContainerPorts = containerPorts
	}

	return containerInfo
}

func getStringValueFromLabel(labels map[string]string, label string) string {
	if value, found := labels[label]; found {
		return value
	}
	// Do not report error, because there should be many old containers without label now.
	// Return empty string "" for these containers, the caller will get value by other ways.
	return ""
}

func getIntValueFromLabel(labels map[string]string, label string) (int, error) {
	if strValue, found := labels[label]; found {
		intValue, err := strconv.Atoi(strValue)
		if err != nil {
			// This really should not happen. Just set value to 0 to handle this abnormal case
			return 0, err
		}
		return intValue, nil
	}
	// Do not report error, because there should be many old containers without label now.
	// Just set the value to 0
	return 0, nil
}

func getUint64ValueFromLabel(labels map[string]string, label string) (uint64, error) {
	if strValue, found := labels[label]; found {
		intValue, err := strconv.ParseUint(strValue, 16, 64)
		if err != nil {
			// This really should not happen. Just set value to 0 to handle this abnormal case
			return 0, err
		}
		return intValue, nil
	}
	// Do not report error, because there should be many old containers without label now.
	// Just set the value to 0
	return 0, nil
}

func getInt64PointerFromLabel(labels map[string]string, label string) (*int64, error) {
	if strValue, found := labels[label]; found {
		int64Value, err := strconv.ParseInt(strValue, 10, 64)
		if err != nil {
			return nil, err
		}
		return &int64Value, nil
	}
	// If the label is not found, return pointer nil.
	return nil, nil
}

// getJSONObjectFromLabel returns a bool value indicating whether an object is found.
func getJSONObjectFromLabel(labels map[string]string, label string, value interface{}) (bool, error) {
	if strValue, found := labels[label]; found {
		err := json.Unmarshal([]byte(strValue), value)
		return found, err
	}
	// If the label is not found, return not found.
	return false, nil
}
