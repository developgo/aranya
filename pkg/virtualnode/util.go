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
