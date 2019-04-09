package node

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubelet/util"
	utilnode "k8s.io/kubernetes/pkg/util/node"
)

const (
	statusReady   = 0
	statusRunning = 1
	statusStopped = 2
)

const (
	nodeStatusUpdateRetry = 5
)

func (n *Node) syncNodeStatus() {
	n.log.Info("update node status")
	for i := 0; i < nodeStatusUpdateRetry; i++ {
		if err := n.tryUpdateNodeStatus(i); err != nil {
			n.log.Error(err, "failed to update node status, retry")
		} else {
			n.log.Info("update node status success")
			return
		}
	}
	n.log.Info("update node status exceeds retry count")
}

func (n *Node) tryUpdateNodeStatus(tryNumber int) error {
	opts := metav1.GetOptions{}
	if tryNumber == 0 {
		util.FromApiserverCache(&opts)
	}

	oldNode, err := n.kubeClient.CoreV1().Nodes().Get(n.name, opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", n.name, err)
	}

	// Patch the current status on the API server
	newNode := oldNode.DeepCopy()
	cachedNode := n.nodeCache.Get()
	newNode.Status = cachedNode.Status

	updatedNode, _, err := utilnode.PatchNodeStatus(n.kubeClient.CoreV1(), types.NodeName(n.name), oldNode, newNode)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}
