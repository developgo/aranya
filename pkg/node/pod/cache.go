package pod

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

func newCache() *Cache {
	return &Cache{
		m: make(map[types.UID]*corev1.Pod),
	}
}

type Cache struct {
	m  map[types.UID]*corev1.Pod
	mu sync.RWMutex
}

func (c *Cache) Update(pod *corev1.Pod) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.m[pod.UID] = pod.DeepCopy()
}

func (c *Cache) Get(podUID types.UID) *corev1.Pod {
	c.mu.RLock()
	defer c.mu.RUnlock()

	return c.m[podUID]
}

func (c *Cache) Delete(podUID types.UID) {
	c.mu.Lock()
	defer c.mu.Unlock()
}
