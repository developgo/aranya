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
	n.log.V(10).Info("update node status")
	for i := 0; i < nodeStatusUpdateRetry; i++ {
		if err := n.tryUpdateNodeStatus(i); err != nil {
			// if i > 0 && kl.onRepeatedHeartbeatFailure != nil {
			// 	kl.onRepeatedHeartbeatFailure()
			// }
			n.log.Error(err, "update node status failed, will retry")
		} else {
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

	node, err := n.kubeClient.CoreV1().Nodes().Get(n.name, opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", n.name, err)
	}

	originalNode := node.DeepCopy()
	if originalNode == nil {
		return fmt.Errorf("nil %q node object", n.name)
	}

	// Patch the current status on the API server
	updatedNode, _, err := utilnode.PatchNodeStatus(n.kubeClient.CoreV1(), types.NodeName(n.name), originalNode, node)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}
