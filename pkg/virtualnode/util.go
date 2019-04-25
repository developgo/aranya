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

package virtualnode

import (
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newNodeCache(node corev1.NodeStatus) *NodeCache {
	return &NodeCache{nodeStatus: &node}
}

type NodeCache struct {
	nodeStatus *corev1.NodeStatus
	mu         sync.RWMutex
}

func (c *NodeCache) Update(nodeStatus corev1.NodeStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.nodeStatus != nil {
		// preserve node addresses and kubelet listen port
		nodeStatus.DaemonEndpoints = c.nodeStatus.DaemonEndpoints
		nodeStatus.Addresses = c.nodeStatus.Addresses
	}

	c.nodeStatus = &nodeStatus
}

func (c *NodeCache) Get() corev1.NodeStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if c.nodeStatus == nil {
		return corev1.NodeStatus{}
	}

	nodeStatus := c.nodeStatus.DeepCopy()
	now := metav1.NewTime(time.Now())
	for i := range nodeStatus.Conditions {
		nodeStatus.Conditions[i].LastHeartbeatTime = now
	}

	return *nodeStatus
}
