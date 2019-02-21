package stats

import (
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
