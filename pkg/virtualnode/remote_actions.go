package virtualnode

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

// generate in cluster node cache for remote device
func (vn *VirtualNode) generateCacheForNodeInDevice() error {
	msgCh, err := vn.opt.Manager.PostCmd(vn.ctx, connectivity.NewNodeCmd(connectivity.GetInfoAll))
	if err != nil {
		return err
	}

	for msg := range msgCh {
		if err := msg.Err(); err != nil {
			return err
		}

		deviceNodeStatus := msg.GetNodeStatus()
		if deviceNodeStatus == nil {
			vn.log.Info("unexpected non node status message", "msg", msg)
			continue
		}

		if err := vn.updateNodeCache(deviceNodeStatus); err != nil {
			vn.log.Error(err, "failed to update node status")
			continue
		}
	}

	return nil
}

func (vn *VirtualNode) handleGlobalMsg(msg *connectivity.Msg) {
	if msg.Err() != nil {
		vn.log.Error(msg.Err(), "received error from remote device")
	}
	switch m := msg.GetMsg().(type) {
	case *connectivity.Msg_NodeStatus:
		vn.log.Info("received async node status update")
		if err := vn.updateNodeCache(m.NodeStatus); err != nil {
			vn.log.Error(err, "failed to update node cache")
		}
	case *connectivity.Msg_PodStatus:
		// TODO: handle async pod status update
		vn.log.Info("received async pod status update")
		err := vn.podManager.UpdateMirrorPod(nil, m.PodStatus)
		if err != nil {
			vn.log.Error(err, "failed to update pod status for async pod status update")
		}
	default:
		// we don't know how to handle this kind of messages, discard
	}
}

func (vn *VirtualNode) updateNodeCache(node *connectivity.NodeStatus) error {
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
