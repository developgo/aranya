package configmap

import (
	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
)

func NewManager(lister kubeListersCoreV1.ConfigMapLister) *Manager {
	return &Manager{lister: lister}
}

type Manager struct {
	lister kubeListersCoreV1.ConfigMapLister
}
