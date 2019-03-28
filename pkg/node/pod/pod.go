package pod

import (
	"context"
	"time"

	corev1 "k8s.io/api/core/v1"
	kubeListersCoreV1 "k8s.io/client-go/listers/core/v1"
	kubeletpod "k8s.io/kubernetes/pkg/kubelet/pod"
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

func NewManager(ctx context.Context, podLister kubeListersCoreV1.PodLister, manager kubeletpod.Manager, remoteManager connectivitySrv.Interface) *Manager {
	return &Manager{ctx: ctx, lister: podLister, Manager: manager, remoteManager: remoteManager}
}

type Manager struct {
	kubeletpod.Manager
	lister        kubeListersCoreV1.PodLister
	remoteManager connectivitySrv.Interface
	ctx           context.Context
}

// PodResourcesAreReclaimed
// implements PodDeletionSafetyProvider
func (m *Manager) PodResourcesAreReclaimed(pod *corev1.Pod, status corev1.PodStatus) bool {
	return true
}

func (m *Manager) SyncPodInDevice(pod *corev1.Pod) error {
	syncLog := log.WithValues("pod.ns", pod.Namespace, "pod.name", pod.Name)
	if pod.DeletionTimestamp != nil {
		if err := m.DeletePodInDevice(pod.Namespace, pod.Name); err != nil {
			syncLog.Error(err, "failed to delete pod in edge device")
			return err
		}
		return nil
	}

	if pod.Status.Phase == corev1.PodFailed || pod.Status.Phase == corev1.PodSucceeded {
		syncLog.Info("skipping sync of pod", "phase", pod.Status.Phase)
		return nil
	}

	if err := m.CreatePodInDevice(pod); err != nil {
		syncLog.Error(err, "failed to sync edge pod")
		return err
	}

	return nil
}
