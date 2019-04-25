/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pod

import (
	"fmt"
	"strconv"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/connectivity"
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
			status := &corev1.ContainerStatus{
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

			statuses[i] = *status
		} else {
			statuses[i] = corev1.ContainerStatus{
				Name: ctr.Name,
			}
		}
	}

	return podPhase, statuses
}

func translatePodCreateOptions(pod *corev1.Pod, envs map[string]map[string]string, authConfigs map[string]*connectivity.AuthConfig, volumeData map[string]*connectivity.NamedData) *connectivity.CreateOptions {
	var (
		containers = make(map[string]*connectivity.ContainerSpec)
		ports      = make(map[string]*connectivity.ContainerPort)
		hostPaths  = make(map[string]string)
	)

	for _, vol := range pod.Spec.Volumes {
		if vol.HostPath != nil {
			hostPaths[vol.Name] = vol.HostPath.Path
		}
	}

	for _, ctr := range pod.Spec.Containers {
		for i, p := range ctr.Ports {
			if p.Name == "" {
				p.Name = strconv.FormatInt(int64(i), 10)
			}
			portName := fmt.Sprintf("%s/%s", ctr.Name, p.Name)

			ports[portName] = &connectivity.ContainerPort{
				Protocol:      string(p.Protocol),
				HostPort:      p.HostPort,
				ContainerPort: p.ContainerPort,
			}
		}

		mounts := make(map[string]*connectivity.MountOptions)
		for _, volMount := range ctr.VolumeMounts {
			mounts[volMount.Name] = &connectivity.MountOptions{
				MountPath: volMount.MountPath,
				SubPath:   volMount.SubPath,
				ReadOnly:  volMount.ReadOnly,
				Type:      "",
				Options:   nil,
			}
		}

		containers[ctr.Name] = &connectivity.ContainerSpec{
			Image:           ctr.Image,
			ImagePullPolicy: translateImagePullPolicy(ctr.ImagePullPolicy),

			Command: ctr.Command,
			Args:    ctr.Args,

			WorkingDir: ctr.WorkingDir,
			Stdin:      ctr.Stdin,
			Tty:        ctr.TTY,

			Envs:   envs[ctr.Name],
			Mounts: mounts,

			Security: translateContainerSecOpts(pod.Spec.SecurityContext, ctr.SecurityContext),
		}
	}

	return &connectivity.CreateOptions{
		PodUid:    string(pod.UID),
		Namespace: pod.Namespace,
		Name:      pod.Name,

		RestartPolicy: translateRestartPolicy(pod.Spec.RestartPolicy),

		HostIpc:     pod.Spec.HostIPC,
		HostNetwork: pod.Spec.HostNetwork,
		HostPid:     pod.Spec.HostPID,
		Hostname:    pod.Spec.Hostname,

		Containers:          containers,
		ImagePullAuthConfig: authConfigs,
		Ports:               ports,

		HostPaths:  hostPaths,
		VolumeData: volumeData,
	}
}

func translateContainerSecOpts(podSecOpts *corev1.PodSecurityContext, ctrSecOpts *corev1.SecurityContext) *connectivity.SecurityOptions {
	result := resolveCommonSecOpts(podSecOpts, ctrSecOpts)
	if result == nil || ctrSecOpts == nil {
		return result
	}

	if ctrSecOpts.Privileged != nil {
		result.Privileged = *ctrSecOpts.Privileged
	}

	if ctrSecOpts.AllowPrivilegeEscalation != nil {
		result.AllowNewPrivileges = *ctrSecOpts.AllowPrivilegeEscalation
	}

	if ctrSecOpts.ReadOnlyRootFilesystem != nil {
		result.ReadOnlyRootfs = *ctrSecOpts.ReadOnlyRootFilesystem
	}

	if ctrSecOpts.ProcMount != nil {
		switch *ctrSecOpts.ProcMount {
		case corev1.UnmaskedProcMount:
			result.ProcMountKind = connectivity.ProcMountUnmasked
		default:
			result.ProcMountKind = connectivity.ProcMountDefault
		}
	}

	if ctrSecOpts.Capabilities != nil {
		for _, capAdd := range ctrSecOpts.Capabilities.Add {
			result.CapsAdd = append(result.CapsAdd, string(capAdd))
		}

		for _, capDrop := range ctrSecOpts.Capabilities.Drop {
			result.CapsDrop = append(result.CapsDrop, string(capDrop))
		}
	}

	return result
}

func resolveCommonSecOpts(podSecOpts *corev1.PodSecurityContext, ctrSecOpts *corev1.SecurityContext) *connectivity.SecurityOptions {
	if podSecOpts == nil && ctrSecOpts != nil {
		return nil
	}

	return &connectivity.SecurityOptions{
		NonRoot: func() bool {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsNonRoot != nil:
				return *ctrSecOpts.RunAsNonRoot
			case podSecOpts != nil && podSecOpts.RunAsNonRoot != nil:
				return *podSecOpts.RunAsNonRoot
			}
			return false
		}(),
		User: func() int64 {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsUser != nil:
				return *ctrSecOpts.RunAsUser
			case podSecOpts != nil && podSecOpts.RunAsUser != nil:
				return *podSecOpts.RunAsUser
			}
			return -1
		}(),
		Group: func() int64 {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.RunAsGroup != nil:
				return *ctrSecOpts.RunAsGroup
			case podSecOpts != nil && podSecOpts.RunAsGroup != nil:
				return *podSecOpts.RunAsGroup
			}
			return -1
		}(),
		SelinuxOptions: func() *connectivity.SELinuxOptions {
			switch {
			case ctrSecOpts != nil && ctrSecOpts.SELinuxOptions != nil:
				return &connectivity.SELinuxOptions{
					Type:  ctrSecOpts.SELinuxOptions.Type,
					Level: ctrSecOpts.SELinuxOptions.Level,
					Role:  ctrSecOpts.SELinuxOptions.Role,
					User:  ctrSecOpts.SELinuxOptions.User,
				}
			case podSecOpts != nil && podSecOpts.SELinuxOptions != nil:
				return &connectivity.SELinuxOptions{
					Type:  podSecOpts.SELinuxOptions.Type,
					Level: podSecOpts.SELinuxOptions.Level,
					Role:  podSecOpts.SELinuxOptions.Role,
					User:  podSecOpts.SELinuxOptions.User,
				}
			}
			return nil
		}(),
	}
}

func translateImagePullPolicy(policy corev1.PullPolicy) connectivity.ImagePullPolicy {
	switch policy {
	case corev1.PullNever:
		return connectivity.ImagePullNever
	case corev1.PullIfNotPresent:
		return connectivity.ImagePullIfNotPresent
	case corev1.PullAlways:
		return connectivity.ImagePullAlways
	default:
		return connectivity.ImagePullNever
	}
}

func translateRestartPolicy(policy corev1.RestartPolicy) connectivity.RestartPolicy {
	switch policy {
	case corev1.RestartPolicyAlways:
		return connectivity.RestartAlways
	case corev1.RestartPolicyNever:
		return connectivity.RestartNever
	case corev1.RestartPolicyOnFailure:
		return connectivity.RestartOnFailure
	}
	return connectivity.RestartAlways
}
