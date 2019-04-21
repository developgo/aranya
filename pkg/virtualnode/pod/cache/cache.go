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

// PodCache
type PodCache struct {
	uidMap  map[types.UID]*corev1.Pod
	nameMap map[types.NamespacedName]*corev1.Pod

	mu sync.RWMutex
}

func (c *PodCache) Update(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.uidMap[pod.UID] = pod
	c.nameMap[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}] = pod
}

func (c *PodCache) GetByID(podUID types.UID) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.uidMap[podUID]
	if !ok {
		return nil, false
	}

	// deep copy before return, so the caller can make any changes to that pod object
	return pod.DeepCopy(), true
}

func (c *PodCache) GetByName(namespace, name string) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.nameMap[types.NamespacedName{Namespace: namespace, Name: name}]
	if !ok {
		return nil, false
	}

	// deep copy before return, so the caller can make any changes to that pod object
	return pod.DeepCopy(), true
}

func (c *PodCache) Delete(podUID types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if pod, ok := c.uidMap[podUID]; ok {
		delete(c.nameMap, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
		delete(c.uidMap, podUID)
	}
}

func (c *PodCache) Cleanup() {
	c.mu.Lock()
	defer c.mu.Unlock()

	var uidAll []types.UID
	for uid, pod := range c.uidMap {
		uidAll = append(uidAll, uid)
		delete(c.nameMap, types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name})
	}

	for _, uid := range uidAll {
		delete(c.uidMap, uid)
	}
}