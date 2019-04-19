package node

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubelet/util"
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

	oldNode, err := n.kubeNodeClient.Get(n.name, opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", n.name, err)
	}

	// Patch the current status on the API server
	newNode := oldNode.DeepCopy()
	newNode.Status = n.nodeStatusCache.Get()

	updatedNode, err := n.kubeNodeClient.UpdateStatus(newNode)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}
