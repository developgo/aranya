package pod

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
)

func (m *Manager) GetMirrorPods() ([]*corev1.Pod, error) {
	return m.lister.List(labels.Everything())
}

func (m *Manager) GetMirrorPod(namespace, name string) (*corev1.Pod, error) {
	return m.lister.Pods(namespace).Get(name)
}
