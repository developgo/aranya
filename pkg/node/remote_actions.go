package node

import (
	"context"
	"fmt"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"

	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/connectivity"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		connCtx, cancel := context.WithCancel(n.ctx)
		wg := &sync.WaitGroup{}

		<-n.connectivityManager.DeviceConnected()
		n.log.Info("device connected")

		wg.Add(1)
		go func() {
			defer wg.Done()

			for msg := range n.connectivityManager.GlobalMessages() {
				n.handleGlobalMsg(msg)
			}
		}()

		n.log.Info("sync device pods")
		if err := n.podManager.SyncDevicePods(); err != nil {
			n.log.Error(err, "failed to sync device pods")
			goto waitForDeviceDisconnect
		}

		n.log.Info("sync device info")
		if err := n.generateCacheForNodeInDevice(); err != nil {
			n.log.Error(err, "failed to sync device node info")
			goto waitForDeviceDisconnect
		}

		// sync node status after device has been connected
		go wait.Until(n.syncNodeStatus, constant.DefaultNodeStatusSyncInterval, connCtx.Done())
	waitForDeviceDisconnect:
		wg.Wait()
		cancel()
	}
}

// generate in cluster node cache for remote device
func (n *Node) generateCacheForNodeInDevice() error {
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, connectivity.NewNodeGetSystemInfoCmd())
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

		if err := n.updateNodeCache(msg.GetNode()); err != nil {
			return err
		}
	}

	return nil
}

func (n *Node) handleGlobalMsg(msg *connectivity.Msg) {
	switch m := msg.GetMsg().(type) {
	case *connectivity.Msg_Ack:
		switch m.Ack.GetValue().(type) {
		case *connectivity.Ack_Error:
			n.log.Error(msg.Error(), "received error from remote device")
		}
	case *connectivity.Msg_Node:
		if err := n.updateNodeCache(m.Node); err != nil {
			n.log.Error(err, "failed to update node cache")
		}
	case *connectivity.Msg_Pod:
		err := n.podManager.UpdateMirrorPod(m.Pod)
		if err != nil {
			n.log.Error(err, "failed to update pod status in global msg handle")
			return
		}
	default:
		// we don't know how to handle this kind of messages, discard
	}
}

func (n *Node) updateNodeCache(node *connectivity.Node) error {
	apiNode, err := n.kubeNodeClient.Get(n.name, metav1.GetOptions{})
	if err != nil {
		n.log.Error(err, "failed to get node info")
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

	n.nodeStatusCache.Update(*nodeStatus)
	return nil
}
