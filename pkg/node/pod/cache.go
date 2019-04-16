package pod

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newCache() *Cache {
	return &Cache{
		m:  make(map[types.UID]*corev1.Pod),
		ma: make(map[types.NamespacedName]*corev1.Pod),
	}
}

type Cache struct {
	m  map[types.UID]*corev1.Pod
	ma map[types.NamespacedName]*corev1.Pod

	mu sync.RWMutex
}

func (c *Cache) Update(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	newPod := pod.DeepCopy()
	c.m[pod.UID] = newPod
	c.ma[types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}] = newPod
}

func (c *Cache) GetByID(podUID types.UID) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.m[podUID]
	if !ok {
		return nil, false
	}

	return pod, true
}

func (c *Cache) GetByName(namespace, name string) (*corev1.Pod, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	pod, ok := c.ma[types.NamespacedName{Namespace: namespace, Name: name}]
	if !ok {
		return nil, false
	}
	return pod, true
}

func (c *Cache) Delete(podUID types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()
}

func (c *Cache) Cleanup() {

}
