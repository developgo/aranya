/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

	newPod := pod.DeepCopy()
	c.uidMap[newPod.UID] = newPod
	c.nameMap[types.NamespacedName{Namespace: newPod.Namespace, Name: newPod.Name}] = newPod
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
