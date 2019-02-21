package metrics

import (
	statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var (
	log = logf.Log.WithName("aranya.node.stats")
)

type Manager struct {
}

func NewManager() *Manager {
	return &Manager{}
}

func (m *Manager) GetStatsSummary() (*statsv1.Summary, error) {
	return nil, nil
}
