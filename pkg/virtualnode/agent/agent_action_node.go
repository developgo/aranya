package agent

import (
	"time"

	"github.com/denisbrodbeck/machineid"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"arhat.dev/aranya/pkg/virtualnode/connectivity"
)

func (b *baseAgent) doGetNodeInfoAll(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, b.getSystemInfo(), b.getResourceCapacity(), b.getResourceAllocatable(), b.getConditions())
	if err := b.doPostMsg(nodeMsg); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doGetNodeSystemInfo(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, b.getSystemInfo(), nil, nil, nil)
	if err := b.doPostMsg(nodeMsg); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doGetNodeResources(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, nil, b.getResourceCapacity(), b.getResourceAllocatable(), nil)
	if err := b.doPostMsg(nodeMsg); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) doGetNodeConditions(sid uint64) {
	nodeMsg := connectivity.NewNodeMsg(sid, nil, nil, nil, b.getConditions())
	if err := b.doPostMsg(nodeMsg); err != nil {
		b.handleError(sid, err)
		return
	}
}

func (b *baseAgent) getSystemInfo() *corev1.NodeSystemInfo {
	nodeSystemInfo := systemInfo()
	nodeSystemInfo.MachineID, _ = machineid.ID()
	nodeSystemInfo.OperatingSystem = b.runtime.OS()
	nodeSystemInfo.Architecture = b.runtime.Arch()
	nodeSystemInfo.KernelVersion = b.runtime.KernelVersion()
	nodeSystemInfo.ContainerRuntimeVersion = b.runtime.Name() + "://" + b.runtime.Version()
	// TODO: set KubeletVersion and KubeProxyVersion at server side
	// nodeSystemInfo.KubeletVersion
	// nodeSystemInfo.KubeProxyVersion
	return nodeSystemInfo
}

func (b *baseAgent) getResourceCapacity() corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(1, resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(512*(2<<20), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(20, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(1*(2<<30), resource.BinarySI),
	}
}

func (b *baseAgent) getResourceAllocatable() corev1.ResourceList {
	return corev1.ResourceList{
		corev1.ResourceCPU:              *resource.NewQuantity(1, resource.DecimalSI),
		corev1.ResourceMemory:           *resource.NewQuantity(512*(2<<20), resource.BinarySI),
		corev1.ResourcePods:             *resource.NewQuantity(20, resource.DecimalSI),
		corev1.ResourceEphemeralStorage: *resource.NewQuantity(1*(2<<30), resource.BinarySI),
	}
}

func (b *baseAgent) getConditions() []corev1.NodeCondition {
	now := metav1.NewTime(time.Now())

	return []corev1.NodeCondition{
		{Type: corev1.NodeReady, Status: corev1.ConditionTrue, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeOutOfDisk, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeDiskPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodePIDPressure, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
		{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionFalse, LastHeartbeatTime: now, LastTransitionTime: now},
	}
}
