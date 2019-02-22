package edgedevice

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	aranyav1alpha1 "arhat.dev/aranya/pkg/apis/aranya/v1alpha1"
	"arhat.dev/aranya/pkg/constant"
	"arhat.dev/aranya/pkg/node/util"
)

// create a node object in kubernetes, handle it in a dedicated arhat.dev/aranya/pkg/node.Node instance
func newNodeForEdgeDevice(device *aranyav1alpha1.EdgeDevice, hostIP string, kubeletPort int32) *corev1.Node {
	virtualNodeName := util.GetVirtualNodeName(device.Name)
	createdAt := metav1.Now()

	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{
			Name:      virtualNodeName,
			Namespace: corev1.NamespaceAll,
			Labels: map[string]string{
				constant.LabelType: constant.LabelTypeValueVirtualNode,
				// TODO: use corev1.LabelHostname in future when controller-runtime updated
				"kubernetes.io/hostname": virtualNodeName,
			},
			ClusterName: device.ClusterName,
		},
		Spec: corev1.NodeSpec{
			Taints: []corev1.Taint{{
				Key:    constant.TaintKeyDedicated,
				Value:  constant.TaintValueDedicatedForEdgeDevice,
				Effect: corev1.TaintEffectNoSchedule,
			}},
		},
		Status: corev1.NodeStatus{
			// fill address and port when actually create
			Addresses: []corev1.NodeAddress{{
				Type:    corev1.NodeInternalIP,
				Address: hostIP,
			}},
			DaemonEndpoints: corev1.NodeDaemonEndpoints{KubeletEndpoint: corev1.DaemonEndpoint{Port: kubeletPort}},

			Phase: corev1.NodePending,
			Conditions: []corev1.NodeCondition{
				{Type: corev1.NodeReady, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
				{Type: corev1.NodeOutOfDisk, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
				{Type: corev1.NodeMemoryPressure, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
				{Type: corev1.NodeDiskPressure, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
				{Type: corev1.NodePIDPressure, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
				{Type: corev1.NodeNetworkUnavailable, Status: corev1.ConditionUnknown, LastTransitionTime: createdAt},
			},
		},
	}
}
