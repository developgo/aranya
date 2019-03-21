package containerd

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	utilValidation "k8s.io/apimachinery/pkg/util/validation"
	utilFeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/kubernetes/pkg/features"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletCmTypes "k8s.io/kubernetes/pkg/kubelet/cm"
	kubecontainer "k8s.io/kubernetes/pkg/kubelet/container"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
	kubeletTypes "k8s.io/kubernetes/pkg/kubelet/types"
	kubeVolMgrCache "k8s.io/kubernetes/pkg/kubelet/volumemanager/cache"
	"k8s.io/kubernetes/pkg/volume"
	"k8s.io/kubernetes/pkg/volume/util"
	"k8s.io/kubernetes/pkg/volume/util/operationexecutor"
	volumeUtilTypes "k8s.io/kubernetes/pkg/volume/util/types"
)

const (
	defaultRootDir       = "/var/lib/kubelet"
	podLogsRootDirectory = "/var/log/pods"
)

var (
	attachedVolumes = make(map[corev1.UniqueVolumeName]attachedVolume)
)

// verifySandboxStatus verified whether all required fields are set in remotePodSandboxStatus.
func verifySandboxStatus(status *criRuntime.PodSandboxStatus) error {
	if status.Id == "" {
		return fmt.Errorf("Id is not set ")
	}

	if status.Metadata == nil {
		return fmt.Errorf("Metadata is not set ")
	}

	metadata := status.Metadata
	if metadata.Name == "" || metadata.Namespace == "" || metadata.Uid == "" {
		return fmt.Errorf("Name, Namespace or Uid is not in metadata %q ", metadata)
	}

	if status.CreatedAt == 0 {
		return fmt.Errorf("CreatedAt is not set")
	}

	return nil
}

// ContainerToKillInfo contains necessary information to kill a container.
type ContainerToKillInfo struct {
	// The spec of the container.
	container *corev1.Container
	// The name of the container.
	name string
	// The message indicates why the container will be killed.
	message string
}

// PodActions keeps information what to do for a pod.
type PodActions struct {
	// Stop all running (regular and init) containers and the sandbox for the pod.
	KillPod bool
	// Whether need to create a new sandbox. If needed to kill pod and create a
	// a new pod sandbox, all init containers need to be purged (i.e., removed).
	CreateSandbox bool
	// The id of existing sandbox. It is used for starting containers in ContainersToStart.
	SandboxID string
	// The attempt number of creating sandboxes for the pod.
	Attempt uint32

	// The next init container to start.
	NextInitContainerToStart *corev1.Container
	// ContainersToStart keeps a list of indexes for the containers to start,
	// where the index is the index of the specific container in the pod spec (
	// pod.Spec.Containers.
	ContainersToStart []int
	// ContainersToKill keeps a map of containers that need to be killed, note that
	// the key is the container ID of the container, while
	// the value contains necessary information to kill a container.
	ContainersToKill map[kubecontainer.ContainerID]ContainerToKillInfo
}

// podSandboxChanged checks whether the spec of the pod is changed and returns
// (changed, new attempt, original sandboxID if exist).
func podSandboxChanged(pod *corev1.Pod, podStatus *kubecontainer.PodStatus) (changed bool, attempt uint32, originID string) {
	if len(podStatus.SandboxStatuses) == 0 {
		return true, 0, ""
	}

	readySandboxCount := 0
	for _, s := range podStatus.SandboxStatuses {
		if s.State == criRuntime.PodSandboxState_SANDBOX_READY {
			readySandboxCount++
		}
	}

	// Needs to create a new sandbox when readySandboxCount > 1 or the ready sandbox is not the latest one.
	sandboxStatus := podStatus.SandboxStatuses[0]
	if readySandboxCount > 1 {
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	if sandboxStatus.State != criRuntime.PodSandboxState_SANDBOX_READY {
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	// Needs to create a new sandbox when network namespace changed.
	if sandboxStatus.GetLinux().GetNamespaces().GetOptions().GetNetwork() != networkNamespaceForPod(pod) {
		return true, sandboxStatus.Metadata.Attempt + 1, ""
	}

	// Needs to create a new sandbox when the sandbox does not have an IP address.
	if !kubecontainer.IsHostNetworkPod(pod) && sandboxStatus.Network.Ip == "" {
		return true, sandboxStatus.Metadata.Attempt + 1, sandboxStatus.Id
	}

	return false, sandboxStatus.Metadata.Attempt, sandboxStatus.Id
}

func shouldRestartOnFailure(pod *corev1.Pod) bool {
	return pod.Spec.RestartPolicy != corev1.RestartPolicyNever
}

func containerSucceeded(c *corev1.Container, podStatus *kubecontainer.PodStatus) bool {
	cStatus := podStatus.FindContainerStatusByName(c.Name)
	if cStatus == nil || cStatus.State == kubecontainer.ContainerStateRunning {
		return false
	}
	return cStatus.ExitCode == 0
}

// isContainerFailed returns true if container has exited and exitcode is not zero.
func isContainerFailed(status *kubecontainer.ContainerStatus) bool {
	if status.State == kubecontainer.ContainerStateExited && status.ExitCode != 0 {
		return true
	}

	return false
}

// findNextInitContainerToRun returns the status of the last failed container, the
// next init container to start, or done if there are no further init containers.
// remoteStatus is only returned if an init container is failed, in which case next will
// point to the current container.
func findNextInitContainerToRun(pod *corev1.Pod, podStatus *kubecontainer.PodStatus) (status *kubecontainer.ContainerStatus, next *corev1.Container, done bool) {
	if len(pod.Spec.InitContainers) == 0 {
		return nil, nil, true
	}

	// If there are failed containers, return the status of the last failed one.
	for i := len(pod.Spec.InitContainers) - 1; i >= 0; i-- {
		container := &pod.Spec.InitContainers[i]
		status := podStatus.FindContainerStatusByName(container.Name)
		if status != nil && isContainerFailed(status) {
			return status, container, false
		}
	}

	// There are no failed containers now.
	for i := len(pod.Spec.InitContainers) - 1; i >= 0; i-- {
		container := &pod.Spec.InitContainers[i]
		status := podStatus.FindContainerStatusByName(container.Name)
		if status == nil {
			continue
		}

		// container is still running, return not done.
		if status.State == kubecontainer.ContainerStateRunning {
			return nil, nil, false
		}

		if status.State == kubecontainer.ContainerStateExited {
			// all init containers successful
			if i == (len(pod.Spec.InitContainers) - 1) {
				return nil, nil, true
			}

			// all containers up to i successful, go to i+1
			return nil, &pod.Spec.InitContainers[i+1], false
		}
	}

	return nil, &pod.Spec.InitContainers[0], false
}

func containerChanged(container *corev1.Container, containerStatus *kubecontainer.ContainerStatus) (uint64, uint64, bool) {
	expectedHash := kubecontainer.HashContainer(container)
	return expectedHash, containerStatus.Hash, containerStatus.Hash != expectedHash
}

// computePodActions checks whether the pod spec has changed and returns the changes if true.
func computePodActions(pod *corev1.Pod, podStatus *kubecontainer.PodStatus) PodActions {
	createPodSandbox, attempt, sandboxID := podSandboxChanged(pod, podStatus)
	changes := PodActions{
		KillPod:           createPodSandbox,
		CreateSandbox:     createPodSandbox,
		SandboxID:         sandboxID,
		Attempt:           attempt,
		ContainersToStart: []int{},
		ContainersToKill:  make(map[kubecontainer.ContainerID]ContainerToKillInfo),
	}

	// If we need to (re-)create the pod sandbox, everything will need to be
	// killed and recreated, and init containers should be purged.
	if createPodSandbox {
		if !shouldRestartOnFailure(pod) && attempt != 0 {
			// Should not restart the pod, just return.
			// we should not create a sandbox for a pod if it is already done.
			// if all containers are done and should not be started, there is no need to create a new sandbox.
			// this stops confusing logs on pods whose containers all have exit codes, but we recreate a sandbox before terminating it.
			changes.CreateSandbox = false
			return changes
		}
		if len(pod.Spec.InitContainers) != 0 {
			// Pod has init containers, return the first one.
			changes.NextInitContainerToStart = &pod.Spec.InitContainers[0]
			return changes
		}
		// Start all containers by default but exclude the ones that succeeded if
		// RestartPolicy is OnFailure.
		for idx, c := range pod.Spec.Containers {
			if containerSucceeded(&c, podStatus) && pod.Spec.RestartPolicy == corev1.RestartPolicyOnFailure {
				continue
			}
			changes.ContainersToStart = append(changes.ContainersToStart, idx)
		}
		return changes
	}

	// Check initialization progress.
	initLastStatus, next, done := findNextInitContainerToRun(pod, podStatus)
	if !done {
		if next != nil {
			initFailed := initLastStatus != nil && isContainerFailed(initLastStatus)
			if initFailed && !shouldRestartOnFailure(pod) {
				changes.KillPod = true
			} else {
				changes.NextInitContainerToStart = next
			}
		}
		// Initialization failed or still in progress. Skip inspecting non-init
		// containers.
		return changes
	}

	// Number of running containers to keep.
	keepCount := 0
	// check the status of containers.
	for idx, container := range pod.Spec.Containers {
		containerStatus := podStatus.FindContainerStatusByName(container.Name)

		// Call internal container post-stop lifecycle hook for any non-running container so that any
		// allocated cpus are released immediately. If the container is restarted, cpus will be re-allocated
		// to it.
		if containerStatus != nil && containerStatus.State != kubecontainer.ContainerStateRunning {
			// if err := m.internalLifecycle.PostStopContainer(containerStatus.ID.ID); err != nil {
			// 	klog.Errorf("internal container post-stop lifecycle hook failed for container %v in pod %v with error %v",
			// 		container.Name, pod.Name, err)
			// }
		}

		// If container does not exist, or is not running, check whether we
		// need to restart it.
		if containerStatus == nil || containerStatus.State != kubecontainer.ContainerStateRunning {
			if kubecontainer.ShouldContainerBeRestarted(&container, pod, podStatus) {
				// message := fmt.Sprintf("Container %+v is dead, but RestartPolicy says that we should restart it.", container)
				// klog.V(3).Infof(message)
				changes.ContainersToStart = append(changes.ContainersToStart, idx)
			}
			continue
		}
		// The container is running, but kill the container if any of the following condition is met.
		reason := ""
		restart := shouldRestartOnFailure(pod)
		if expectedHash, actualHash, changed := containerChanged(&container, containerStatus); changed {
			reason = fmt.Sprintf("Container spec hash changed (%d vs %d).", actualHash, expectedHash)
			// Restart regardless of the restart policy because the container
			// spec changed.
			restart = true
		} else {
			// Keep the container.
			keepCount++
			continue
		}

		// We need to kill the container, but if we also want to restart the
		// container afterwards, make the intent clear in the message. Also do
		// not kill the entire pod since we expect container to be running eventually.
		message := reason
		if restart {
			message = fmt.Sprintf("%s. Container will be killed and recreated.", message)
			changes.ContainersToStart = append(changes.ContainersToStart, idx)
		}

		changes.ContainersToKill[containerStatus.ID] = ContainerToKillInfo{
			name:      containerStatus.Name,
			container: &pod.Spec.Containers[idx],
			message:   message,
		}
		// klog.V(2).Infof("Container %q (%q) of pod %s: %s", container.Name, containerStatus.ID, format.Pod(pod), message)
	}

	if keepCount == 0 && len(changes.ContainersToStart) == 0 {
		changes.KillPod = true
	}

	return changes
}

func translatePodV1ToCRIPodConfig(pod *corev1.Pod, attempt uint32) (*criRuntime.PodSandboxConfig, error) {
	podUID := string(pod.UID)
	podSandboxConfig := &criRuntime.PodSandboxConfig{
		Metadata: &criRuntime.PodSandboxMetadata{
			Name:      pod.Name,
			Namespace: pod.Namespace,
			Uid:       podUID,
			Attempt:   attempt,
		},
		Labels:      newPodLabels(pod),
		Annotations: pod.Annotations,
	}

	// dnsConfig, err := m.runtimeHelper.GetPodDNS(pod)
	// if err != nil {
	// 	return nil, err
	// }
	// podSandboxConfig.DnsConfig = dnsConfig

	if !kubeletContainer.IsHostNetworkPod(pod) {
		hostname, _, err := generatePodHostNameAndDomain(pod)
		if err != nil {
			return nil, err
		}
		podSandboxConfig.Hostname = hostname
	}

	logDir := buildPodLogsDirectory(pod.UID)
	podSandboxConfig.LogDirectory = logDir

	var portMappings []*criRuntime.PortMapping
	{
	}
	for _, c := range pod.Spec.Containers {
		containerPortMappings := kubeletContainer.MakePortMappings(&c)

		for idx := range containerPortMappings {
			port := containerPortMappings[idx]
			hostPort := int32(port.HostPort)
			containerPort := int32(port.ContainerPort)
			protocol := toRuntimeProtocol(port.Protocol)
			portMappings = append(portMappings, &criRuntime.PortMapping{
				HostIp:        port.HostIP,
				HostPort:      hostPort,
				ContainerPort: containerPort,
				Protocol:      protocol,
			})
		}

	}
	if len(portMappings) > 0 {
		podSandboxConfig.PortMappings = portMappings
	}

	lc, err := generatePodSandboxLinuxConfig(pod)
	if err != nil {
		return nil, err
	}
	podSandboxConfig.Linux = lc

	return podSandboxConfig, nil
}

// newPodLabels creates pod labels from v1.Pod.
func newPodLabels(pod *corev1.Pod) map[string]string {
	labels := map[string]string{}

	// Get labels from v1.Pod
	for k, v := range pod.Labels {
		labels[k] = v
	}

	labels[kubeletTypes.KubernetesPodNameLabel] = pod.Name
	labels[kubeletTypes.KubernetesPodNamespaceLabel] = pod.Namespace
	labels[kubeletTypes.KubernetesPodUIDLabel] = string(pod.UID)

	return labels
}

// generatePodHostNameAndDomain creates a hostname and domain name for a pod,
// given that pod's spec and annotations or returns an error.
func generatePodHostNameAndDomain(pod *corev1.Pod) (string, string, error) {
	clusterDomain := "local"

	hostname := pod.Name
	if len(pod.Spec.Hostname) > 0 {
		if msgs := utilValidation.IsDNS1123Label(pod.Spec.Hostname); len(msgs) != 0 {
			return "", "", fmt.Errorf("Pod Hostname %q is not a valid DNS label: %s ", pod.Spec.Hostname, strings.Join(msgs, ";"))
		}
		hostname = pod.Spec.Hostname
	}

	hostname, err := truncatePodHostnameIfNeeded(pod.Name, hostname)
	if err != nil {
		return "", "", err
	}

	hostDomain := ""
	if len(pod.Spec.Subdomain) > 0 {
		if msgs := utilValidation.IsDNS1123Label(pod.Spec.Subdomain); len(msgs) != 0 {
			return "", "", fmt.Errorf("Pod Subdomain %q is not a valid DNS label: %s ", pod.Spec.Subdomain, strings.Join(msgs, ";"))
		}
		hostDomain = fmt.Sprintf("%s.%s.svc.%s", pod.Spec.Subdomain, pod.Namespace, clusterDomain)
	}

	return hostname, hostDomain, nil
}

// truncatePodHostnameIfNeeded truncates the pod hostname if it's longer than 63 chars.
func truncatePodHostnameIfNeeded(podName, hostname string) (string, error) {
	// Cap hostname at 63 chars (specification is 64bytes which is 63 chars and the null terminating char).
	const hostnameMaxLen = 63
	if len(hostname) <= hostnameMaxLen {
		return hostname, nil
	}
	truncated := hostname[:hostnameMaxLen]

	// hostname should not end with '-' or '.'
	truncated = strings.TrimRight(truncated, "-.")
	if len(truncated) == 0 {
		// This should never happen.
		return "", fmt.Errorf("hostname for pod %q was invalid: %q", podName, hostname)
	}
	return truncated, nil
}

// buildPodLogsDirectory builds absolute log directory path for a pod sandbox.
func buildPodLogsDirectory(podUID types.UID) string {
	return filepath.Join(podLogsRootDirectory, string(podUID))
}

// toRuntimeProtocol converts v1.Protocol to criRuntime.Protocol.
func toRuntimeProtocol(protocol corev1.Protocol) criRuntime.Protocol {
	switch protocol {
	case corev1.ProtocolTCP:
		return criRuntime.Protocol_TCP
	case corev1.ProtocolUDP:
		return criRuntime.Protocol_UDP
	case corev1.ProtocolSCTP:
		return criRuntime.Protocol_SCTP
	}

	return criRuntime.Protocol_TCP
}

// generatePodSandboxLinuxConfig generates LinuxPodSandboxConfig from v1.Pod.
func generatePodSandboxLinuxConfig(pod *corev1.Pod) (*criRuntime.LinuxPodSandboxConfig, error) {
	cgroupParent := getPodCgroupParent(pod)
	lc := &criRuntime.LinuxPodSandboxConfig{
		CgroupParent: cgroupParent,
		SecurityContext: &criRuntime.LinuxSandboxSecurityContext{
			Privileged:         kubeletContainer.HasPrivilegedContainer(pod),
			SeccompProfilePath: getSeccompProfileFromAnnotations(pod.Annotations, ""),
		},
	}

	sysctls := make(map[string]string)
	if utilFeature.DefaultFeatureGate.Enabled(features.Sysctls) {
		if pod.Spec.SecurityContext != nil {
			for _, c := range pod.Spec.SecurityContext.Sysctls {
				sysctls[c.Name] = c.Value
			}
		}
	}

	lc.Sysctls = sysctls

	if pod.Spec.SecurityContext != nil {
		sc := pod.Spec.SecurityContext
		if sc.RunAsUser != nil {
			lc.SecurityContext.RunAsUser = &criRuntime.Int64Value{Value: int64(*sc.RunAsUser)}
		}
		if sc.RunAsGroup != nil {
			lc.SecurityContext.RunAsGroup = &criRuntime.Int64Value{Value: int64(*sc.RunAsGroup)}
		}
		lc.SecurityContext.NamespaceOptions = namespacesForPod(pod)

		if sc.FSGroup != nil {
			lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, int64(*sc.FSGroup))
		}
		if groups := getExtraSupplementalGroupsForPod(pod); len(groups) > 0 {
			lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, groups...)
		}
		if sc.SupplementalGroups != nil {
			for _, sg := range sc.SupplementalGroups {
				lc.SecurityContext.SupplementalGroups = append(lc.SecurityContext.SupplementalGroups, int64(sg))
			}
		}
		if sc.SELinuxOptions != nil {
			lc.SecurityContext.SelinuxOptions = &criRuntime.SELinuxOption{
				User:  sc.SELinuxOptions.User,
				Role:  sc.SELinuxOptions.Role,
				Type:  sc.SELinuxOptions.Type,
				Level: sc.SELinuxOptions.Level,
			}
		}
	}

	return lc, nil
}

// namespacesForPod returns the criRuntime.NamespaceOption for a given pod.
// An empty or nil pod can be used to get the namespace defaults for v1.Pod.
func namespacesForPod(pod *corev1.Pod) *criRuntime.NamespaceOption {
	return &criRuntime.NamespaceOption{
		Ipc:     ipcNamespaceForPod(pod),
		Network: networkNamespaceForPod(pod),
		Pid:     pidNamespaceForPod(pod),
	}
}

func ipcNamespaceForPod(pod *corev1.Pod) criRuntime.NamespaceMode {
	if pod != nil && pod.Spec.HostIPC {
		return criRuntime.NamespaceMode_NODE
	}
	return criRuntime.NamespaceMode_POD
}

func networkNamespaceForPod(pod *corev1.Pod) criRuntime.NamespaceMode {
	if pod != nil && pod.Spec.HostNetwork {
		return criRuntime.NamespaceMode_NODE
	}
	return criRuntime.NamespaceMode_POD
}

func pidNamespaceForPod(pod *corev1.Pod) criRuntime.NamespaceMode {
	if pod != nil {
		if pod.Spec.HostPID {
			return criRuntime.NamespaceMode_NODE
		}
		if utilFeature.DefaultFeatureGate.Enabled(features.PodShareProcessNamespace) && pod.Spec.ShareProcessNamespace != nil && *pod.Spec.ShareProcessNamespace {
			return criRuntime.NamespaceMode_POD
		}
	}
	// Note that PID does not default to the zero value for v1.Pod
	return criRuntime.NamespaceMode_CONTAINER
}

func getExtraSupplementalGroupsForPod(pod *corev1.Pod) []int64 {
	podName := util.GetUniquePodName(pod)
	supplementalGroups := sets.NewString()

	for _, mountedVolume := range getMountedVolumesForPod(podName) {
		if mountedVolume.VolumeGidValue != "" {
			supplementalGroups.Insert(mountedVolume.VolumeGidValue)
		}
	}

	result := make([]int64, 0, supplementalGroups.Len())
	for _, group := range supplementalGroups.List() {
		iGroup, extra := getExtraSupplementalGid(group, pod)
		if !extra {
			continue
		}

		result = append(result, int64(iGroup))
	}

	return result
}

func getMountedVolumesForPod(podName volumeUtilTypes.UniquePodName) []kubeVolMgrCache.MountedVolume {
	mountedVolume := make([]kubeVolMgrCache.MountedVolume, 0, len(attachedVolumes))

	for _, volumeObj := range attachedVolumes {
		for mountedPodName, podObj := range volumeObj.mountedPods {
			if mountedPodName == podName {
				mountedVolume = append(
					mountedVolume,
					getMountedVolume(&podObj, &volumeObj))
			}
		}
	}

	return mountedVolume
}

// attachedVolume represents a volume the kubelet volume manager believes to be
// successfully attached to a node it is managing. Volume types that do not
// implement an attacher are assumed to be in this state.
type attachedVolume struct {
	// volumeName contains the unique identifier for this volume.
	volumeName corev1.UniqueVolumeName

	// mountedPods is a map containing the set of pods that this volume has been
	// successfully mounted to. The key in this map is the name of the pod and
	// the value is a mountedPod object containing more information about the
	// pod.
	mountedPods map[volumeUtilTypes.UniquePodName]mountedPod

	// spec is the volume spec containing the specification for this volume.
	// Used to generate the volume plugin object, and passed to plugin methods.
	// In particular, the Unmount method uses spec.Name() as the volumeSpecName
	// in the mount path:
	// /var/lib/kubelet/pods/{podUID}/volumes/{escapeQualifiedPluginName}/{volumeSpecName}/
	spec *volume.Spec

	// pluginName is the Unescaped Qualified name of the volume plugin used to
	// attach and mount this volume. It is stored separately in case the full
	// volume spec (everything except the name) can not be reconstructed for a
	// volume that should be unmounted (which would be the case for a mount path
	// read from disk without a full volume spec).
	pluginName string

	// pluginIsAttachable indicates the volume plugin used to attach and mount
	// this volume implements the volume.Attacher interface
	pluginIsAttachable bool

	// globallyMounted indicates that the volume is mounted to the underlying
	// device at a global mount point. This global mount point must be unmounted
	// prior to detach.
	globallyMounted bool

	// devicePath contains the path on the node where the volume is attached for
	// attachable volumes
	devicePath string

	// deviceMountPath contains the path on the node where the device should
	// be mounted after it is attached.
	deviceMountPath string
}

// The mountedPod object represents a pod for which the kubelet volume manager
// believes the underlying volume has been successfully been mounted.
type mountedPod struct {
	// the name of the pod
	podName volumeUtilTypes.UniquePodName

	// the UID of the pod
	podUID types.UID

	// mounter used to mount
	mounter volume.Mounter

	// mapper used to block volumes support
	blockVolumeMapper volume.BlockVolumeMapper

	// spec is the volume spec containing the specification for this volume.
	// Used to generate the volume plugin object, and passed to plugin methods.
	// In particular, the Unmount method uses spec.Name() as the volumeSpecName
	// in the mount path:
	// /var/lib/kubelet/pods/{podUID}/volumes/{escapeQualifiedPluginName}/{volumeSpecName}/
	volumeSpec *volume.Spec

	// outerVolumeSpecName is the volume.Spec.Name() of the volume as referenced
	// directly in the pod. If the volume was referenced through a persistent
	// volume claim, this contains the volume.Spec.Name() of the persistent
	// volume claim
	outerVolumeSpecName string

	// remountRequired indicates the underlying volume has been successfully
	// mounted to this pod but it should be remounted to reflect changes in the
	// referencing pod.
	// Atomically updating volumes depend on this to update the contents of the
	// volume. All volume mounting calls should be idempotent so a second mount
	// call for volumes that do not need to update contents should not fail.
	remountRequired bool

	// volumeGidValue contains the value of the GID annotation, if present.
	volumeGidValue string

	// fsResizeRequired indicates the underlying volume has been successfully
	// mounted to this pod but its size has been expanded after that.
	fsResizeRequired bool
}

// getMountedVolume constructs and returns a MountedVolume object from the given
// mountedPod and attachedVolume objects.
func getMountedVolume(mountedPod *mountedPod, attachedVolume *attachedVolume) kubeVolMgrCache.MountedVolume {
	return kubeVolMgrCache.MountedVolume{
		MountedVolume: operationexecutor.MountedVolume{
			PodName:             mountedPod.podName,
			VolumeName:          attachedVolume.volumeName,
			InnerVolumeSpecName: mountedPod.volumeSpec.Name(),
			OuterVolumeSpecName: mountedPod.outerVolumeSpecName,
			PluginName:          attachedVolume.pluginName,
			PodUID:              mountedPod.podUID,
			Mounter:             mountedPod.mounter,
			BlockVolumeMapper:   mountedPod.blockVolumeMapper,
			VolumeGidValue:      mountedPod.volumeGidValue,
			VolumeSpec:          mountedPod.volumeSpec,
			DeviceMountPath:     attachedVolume.deviceMountPath}}
}

// getExtraSupplementalGid returns the value of an extra supplemental GID as
// defined by an annotation on a volume and a boolean indicating whether the
// volume defined a GID that the pod doesn't already request.
func getExtraSupplementalGid(volumeGidValue string, pod *corev1.Pod) (int64, bool) {
	if volumeGidValue == "" {
		return 0, false
	}

	gid, err := strconv.ParseInt(volumeGidValue, 10, 64)
	if err != nil {
		return 0, false
	}

	if pod.Spec.SecurityContext != nil {
		for _, existingGid := range pod.Spec.SecurityContext.SupplementalGroups {
			if gid == int64(existingGid) {
				return 0, false
			}
		}
	}

	return gid, true
}

// getSeccompProfileFromAnnotations gets seccomp profile from annotations.
// It gets pod's profile if containerName is empty.
func getSeccompProfileFromAnnotations(annotations map[string]string, containerName string) string {
	// try the pod profile.
	profile, profileOK := annotations[corev1.SeccompPodAnnotationKey]
	if containerName != "" {
		// try the container profile.
		cProfile, cProfileOK := annotations[corev1.SeccompContainerAnnotationKeyPrefix+containerName]
		if cProfileOK {
			profile = cProfile
			profileOK = cProfileOK
		}
	}

	if !profileOK {
		return ""
	}

	if strings.HasPrefix(profile, "localhost/") {
		name := strings.TrimPrefix(profile, "localhost/")
		fname := filepath.Join(defaultRootDir, "seccomp", filepath.FromSlash(name))
		return "localhost/" + fname
	}

	return profile
}

// getPodCgroupParent gets pod cgroup parent from container manager.
func getPodCgroupParent(pod *corev1.Pod) string {
	_, cgroupParent := getPodContainerName(pod)
	return cgroupParent
}

// getPodContainerName returns the CgroupName identifier, and its literal cgroupfs form on the host.
func getPodContainerName(pod *corev1.Pod) (kubeletCmTypes.CgroupName, string) {
	// always use root cgroup
	rootCgroup := kubeletCmTypes.CgroupName{"/"}

	// // Get the parent QOS container name
	// podQOS := corev1HelperQos.GetPodQOS(pod)

	// var parentContainer kubeletCmTypes.CgroupName
	// switch podQOS {
	// case corev1.PodQOSGuaranteed:
	// 	parentContainer = rootCgroup
	// case corev1.PodQOSBurstable:
	// 	parentContainer = rootCgroup
	// case corev1.PodQOSBestEffort:
	// 	parentContainer = rootCgroup
	// }
	podContainer := kubeletCmTypes.GetPodCgroupNameSuffix(pod.UID)

	// Get the absolute path of the cgroup
	cgroupName := kubeletCmTypes.NewCgroupName(rootCgroup, podContainer)
	// Get the literal cgroupfs name
	// always use cgroupfs instead of systemd
	return cgroupName, cgroupName.ToCgroupfs()
}
