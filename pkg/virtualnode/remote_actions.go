/*
Copyright 2019 The arhat.dev Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package virtualnode

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/connectivity"
)

// generate in cluster node cache for remote device
func (vn *VirtualNode) syncDeviceNodeStatus() error {
	msgCh, err := vn.opt.ConnectivityManager.PostCmd(vn.ctx, connectivity.NewNodeCmd(connectivity.GetInfoAll))
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

	if sysInfo := node.GetSystemInfo(); sysInfo != nil {
		nodeStatus.NodeInfo = corev1.NodeSystemInfo{
			OperatingSystem:         sysInfo.GetOs(),
			Architecture:            sysInfo.GetArch(),
			OSImage:                 sysInfo.GetOsImage(),
			KernelVersion:           sysInfo.GetKernelVersion(),
			MachineID:               sysInfo.GetMachineId(),
			SystemUUID:              sysInfo.GetSystemUuid(),
			BootID:                  sysInfo.GetBootId(),
			ContainerRuntimeVersion: fmt.Sprintf("%s://%s", sysInfo.GetRuntimeInfo().GetName(), sysInfo.GetRuntimeInfo().GetVersion()),
			// TODO: how we should report kubelet and kube-proxy version?
			//       be the same with host node?
			KubeletVersion:   "",
			KubeProxyVersion: "",
		}
	}

	if conditions := node.GetConditions(); conditions != nil {
		nodeStatus.Conditions = translateDeviceCondition(conditions)
	}

	if capacity := node.GetCapacity(); capacity != nil {
		nodeStatus.Capacity = translateDeviceResources(capacity)
	}

	if allocatable := node.GetAllocatable(); allocatable != nil {
		nodeStatus.Allocatable = translateDeviceResources(allocatable)
	}

	vn.nodeStatusCache.Update(*nodeStatus)
	return nil
}

func translateDeviceResources(res *connectivity.NodeResources) corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(int64(res.GetCpuCount()), resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(int64(res.GetMemoryBytes()), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(int64(res.GetPodCount()), resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(int64(res.GetStorageBytes()), resource.BinarySI),
	}
}

func translateDeviceCondition(cond *connectivity.NodeConditions) []corev1.NodeCondition {
	translate := func(condition connectivity.NodeConditions_Condition) corev1.ConditionStatus {
		switch condition {
		case connectivity.Healthy:
			return corev1.ConditionFalse
		case connectivity.Unhealthy:
			return corev1.ConditionTrue
		default:
			return corev1.ConditionUnknown
		}
	}

	result := []corev1.NodeCondition{
		{
			Type: corev1.NodeReady,
			Status: func() corev1.ConditionStatus {
				switch cond.GetReady() {
				case connectivity.Healthy:
					return corev1.ConditionTrue
				case connectivity.Unhealthy:
					return corev1.ConditionFalse
				default:
					return corev1.ConditionUnknown
				}
			}(),
		},
		{Type: corev1.NodeOutOfDisk, Status: translate(cond.GetPod())},
		{Type: corev1.NodeMemoryPressure, Status: translate(cond.GetMemory())},
		{Type: corev1.NodeDiskPressure, Status: translate(cond.GetDisk())},
		{Type: corev1.NodePIDPressure, Status: translate(cond.GetPid())},
		{Type: corev1.NodeNetworkUnavailable, Status: translate(cond.GetNetwork())},
	}

	now := metav1.Now()
	for i := range result {
		result[i].LastHeartbeatTime = now
		result[i].LastTransitionTime = now
	}

	return result
}
