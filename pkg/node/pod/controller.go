package pod

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"

	"arhat.dev/aranya/pkg/node/pod/queue"
)

func (m *Manager) onPodCreate(newObj interface{}) {
	newPod := newObj.(*corev1.Pod)
	m.podWorkQueue.Offer(queue.ActionCreate, newPod.UID)
}

func (m *Manager) onPodUpdate(oldObj, newObj interface{}) {
	oldPod := oldObj.(*corev1.Pod).DeepCopy()
	newPod := newObj.(*corev1.Pod).DeepCopy()

	newPod.ResourceVersion = oldPod.ResourceVersion
	if reflect.DeepEqual(oldPod.ObjectMeta, newPod.ObjectMeta) && reflect.DeepEqual(oldPod.Spec, newPod.Spec) {
		log.Info("skip enqueue pod update")
		return
	}

	podDeleted := !(newPod.GetDeletionTimestamp() == nil || newPod.GetDeletionTimestamp().IsZero())
	if podDeleted {
		m.podWorkQueue.Offer(queue.ActionDelete, newPod.UID)
		return
	}

	m.podWorkQueue.Offer(queue.ActionUpdate, newPod.UID)
}

func (m *Manager) onPodDelete(oldObj interface{}) {
	oldPod := oldObj.(*corev1.Pod)
	m.podWorkQueue.Offer(queue.ActionDelete, oldPod.UID)
}
