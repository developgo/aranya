package node

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/node/connectivity"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		n.connectivityManager.WaitUntilDeviceConnected()
		msgCh := n.connectivityManager.ConsumeGlobalMsg()
		for msg := range msgCh {
			switch msg.GetMsg().(type) {
			case *connectivity.Msg_NodeInfo:
				nodeInfo := msg.GetNodeInfo()
				// update node info
				newNode := &corev1.Node{}
				if err := newNode.Unmarshal(nodeInfo.GetNodeV1()); err != nil {
					n.log.Error(err, "unmarshal device status failed")
					continue
				}

				me, err := n.nodeClient.Get(n.name, metav1.GetOptions{})
				if err != nil {
					n.log.Error(err, "get self node info failed")
					continue
				}

				resolveDeviceStatus(me.Status, newNode.Status)
				updatedMe, err := n.nodeClient.UpdateStatus(me)
				if err != nil {
					n.log.Error(err, "update node status failed")
					return
				}
				_ = updatedMe
			case *connectivity.Msg_PodInfo:
				podInfo := msg.GetPodInfo()
				pod := &corev1.Pod{}
				if err := pod.Unmarshal(podInfo.GetPodV1()); err != nil {
					n.log.Error(err, "unmarshal pod status failed")
				}
			case *connectivity.Msg_PodData:
				// we don't know how to handle this kind of message, discard
			}
		}
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
