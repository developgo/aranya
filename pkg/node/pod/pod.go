package pod

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	kubeListerCoreV1 "k8s.io/client-go/listers/core/v1"
	kubeletPod "k8s.io/kubernetes/pkg/kubelet/pod"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"

	connectivitySrv "arhat.dev/aranya/pkg/node/connectivity/server"
)

const (
	idleTimeout           = time.Second * 30
	streamCreationTimeout = time.Second * 30
)

var (
	log = logf.Log.WithName("aranya.node.pod")
)

func NewManager(ctx context.Context, podLister kubeListerCoreV1.PodLister, manager kubeletPod.Manager, remoteManager connectivitySrv.Interface) *Manager {
	return &Manager{ctx: ctx, lister: podLister, Manager: manager, remoteManager: remoteManager}
}

type Manager struct {
	kubeletPod.Manager
	lister        kubeListerCoreV1.PodLister
	remoteManager connectivitySrv.Interface
	ctx           context.Context
}

// PodResourcesAreReclaimed
// implements PodDeletionSafetyProvider
func (m *Manager) PodResourcesAreReclaimed(pod *corev1.Pod, status corev1.PodStatus) bool {
	return true
}
