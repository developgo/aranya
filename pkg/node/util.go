package node

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"
)

func newPodCache() *PodCache {
	return &PodCache{
		m: make(map[types.NamespacedName]*PodPair),
	}
}

type PodPair struct {
	pod    *corev1.Pod
	status *kubeletContainer.PodStatus
}

type PodCache struct {
	// pod full name to kubeletContainer.PodPair
	m  map[types.NamespacedName]*PodPair
	mu sync.RWMutex
}

func (c *PodCache) Update(pod *corev1.Pod, status *kubeletContainer.PodStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	name := types.NamespacedName{Namespace: pod.Namespace, Name: pod.Name}
	c.m[name] = &PodPair{
		pod:    pod,
		status: status,
	}
}

func (c *PodCache) Get(name types.NamespacedName) (*corev1.Pod, *kubeletContainer.PodStatus) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	podPair, ok := c.m[name]
	if !ok {
		return nil, nil
	}

	return podPair.pod, podPair.status
}
