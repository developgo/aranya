package node

import (
	"context"
	"sync"

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
	msgCh, err := n.connectivityManager.PostCmd(n.ctx, connectivity.NewNodeCmd())
	if err != nil {
		return err
	}

	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		switch nodeMsg := msg.GetMsg().(type) {
		case *connectivity.Msg_Node:
			if err := n.updateNodeCache(nodeMsg); err != nil {
				return err
			}
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
		if err := n.updateNodeCache(m); err != nil {
			n.log.Error(err, "failed to update node cache")
		}
	case *connectivity.Msg_Pod:
		err := n.podManager.UpdateMirrorPod(m.Pod)
		if err != nil {
			n.log.Error(err, "failed to update pod status in global msg handle")
			return
		}
	default:
		// we don't know how to handle this kind of message, discard
	}
}

func (n *Node) updateNodeCache(node *connectivity.Msg_Node) error {
	apiNode, err := node.Node.GetResolvedCoreV1Node()
	if err != nil {
		n.log.Error(err, "failed to resolve node")
		return err
	}

	me, err := n.kubeClient.CoreV1().Nodes().Get(n.name, metav1.GetOptions{})
	if err != nil {
		n.log.Error(err, "failed to get node info")
		return err
	}
	if me == nil {
		n.log.Error(nil, "empty node object")
		return err
	}

	n.nodeStatusCache.Update(apiNode.Status)
	return nil
}
