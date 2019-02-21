package secret

import (
	corev1 "k8s.io/api/core/v1"
	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
)

type Manager struct {
	lister kubeListersCoreV1.SecretLister
}

func NewManager(lister kubeListersCoreV1.SecretLister) *Manager {
	return &Manager{lister: lister}
}

// Get secret by secret namespace and name.
func (m *Manager) GetSecret(namespace, name string) (*corev1.Secret, error) {
	return m.lister.Secrets(namespace).Get(name)
}
