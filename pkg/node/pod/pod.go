package pod

import (
	"time"

	corev1 "k8s.io/api/core/v1"
	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
	kubeletpod "k8s.io/kubernetes/pkg/kubelet/pod"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	"arhat.dev/aranya/pkg/node/connectivity"
)

const (
	idleTimeout           = time.Second * 30
	streamCreationTimeout = time.Second * 30
)

var (
	log = logf.Log.WithName("aranya.node.pod")
)

func NewManager(podLister kubeListersCoreV1.PodLister, manager kubeletpod.Manager, remoteManager connectivity.Interface) *Manager {
	return &Manager{lister: podLister, Manager: manager, remoteManager: remoteManager}
}

type Manager struct {
	kubeletpod.Manager
	lister        kubeListersCoreV1.PodLister
	remoteManager connectivity.Interface
}

func (m *Manager) PodResourcesAreReclaimed(pod *corev1.Pod, status corev1.PodStatus) bool {
	return true
}
