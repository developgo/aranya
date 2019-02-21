package pod

import (
	"time"

	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

const (
	idleTimeout           = time.Second * 30
	streamCreationTimeout = time.Second * 30
)

var (
	log = logf.Log.WithName("aranya.node.pod")
)

func NewManager(podLister kubeListersCoreV1.PodLister) *Manager {
	return &Manager{lister: podLister}
}

type Manager struct {
	lister kubeListersCoreV1.PodLister
}
