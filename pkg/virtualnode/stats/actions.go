package stats

import (
	statsv1 "k8s.io/kubernetes/pkg/kubelet/apis/stats/v1alpha1"
)

func (m *Manager) GetStatsSummary() (*statsv1.Summary, error) {
	return &statsv1.Summary{}, nil
}
