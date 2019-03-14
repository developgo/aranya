package node

import (
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	criRuntime "k8s.io/kubernetes/pkg/kubelet/apis/cri/runtime/v1alpha2"
	kubeletContainer "k8s.io/kubernetes/pkg/kubelet/container"

	"arhat.dev/aranya/pkg/node/connectivity"
	"arhat.dev/aranya/pkg/node/connectivity/server"
)

func (n *Node) InitializeRemoteDevice() {
	for !n.closing() {
		wg := &sync.WaitGroup{}

		n.connectivityManager.WaitUntilDeviceConnected()

		wg.Add(1)
		go func() {
			defer wg.Wait()

			n.handleGlobalMsg(n.connectivityManager.ConsumeGlobalMsg())
		}()

		podListCmd := server.NewPodListCmd("", "")
		msgCh, err := n.connectivityManager.PostCmd(podListCmd, 0)
		if err != nil {
			// log error
		}
		var podsInDevice []*kubeletContainer.PodStatus
		for msg := range msgCh {
			msg.GetPod()

			podStatus := connectivity.TranslatePodToPodStatus(msg.GetPod())
			podsInDevice = append(podsInDevice, podStatus)
		}
		// TODO: update pods according podsInDevice

		wg.Wait()
	}
}

func (n *Node) handleGlobalMsg(msgCh <-chan *connectivity.Msg) {
	for msg := range msgCh {
		switch m := msg.GetMsg().(type) {
		case *connectivity.Msg_Node:
			switch node := m.Node.GetNode().(type) {
			case *connectivity.Node_NodeV1:
				// update node info
				newNode := &corev1.Node{}
				if err := newNode.Unmarshal(node.NodeV1); err != nil {
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
			}
		case *connectivity.Msg_Pod:
			_ = m.Pod.GetName()
			_ = m.Pod.GetNamespace()
			_ = m.Pod.GetIp()
			_ = m.Pod.GetUid()

			switch containerStatus := m.Pod.GetCriContainerStatus().(type) {
			case *connectivity.Pod_ContainerStatusV1Alpha2:
				allBytes := containerStatus.ContainerStatusV1Alpha2.GetV1Alpha2()

				containerStatuses := make([]*criRuntime.ContainerStatus, len(allBytes))
				for i, statusBytes := range allBytes {
					status := &criRuntime.ContainerStatus{}
					err := status.Unmarshal(statusBytes)
					if err != nil {
						continue
					}

					containerStatuses[i] = status
				}
			}

			switch podStatus := m.Pod.GetCriPodStatus().(type) {
			case *connectivity.Pod_PodStatusV1Alpha2:
				allBytes := podStatus.PodStatusV1Alpha2.GetV1Alpha2()

				podStatuses := make([]*criRuntime.PodSandboxStatus, len(allBytes))
				for i, statusBytes := range allBytes {
					status := &criRuntime.PodSandboxStatus{}
					err := status.Unmarshal(statusBytes)
					if err != nil {
						continue
					}

					podStatuses[i] = status
				}
			}
		case *connectivity.Msg_Data:
			// we don't know how to handle this kind of message, discard
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
