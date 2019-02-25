package node

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/kubernetes/pkg/kubelet/util"
	nodeutil "k8s.io/kubernetes/pkg/util/node"
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
	// In large clusters, GET and PUT operations on Node objects coming
	// from here are the majority of load on apiserver and etcd.
	// To reduce the load on etcd, we are serving GET operations from
	// apiserver cache (the data might be slightly delayed but it doesn't
	// seem to cause more conflict - the delays are pretty small).
	// If it result in a conflict, all retries are served directly from etcd.
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
	updatedNode, _, err := nodeutil.PatchNodeStatus(n.kubeClient.CoreV1(), types.NodeName(n.name), originalNode, node)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}
