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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/constant"
)

const (
	virtualContainerName  = "v-ctr"
	virtualContainerImage = "v-img"
	virtualContainerID    = "virtual://v-ctr-id"
)

func (m *Manager) createVirtualPod() {
	ns := constant.WatchNamespace()
	// best effort (user can create it if they want)
	_, err := m.kubeClient.CoreV1().Pods(ns).Create(newVirtualPod(ns, m.nodeName))
	if err != nil && !errors.IsAlreadyExists(err) {
		m.log.Info("failed to create virtual pod", "err", err.Error())
	}
}

func (m *Manager) deleteVirtualPod() {
	// best effort (user can delete it if they want)
	err := m.kubeClient.CoreV1().Pods(constant.WatchNamespace()).Delete(m.nodeName, metav1.NewDeleteOptions(0))
	if err != nil && !errors.IsNotFound(err) {
		m.log.Info("failed to delete virtual pod", "err", err.Error())
	}
}

func (m *Manager) updateVirtualPodToRunningPhase(pod *corev1.Pod) {
	now := metav1.Now()
	pod.Status = corev1.PodStatus{
		Phase: corev1.PodRunning,
		ContainerStatuses: []corev1.ContainerStatus{{
			Name: virtualContainerName,
			State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{StartedAt: now},
			},
			Ready:       true,
			Image:       virtualContainerImage,
			ContainerID: virtualContainerID,
		}},
		Conditions: []corev1.PodCondition{{
			Type:               corev1.ContainersReady,
			Status:             corev1.ConditionTrue,
			LastProbeTime:      now,
			LastTransitionTime: now,
			Reason:             "virtual pod created",
			Message:            "virtual pod created",
		}},
	}

	_, err := m.kubeClient.CoreV1().Pods(constant.WatchNamespace()).UpdateStatus(pod)
	if err != nil {
		m.log.Info("failed to update virtual pod", "err", err.Error())
	}
}

func newVirtualPod(ns, nodeName string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: ns,
			Name:      nodeName,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:  virtualContainerName,
				Image: virtualContainerImage,
			}},
			Tolerations: []corev1.Toleration{{
				Key:      constant.TaintKeyNamespace,
				Operator: corev1.TolerationOpEqual,
				Value:    nodeName,
			}},
			NodeName: nodeName,
		},
	}
}
