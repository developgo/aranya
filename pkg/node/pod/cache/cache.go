package cache

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func NewPodCache() *PodCache {
	return &PodCache{
		uidMap:  make(map[types.UID]*corev1.Pod),
		nameMap: make(map[types.NamespacedName]*corev1.Pod),
	}
}

type PodCache struct {
	uidMap  map[types.UID]*corev1.Pod
	nameMap map[types.NamespacedName]*corev1.Pod

	mu sync.RWMutex
}

func (c *PodCache) Update(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newPod := pod.DeepCopy()
	c.uidMap[pod.UID] = newPod
	c.nameMap[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}] = newPod
}

func (c *PodCache) GetByID(podUID types.UID) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.uidMap[podUID]
	if !ok {
		return nil, false
	}

	return pod, true
}

func (c *PodCache) GetByName(namespace, name string) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.nameMap[types.NamespacedName{Namespace: namespace, Name: name}]
	if !ok {
		return nil, false
	}
	return pod, true
}

func (c *PodCache) Delete(podUID types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()
}

func (c *PodCache) Cleanup() {

}
