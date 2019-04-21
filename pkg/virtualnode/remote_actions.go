package virtualnode

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

// generate in cluster node cache for remote device
func (vn *VirtualNode) generateCacheForNodeInDevice() error {
	msgCh, err := vn.opt.Manager.PostCmd(vn.ctx, connectivity.NewNodeGetInfoAllCmd())
	if err != nil {
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		_, ok := msg.GetMsg().(*connectivity.Msg_Node)
		if !ok {
			return fmt.Errorf("unexpected message type: %T", msg)
		}

		if err := vn.updateNodeCache(msg.GetNode()); err != nil {
			return err
		}
	}

	return nil
}

func (vn *VirtualNode) handleGlobalMsg(msg *connectivity.Msg) {
	switch m := msg.GetMsg().(type) {
	case *connectivity.Msg_Ack:
		switch m.Ack.GetValue().(type) {
		case *connectivity.Ack_Error:
			vn.log.Error(msg.Error(), "received error from remote device")
		}
	case *connectivity.Msg_Node:
		if err := vn.updateNodeCache(m.Node); err != nil {
			vn.log.Error(err, "failed to update node cache")
		}
	case *connectivity.Msg_Pod:
		err := vn.podManager.UpdateMirrorPod(m.Pod)
		if err != nil {
			vn.log.Error(err, "failed to update pod status in global msg handle")
			return
		}
	default:
		// we don't know how to handle this kind of messages, discard
	}
}

func (vn *VirtualNode) updateNodeCache(node *connectivity.Node) error {
	apiNode, err := vn.kubeNodeClient.Get(vn.name, metav1.GetOptions{})
	if err != nil {
		vn.log.Error(err, "failed to get node info")
		return err
	}

	nodeStatus := &apiNode.Status
	nodeStatus.Phase = corev1.NodeRunning

	if node.HasSystemInfo() {
		systemInfo, err := node.GetResolvedSystemInfo()
		if err != nil {
			return err
		}
		nodeStatus.NodeInfo = *systemInfo
	}

	if node.HasConditions() {
		conditions, err := node.GetResolvedConditions()
		if err != nil {
			return err
		}
		nodeStatus.Conditions = conditions
	}

	if node.HasCapacity() {
		capacity, err := node.GetResolvedCapacity()
		if err != nil {
			return err
		}
		nodeStatus.Capacity = capacity
	}

	if node.HasAllocatable() {
		allocatable, err := node.GetResolvedAllocatable()
		if err != nil {
			return err
		}
		nodeStatus.Allocatable = allocatable
	}

	vn.nodeStatusCache.Update(*nodeStatus)
	return nil
}
