package virtualnode

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

func (vn *VirtualNode) syncNodeStatus() {
	vn.log.Info("update node status")
	for i := 0; i < nodeStatusUpdateRetry; i++ {
		if err := vn.tryUpdateNodeStatus(i); err != nil {
			vn.log.Error(err, "failed to update node status, retry")
		} else {
			vn.log.Info("update node status success")
			return
		}
	}
	vn.log.Info("update node status exceeds retry count")
}

func (vn *VirtualNode) tryUpdateNodeStatus(tryNumber int) error {
	opts := metav1.GetOptions{}
	if tryNumber == 0 {
		util.FromApiserverCache(&opts)
	}

	oldNode, err := vn.kubeNodeClient.Get(vn.name, opts)
	if err != nil {
		return fmt.Errorf("error getting node %q: %v", vn.name, err)
	}

	// Patch the current status on the API server
	newNode := oldNode.DeepCopy()
	newNode.Status = vn.nodeStatusCache.Get()

	updatedNode, err := vn.kubeNodeClient.UpdateStatus(newNode)
	if err != nil {
		return err
	}

	_ = updatedNode

	return nil
}