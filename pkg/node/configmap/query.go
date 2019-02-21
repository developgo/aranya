package configmap

import (
	corev1 "k8s.io/api/core/v1"
)

// Get configmap by configmap namespace and name.
func (m *Manager) GetConfigMap(namespace, name string) (*corev1.ConfigMap, error) {
	return m.lister.ConfigMaps(namespace).Get(name)
}