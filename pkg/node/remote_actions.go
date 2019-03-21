package node

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		wg := &sync.WaitGroup{}

		<-n.connectivityManager.WaitUntilDeviceConnected()

		wg.Add(1)
		go func() {
			defer wg.Wait()

			globalMsgCh := n.connectivityManager.ConsumeGlobalMsg()
			for msg := range globalMsgCh {
				n.handleGlobalMsg(msg)
			}
		}()

		if err := n.generateCacheForPods(); err != nil {
			n.log.Error(err, "generate pods cache failed")
			goto waitForDeviceDisconnect
		}

		if err := n.generateCacheForNode(); err != nil {
			n.log.Error(err, "generate node cache failed")
			goto waitForDeviceDisconnect
		}

	waitForDeviceDisconnect:
		wg.Wait()
	}
}

// generate in cluster pod cache for remote device
func (n *Node) generateCacheForPods() error {
	msgCh, err := n.connectivityManager.PostCmd(connectivity.NewPodListCmd("", ""), 0)
	if err != nil {
		return err
	}

	var podStatuses []*kubeletContainer.PodStatus
	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		podStatus, err := msg.GetPod().GetResolvedKubePodStatus()
		if err != nil {
			return err
		}
		podStatuses = append(podStatuses, podStatus)
	}

	for _, podStatus := range podStatuses {
		apiPod, err := n.podManager.GetMirrorPod(podStatus.Namespace, podStatus.Name)
		if err != nil {
			return err
		}

		n.podCache.Update(apiPod, podStatus)
	}

	return nil
}

// generate in cluster node cache for remote device
func (n *Node) generateCacheForNode() error {
	msgCh, err := n.connectivityManager.PostCmd(connectivity.NewNodeCmd(), 0)
	if err != nil {
		return err
	}

	var node *corev1.Node
	for msg := range msgCh {
		if err := msg.Error(); err != nil {
			return err
		}

		node, err = msg.GetResolvedCoreV1Node()
		if err != nil {
			return err
		}
	}
	// TODO: store node cache
	_ = node

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
		switch node := m.Node.GetNode().(type) {
		case *connectivity.Node_NodeV1:
			// update node info
			newNode := &corev1.Node{}
			if err := newNode.Unmarshal(node.NodeV1); err != nil {
				n.log.Error(err, "unmarshal device status failed")
				return
			}

			me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
			if err != nil {
				n.log.Error(err, "get self node info failed")
				return
			}

			resolveDeviceStatus(me.Status, newNode.Status)
			updatedMe, err := n.nodeClient.UpdateStatus(me)
			if err != nil {
				n.log.Error(err, "update node status failed")
				return
			}
			_ = updatedMe
		}
	case *connectivity.Msg_Pod:
		podStatus, err := m.Pod.GetResolvedKubePodStatus()
		if err != nil {
			n.log.Error(err, "resolve kube pod status failed")
			return
		}

		_ = podStatus
		apiPod, err := n.podManager.GetMirrorPod(podStatus.Namespace, podStatus.Name)
		if err != nil {
			n.log.Error(err, "get mirror pod failed")
			return
		}
		n.podCache.Update(apiPod, podStatus)
	case *connectivity.Msg_Data:
		// we don't know how to handle this kind of message, discard
	}
}

func resolveDeviceStatus(old, new corev1.NodeStatus) *corev1.NodeStatus {
	resolved := old.DeepCopy()
	newObj := new.DeepCopy()

	if newObj.Capacity != nil && len(newObj.Capacity) > 0 {
		resolved.Capacity = newObj.Capacity
	}
	if newObj.Allocatable != nil && len(newObj.Allocatable) > 0 {
		resolved.Allocatable = newObj.Allocatable
	}
	if newObj.Phase != "" {
		resolved.Phase = newObj.Phase
	}
	if newObj.Conditions != nil && len(newObj.Conditions) > 0 {
		resolved.Conditions = newObj.Conditions
	}
	if newObj.NodeInfo.MachineID != "" {
		resolved.NodeInfo.MachineID = newObj.NodeInfo.MachineID
	}
	if newObj.NodeInfo.SystemUUID != "" {
		resolved.NodeInfo.SystemUUID = newObj.NodeInfo.SystemUUID
	}
	if newObj.NodeInfo.BootID != "" {
		resolved.NodeInfo.BootID = newObj.NodeInfo.BootID
	}
	if newObj.NodeInfo.KernelVersion != "" {
		resolved.NodeInfo.KernelVersion = newObj.NodeInfo.KernelVersion
	}
	if newObj.NodeInfo.OSImage != "" {
		resolved.NodeInfo.OSImage = newObj.NodeInfo.OSImage
	}
	if newObj.NodeInfo.ContainerRuntimeVersion != "" {
		resolved.NodeInfo.ContainerRuntimeVersion = newObj.NodeInfo.ContainerRuntimeVersion
	}
	if newObj.NodeInfo.KubeletVersion != "" {
		resolved.NodeInfo.KubeletVersion = newObj.NodeInfo.KubeletVersion
	}
	if newObj.NodeInfo.KubeProxyVersion != "" {
		resolved.NodeInfo.KubeProxyVersion = newObj.NodeInfo.KubeProxyVersion
	}
	if newObj.NodeInfo.OperatingSystem != "" {
		resolved.NodeInfo.OperatingSystem = newObj.NodeInfo.OperatingSystem
	}
	if newObj.NodeInfo.Architecture != "" {
		resolved.NodeInfo.Architecture = newObj.NodeInfo.Architecture
	}
	if newObj.Images != nil && len(newObj.Images) > 0 {
		resolved.Images = newObj.Images
	}
	if newObj.VolumesInUse != nil && len(newObj.VolumesInUse) > 0 {
		resolved.VolumesInUse = newObj.VolumesInUse
	}
	if newObj.VolumesAttached != nil && len(newObj.VolumesAttached) > 0 {
		resolved.VolumesAttached = newObj.VolumesAttached
	}

	return resolved
}
